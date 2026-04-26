package errors

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func New(code codes.Code, reason, message string, metadata map[string]string) error {
	st := status.New(code, message)
	v := &errdetails.ErrorInfo{
		Reason:   reason,
		Domain:   "api-gateway",
		Metadata: metadata,
	}
	st, _ = st.WithDetails(v)
	return st.Err()
}

var (
	ErrUserIDNotFound = New(
		codes.InvalidArgument,
		"USER_ID_NOT_FOUND",
		"User ID not found in context",
		nil,
	)

	ErrInvalidRequestBody = New(
		codes.InvalidArgument,
		"INVALID_REQUEST_BODY",
		"Invalid request body",
		nil,
	)

	ErrAuthHeaderRequired = New(
		codes.InvalidArgument,
		"AUTH_HEADER_REQUIRED",
		"Authorization header is required",
		nil,
	)

	ErrInvalidAuthHeaderFormat = New(
		codes.InvalidArgument,
		"INVALID_AUTH_HEADER_FORMAT",
		"Invalid authorization header format",
		nil,
	)

	ErrAvatarFileRequired = New(
		codes.InvalidArgument,
		"AVATAR_FILE_REQUIRED",
		"Avatar file is required",
		nil,
	)

	ErrFileSizeExceeds5MB = New(
		codes.InvalidArgument,
		"FILE_SIZE_EXCEEDS_LIMIT",
		"File size exceeds 5MB limit",
		nil,
	)

	ErrFailedToReadFile = New(
		codes.InvalidArgument,
		"FAILED_TO_READ_FILE",
		"Failed to read file",
		nil,
	)

	ErrFailedToReadFileContent = New(
		codes.InvalidArgument,
		"FAILED_TO_READ_FILE_CONTENT",
		"Failed to read file content",
		nil,
	)

	ErrGroupIDRequired = New(
		codes.InvalidArgument,
		"GROUP_ID_REQUIRED",
		"Group ID is required",
		nil,
	)

	ErrNotificationIDsRequired = New(
		codes.InvalidArgument,
		"NOTIFICATION_IDS_REQUIRED",
		"Notification IDs are required",
		nil,
	)

	ErrQuestionNotFound = New(
		codes.NotFound,
		"QUESTION_NOT_FOUND",
		"Question not found in instance",
		nil,
	)

	ErrReviewOnlyForOpen = New(
		codes.InvalidArgument,
		"REVIEW_ONLY_FOR_OPEN",
		"AI review is only available for open-type questions",
		nil,
	)

	ErrAnswerNotFound = New(
		codes.NotFound,
		"ANSWER_NOT_FOUND",
		"Participant has not answered this question",
		nil,
	)

	ErrForbidden = New(
		codes.PermissionDenied,
		"FORBIDDEN",
		"You do not have permission to perform this action",
		nil,
	)

	ErrGameServiceUnavailable = New(
		codes.Unavailable,
		"GAME_SERVICE_UNAVAILABLE",
		"Failed to terminate active game session",
		nil,
	)
)

var (
	ErrTokenMissing = New(
		codes.Unauthenticated,
		"TOKEN_MISSING",
		"Authorization token is missing",
		nil,
	)

	ErrInvalidTokenFormat = New(
		codes.Unauthenticated,
		"INVALID_TOKEN_FORMAT",
		"Invalid token format",
		nil,
	)

	ErrInvalidToken = New(
		codes.Unauthenticated,
		"INVALID_TOKEN",
		"Invalid or expired token",
		nil,
	)

	ErrTokenQueryRequired = New(
		codes.Unauthenticated,
		"TOKEN_QUERY_REQUIRED",
		"Token query parameter is required",
		nil,
	)
)

var (
	ErrAIFeaturesDisabled = New(
		codes.PermissionDenied,
		"AI_FEATURES_DISABLED",
		"AI features are currently disabled",
		nil,
	)

	ErrUserIDBanRequired = New(
		codes.InvalidArgument,
		"USER_ID_REQUIRED",
		"User ID is required",
		nil,
	)
)

func NewAIBannedError(reason string) error {
	metadata := map[string]string{}
	if reason != "" {
		metadata["reason"] = reason
	}
	return New(
		codes.PermissionDenied,
		"AI_USER_BANNED",
		"You are banned from using AI features",
		metadata,
	)
}
