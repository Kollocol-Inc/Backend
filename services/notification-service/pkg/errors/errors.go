package errors

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceName = "notification-service"

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
	ReasonNotificationNotFound      = "NOTIFICATION_NOT_FOUND"
	ReasonNotificationMarkFailed    = "NOTIFICATION_MARK_FAILED"
	ReasonNotificationDeleteFailed  = "NOTIFICATION_DELETE_FAILED"
	ReasonNotificationCreateFailed  = "NOTIFICATION_CREATE_FAILED"
	ReasonNotificationsFetchFailed  = "NOTIFICATIONS_FETCH_FAILED"
	ReasonUserIDRequired            = "USER_ID_REQUIRED"
)