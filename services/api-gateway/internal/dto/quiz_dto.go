package dto

import "encoding/json"

type QuizSettings struct {
	RandomOrder        bool  `json:"random_order"`
	TimeLimitTotal     int32 `json:"time_limit_total"`
	ShowCorrectAnswers bool  `json:"show_correct_answers"`
	AllowReview        bool  `json:"allow_review"`
}

type QuestionInput struct {
	ID            string          `json:"id"`
	Text          string          `json:"text" binding:"required"`
	Type          string          `json:"type" binding:"required,oneof=single multiple open"`
	Options       []string        `json:"options"`
	CorrectAnswer json.RawMessage `json:"correct_answer" binding:"required"`
	OrderIndex    int32           `json:"order_index"`
	MaxScore      int32           `json:"max_score" binding:"required"`
	TimeLimitSec  int32           `json:"time_limit_sec"`
}

type CreateTemplateRequest struct {
	Title       string          `json:"title" binding:"required"`
	Description string          `json:"description"`
	QuizType    string          `json:"quiz_type" binding:"required,oneof=sync async"`
	Settings    QuizSettings    `json:"settings"`
	Questions   []QuestionInput `json:"questions" binding:"required,min=1"`
}

type QuestionDTO struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	Type          string   `json:"type"`
	Options       []string `json:"options,omitempty"`
	CorrectAnswer any      `json:"correct_answer,omitempty"`
	OrderIndex    int32    `json:"order_index"`
	MaxScore      int32    `json:"max_score"`
	TimeLimitSec  int32    `json:"time_limit_sec"`
	AIAnswer      string   `json:"ai_answer,omitempty"`
}

type TemplateDTO struct {
	ID             string        `json:"id"`
	UserID         string        `json:"user_id"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	QuizType       string        `json:"quiz_type"`
	Settings       QuizSettings  `json:"settings"`
	Questions      []QuestionDTO `json:"questions"`
	CreatedAt      string        `json:"created_at"`
	UpdatedAt      string        `json:"updated_at"`
	TotalTime      uint64        `json:"total_time"`
	TotalQuestions uint64        `json:"total_questions"`
}

type CreateTemplateResponse struct {
	TemplateID string `json:"template_id"`
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
	TemplateID string `json:"template_id" binding:"required"`
	Title      string `json:"title" binding:"required"`
	GroupID    string `json:"group_id"`
	Deadline   string `json:"deadline"` // ISO 8601 format
}

type InstanceDTO struct {
	ID             string       `json:"id"`
	TemplateID     string       `json:"template_id"`
	HostUserID     string       `json:"host_user_id"`
	Title          string       `json:"title"`
	AccessCode     string       `json:"access_code"`
	GroupID        string       `json:"group_id,omitempty"`
	Status         string       `json:"status"`
	QuizType       string       `json:"quiz_type"`
	Settings       QuizSettings `json:"settings"`
	CreatedAt      string       `json:"created_at"`
	Deadline       string       `json:"deadline,omitempty"`
	TotalTime      uint64       `json:"total_time"`
	TotalQuestions uint64       `json:"total_questions"`
}

type CreateInstanceResponse struct {
	InstanceID string `json:"instance_id"`
	AccessCode string `json:"access_code"`
}

type GetInstanceResponse struct {
	Instance  InstanceDTO   `json:"instance"`
	Questions []QuestionDTO `json:"questions"`
}

type ParticipatingInstanceDTO struct {
	Instance      InstanceDTO `json:"instance"`
	SessionStatus string      `json:"session_status"` // "not_started", "joined", "in_progress", "finished"
}

type GetParticipatingInstancesResponse struct {
	Instances []ParticipatingInstanceDTO `json:"instances"`
}

type GetHostingInstancesResponse struct {
	Instances []InstanceDTO `json:"instances"`
}

type UserAnswerDTO struct {
	UserID      string `json:"user_id"`
	QuestionID  string `json:"question_id"`
	Answer      string `json:"answer"`
	Score       int32  `json:"score"`
	MaxScore    int32  `json:"max_score"`
	IsCorrect   bool   `json:"is_correct"`
	GradedBy    string `json:"graded_by,omitempty"`
	SubmittedAt string `json:"submitted_at"`
}

type UserResultDTO struct {
	UserID      string          `json:"user_id"`
	TotalScore  int32           `json:"total_score"`
	MaxScore    int32           `json:"max_score"`
	Percentage  float32         `json:"percentage"`
	Answers     []UserAnswerDTO `json:"answers"`
	CompletedAt string          `json:"completed_at,omitempty"`
}

type GetResultsResponse struct {
	InstanceID string          `json:"instance_id"`
	Results    []UserResultDTO `json:"results"`
}

type GradeAnswerRequest struct {
	UserID     string `json:"user_id" binding:"required"`
	QuestionID string `json:"question_id" binding:"required"`
	Score      int32  `json:"score" binding:"required"`
}

type GradeAnswerResponse struct {
}

type PublishResultsResponse struct {
}
