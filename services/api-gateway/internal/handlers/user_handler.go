package handlers

import (
	"context"
	"io"
	"net/http"
	"time"

	"api-gateway/internal/client"
	"api-gateway/internal/dto"
	pb "api-gateway/proto"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userClient *client.UserClient
}

func NewUserHandler(userClient *client.UserClient) *UserHandler {
	return &UserHandler{
		userClient: userClient,
	}
}

// Register godoc
// @Summary Register new user
// @Description Complete user registration after authentication
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.RegisterRequest true "Registration request"
// @Success 200 {object} dto.UserDTO
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /users/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.Register(ctx, userID.(string), req.FirstName, req.LastName, nil, "")
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to register user")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusBadRequest, resp.Message)
		return
	}

	c.JSON(http.StatusOK, convertUserToDTO(resp.User))
}

// GetProfile godoc
// @Summary Get user profile
// @Description Get current user's profile
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.UserDTO
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /users/me [get]
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.GetProfile(ctx, userID.(string))
	if err != nil {
		dto.JsonError(c, http.StatusNotFound, "User profile not found")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusNotFound, resp.Message)
		return
	}

	c.JSON(http.StatusOK, convertUserToDTO(resp.User))
}

// UpdateProfile godoc
// @Summary Update user profile
// @Description Update current user's profile
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateProfileRequest true "Profile update request"
// @Success 200 {object} dto.UserDTO
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /users/me [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.UpdateProfile(ctx, userID.(string), req.FirstName, req.LastName, nil, "")
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusBadRequest, resp.Message)
		return
	}

	c.JSON(http.StatusOK, convertUserToDTO(resp.User))
}

// UploadAvatar godoc
// @Summary Upload user avatar
// @Description Upload avatar image for current user
// @Tags users
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param avatar formData file true "Avatar image"
// @Success 200 {object} dto.UserDTO
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /users/me/avatar [post]
func (h *UserHandler) UploadAvatar(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		dto.JsonError(c, http.StatusBadRequest, "Avatar file is required")
		return
	}

	if file.Size > 5*1024*1024 {
		dto.JsonError(c, http.StatusBadRequest, "File size exceeds 5MB limit")
		return
	}

	src, err := file.Open()
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to read file")
		return
	}
	defer src.Close()

	avatarData, err := io.ReadAll(src)
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to read file content")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := h.userClient.UpdateProfile(ctx, userID.(string), "", "", avatarData, file.Filename)
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to upload avatar")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusBadRequest, resp.Message)
		return
	}

	c.JSON(http.StatusOK, convertUserToDTO(resp.User))
}

// GetNotificationSettings godoc
// @Summary Get notification settings
// @Description Get current user's notification settings
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.NotificationSettingsDTO
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /users/me/notifications [get]
func (h *UserHandler) GetNotificationSettings(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.GetNotificationSettings(ctx, userID.(string))
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to get notification settings")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusInternalServerError, resp.Message)
		return
	}

	c.JSON(http.StatusOK, convertNotificationSettingsToDTO(resp.Settings))
}

// UpdateNotificationSettings godoc
// @Summary Update notification settings
// @Description Update current user's notification settings
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateNotificationSettingsRequest true "Notification settings"
// @Success 200 {object} dto.NotificationSettingsDTO
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /users/me/notifications [put]
func (h *UserHandler) UpdateNotificationSettings(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	var req dto.UpdateNotificationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	updateReq := &pb.UpdateNotificationSettingsRequest{
		UserId:           userID.(string),
		NewQuizzes:       req.NewQuizzes,
		QuizResults:      req.QuizResults,
		GroupInvites:     req.GroupInvites,
		DeadlineReminder: req.DeadlineReminder,
	}

	resp, err := h.userClient.UpdateNotificationSettings(ctx, updateReq)
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to update notification settings")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusBadRequest, resp.Message)
		return
	}

	c.JSON(http.StatusOK, convertNotificationSettingsToDTO(resp.Settings))
}

// CreateGroup godoc
// @Summary Create new group
// @Description Create a new group and invite members
// @Tags groups
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateGroupRequest true "Group creation request"
// @Success 201 {object} dto.GroupDTO
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /groups [post]
func (h *UserHandler) CreateGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	var req dto.CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.CreateGroup(ctx, userID.(string), req.Name, req.MemberEmails)
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to create group")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusBadRequest, resp.Message)
		return
	}

	c.JSON(http.StatusCreated, convertGroupToDTO(resp.Group))
}

