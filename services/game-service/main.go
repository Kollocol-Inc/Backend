package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"game-service/config"
	"game-service/internal/client"
	"game-service/internal/handlers"
	"game-service/internal/repository"
	ws "game-service/internal/websocket"
	"game-service/pkg/cache"
	"game-service/pkg/database"

	"github.com/gin-gonic/gin"
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

	quizClient, err := client.NewQuizClient(cfg.Quiz.Host, cfg.Quiz.Port)
	if err != nil {
		log.Fatalf("Failed to connect to Quiz Service: %v", err)
	}
	log.Println("Connected to Quiz Service")
	defer quizClient.Close()

	sessionRepo := repository.NewSessionRepository(pgClient.GetDB())

	hub := ws.NewHub(quizClient, redisClient, sessionRepo, pgClient.GetDB())
	go hub.Run()
	log.Println("WebSocket hub started")

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "game-service",
		})
	})

	router.GET("/ready", func(c *gin.Context) {
		if pgClient == nil || redisClient == nil || quizClient == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	})

	wsHandler := handlers.NewWebSocketHandler(hub, cfg, quizClient)
	router.GET("/ws", wsHandler.HandleWebSocket)

	httpAddr := ":" + cfg.Server.HTTPPort
	log.Printf("Game Service HTTP server starting on port %s...", cfg.Server.HTTPPort)

	go func() {
		if err := router.Run(httpAddr); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Game service stopped")
}
