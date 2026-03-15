package dto

import "encoding/json"

type ParaphraseRequest struct {
	Text string `json:"text" binding:"required" example:"Some text"`
}

type ParaphraseResponse struct {
	Text string `json:"text" example:"Some perephrased text"`
}

type GenerateTemplateMLRequest struct {
	Text string `json:"text" binding:"required" example:"Math quiz"`
}

type GenerateQuestionsMLRequest struct {
	Text      string          `json:"text" example:"Some text"`
	Questions []QuestionInput `json:"questions"`
}

type GeneratedQuestionDTO struct {
	Text          string          `json:"text" example:"What is 2+2?"`
	Type          string          `json:"type" enums:"single,multiple,open" example:"single"`
	Options       []string        `json:"options,omitempty" example:"4,3,5,6"`
	CorrectAnswer json.RawMessage `json:"correct_answer" swaggertype:"string" example:"0"`
	MaxScore      int32           `json:"max_score" example:"1"`
	TimeLimitSec  int32           `json:"time_limit_sec" example:"30"`
}

type GeneratedTemplateDTO struct {
	Title     string                 `json:"title" example:"Math Quiz"`
	Questions []GeneratedQuestionDTO `json:"questions"`
}

type GenerateQuestionsResponse struct {
	Questions []GeneratedQuestionDTO `json:"questions"`
}
