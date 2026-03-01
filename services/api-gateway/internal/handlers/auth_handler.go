package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"api-gateway/internal/client"
	"api-gateway/internal/dto"
	"api-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authClient *client.AuthClient
}

func NewAuthHandler(authClient *client.AuthClient) *AuthHandler {
	return &AuthHandler{
		authClient: authClient,
	}
}

// Login godoc
// @Summary Start login process
// @Description Send verification code to email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login request"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrInvalidRequestBody)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.authClient.Login(ctx, req.Email)
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.LoginResponse{})
}

// VerifyCode godoc
// @Summary Verify code and get tokens
// @Description Verify the code sent to email and receive JWT tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.VerifyCodeRequest true "Verify code request"
// @Success 200 {object} dto.VerifyCodeResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 429 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /auth/verify [post]
func (h *AuthHandler) VerifyCode(c *gin.Context) {
	var req dto.VerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrInvalidRequestBody)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.authClient.VerifyCode(ctx, req.Email, req.Code)
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.VerifyCodeResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		IsRegistered: resp.IsRegistered,
		UserID:       resp.UserId,
	})
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Get a new access token using refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "Refresh token request"
// @Success 200 {object} dto.RefreshTokenResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrInvalidRequestBody)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := h.authClient.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.RefreshTokenResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}

// Logout godoc
// @Summary Logout user
// @Description Revoke access and refresh tokens
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.LogoutResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		dto.JsonError(c, errors.ErrAuthHeaderRequired)
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		dto.JsonError(c, errors.ErrInvalidAuthHeaderFormat)
		return
	}

	accessToken := parts[1]

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	c.ShouldBindJSON(&req)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.authClient.Logout(ctx, accessToken, req.RefreshToken)
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.LogoutResponse{})
}
