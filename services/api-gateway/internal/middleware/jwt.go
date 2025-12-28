package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"api-gateway/internal/client"
	"api-gateway/internal/dto"

	"github.com/gin-gonic/gin"
)

func JWTAuth(authClient *client.AuthClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   http.StatusText(http.StatusUnauthorized),
				Message: "Authorization header is required",
			})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   http.StatusText(http.StatusUnauthorized),
				Message: "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		token := parts[1]

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		resp, err := authClient.ValidateToken(ctx, token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   http.StatusText(http.StatusUnauthorized),
				Message: "Failed to validate token",
			})
			c.Abort()
			return
		}

		if !resp.Valid {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error:   http.StatusText(http.StatusUnauthorized),
				Message: resp.Message,
			})
			c.Abort()
			return
		}

		c.Set("user_id", resp.UserId)
		c.Set("email", resp.Email)

		c.Next()
	}
}
