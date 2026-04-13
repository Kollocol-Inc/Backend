package errors

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const serviceName = "quiz-service"

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
	ReasonTemplateNotFound      = "TEMPLATE_NOT_FOUND"
	ReasonTemplateCreateFailed  = "TEMPLATE_CREATE_FAILED"
	ReasonTemplateUpdateFailed  = "TEMPLATE_UPDATE_FAILED"
	ReasonTemplateDeleteFailed  = "TEMPLATE_DELETE_FAILED"
	ReasonUnauthorized          = "UNAUTHORIZED"
	ReasonQuestionCreateFailed  = "QUESTION_CREATE_FAILED"
	ReasonQuestionDeleteFailed  = "QUESTION_DELETE_FAILED"
	ReasonInstanceNotFound      = "INSTANCE_NOT_FOUND"
	ReasonInstanceCreateFailed  = "INSTANCE_CREATE_FAILED"
	ReasonInstanceDeleteFailed  = "INSTANCE_DELETE_FAILED"
	ReasonAccessDenied          = "ACCESS_DENIED"
	ReasonInvalidFilter         = "INVALID_FILTER"
	ReasonSettingsMarshalFailed = "SETTINGS_MARSHAL_FAILED"
	ReasonOptionsMarshalFailed  = "OPTIONS_MARSHAL_FAILED"
	ReasonAnswerMarshalFailed   = "ANSWER_MARSHAL_FAILED"
	ReasonParticipantNotFound   = "PARTICIPANT_NOT_FOUND"
	ReasonQuestionNotFound      = "QUESTION_NOT_FOUND"
	ReasonGradeFailed           = "GRADE_FAILED"
	ReasonInvalidScore          = "INVALID_SCORE"
	ReasonNotAllReviewed        = "NOT_ALL_REVIEWED"
	ReasonPublishFailed         = "PUBLISH_FAILED"
)