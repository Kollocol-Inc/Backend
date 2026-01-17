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

	"notification-service/config"
	"notification-service/internal/service"
	"notification-service/pkg/database"
	"notification-service/pkg/email"
	"notification-service/pkg/messaging"
	pb "notification-service/proto"

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

	rabbitClient, err := messaging.NewRabbitMQClient(&cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	log.Println("Connected to RabbitMQ")
	defer rabbitClient.Close()

	smtpClient := email.NewSMTPClient(&cfg.SMTP)
	log.Println("SMTP client initialized")

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "notification-service",
		})
	})

	router.GET("/ready", func(c *gin.Context) {
		if pgClient == nil || rabbitClient == nil {
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
	log.Printf("Notification Service HTTP server starting on port %s...", cfg.Server.HTTPPort)
	go func() {
		if err := router.Run(httpAddr); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	notificationService := service.NewNotificationService(pgClient.GetDB(), smtpClient)

	grpcServer := grpc.NewServer()
	pb.RegisterNotificationServiceServer(grpcServer, notificationService)
	reflection.Register(grpcServer)
	log.Printf("Notification Service gRPC server starting on port %s...", cfg.Server.GRPCPort)

	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.Server.GRPCPort)
		if err != nil {
			log.Fatalf("Failed to listen on gRPC port: %v", err)
		}

		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	// Start RabbitMQ consumers
	log.Println("Starting RabbitMQ consumers...")
	startConsumers(rabbitClient, notificationService)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	grpcServer.GracefulStop()

	log.Println("Notification Service stopped")
}

func startConsumers(rabbitClient *messaging.RabbitMQClient, notificationService *service.NotificationService) {
	ctx := context.Background()

	go consumeQueue(ctx, rabbitClient, "auth.send_code", notificationService.HandleSendAuthCode)
	go consumeQueue(ctx, rabbitClient, "user.group_invites", notificationService.HandleGroupInvite)
	go consumeQueue(ctx, rabbitClient, "quiz.created", notificationService.HandleQuizCreated)
	go consumeQueue(ctx, rabbitClient, "quiz.results_ready", notificationService.HandleQuizResultsReady)
	go consumeQueue(ctx, rabbitClient, "notifications.email", notificationService.HandleSendEmail)
	go consumeQueue(ctx, rabbitClient, "notifications.create", notificationService.HandleCreateNotification)

	log.Println("All RabbitMQ consumers started")
}

func consumeQueue(ctx context.Context, rabbitClient *messaging.RabbitMQClient, queueName string, handler func(context.Context, []byte) error) {
	msgs, err := rabbitClient.Consume(queueName)
	if err != nil {
		log.Printf("Failed to start consumer for queue %s: %v", queueName, err)
		return
	}

	log.Printf("Started consumer for queue: %s", queueName)

	for msg := range msgs {
		if err := handler(ctx, msg.Body); err != nil {
			log.Printf("Error handling message from %s: %v", queueName, err)
			msg.Nack(false, true)
		} else {
			msg.Ack(false)
		}
	}
}
