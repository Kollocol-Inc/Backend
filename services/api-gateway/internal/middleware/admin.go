package middleware

import (
	"api-gateway/internal/dto"
	"api-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
)

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role != "admin" {
			dto.JsonError(c, errors.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}
