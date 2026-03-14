package dto

import "encoding/json"

type QuizSettings struct {
	RandomOrder        bool  `json:"random_order" example:"false"`
	TimeLimitTotal     int32 `json:"time_limit_total" example:"0"`
	ShowCorrectAnswers bool  `json:"show_correct_answers" example:"true"`
	AllowReview        bool  `json:"allow_review" example:"false"`
}

type QuestionInput struct {
	ID   string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Text string `json:"text" binding:"required" example:"What is 2+2?"`
	// Question type. Determines the format of correct_answer and whether options are required.
	Type string `json:"type" binding:"required,oneof=single multiple open" enums:"single,multiple,open" example:"single"`
	// Required for single and multiple types. Not used for open.
	Options []string `json:"options" example:"4,3,5,6"`
	// Format depends on type: single -> integer index (e.g. 0), multiple -> array of indices (e.g. [0,2]), open -> string (e.g. "Math")
	CorrectAnswer json.RawMessage `json:"correct_answer" binding:"required" swaggertype:"string" example:"0"`
	OrderIndex    int32           `json:"order_index" example:"1"`
	MaxScore      int32           `json:"max_score" binding:"required" example:"1"`
	TimeLimitSec  int32           `json:"time_limit_sec" example:"30"`
}

type CreateTemplateRequest struct {
	Title       string          `json:"title" binding:"required" example:"Math Quiz"`
	Description string          `json:"description" example:"Basic math operations"`
	QuizType    string          `json:"quiz_type" binding:"required,oneof=sync async" enums:"sync,async" example:"sync"`
	Settings    QuizSettings    `json:"settings"`
	Questions   []QuestionInput `json:"questions" binding:"required,min=1"`
}

type QuestionDTO struct {
	ID   string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Text string `json:"text" example:"What is 2+2?"`
	Type string `json:"type" enums:"single,multiple,open" example:"single"`
	// Present for single and multiple types
	Options []string `json:"options,omitempty" example:"4,3,5,6"`
	// Format depends on type: single -> integer index (e.g. 0), multiple -> array of indices (e.g. [0,2]), open -> string (e.g. "Math")
	CorrectAnswer any    `json:"correct_answer,omitempty" swaggertype:"string" example:"0"`
	OrderIndex    int32  `json:"order_index" example:"1"`
	MaxScore      int32  `json:"max_score" example:"1"`
	TimeLimitSec  int32  `json:"time_limit_sec" example:"30"`
	AIAnswer      string `json:"ai_answer,omitempty" example:"The answer is 4"`
}

type TemplateDTO struct {
	ID          string        `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID      string        `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Title       string        `json:"title" example:"Math Quiz"`
	Description string        `json:"description" example:"Basic math operations"`
	QuizType    string        `json:"quiz_type" enums:"sync,async" example:"sync"`
	Settings    QuizSettings  `json:"settings"`
	Questions   []QuestionDTO `json:"questions"`
	CreatedAt   string        `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt   string        `json:"updated_at" example:"2024-01-15T10:30:00Z"`
	TotalTime      uint64 `json:"total_time" example:"360"`
	TotalQuestions uint64 `json:"total_questions" example:"5"`
}

type CreateTemplateResponse struct {
	TemplateID string `json:"template_id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type GetTemplatesResponse struct {
	Templates []TemplateDTO `json:"templates"`
}

type GetTemplateResponse struct {
	Template TemplateDTO `json:"template"`
}

type DeleteTemplateResponse struct {
}

type CreateInstanceRequest struct {
	TemplateID string `json:"template_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	Title      string `json:"title" binding:"required" example:"Class Quiz — March"`
	GroupID    string `json:"group_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Deadline   string `json:"deadline" example:"2024-06-01T15:00:00Z"`
}

type InstanceDTO struct {
	ID         string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	TemplateID string `json:"template_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	HostUserID string `json:"host_user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Title      string `json:"title" example:"Class Quiz — March"`
	AccessCode string `json:"access_code" example:"481623"`
	GroupID    string `json:"group_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status    string       `json:"status" enums:"waiting,active,pending_review,reviewed" example:"waiting"`
	QuizType  string       `json:"quiz_type" enums:"sync,async" example:"sync"`
	Settings  QuizSettings `json:"settings"`
	CreatedAt string       `json:"created_at" example:"2024-01-15T10:30:00Z"`
	Deadline  string       `json:"deadline,omitempty" example:"2024-06-01T15:00:00Z"`
	TotalTime      uint64 `json:"total_time" example:"360"`
	TotalQuestions uint64 `json:"total_questions" example:"5"`
}

type CreateInstanceResponse struct {
	InstanceID string `json:"instance_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	AccessCode string `json:"access_code" example:"481623"`
}

type GetInstanceResponse struct {
	Instance  InstanceDTO   `json:"instance"`
	Questions []QuestionDTO `json:"questions"`
}

type ParticipatingInstanceDTO struct {
	Instance InstanceDTO `json:"instance"`
	SessionStatus string `json:"session_status" enums:"not_started,joined,in_progress,finished" example:"not_started"`
}

type GetParticipatingInstancesResponse struct {
	Instances []ParticipatingInstanceDTO `json:"instances"`
}

type GetHostingInstancesResponse struct {
	Instances []InstanceDTO `json:"instances"`
}

type UserAnswerDTO struct {
	UserID      string `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	QuestionID  string `json:"question_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Answer      string `json:"answer" example:"0"`
	Score       int32  `json:"score" example:"1"`
	MaxScore    int32  `json:"max_score" example:"1"`
	IsCorrect   bool   `json:"is_correct" example:"true"`
	GradedBy    string `json:"graded_by,omitempty" example:"auto"`
	SubmittedAt string `json:"submitted_at" example:"2024-01-15T10:30:00Z"`
}

type UserResultDTO struct {
	UserID      string          `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	TotalScore  int32           `json:"total_score" example:"8"`
	MaxScore    int32           `json:"max_score" example:"10"`
	Percentage  float32         `json:"percentage" example:"80.0"`
	Answers     []UserAnswerDTO `json:"answers"`
	CompletedAt string          `json:"completed_at,omitempty" example:"2024-01-15T10:30:00Z"`
}

type GetResultsResponse struct {
	InstanceID string          `json:"instance_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Results    []UserResultDTO `json:"results"`
}

type GradeAnswerRequest struct {
	UserID     string `json:"user_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	QuestionID string `json:"question_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
	Score      int32  `json:"score" binding:"required" example:"1"`
}

type GradeAnswerResponse struct {
}

type PublishResultsResponse struct {
}
