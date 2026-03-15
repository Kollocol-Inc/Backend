package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"api-gateway/internal/client"
	"api-gateway/internal/dto"
	"api-gateway/pkg/errors"
	pb "api-gateway/proto"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type QuizHandler struct {
	quizClient *client.QuizClient
}

func NewQuizHandler(quizClient *client.QuizClient) *QuizHandler {
	return &QuizHandler{
		quizClient: quizClient,
	}
}

// CreateTemplate godoc
// @Summary Create quiz template
// @Tags Quiz
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateTemplateRequest true "Template data"
// @Success 200 {object} dto.CreateTemplateResponse
// @Router /quizzes/templates [post]
func (h *QuizHandler) CreateTemplate(c *gin.Context) {
	userID := c.GetString("user_id")

	var req dto.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrInvalidRequestBody)
		return
	}

	questions := make([]*pb.QuestionInput, len(req.Questions))
	for i, q := range req.Questions {
		questions[i] = questionInputToProto(&q)
	}

	protoReq := &pb.CreateTemplateRequest{
		UserId:    userID,
		Title:     req.Title,
		QuizType:  req.QuizType,
		Questions: questions,
	}

	if asyncSettings := parseAsyncSettings(req.QuizType, req.Settings); asyncSettings != nil {
		protoReq.Settings = &pb.CreateTemplateRequest_AsyncSettings{AsyncSettings: asyncSettings}
	} else {
		protoReq.Settings = &pb.CreateTemplateRequest_SyncSettings{SyncSettings: &pb.QuizSyncSettings{}}
	}

	resp, err := h.quizClient.CreateTemplate(c.Request.Context(), protoReq)

	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.CreateTemplateResponse{
		TemplateID: resp.Template.Id,
	})
}

// GetTemplates godoc
// @Summary Get user's quiz templates
// @Tags Quiz
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.GetTemplatesResponse
// @Router /quizzes/templates [get]
func (h *QuizHandler) GetTemplates(c *gin.Context) {
	userID := c.GetString("user_id")

	resp, err := h.quizClient.GetTemplates(c.Request.Context(), &pb.GetTemplatesRequest{
		UserId: userID,
	})

	if err != nil {
		dto.JsonError(c, err)
		return
	}

	templates := make([]dto.TemplateDTO, len(resp.Templates))
	for i, twq := range resp.Templates {
		templates[i] = templateProtoToDTO(twq.Template, twq.Questions)
	}

	c.JSON(http.StatusOK, dto.GetTemplatesResponse{
		Templates: templates,
	})
}

// GetTemplate godoc
// @Summary Get quiz template by ID
// @Tags Quiz
// @Produce json
// @Security BearerAuth
// @Param id path string true "Template ID"
// @Success 200 {object} dto.GetTemplateResponse
// @Router /quizzes/templates/{id} [get]
func (h *QuizHandler) GetTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	templateID := c.Param("id")

	resp, err := h.quizClient.GetTemplate(c.Request.Context(), &pb.GetTemplateRequest{
		TemplateId: templateID,
		UserId:     userID,
	})

	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.GetTemplateResponse{
		Template: templateProtoToDTO(resp.Template, resp.Questions),
	})
}

// UpdateTemplate godoc
// @Summary Update quiz template
// @Tags Quiz
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Template ID"
// @Param request body dto.CreateTemplateRequest true "Template data"
// @Success 200 {object} dto.CreateTemplateResponse
// @Router /quizzes/templates/{id} [put]
func (h *QuizHandler) UpdateTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	templateID := c.Param("id")

	var req dto.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrInvalidRequestBody)
		return
	}

	questions := make([]*pb.QuestionInput, len(req.Questions))
	for i, q := range req.Questions {
		questions[i] = questionInputToProto(&q)
	}

	updateReq := &pb.UpdateTemplateRequest{
		TemplateId: templateID,
		UserId:     userID,
		Title:      req.Title,
		QuizType:   req.QuizType,
		Questions:  questions,
	}

	if asyncSettings := parseAsyncSettings(req.QuizType, req.Settings); asyncSettings != nil {
		updateReq.Settings = &pb.UpdateTemplateRequest_AsyncSettings{AsyncSettings: asyncSettings}
	} else {
		updateReq.Settings = &pb.UpdateTemplateRequest_SyncSettings{SyncSettings: &pb.QuizSyncSettings{}}
	}

	resp, err := h.quizClient.UpdateTemplate(c.Request.Context(), updateReq)

	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.CreateTemplateResponse{
		TemplateID: resp.Template.Id,
	})
}