// GetGroups godoc
// @Summary Get user's groups
// @Description Get all groups the user is a member of
// @Tags groups
// @Produce json
// @Security BearerAuth
// @Param filter query string false "Filter: 'my' or 'created'" default(my)
// @Success 200 {object} dto.GetGroupsResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /groups [get]
func (h *UserHandler) GetGroups(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	filter := c.DefaultQuery("filter", "my")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.GetGroups(ctx, userID.(string), filter)
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to get groups")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusInternalServerError, resp.Message)
		return
	}

	groups := make([]dto.GroupDTO, len(resp.Groups))
	for i, g := range resp.Groups {
		groups[i] = convertGroupToDTO(g)
	}
	c.JSON(http.StatusOK, dto.GetGroupsResponse{Groups: groups})
}

// GetGroup godoc
// @Summary Get group details
// @Description Get details of a specific group
// @Tags groups
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Success 200 {object} dto.GroupWithMembersDTO
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /groups/{id} [get]
func (h *UserHandler) GetGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	groupID := c.Param("id")
	if groupID == "" {
		dto.JsonError(c, http.StatusBadRequest, "Group ID is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.GetGroup(ctx, groupID, userID.(string))
	if err != nil {
		dto.JsonError(c, http.StatusNotFound, "Group not found or access denied")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusNotFound, resp.Message)
		return
	}

	c.JSON(http.StatusOK, convertGroupWithMembersToDTO(resp.Group))
}

// UpdateGroup godoc
// @Summary Update group
// @Description Update group information (owner only)
// @Tags groups
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Param request body dto.UpdateGroupRequest true "Group update request"
// @Success 200 {object} dto.GroupDTO
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /groups/{id} [put]
func (h *UserHandler) UpdateGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	groupID := c.Param("id")
	if groupID == "" {
		dto.JsonError(c, http.StatusBadRequest, "Group ID is required")
		return
	}

	var req dto.UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.UpdateGroup(ctx, groupID, userID.(string), req.Name, req.MemberEmails)
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to update group")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusBadRequest, resp.Message)
		return
	}

	c.JSON(http.StatusOK, convertGroupToDTO(resp.Group))
}

// DeleteGroup godoc
// @Summary Delete group
// @Description Delete a group (owner only)
// @Tags groups
// @Produce json
// @Security BearerAuth
// @Param id path string true "Group ID"
// @Success 204
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /groups/{id} [delete]
func (h *UserHandler) DeleteGroup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		dto.JsonError(c, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	groupID := c.Param("id")
	if groupID == "" {
		dto.JsonError(c, http.StatusBadRequest, "Group ID is required")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.userClient.DeleteGroup(ctx, groupID, userID.(string))
	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, "Failed to delete group")
		return
	}

	if !resp.Success {
		dto.JsonError(c, http.StatusBadRequest, resp.Message)
		return
	}

	c.Status(http.StatusNoContent)
}

func convertUserToDTO(u *pb.User) dto.UserDTO {
	return dto.UserDTO{
		ID:        u.Id,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		AvatarURL: u.AvatarUrl,
		CreatedAt: time.Unix(u.CreatedAt, 0).Format(time.RFC3339),
		UpdatedAt: time.Unix(u.CreatedAt, 0).Format(time.RFC3339),
	}
}

func convertNotificationSettingsToDTO(s *pb.NotificationSettings) dto.NotificationSettingsDTO {
	return dto.NotificationSettingsDTO{
		UserID:           s.UserId,
		NewQuizzes:       s.NewQuizzes,
		QuizResults:      s.QuizResults,
		GroupInvites:     s.GroupInvites,
		DeadlineReminder: s.DeadlineReminder,
	}
}

func convertGroupToDTO(g *pb.Group) dto.GroupDTO {
	return dto.GroupDTO{
		ID:          g.Id,
		Name:        g.Name,
		OwnerID:     g.OwnerId,
		MemberCount: g.MemberCount,
		CreatedAt:   time.Unix(g.CreatedAt, 0).Format(time.RFC3339),
		UpdatedAt:   time.Unix(g.CreatedAt, 0).Format(time.RFC3339),
	}
}

func convertGroupWithMembersToDTO(gwm *pb.GroupWithMembers) dto.GroupWithMembersDTO {
	members := make([]dto.GroupMemberDTO, len(gwm.Members))
	for i, m := range gwm.Members {
		members[i] = dto.GroupMemberDTO{
			UserID:    m.Id,
			Email:     m.Email,
			FirstName: m.FirstName,
			LastName:  m.LastName,
			AvatarURL: m.AvatarUrl,
			JoinedAt:  time.Unix(m.CreatedAt, 0).Format(time.RFC3339),
		}
	}

	return dto.GroupWithMembersDTO{
		ID:        gwm.Group.Id,
		Name:      gwm.Group.Name,
		OwnerID:   gwm.Group.OwnerId,
		Members:   members,
		CreatedAt: time.Unix(gwm.Group.CreatedAt, 0).Format(time.RFC3339),
		UpdatedAt: time.Unix(gwm.Group.CreatedAt, 0).Format(time.RFC3339),
	}
}
