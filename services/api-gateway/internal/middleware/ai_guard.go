package middleware

import (
	"context"
	"time"

	"api-gateway/internal/client"
	"api-gateway/internal/dto"
	"api-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
)

func AIFeatureGuard(aiEnabled bool, authClient *client.AuthClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !aiEnabled {
			dto.JsonError(c, errors.ErrAIFeaturesDisabled)
			c.Abort()
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			dto.JsonError(c, errors.ErrUserIDNotFound)
			c.Abort()
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		resp, err := authClient.CheckAIBan(ctx, userID.(string))
		if err != nil {
			dto.JsonError(c, err)
			c.Abort()
			return
		}

		if resp.IsBanned {
			dto.JsonError(c, errors.NewAIBannedError(resp.Reason))
			c.Abort()
			return
		}

		c.Next()
	}
}

