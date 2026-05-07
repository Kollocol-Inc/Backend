package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"api-gateway/internal/client"
	"api-gateway/internal/dto"
	"api-gateway/pkg/errors"
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
		dto.JsonError(c, errors.ErrUserIDNotFound)
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.notificationClient.GetNotifications(ctx, userID.(string), int32(limit), int32(offset))
	if err != nil {
		dto.JsonError(c, err)
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
// @Summary Mark notifications as read
// @Description Mark multiple notifications as read
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.NotificationIDsRequest true "Notification IDs"
// @Success 204
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /notifications/read [put]
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, errors.ErrUserIDNotFound)
		return
	}

	var req dto.NotificationIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrNotificationIDsRequired)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := h.notificationClient.MarkAsRead(ctx, req.IDs, userID.(string))
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// DeleteNotification godoc
// @Summary Delete notifications
// @Description Delete multiple notifications
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.NotificationIDsRequest true "Notification IDs"
// @Success 204
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /notifications/delete [delete]
func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, errors.ErrUserIDNotFound)
		return
	}

	var req dto.NotificationIDsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrNotificationIDsRequired)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.notificationClient.DeleteNotification(ctx, req.IDs, userID.(string))
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func convertNotificationToDTO(n *pb.Notification) dto.NotificationDTO {
	return dto.NotificationDTO{
		ID:             n.Id,
		UserID:         n.UserId,
		Type:           n.Type,
		Title:          n.Title,
		Content:        n.Content,
		IsRead:         n.IsRead,
		CreatedAt:      n.CreatedAt,
		RequiresAction: n.RequiresAction,
	}
}