// DeleteTemplate godoc
// @Summary Delete quiz template
// @Tags Quiz
// @Produce json
// @Security BearerAuth
// @Param id path string true "Template ID"
// @Success 200 {object} dto.DeleteTemplateResponse
// @Router /quizzes/templates/{id} [delete]
func (h *QuizHandler) DeleteTemplate(c *gin.Context) {
	userID := c.GetString("user_id")
	templateID := c.Param("id")

	_, err := h.quizClient.DeleteTemplate(c.Request.Context(), &pb.DeleteTemplateRequest{
		TemplateId: templateID,
		UserId:     userID,
	})

	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// CreateInstance godoc
// @Summary Create quiz instance (start quiz)
// @Tags Quiz
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateInstanceRequest true "Instance data"
// @Success 200 {object} dto.CreateInstanceResponse
// @Router /quizzes/instances [post]
func (h *QuizHandler) CreateInstance(c *gin.Context) {
	userID := c.GetString("user_id")

	var req dto.CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.JsonError(c, errors.ErrInvalidRequestBody)
		return
	}

	protoReq := &pb.CreateInstanceRequest{
		UserId:     userID,
		TemplateId: req.TemplateID,
		Title:      req.Title,
		GroupId:    req.GroupID,
	}

	if req.Deadline != "" {
		deadline, err := time.Parse(time.RFC3339, req.Deadline)
		if err != nil {
			dto.JsonError(c, errors.ErrInvalidRequestBody)
			return
		}
		protoReq.Deadline = timestamppb.New(deadline)
	}

	resp, err := h.quizClient.CreateInstance(c.Request.Context(), protoReq)

	if err != nil {
		dto.JsonError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.CreateInstanceResponse{
		InstanceID: resp.Instance.Id,
		AccessCode: resp.Instance.AccessCode,
	})
}

// GetInstance godoc
// @Summary Get quiz instance details
// @Tags Quiz
// @Produce json
// @Security BearerAuth
// @Param id path string true "Instance ID"
// @Success 200 {object} dto.GetInstanceResponse
// @Router /quizzes/instances/{id} [get]
func (h *QuizHandler) GetInstance(c *gin.Context) {
	userID := c.GetString("user_id")
	instanceID := c.Param("id")

	resp, err := h.quizClient.GetInstance(c.Request.Context(), &pb.GetInstanceRequest{
		InstanceId: instanceID,
		UserId:     userID,
	})

	if err != nil {
		dto.JsonError(c, err)
		return
	}

	inst := resp.Instance
	instance := instanceProtoToDTO(inst)

	if inst.Deadline != nil {
		instance.Deadline = inst.Deadline.AsTime().Format(time.RFC3339)
	}

	questions := make([]dto.QuestionDTO, len(resp.Questions))
	for i, q := range resp.Questions {
		questions[i] = questionProtoToDTO(q)
	}

	c.JSON(http.StatusOK, dto.GetInstanceResponse{
		Instance:  instance,
		Questions: questions,
	})
}

// GetHostingInstances godoc
// @Summary Get quizzes created by user
// @Tags Quiz
// @Produce json
// @Security BearerAuth
// @Param status query string false "Instance status filter" Enums(waiting,active,pending_review,reviewed)
// @Success 200 {object} dto.GetHostingInstancesResponse
// @Router /quizzes/instances/hosting [get]
func (h *QuizHandler) GetHostingInstances(c *gin.Context) {
	userID := c.GetString("user_id")
	status := c.Query("status")

	resp, err := h.quizClient.GetHostingInstances(c.Request.Context(), &pb.GetHostingInstancesRequest{
		UserId: userID,
		Status: status,
	})

	if err != nil {
		dto.JsonError(c, err)
		return
	}

	instances := make([]dto.InstanceDTO, len(resp.Instances))
	for i, inst := range resp.Instances {
		instances[i] = instanceProtoToDTO(inst)
		if inst.Deadline != nil {
			instances[i].Deadline = inst.Deadline.AsTime().Format(time.RFC3339)
		}
	}

	c.JSON(http.StatusOK, dto.GetHostingInstancesResponse{
		Instances: instances,
	})
}

// GetParticipatingInstances godoc
// @Summary Get quizzes where user is a participant
// @Tags Quiz
// @Produce json
// @Security BearerAuth
// @Param session_status query string false "Session status filter" Enums(not_started,joined,in_progress,finished)
// @Success 200 {object} dto.GetParticipatingInstancesResponse
// @Router /quizzes/instances/participating [get]
func (h *QuizHandler) GetParticipatingInstances(c *gin.Context) {
	userID := c.GetString("user_id")
	sessionStatus := c.Query("session_status")

	resp, err := h.quizClient.GetParticipatingInstances(c.Request.Context(), &pb.GetParticipatingInstancesRequest{
		UserId:        userID,
		SessionStatus: sessionStatus,
	})

	if err != nil {
		dto.JsonError(c, err)
		return
	}

	instances := make([]dto.ParticipatingInstanceDTO, len(resp.Instances))
	for i, pi := range resp.Instances {
		inst := pi.Instance
		instance := instanceProtoToDTO(inst)
		if inst.Deadline != nil {
			instance.Deadline = inst.Deadline.AsTime().Format(time.RFC3339)
		}
		instances[i] = dto.ParticipatingInstanceDTO{
			Instance:      instance,
			SessionStatus: pi.SessionStatus,
		}
	}

	c.JSON(http.StatusOK, dto.GetParticipatingInstancesResponse{
		Instances: instances,
	})
}

