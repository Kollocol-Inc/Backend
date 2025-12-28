package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auth-service/config"
	"auth-service/internal/service"
	"auth-service/pkg/cache"
	"auth-service/pkg/database"
	"auth-service/pkg/messaging"
	pb "auth-service/proto"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.Load()
	log.Println("Configuration loaded")

	pgClient, err := database.NewPostgresClient(&cfg.DB)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	log.Println("Connected to PostgreSQL")
	defer pgClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := pgClient.InitSchema(ctx); err != nil {
		log.Printf("Warning: Failed to initialize PostgreSQL schema: %v", err)
	} else {
		log.Println("PostgreSQL schema initialized")
	}
	cancel()

	redisClient, err := cache.NewRedisClient(&cfg.Redis)
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v", err)
		redisClient = nil
	} else {
		log.Println("Connected to Redis")
		defer redisClient.Close()
	}

	rabbitClient, err := messaging.NewRabbitMQClient(&cfg.RabbitMQ)
	if err != nil {
		log.Printf("Warning: Failed to connect to RabbitMQ: %v", err)
		rabbitClient = nil
	} else {
		log.Println("Connected to RabbitMQ")
		defer rabbitClient.Close()
	}

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "auth-service",
		})
	})

	router.GET("/ready", func(c *gin.Context) {
		if pgClient == nil || redisClient == nil || rabbitClient == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	})

	httpAddr := ":" + cfg.Server.HTTPPort
	log.Printf("Auth Service HTTP server starting on port %s...", cfg.Server.HTTPPort)
	go func() {
		if err := router.Run(httpAddr); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	grpcServer := grpc.NewServer()

	authService := service.NewAuthService(redisClient, pgClient.GetDB(), rabbitClient, cfg.JWT.Secret)
	pb.RegisterAuthServiceServer(grpcServer, authService)

	reflection.Register(grpcServer)

	log.Printf("Auth Service gRPC server starting on port %s...", cfg.Server.GRPCPort)

	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.Server.GRPCPort)
		if err != nil {
			log.Fatalf("Failed to listen on gRPC port: %v", err)
		}

		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	grpcServer.GracefulStop()

	log.Println("Auth Service stopped")
}