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
			dto.JsonError(c, http.StatusUnauthorized, "Authorization header is required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			dto.JsonError(c, http.StatusUnauthorized, "Invalid authorization header format")
			c.Abort()
			return
		}

		validateToken(c, authClient, parts[1])
	}
}

func JWTAuthWS(authClient *client.AuthClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			dto.JsonError(c, http.StatusUnauthorized, "Token query parameter is required")
			c.Abort()
			return
		}

		validateToken(c, authClient, token)
	}
}


func validateToken(c *gin.Context, authClient *client.AuthClient, token string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := authClient.ValidateToken(ctx, token)
	if err != nil {
		dto.JsonError(c, http.StatusUnauthorized, "Failed to validate token")
		c.Abort()
		return
	}

	if !resp.Valid {
		dto.JsonError(c, http.StatusUnauthorized, resp.Message)
		c.Abort()
		return
	}

	c.Set("user_id", resp.UserId)
	c.Set("email", resp.Email)

	c.Next()
}
