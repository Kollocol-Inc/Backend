package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"game-service/config"
	"game-service/internal/client"
	ws "game-service/internal/websocket"
	"game-service/pkg/safego"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func (h *WebSocketHandler) TerminateInstance(c *gin.Context) {
	instanceID := c.Param("id")
	if instanceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing instance_id"})
		return
	}

	h.hub.TerminateInstance(instanceID)
	c.Status(http.StatusNoContent)
}

func writeQuizAccessError(c *gin.Context, err error, notFoundMsg, deniedMsg, defaultMsg string) {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.NotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": notFoundMsg})
			return
		case codes.PermissionDenied:
			c.JSON(http.StatusForbidden, gin.H{"error": deniedMsg})
			return
		}
	}
	log.Printf("%s: %v", defaultMsg, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": defaultMsg})
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

	if instanceID == "" && accessCode != "" {
		resp, err := h.quizClient.GetInstanceByAccessCode(ctx, accessCode, userID)
		if err != nil {
			writeQuizAccessError(c, err, "Quiz not found", "Access denied to this quiz", "Failed to resolve access code")
			return
		}

		instanceID = resp.Instance.Id
		isCreator = resp.Instance.CreatedBy == userID
	} else {
		resp, err := h.quizClient.GetInstance(ctx, instanceID, userID)
		if err != nil {
			writeQuizAccessError(c, err, "Quiz instance not found", "Access denied to this quiz", "Failed to get quiz instance")
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

	safego.Go("client.WritePump", client.WritePump)
	safego.Go("client.ReadPump", client.ReadPump)
}