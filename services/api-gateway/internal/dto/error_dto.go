package dto

import (
	"github.com/gin-gonic/gin"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ErrorResponse struct {
	Reason   string            `json:"reason"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func GrpcErrorToHTTP(err error) (int, *ErrorResponse) {
	st, ok := status.FromError(err)
	if !ok {
		return 500, &ErrorResponse{
			Reason:   "INTERNAL_ERROR",
			Metadata: map[string]string{"message": "Internal server error"},
		}
	}

	for _, detail := range st.Details() {
		if errInfo, ok := detail.(*errdetails.ErrorInfo); ok {
			return grpcCodeToHTTP(st.Code()), &ErrorResponse{
				Reason:   errInfo.Reason,
				Metadata: errInfo.Metadata,
			}
		}
	}

	return grpcCodeToHTTP(st.Code()), &ErrorResponse{
		Reason:   "UNKNOWN_ERROR",
		Metadata: map[string]string{"message": st.Message()},
	}
}

func grpcCodeToHTTP(code codes.Code) int {
	switch code {
	case codes.OK:
		return 200
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return 400
	case codes.Unauthenticated:
		return 401
	case codes.PermissionDenied:
		return 403
	case codes.NotFound:
		return 404
	case codes.AlreadyExists, codes.Aborted:
		return 409
	case codes.ResourceExhausted:
		return 429
	case codes.Canceled:
		return 499
	case codes.Internal, codes.Unknown, codes.DataLoss:
		return 500
	case codes.Unimplemented:
		return 501
	case codes.Unavailable:
		return 503
	case codes.DeadlineExceeded:
		return 504
	default:
		return 500
	}
}

func JsonError(c *gin.Context, err error) {
	statusCode, errResp := GrpcErrorToHTTP(err)
	c.JSON(statusCode, errResp)
}
