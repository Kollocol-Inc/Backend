package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"api-gateway/internal/client"
	"api-gateway/internal/dto"
	pb "api-gateway/proto"

	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	notificationClient *client.NotificationClient
}

func NewNotificationHandler(notificationClient *client.NotificationClient) *NotificationHandler {
	return &NotificationHandler{
		notificationClient: notificationClient,
	}
}

// GetNotifications godoc
// @Summary Get user notifications
// @Description Get list of notifications for current user
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Limit" default(30)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} dto.GetNotificationsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /notifications [get]
func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.notificationClient.GetNotifications(ctx, userID.(string), int32(limit), int32(offset))
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to get notifications")
		return
	}

	notifications := make([]dto.NotificationDTO, len(resp.Notifications))
	for i, n := range resp.Notifications {
		notifications[i] = convertNotificationToDTO(n)
	}

	c.JSON(http.StatusOK, dto.GetNotificationsResponse{
		Notifications: notifications,
		Total:         resp.Total,
	})
}

// MarkAsRead godoc
// @Summary Mark notification as read
// @Description Mark a specific notification as read
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /notifications/{id}/read [put]
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	notificationID := c.Param("id")
	if notificationID == "" {
		dto.JsonError(c, http.StatusBadRequest, "Notification ID is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.notificationClient.MarkAsRead(ctx, notificationID, userID.(string))
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to mark notification as read")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusBadRequest, resp.Message)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": resp.Message,
	})
}

// DeleteNotification godoc
// @Summary Delete notification
// @Description Delete a specific notification
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 204
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /notifications/{id} [delete]
func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	notificationID := c.Param("id")
	if notificationID == "" {
		dto.JsonError(c, http.StatusBadRequest, "Notification ID is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.notificationClient.DeleteNotification(ctx, notificationID, userID.(string))
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to delete notification")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusBadRequest, resp.Message)
		return
	}

	c.Status(http.StatusNoContent)
}

func convertNotificationToDTO(n *pb.Notification) dto.NotificationDTO {
	return dto.NotificationDTO{
		ID:        n.Id,
		UserID:    n.UserId,
		Type:      n.Type,
		Title:     n.Title,
		Content:   n.Content,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
	}
}