package errors

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceName = "auth-service"

func New(code codes.Code, reason, message string, metadata map[string]string) error {
	st := status.New(code, message)
	v := &errdetails.ErrorInfo{
		Reason:   reason,
		Domain:   serviceName,
		Metadata: metadata,
	}
	st, _ = st.WithDetails(v)
	return st.Err()
}

const (
	ReasonInvalidEmail          = "INVALID_EMAIL"
	ReasonCodeGenerationFailed  = "CODE_GENERATION_FAILED"
	ReasonCodeSaveFailed        = "CODE_SAVE_FAILED"
	ReasonCodeNotFound          = "CODE_NOT_FOUND"
	ReasonCodeExpired           = "CODE_EXPIRED"
	ReasonInvalidCode           = "INVALID_CODE"
	ReasonTooManyAttempts       = "TOO_MANY_ATTEMPTS"
	ReasonUserNotFound          = "USER_NOT_FOUND"
	ReasonUserProcessFailed     = "USER_PROCESS_FAILED"
	ReasonTokenGenerationFailed = "TOKEN_GENERATION_FAILED"
	ReasonTokenSaveFailed       = "TOKEN_SAVE_FAILED"
	ReasonInvalidToken          = "INVALID_TOKEN"
	ReasonTokenNotFound         = "TOKEN_NOT_FOUND"
	ReasonTokenExpired          = "TOKEN_EXPIRED"
	ReasonTokenRevoked          = "TOKEN_REVOKED"
	ReasonTokenValidationFailed = "TOKEN_VALIDATION_FAILED"
	ReasonBlacklistAddFailed    = "BLACKLIST_ADD_FAILED"

	ReasonAIBanCreateFailed = "AI_BAN_CREATE_FAILED"
	ReasonAIBanDeleteFailed = "AI_BAN_DELETE_FAILED"
	ReasonAIBanNotFound     = "AI_BAN_NOT_FOUND"
	ReasonAIBanListFailed   = "AI_BAN_LIST_FAILED"
	ReasonAIBanCheckFailed  = "AI_BAN_CHECK_FAILED"
	ReasonUserIDRequired    = "USER_ID_REQUIRED"
)
