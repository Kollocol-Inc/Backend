package dto

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func makeGrpcError(code codes.Code, message, reason string, metadata map[string]string) error {
	st := status.New(code, message)
	info := &errdetails.ErrorInfo{
		Reason:   reason,
		Domain:   "test",
		Metadata: metadata,
	}
	st, _ = st.WithDetails(info)
	return st.Err()
}

func TestGrpcErrorToHTTP_WithErrorInfo(t *testing.T) {
	tests := []struct {
		name       string
		code       codes.Code
		reason     string
		wantStatus int
	}{
		{"invalid argument", codes.InvalidArgument, "INVALID_EMAIL", 400},
		{"not found", codes.NotFound, "USER_NOT_FOUND", 404},
		{"unauthenticated", codes.Unauthenticated, "INVALID_TOKEN", 401},
		{"permission denied", codes.PermissionDenied, "UNAUTHORIZED", 403},
		{"already exists", codes.AlreadyExists, "ALREADY_EXISTS", 409},
		{"internal", codes.Internal, "INTERNAL_ERROR", 500},
		{"deadline exceeded", codes.DeadlineExceeded, "TIMEOUT", 504},
		{"failed precondition", codes.FailedPrecondition, "TOO_MANY_ATTEMPTS", 400},
		{"resource exhausted", codes.ResourceExhausted, "RATE_LIMITED", 429},
		{"unavailable", codes.Unavailable, "SERVICE_DOWN", 503},
		{"unimplemented", codes.Unimplemented, "NOT_SUPPORTED", 501},
		{"canceled", codes.Canceled, "CANCELED", 499},
		{"aborted", codes.Aborted, "CONFLICT", 409},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := makeGrpcError(tt.code, "test", tt.reason, map[string]string{"key": "value"})
			httpStatus, resp := GrpcErrorToHTTP(err)
			assert.Equal(t, tt.wantStatus, httpStatus)
			assert.Equal(t, tt.reason, resp.Reason)
			assert.Equal(t, "value", resp.Metadata["key"])
		})
	}
}

func TestGrpcErrorToHTTP_WithoutErrorInfo(t *testing.T) {
	st := status.New(codes.NotFound, "resource not found")
	err := st.Err()

	httpStatus, resp := GrpcErrorToHTTP(err)
	assert.Equal(t, 404, httpStatus)
	assert.Equal(t, "UNKNOWN_ERROR", resp.Reason)
	assert.Equal(t, "resource not found", resp.Metadata["message"])
}

func TestGrpcErrorToHTTP_NonGrpcError(t *testing.T) {
	err := fmt.Errorf("plain error")
	httpStatus, resp := GrpcErrorToHTTP(err)
	assert.Equal(t, 500, httpStatus)
	assert.Equal(t, "INTERNAL_ERROR", resp.Reason)
}

func TestGrpcCodeToHTTP_AllCodes(t *testing.T) {
	tests := []struct {
		code codes.Code
		want int
	}{
		{codes.OK, 200},
		{codes.InvalidArgument, 400},
		{codes.FailedPrecondition, 400},
		{codes.OutOfRange, 400},
		{codes.Unauthenticated, 401},
		{codes.PermissionDenied, 403},
		{codes.NotFound, 404},
		{codes.AlreadyExists, 409},
		{codes.Aborted, 409},
		{codes.ResourceExhausted, 429},
		{codes.Canceled, 499},
		{codes.Internal, 500},
		{codes.Unknown, 500},
		{codes.DataLoss, 500},
		{codes.Unimplemented, 501},
		{codes.Unavailable, 503},
		{codes.DeadlineExceeded, 504},
	}

	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			got := grpcCodeToHTTP(tt.code)
			assert.Equal(t, tt.want, got)
		})
	}
}
