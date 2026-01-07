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

	"user-service/config"
	"user-service/internal/service"
	"user-service/pkg/cache"
	"user-service/pkg/database"
	"user-service/pkg/messaging"
	"user-service/pkg/storage"
	pb "user-service/proto"

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

	s3Client, err := storage.NewS3Client(&cfg.S3)
	if err != nil {
		log.Fatalf("Warning: Failed to connect to S3: %v", err)
	} else {
		log.Println("Connected to S3")
	}

	log.Println("Creating S3 buckets...")
	s3Ctx, s3Cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := s3Client.CreateBucket(s3Ctx, "user-avatars"); err != nil {
		log.Printf("Warning: Failed to create user-avatars bucket: %v", err)
	}
	if err := s3Client.CreateBucket(s3Ctx, "user-files"); err != nil {
		log.Printf("Warning: Failed to create user-files bucket: %v", err)
	}
	s3Cancel()
	log.Println("S3 buckets ready")

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "user-service",
		})
	})

	router.GET("/ready", func(c *gin.Context) {
		if pgClient == nil || redisClient == nil || rabbitClient == nil || s3Client == nil {
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
	log.Printf("User Service HTTP server starting on port %s...", cfg.Server.HTTPPort)
	go func() {
		if err := router.Run(httpAddr); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	userService := service.NewUserService(pgClient.GetDB(), s3Client, rabbitClient)

	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, userService)
	reflection.Register(grpcServer)
	log.Printf("User Service gRPC server starting on port %s...", cfg.Server.GRPCPort)

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

	log.Println("User service stopped")
}
