package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"game-service/config"
	"game-service/internal/client"
	ws "game-service/internal/websocket"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: Change allow all origins in prod
	},
}

type WebSocketHandler struct {
	hub        *ws.Hub
	config     *config.Config
	quizClient *client.QuizClient
}

func NewWebSocketHandler(hub *ws.Hub, cfg *config.Config, quizClient *client.QuizClient) *WebSocketHandler {
	return &WebSocketHandler{
		hub:        hub,
		config:     cfg,
		quizClient: quizClient,
	}
}

func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	instanceID := c.Query("instance_id")
	accessCode := c.Query("access_code")

	if instanceID == "" && accessCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing instance_id or access_code"})
		return
	}

	var isCreator bool
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// If access_code provided, resolve to instance_id
	if instanceID == "" && accessCode != "" {
		resp, err := h.quizClient.GetInstanceByAccessCode(ctx, accessCode, userID)
		if err != nil {
			log.Printf("Failed to resolve access code: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve access code"})
			return
		}

		if resp.Instance == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": resp.ErrorMessage})
			return
		}

		if !resp.HasAccess {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this quiz"})
			return
		}

		instanceID = resp.Instance.Id
		isCreator = resp.Instance.CreatedBy == userID
	} else {
		// If instance_id provided, get instance to verify creator
		resp, err := h.quizClient.GetInstance(ctx, instanceID, userID)
		if err != nil {
			log.Printf("Failed to get instance: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get quiz instance"})
			return
		}

		if resp.Instance == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Quiz instance not found"})
			return
		}

		isCreator = resp.Instance.CreatedBy == userID
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	client := ws.NewClient(h.hub, conn, userID, instanceID, isCreator)

	h.hub.Register <- client

	go client.WritePump()
	go client.ReadPump()
}