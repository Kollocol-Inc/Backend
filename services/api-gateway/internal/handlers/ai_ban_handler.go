package handlers

import (
	"net/http"

	"api-gateway/internal/client"
	"api-gateway/internal/dto"
	"api-gateway/pkg/errors"
	pb "api-gateway/proto"

	"github.com/gin-gonic/gin"
)

type AIBanHandler struct {
	authClient *client.AuthClient
}

func NewAIBanHandler(authClient *client.AuthClient) *AIBanHandler {
	return &AIBanHandler{authClient: authClient}
}

// CreateAIBan godoc
// @Summary Ban a user from AI features
// @Tags AI Bans
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateAIBanRequest true "Ban data"
// @Success 200 {object} dto.CreateAIBanResponse
// @Router /admin/ai/bans [post]
func (h *AIBanHandler) CreateAIBan(c *gin.Context) {
	bannedBy := c.GetString("user_id")

	var req dto.CreateAIBanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrInvalidRequestBody)
		return
	}

	resp, err := h.authClient.CreateAIBan(c.Request.Context(), &pb.CreateAIBanRequest{
		UserId:   req.UserID,
		Reason:   req.Reason,
		BannedBy: bannedBy,
	})
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.CreateAIBanResponse{
		Ban: aiBanProtoToDTO(resp.Ban),
	})
}

// DeleteAIBan godoc
// @Summary Remove AI ban for a user
// @Tags AI Bans
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Success 204
// @Router /admin/ai/bans/{userId} [delete]
func (h *AIBanHandler) DeleteAIBan(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		dto.JsonError(c, errors.ErrUserIDBanRequired)
		return
	}

	_, err := h.authClient.DeleteAIBan(c.Request.Context(), userID)
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetAIBan godoc
// @Summary Get AI ban for a user
// @Tags AI Bans
// @Produce json
// @Security BearerAuth
// @Param userId path string true "User ID"
// @Success 200 {object} dto.GetAIBanResponse
// @Router /admin/ai/bans/{userId} [get]
func (h *AIBanHandler) GetAIBan(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		dto.JsonError(c, errors.ErrUserIDBanRequired)
		return
	}

	resp, err := h.authClient.GetAIBan(c.Request.Context(), userID)
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.GetAIBanResponse{
		Ban: aiBanProtoToDTO(resp.Ban),
	})
}

// ListAIBans godoc
// @Summary List all AI bans
// @Tags AI Bans
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.ListAIBansResponse
// @Router /admin/ai/bans [get]
func (h *AIBanHandler) ListAIBans(c *gin.Context) {
	resp, err := h.authClient.ListAIBans(c.Request.Context())
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	bans := make([]dto.AIBanDTO, len(resp.Bans))
	for i, ban := range resp.Bans {
		bans[i] = aiBanProtoToDTO(ban)
	}

	c.JSON(http.StatusOK, dto.ListAIBansResponse{
		Bans: bans,
	})
}

func aiBanProtoToDTO(ban *pb.AIBan) dto.AIBanDTO {
	return dto.AIBanDTO{
		UserID:    ban.UserId,
		Reason:    ban.Reason,
		BannedBy:  ban.BannedBy,
		CreatedAt: ban.CreatedAt,
	}
}
