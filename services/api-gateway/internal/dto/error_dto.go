package dto

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func JsonError(c *gin.Context, status int, message ...string) {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	c.JSON(status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: msg,
	})
}
