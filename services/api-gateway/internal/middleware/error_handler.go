package middleware

import (
	"api-gateway/internal/dto"
	"log"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				grpcErr := status.Error(codes.Internal, "Internal server error")
				dto.JsonError(c, grpcErr)
				c.Abort()
			}
		}()

		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			log.Printf("Request error: %v", err)

			if _, ok := status.FromError(err); !ok {
				err = status.Error(codes.Internal, err.Error())
			}

			dto.JsonError(c, err)
		}
	}
}
