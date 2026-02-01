package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"api-gateway/config"
	"api-gateway/internal/client"
	"api-gateway/internal/handlers"
	"api-gateway/internal/middleware"

	_ "api-gateway/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Kollocol API
// @version 1.0
// @termsOfService http://swagger.io/terms/

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	cfg := config.Load()

	authClient, err := client.NewAuthClient(cfg.Auth.Host, cfg.Auth.Port)
	if err != nil {
		log.Fatalf("Failed to connect to Auth Service: %v", err)
	}
	defer authClient.Close()

	userClient, err := client.NewUserClient(cfg.User.Host, cfg.User.Port)
	if err != nil {
		log.Fatalf("Failed to connect to User Service: %v", err)
	}
	defer userClient.Close()

	quizClient, err := client.NewQuizClient(cfg.Quiz.Host, cfg.Quiz.Port)
	if err != nil {
		log.Fatalf("Failed to connect to Quiz Service: %v", err)
	}
	defer quizClient.Close()

	notificationClient, err := client.NewNotificationClient(cfg.Notification.Host, cfg.Notification.Port)
	if err != nil {
		log.Fatalf("Failed to connect to Notification Service: %v", err)
	}
	defer notificationClient.Close()

	authHandler := handlers.NewAuthHandler(authClient)
	userHandler := handlers.NewUserHandler(userClient)
	quizHandler := handlers.NewQuizHandler(quizClient)
	notificationHandler := handlers.NewNotificationHandler(notificationClient)
	gameHandler := handlers.NewGameHandler(cfg.Game.Host, cfg.Game.Port)

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.ErrorHandler())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "api-gateway",
		})
	})

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	authGroup := router.Group("/auth")
	{
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/verify", authHandler.VerifyCode)
		authGroup.POST("/refresh", authHandler.RefreshToken)
		authGroup.POST("/resend-code", authHandler.ResendCode)
	}

	authProtected := router.Group("/auth")
	authProtected.Use(middleware.JWTAuth(authClient))
	{
		authProtected.POST("/logout", authHandler.Logout)
	}

	usersGroup := router.Group("/users")
	usersGroup.Use(middleware.JWTAuth(authClient))
	{
		usersGroup.POST("/register", userHandler.Register)
		usersGroup.GET("/me", userHandler.GetProfile)
		usersGroup.PUT("/me", userHandler.UpdateProfile)
		usersGroup.POST("/me/avatar", userHandler.UploadAvatar)
		usersGroup.GET("/me/notifications", userHandler.GetNotificationSettings)
		usersGroup.PUT("/me/notifications", userHandler.UpdateNotificationSettings)
	}

	groupsGroup := router.Group("/groups")
	groupsGroup.Use(middleware.JWTAuth(authClient))
	{
		groupsGroup.POST("", userHandler.CreateGroup)
		groupsGroup.GET("", userHandler.GetGroups)
		groupsGroup.GET("/:id", userHandler.GetGroup)
		groupsGroup.PUT("/:id", userHandler.UpdateGroup)
		groupsGroup.DELETE("/:id", userHandler.DeleteGroup)
	}

	quizzesGroup := router.Group("/quizzes")
	quizzesGroup.Use(middleware.JWTAuth(authClient))
	{
		quizzesGroup.POST("/templates", quizHandler.CreateTemplate)
		quizzesGroup.GET("/templates", quizHandler.GetTemplates)
		quizzesGroup.GET("/templates/:id", quizHandler.GetTemplate)
		quizzesGroup.PUT("/templates/:id", quizHandler.UpdateTemplate)
		quizzesGroup.DELETE("/templates/:id", quizHandler.DeleteTemplate)

		quizzesGroup.POST("/instances", quizHandler.CreateInstance)
		quizzesGroup.GET("/instances/hosting", quizHandler.GetHostingInstances)
		quizzesGroup.GET("/instances/:id", quizHandler.GetInstance)
	}

	notificationsGroup := router.Group("/notifications")
	notificationsGroup.Use(middleware.JWTAuth(authClient))
	{
		notificationsGroup.GET("", notificationHandler.GetNotifications)
		notificationsGroup.PUT("/:id/read", notificationHandler.MarkAsRead)
		notificationsGroup.DELETE("/:id", notificationHandler.DeleteNotification)
	}

	router.GET("/ws", middleware.JWTAuthWS(authClient), gameHandler.ProxyWebSocket)

	addr := cfg.GetServerAddress()
	log.Printf("API Gateway starting on %s", addr)
	log.Printf("Swagger doc available at http://%s/swagger/index.html", addr)

	go func() {
		if err := router.Run(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("API Gateway stopped")
}
