package handlers

import (
	"net/http"
	"time"

	"api-gateway/internal/client"
	"api-gateway/internal/dto"
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
		dto.JsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	questions := make([]*pb.QuestionInput, len(req.Questions))
	for i, q := range req.Questions {
		questions[i] = &pb.QuestionInput{
			Text:          q.Text,
			Type:          q.Type,
			Options:       q.Options,
			CorrectAnswer: q.CorrectAnswer,
			OrderIndex:    q.OrderIndex,
			MaxScore:      q.MaxScore,
			TimeLimitSec:  q.TimeLimitSec,
		}
	}

	resp, err := h.quizClient.CreateTemplate(c.Request.Context(), &pb.CreateTemplateRequest{
		UserId:      userID,
		Title:       req.Title,
		Description: req.Description,
		QuizType:    req.QuizType,
		Settings: &pb.QuizSettings{
			RandomOrder:        req.Settings.RandomOrder,
			TimeLimitTotal:     req.Settings.TimeLimitTotal,
			ShowCorrectAnswers: req.Settings.ShowCorrectAnswers,
			AllowReview:        req.Settings.AllowReview,
		},
		Questions: questions,
	})

	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, dto.CreateTemplateResponse{
		TemplateID: resp.Template.Id,
		Message:    "Template created successfully",
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
		dto.JsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	templates := make([]dto.TemplateDTO, len(resp.Templates))
	for i, twq := range resp.Templates {
		t := twq.Template

		questions := make([]dto.QuestionDTO, len(twq.Questions))
		for j, q := range twq.Questions {
			questions[j] = dto.QuestionDTO{
				ID:            q.Id,
				Text:          q.Text,
				Type:          q.Type,
				Options:       q.Options,
				CorrectAnswer: q.CorrectAnswer,
				OrderIndex:    q.OrderIndex,
				MaxScore:      q.MaxScore,
				TimeLimitSec:  q.TimeLimitSec,
			}
		}

		templates[i] = dto.TemplateDTO{
			ID:          t.Id,
			UserID:      t.OwnerId,
			Title:       t.Title,
			Description: t.Description,
			QuizType:    t.QuizType,
			Settings: dto.QuizSettings{
				RandomOrder:        t.Settings.RandomOrder,
				TimeLimitTotal:     t.Settings.TimeLimitTotal,
				ShowCorrectAnswers: t.Settings.ShowCorrectAnswers,
				AllowReview:        t.Settings.AllowReview,
			},
			Questions: questions,
			CreatedAt: t.CreatedAt.AsTime().Format(time.RFC3339),
			UpdatedAt: t.UpdatedAt.AsTime().Format(time.RFC3339),
		}
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
		dto.JsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	t := resp.Template
	questions := make([]dto.QuestionDTO, len(resp.Questions))
	for j, q := range resp.Questions {
		questions[j] = dto.QuestionDTO{
			ID:            q.Id,
			Text:          q.Text,
			Type:          q.Type,
			Options:       q.Options,
			CorrectAnswer: q.CorrectAnswer,
			OrderIndex:    q.OrderIndex,
			MaxScore:      q.MaxScore,
			TimeLimitSec:  q.TimeLimitSec,
		}
	}

	template := dto.TemplateDTO{
		ID:          t.Id,
		UserID:      t.OwnerId,
		Title:       t.Title,
		Description: t.Description,
		QuizType:    t.QuizType,
		Settings: dto.QuizSettings{
			RandomOrder:        t.Settings.RandomOrder,
			TimeLimitTotal:     t.Settings.TimeLimitTotal,
			ShowCorrectAnswers: t.Settings.ShowCorrectAnswers,
			AllowReview:        t.Settings.AllowReview,
		},
		Questions: questions,
		CreatedAt: t.CreatedAt.AsTime().Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.AsTime().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, dto.GetTemplateResponse{
		Template: template,
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
		dto.JsonError(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	questions := make([]*pb.QuestionInput, len(req.Questions))
	for i, q := range req.Questions {
		questions[i] = &pb.QuestionInput{
			Id:            q.ID,
			Text:          q.Text,
			Type:          q.Type,
			Options:       q.Options,
			CorrectAnswer: q.CorrectAnswer,
			OrderIndex:    q.OrderIndex,
			MaxScore:      q.MaxScore,
			TimeLimitSec:  q.TimeLimitSec,
		}
	}

	resp, err := h.quizClient.UpdateTemplate(c.Request.Context(), &pb.UpdateTemplateRequest{
		TemplateId:  templateID,
		UserId:      userID,
		Title:       req.Title,
		Description: req.Description,
		QuizType:    req.QuizType,
		Settings: &pb.QuizSettings{
			RandomOrder:        req.Settings.RandomOrder,
			TimeLimitTotal:     req.Settings.TimeLimitTotal,
			ShowCorrectAnswers: req.Settings.ShowCorrectAnswers,
			AllowReview:        req.Settings.AllowReview,
		},
		Questions: questions,
	})

	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, dto.CreateTemplateResponse{
		TemplateID: resp.Template.Id,
		Message:    "Template updated successfully",
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

	resp, err := h.quizClient.DeleteTemplate(c.Request.Context(), &pb.DeleteTemplateRequest{
		TemplateId: templateID,
		UserId:     userID,
	})

	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, dto.DeleteTemplateResponse{
		Success: resp.Success,
		Message: "Template deleted successfully",
	})
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
		dto.JsonError(c, http.StatusBadRequest, "Invalid request body")
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
			dto.JsonError(c, http.StatusBadRequest, "Invalid request body")
			return
		}
		protoReq.Deadline = timestamppb.New(deadline)
	}

	resp, err := h.quizClient.CreateInstance(c.Request.Context(), protoReq)

	if err != nil {
		dto.JsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, dto.CreateInstanceResponse{
		InstanceID: resp.Instance.Id,
		AccessCode: resp.Instance.AccessCode,
		Message:    "Instance created successfully",
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
		dto.JsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	inst := resp.Instance
	instance := dto.InstanceDTO{
		ID:         inst.Id,
		TemplateID: inst.TemplateId,
		HostUserID: inst.CreatedBy,
		Title:      inst.Title,
		AccessCode: inst.AccessCode,
		GroupID:    inst.GroupId,
		Status:     inst.Status,
		QuizType:   inst.QuizType,
		Settings: dto.QuizSettings{
			RandomOrder:        inst.Settings.RandomOrder,
			TimeLimitTotal:     inst.Settings.TimeLimitTotal,
			ShowCorrectAnswers: inst.Settings.ShowCorrectAnswers,
			AllowReview:        inst.Settings.AllowReview,
		},
		CreatedAt: inst.CreatedAt.AsTime().Format(time.RFC3339),
	}

	if inst.Deadline != nil {
		instance.Deadline = inst.Deadline.AsTime().Format(time.RFC3339)
	}

	questions := make([]dto.QuestionDTO, len(resp.Questions))
	for i, q := range resp.Questions {
		questions[i] = dto.QuestionDTO{
			ID:            q.Id,
			Text:          q.Text,
			Type:          q.Type,
			Options:       q.Options,
			CorrectAnswer: q.CorrectAnswer,
			OrderIndex:    q.OrderIndex,
			MaxScore:      q.MaxScore,
			TimeLimitSec:  q.TimeLimitSec,
		}
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
// @Param status query string false "Status filter"
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
		dto.JsonError(c, http.StatusInternalServerError, err.Error())
		return
	}

	instances := make([]dto.InstanceDTO, len(resp.Instances))
	for i, inst := range resp.Instances {
		instances[i] = dto.InstanceDTO{
			ID:         inst.Id,
			TemplateID: inst.TemplateId,
			HostUserID: inst.CreatedBy,
			Title:      inst.Title,
			AccessCode: inst.AccessCode,
			GroupID:    inst.GroupId,
			Status:     inst.Status,
			QuizType:   inst.QuizType,
			Settings: dto.QuizSettings{
				RandomOrder:        inst.Settings.RandomOrder,
				TimeLimitTotal:     inst.Settings.TimeLimitTotal,
				ShowCorrectAnswers: inst.Settings.ShowCorrectAnswers,
				AllowReview:        inst.Settings.AllowReview,
			},
			CreatedAt: inst.CreatedAt.AsTime().Format(time.RFC3339),
		}

		if inst.Deadline != nil {
			instances[i].Deadline = inst.Deadline.AsTime().Format(time.RFC3339)
		}
	}

	c.JSON(http.StatusOK, dto.GetHostingInstancesResponse{
		Instances: instances,
	})
}