func questionInputToProto(q *dto.QuestionInput) *pb.QuestionInput {
	qi := &pb.QuestionInput{
		Text:         q.Text,
		MaxScore:     q.MaxScore,
		TimeLimitSec: q.TimeLimitSec,
	}

	switch q.Type {
	case "single":
		var correctOption int32
		json.Unmarshal(q.CorrectAnswer, &correctOption)
		qi.Answer = &pb.QuestionInput_SingleChoice{
			SingleChoice: &pb.SingleChoice{Options: q.Options, CorrectOption: correctOption},
		}
	case "multiple":
		var correctOptions []int32
		json.Unmarshal(q.CorrectAnswer, &correctOptions)
		qi.Answer = &pb.QuestionInput_MultipleChoice{
			MultipleChoice: &pb.MultipleChoice{Options: q.Options, CorrectOptions: correctOptions},
		}
	case "open":
		var correctText string
		json.Unmarshal(q.CorrectAnswer, &correctText)
		qi.Answer = &pb.QuestionInput_OpenAnswer{
			OpenAnswer: &pb.OpenAnswer{CorrectText: correctText},
		}
	}

	return qi
}

func templateProtoToDTO(t *pb.QuizTemplate, protoQuestions []*pb.Question) dto.TemplateDTO {
	questions := make([]dto.QuestionDTO, len(protoQuestions))
	var totalTime uint64
	for i, q := range protoQuestions {
		questions[i] = questionProtoToDTO(q)
		totalTime += uint64(q.TimeLimitSec)
	}

	return dto.TemplateDTO{
		ID:             t.Id,
		UserID:         t.OwnerId,
		Title:          t.Title,
		QuizType:       t.QuizType,
		Settings:       settingsProtoToDTO(t.QuizType, t.Settings),
		Questions:      questions,
		CreatedAt:      t.CreatedAt.AsTime().Format(time.RFC3339),
		UpdatedAt:      t.UpdatedAt.AsTime().Format(time.RFC3339),
		TotalTime:      totalTime,
		TotalQuestions: uint64(len(protoQuestions)),
	}
}

func parseAsyncSettings(quizType string, raw json.RawMessage) *pb.QuizAsyncSettings {
	if quizType != "async" {
		return nil
	}
	var s dto.QuizAsyncSettings
	if len(raw) > 0 {
		json.Unmarshal(raw, &s)
	}
	return &pb.QuizAsyncSettings{QuestionsRandomOrder: s.QuestionsRandomOrder}
}

func settingsProtoToDTO(quizType string, settings any) any {
	switch quizType {
	case "async":
		switch s := settings.(type) {
		case *pb.QuizTemplate_AsyncSettings:
			return dto.QuizAsyncSettings{QuestionsRandomOrder: s.AsyncSettings.GetQuestionsRandomOrder()}
		case *pb.QuizInstance_AsyncSettings:
			return dto.QuizAsyncSettings{QuestionsRandomOrder: s.AsyncSettings.GetQuestionsRandomOrder()}
		}
	}
	return dto.QuizSyncSettings{}
}

func instanceProtoToDTO(inst *pb.QuizInstance) dto.InstanceDTO {
	return dto.InstanceDTO{
		ID:             inst.Id,
		TemplateID:     inst.TemplateId,
		HostUserID:     inst.CreatedBy,
		Title:          inst.Title,
		AccessCode:     inst.AccessCode,
		GroupID:        inst.GroupId,
		Status:         inst.Status,
		QuizType:       inst.QuizType,
		Settings:       settingsProtoToDTO(inst.QuizType, inst.Settings),
		CreatedAt:      inst.CreatedAt.AsTime().Format(time.RFC3339),
		TotalTime:      inst.TotalTime,
		TotalQuestions: inst.TotalQuestions,
	}
}

func questionProtoToDTO(q *pb.Question) dto.QuestionDTO {
	d := dto.QuestionDTO{
		ID:           q.Id,
		Text:         q.Text,
		MaxScore:     q.MaxScore,
		TimeLimitSec: q.TimeLimitSec,
	}

	switch a := q.Answer.(type) {
	case *pb.Question_SingleChoice:
		d.Type = "single"
		d.Options = a.SingleChoice.Options
		d.CorrectAnswer = a.SingleChoice.CorrectOption
	case *pb.Question_MultipleChoice:
		d.Type = "multiple"
		d.Options = a.MultipleChoice.Options
		d.CorrectAnswer = a.MultipleChoice.CorrectOptions
	case *pb.Question_OpenAnswer:
		d.Type = "open"
		d.CorrectAnswer = a.OpenAnswer.CorrectText
	}

	return d
}
