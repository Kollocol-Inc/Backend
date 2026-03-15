package handlers

import (
	"encoding/json"
	"net/http"

	"api-gateway/internal/client"
	"api-gateway/internal/dto"
	"api-gateway/pkg/errors"
	pb "api-gateway/proto"

	"github.com/gin-gonic/gin"
)

type MLHandler struct {
	mlClient *client.MLClient
}

func NewMLHandler(mlClient *client.MLClient) *MLHandler {
	return &MLHandler{mlClient: mlClient}
}

// Paraphrase godoc
// @Summary Paraphrase text
// @Tags ML
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ParaphraseRequest true "Text to paraphrase"
// @Success 200 {object} dto.ParaphraseResponse
// @Router /ml/paraphrase [post]
func (h *MLHandler) Paraphrase(c *gin.Context) {
	var req dto.ParaphraseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrInvalidRequestBody)
		return
	}

	resp, err := h.mlClient.Paraphrase(c.Request.Context(), &pb.ParaphraseRequest{
		Text: req.Text,
	})
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ParaphraseResponse{
		Text: resp.Text,
	})
}

// GenerateTemplate godoc
// @Summary Generate quiz template from text
// @Tags ML
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.GenerateTemplateMLRequest true "Text/topic for template generation"
// @Success 200 {object} dto.GeneratedTemplateDTO
// @Router /ml/generate/template [post]
func (h *MLHandler) GenerateTemplate(c *gin.Context) {
	var req dto.GenerateTemplateMLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrInvalidRequestBody)
		return
	}

	resp, err := h.mlClient.GenerateTemplate(c.Request.Context(), &pb.GenerateTemplateRequest{
		Text: req.Text,
	})
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	questions := make([]dto.GeneratedQuestionDTO, len(resp.Questions))
	for i, q := range resp.Questions {
		questions[i] = generatedQuestionProtoToDTO(q)
	}

	c.JSON(http.StatusOK, dto.GeneratedTemplateDTO{
		Title:     resp.Title,
		Questions: questions,
	})
}

// GenerateQuestions godoc
// @Summary Generate additional questions
// @Tags ML
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.GenerateQuestionsMLRequest true "Existing questions and/or topic"
// @Success 200 {object} dto.GenerateQuestionsResponse
// @Router /ml/generate/template/questions [post]
func (h *MLHandler) GenerateQuestions(c *gin.Context) {
	var req dto.GenerateQuestionsMLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrInvalidRequestBody)
		return
	}

	protoQuestions := make([]*pb.GeneratedQuestion, len(req.Questions))
	for i, q := range req.Questions {
		protoQuestions[i] = questionInputToGeneratedProto(&q)
	}

	resp, err := h.mlClient.GenerateQuestions(c.Request.Context(), &pb.GenerateQuestionsRequest{
		Text:      req.Text,
		Questions: protoQuestions,
	})
	if err != nil {
		dto.JsonError(c, err)
		return
	}

	questions := make([]dto.GeneratedQuestionDTO, len(resp.Questions))
	for i, q := range resp.Questions {
		questions[i] = generatedQuestionProtoToDTO(q)
	}

	c.JSON(http.StatusOK, dto.GenerateQuestionsResponse{
		Questions: questions,
	})
}

func generatedQuestionProtoToDTO(q *pb.GeneratedQuestion) dto.GeneratedQuestionDTO {
	d := dto.GeneratedQuestionDTO{
		Text:         q.Text,
		MaxScore:     q.MaxScore,
		TimeLimitSec: q.TimeLimitSec,
	}

	switch a := q.Answer.(type) {
	case *pb.GeneratedQuestion_SingleChoice:
		d.Type = "single"
		d.Options = a.SingleChoice.Options
		d.CorrectAnswer, _ = json.Marshal(a.SingleChoice.CorrectOption)
	case *pb.GeneratedQuestion_MultipleChoice:
		d.Type = "multiple"
		d.Options = a.MultipleChoice.Options
		d.CorrectAnswer, _ = json.Marshal(a.MultipleChoice.CorrectOptions)
	case *pb.GeneratedQuestion_OpenAnswer:
		d.Type = "open"
		d.CorrectAnswer, _ = json.Marshal(a.OpenAnswer.CorrectText)
	}

	return d
}

func questionInputToGeneratedProto(q *dto.QuestionInput) *pb.GeneratedQuestion {
	gq := &pb.GeneratedQuestion{
		Text:         q.Text,
		MaxScore:     q.MaxScore,
		TimeLimitSec: q.TimeLimitSec,
	}

	switch q.Type {
	case "single":
		var correctOption int32
		json.Unmarshal(q.CorrectAnswer, &correctOption)
		gq.Answer = &pb.GeneratedQuestion_SingleChoice{
			SingleChoice: &pb.MLSingleChoice{Options: q.Options, CorrectOption: correctOption},
		}
	case "multiple":
		var correctOptions []int32
		json.Unmarshal(q.CorrectAnswer, &correctOptions)
		gq.Answer = &pb.GeneratedQuestion_MultipleChoice{
			MultipleChoice: &pb.MLMultipleChoice{Options: q.Options, CorrectOptions: correctOptions},
		}
	case "open":
		var correctText string
		json.Unmarshal(q.CorrectAnswer, &correctText)
		gq.Answer = &pb.GeneratedQuestion_OpenAnswer{
			OpenAnswer: &pb.MLOpenAnswer{CorrectText: correctText},
		}
	}

	return gq
}
