package errors

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceName = "user-service"

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
	ReasonUserNotFound          = "USER_NOT_FOUND"
	ReasonUserUpdateFailed      = "USER_UPDATE_FAILED"
	ReasonAvatarUploadFailed    = "AVATAR_UPLOAD_FAILED"
	ReasonAvatarDeleteFailed    = "AVATAR_DELETE_FAILED"
	ReasonNoAvatar              = "NO_AVATAR"
	ReasonSettingsNotFound      = "SETTINGS_NOT_FOUND"
	ReasonSettingsUpdateFailed  = "SETTINGS_UPDATE_FAILED"
	ReasonGroupNotFound         = "GROUP_NOT_FOUND"
	ReasonGroupCreateFailed     = "GROUP_CREATE_FAILED"
	ReasonGroupUpdateFailed     = "GROUP_UPDATE_FAILED"
	ReasonGroupDeleteFailed     = "GROUP_DELETE_FAILED"
	ReasonUnauthorized          = "UNAUTHORIZED"
	ReasonAccessDenied          = "ACCESS_DENIED"
	ReasonInvalidFilter         = "INVALID_FILTER"
	ReasonMembersRetrieveFailed = "MEMBERS_RETRIEVE_FAILED"
	ReasonDeleteUserFailed      = "DELETE_USER_FAILED"
	ReasonUserIDRequired        = "USER_ID_REQUIRED"
	ReasonInvalidLanguage       = "INVALID_LANGUAGE"
)