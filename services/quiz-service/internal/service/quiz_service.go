package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"quiz-service/internal/repository"
	pb "quiz-service/proto"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type RabbitMQPublisher interface {
	Publish(ctx context.Context, queueName string, body []byte) error
}

type UserClient interface {
	CheckGroupMembership(ctx context.Context, groupID, userID string) (bool, string, error)
}

type QuizService struct {
	pb.UnimplementedQuizServiceServer
	templateRepo *repository.TemplateRepository
	instanceRepo *repository.InstanceRepository
	mqPublisher  RabbitMQPublisher
	userClient   UserClient
}

func NewQuizService(
	db *sql.DB,
	mqPublisher RabbitMQPublisher,
	userClient UserClient,
) *QuizService {
	return &QuizService{
		templateRepo: repository.NewTemplateRepository(db),
		instanceRepo: repository.NewInstanceRepository(db),
		mqPublisher:  mqPublisher,
		userClient:   userClient,
	}
}

func (s *QuizService) CreateTemplate(ctx context.Context, req *pb.CreateTemplateRequest) (*pb.CreateTemplateResponse, error) {
	settingsJSON, err := json.Marshal(req.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal settings: %w", err)
	}

	template := &repository.Template{
		OwnerID:     req.UserId,
		Title:       req.Title,
		Description: req.Description,
		QuizType:    req.QuizType,
		Settings:    string(settingsJSON),
	}

	if err := s.templateRepo.CreateTemplate(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	var questions []*repository.Question
	for _, q := range req.Questions {
		optionsJSON, err := json.Marshal(q.Options)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal options: %w", err)
		}

		correctAnswerJSON, err := repository.CorrectAnswerToJSON(q.CorrectAnswer)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal correct_answer: %w", err)
		}

		question := &repository.Question{
			TemplateID:    template.ID,
			Text:          q.Text,
			Type:          q.Type,
			Options:       string(optionsJSON),
			CorrectAnswer: correctAnswerJSON,
			OrderIndex:    int(q.OrderIndex),
			MaxScore:      int(q.MaxScore),
			TimeLimitSec:  int(q.TimeLimitSec),
		}

		if err := s.templateRepo.CreateQuestion(ctx, question); err != nil {
			return nil, fmt.Errorf("failed to create question: %w", err)
		}
		questions = append(questions, question)
	}

	s.publishAIAnswerRequest(ctx, template.ID, questions)

	return &pb.CreateTemplateResponse{
		Template:  s.templateToProto(template),
		Questions: s.questionsToProto(questions),
	}, nil
}

func (s *QuizService) GetTemplate(ctx context.Context, req *pb.GetTemplateRequest) (*pb.GetTemplateResponse, error) {
	template, err := s.templateRepo.GetTemplateByID(ctx, req.TemplateId)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	if template.OwnerID != req.UserId {
		return nil, fmt.Errorf("unauthorized: user is not the owner")
	}

	questions, err := s.templateRepo.GetQuestionsByTemplateID(ctx, req.TemplateId)
	if err != nil {
		return nil, fmt.Errorf("failed to get questions: %w", err)
	}

	return &pb.GetTemplateResponse{
		Template:  s.templateToProto(template),
		Questions: s.questionsToProto(questions),
	}, nil
}

func (s *QuizService) GetTemplates(ctx context.Context, req *pb.GetTemplatesRequest) (*pb.GetTemplatesResponse, error) {
	templates, err := s.templateRepo.GetTemplatesByOwner(ctx, req.UserId)
	if err != nil {
		return nil, fmt.Errorf("failed to get templates: %w", err)
	}

	var responses []*pb.TemplateWithQuestions
	for _, t := range templates {
		questions, err := s.templateRepo.GetQuestionsByTemplateID(ctx, t.ID)
		if err != nil {
			log.Printf("Failed to get questions for template %s: %v", t.ID, err)
			questions = []*repository.Question{}
		}

		responses = append(responses, &pb.TemplateWithQuestions{
			Template:  s.templateToProto(t),
			Questions: s.questionsToProto(questions),
		})
	}

	return &pb.GetTemplatesResponse{
		Templates: responses,
	}, nil
}

func (s *QuizService) UpdateTemplate(ctx context.Context, req *pb.UpdateTemplateRequest) (*pb.UpdateTemplateResponse, error) {
	existing, err := s.templateRepo.GetTemplateByID(ctx, req.TemplateId)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	if existing.OwnerID != req.UserId {
		return nil, fmt.Errorf("unauthorized: user is not the owner")
	}

	settingsJSON, err := json.Marshal(req.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal settings: %w", err)
	}

	template := &repository.Template{
		ID:          req.TemplateId,
		OwnerID:     req.UserId,
		Title:       req.Title,
		Description: req.Description,
		Settings:    string(settingsJSON),
	}

	if err := s.templateRepo.UpdateTemplate(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	existingQuestions, err := s.templateRepo.GetQuestionsByTemplateID(ctx, req.TemplateId)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing questions: %w", err)
	}

	existingQuestionsMap := make(map[string]*repository.Question)
	for _, q := range existingQuestions {
		existingQuestionsMap[q.ID] = q
	}

	if err := s.templateRepo.DeleteQuestionsByTemplateID(ctx, req.TemplateId); err != nil {
		return nil, fmt.Errorf("failed to unlink old questions: %w", err)
	}

	var questions []*repository.Question
	for _, q := range req.Questions {
		optionsJSON, err := json.Marshal(q.Options)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal options: %w", err)
		}

		correctAnswerJSON, err := repository.CorrectAnswerToJSON(q.CorrectAnswer)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal correct_answer: %w", err)
		}

		var questionIDToLink string
		var question *repository.Question

		if q.Id != "" {
			if existingQ, ok := existingQuestionsMap[q.Id]; ok {
				if existingQ.Text == q.Text &&
					existingQ.Type == q.Type &&
					existingQ.Options == string(optionsJSON) &&
					existingQ.CorrectAnswer == correctAnswerJSON &&
					existingQ.MaxScore == int(q.MaxScore) &&
					existingQ.TimeLimitSec == int(q.TimeLimitSec) {

					questionIDToLink = existingQ.ID
					question = existingQ
					question.OrderIndex = int(q.OrderIndex)
				}
			}
		}

		if questionIDToLink != "" {
			if err := s.templateRepo.LinkQuestionToTemplate(ctx, req.TemplateId, questionIDToLink, int(q.OrderIndex)); err != nil {
				return nil, fmt.Errorf("failed to link existing question: %w", err)
			}
		} else {
			question = &repository.Question{
				TemplateID:    req.TemplateId,
				Text:          q.Text,
				Type:          q.Type,
				Options:       string(optionsJSON),
				CorrectAnswer: correctAnswerJSON,
				OrderIndex:    int(q.OrderIndex),
				MaxScore:      int(q.MaxScore),
				TimeLimitSec:  int(q.TimeLimitSec),
			}
			if err := s.templateRepo.CreateQuestion(ctx, question); err != nil {
				return nil, fmt.Errorf("failed to create question: %w", err)
			}
		}
		questions = append(questions, question)
	}

	s.publishAIAnswerRequest(ctx, req.TemplateId, questions)

	updatedTemplate, err := s.templateRepo.GetTemplateByID(ctx, req.TemplateId)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated template: %w", err)
	}

	return &pb.UpdateTemplateResponse{
		Template:  s.templateToProto(updatedTemplate),
		Questions: s.questionsToProto(questions),
	}, nil
}

func (s *QuizService) DeleteTemplate(ctx context.Context, req *pb.DeleteTemplateRequest) (*pb.DeleteTemplateResponse, error) {
	if err := s.templateRepo.DeleteQuestionsByTemplateID(ctx, req.TemplateId); err != nil {
		return nil, fmt.Errorf("failed to delete questions: %w", err)
	}

	if err := s.templateRepo.DeleteTemplate(ctx, req.TemplateId, req.UserId); err != nil {
		return nil, fmt.Errorf("failed to delete template: %w", err)
	}

	return &pb.DeleteTemplateResponse{
		Success: true,
	}, nil
}

func (s *QuizService) CreateInstance(ctx context.Context, req *pb.CreateInstanceRequest) (*pb.CreateInstanceResponse, error) {
	template, err := s.templateRepo.GetTemplateByID(ctx, req.TemplateId)
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	if template.OwnerID != req.UserId {
		return nil, fmt.Errorf("unauthorized: user is not the template owner")
	}

	instance := &repository.Instance{
		TemplateID: sql.NullString{String: req.TemplateId, Valid: true},
		Title:      req.Title,
		CreatedBy:  req.UserId,
		QuizType:   template.QuizType,
		Settings:   template.Settings,
	}

	if req.GroupId != "" {
		instance.GroupID = sql.NullString{String: req.GroupId, Valid: true}
	}

	if req.Deadline != nil {
		instance.Deadline = sql.NullTime{Time: req.Deadline.AsTime(), Valid: true}
	}

	if err := s.instanceRepo.CreateInstance(ctx, instance); err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	s.publishQuizCreated(ctx, instance)

	return &pb.CreateInstanceResponse{
		Instance: s.instanceToProto(instance),
	}, nil
}

func (s *QuizService) GetInstance(ctx context.Context, req *pb.GetInstanceRequest) (*pb.GetInstanceResponse, error) {
	instanceWithQuestions, err := s.instanceRepo.GetInstanceWithQuestions(ctx, req.InstanceId)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	resp := &pb.GetInstanceResponse{
		Instance:  s.instanceToProto(instanceWithQuestions.Instance),
		Questions: s.questionsToProto(instanceWithQuestions.Questions),
	}

	return resp, nil
}

func (s *QuizService) GetHostingInstances(ctx context.Context, req *pb.GetHostingInstancesRequest) (*pb.GetHostingInstancesResponse, error) {
	instances, err := s.instanceRepo.GetHostingInstances(ctx, req.UserId, req.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to get hosting instances: %w", err)
	}

	var protoInstances []*pb.QuizInstance
	for _, inst := range instances {
		protoInstances = append(protoInstances, s.instanceToProto(inst))
	}

	return &pb.GetHostingInstancesResponse{
		Instances: protoInstances,
	}, nil
}

func (s *QuizService) templateToProto(t *repository.Template) *pb.QuizTemplate {
	var settings pb.QuizSettings
	json.Unmarshal([]byte(t.Settings), &settings)

	return &pb.QuizTemplate{
		Id:          t.ID,
		OwnerId:     t.OwnerID,
		Title:       t.Title,
		Description: t.Description,
		QuizType:    t.QuizType,
		Settings:    &settings,
		CreatedAt:   timestamppb.New(t.CreatedAt),
		UpdatedAt:   timestamppb.New(t.UpdatedAt),
	}
}

func (s *QuizService) questionsToProto(questions []*repository.Question) []*pb.Question {
	var protoQuestions []*pb.Question
	for _, q := range questions {
		var options []string
		json.Unmarshal([]byte(q.Options), &options)

		protoQuestion := &pb.Question{
			Id:            q.ID,
			TemplateId:    q.TemplateID,
			Text:          q.Text,
			Type:          q.Type,
			Options:       options,
			CorrectAnswer: q.CorrectAnswer,
			OrderIndex:    int32(q.OrderIndex),
			MaxScore:      int32(q.MaxScore),
			TimeLimitSec:  int32(q.TimeLimitSec),
		}
		if q.AIAnswer.Valid {
			protoQuestion.AiAnswer = q.AIAnswer.String
		}
		protoQuestions = append(protoQuestions, protoQuestion)
	}
	return protoQuestions
}

func (s *QuizService) instanceToProto(i *repository.Instance) *pb.QuizInstance {
	var settings pb.QuizSettings
	json.Unmarshal([]byte(i.Settings), &settings)

	instance := &pb.QuizInstance{
		Id:         i.ID,
		Title:      i.Title,
		AccessCode: i.AccessCode,
		Status:     i.Status,
		CreatedBy:  i.CreatedBy,
		CreatedAt:  timestamppb.New(i.CreatedAt),
		QuizType:   i.QuizType,
		Settings:   &settings,
	}

	if i.TemplateID.Valid {
		instance.TemplateId = i.TemplateID.String
	}

	if i.GroupID.Valid {
		instance.GroupId = i.GroupID.String
	}

	if i.StartTime.Valid {
		instance.StartTime = timestamppb.New(i.StartTime.Time)
	}

	if i.Deadline.Valid {
		instance.Deadline = timestamppb.New(i.Deadline.Time)
	}

	return instance
}

func (s *QuizService) publishAIAnswerRequest(ctx context.Context, templateID string, questions []*repository.Question) {
	if s.mqPublisher == nil {
		return
	}

	type QuestionData struct {
		QuestionID string `json:"question_id"`
		Text       string `json:"text"`
		Type       string `json:"type"`
	}

	type AIAnswerRequestEvent struct {
		TemplateID string         `json:"template_id"`
		Questions  []QuestionData `json:"questions"`
		Models     []string       `json:"models"`
	}

	var questionData []QuestionData
	for _, q := range questions {
		questionData = append(questionData, QuestionData{
			QuestionID: q.ID,
			Text:       q.Text,
			Type:       q.Type,
		})
	}

	event := AIAnswerRequestEvent{
		TemplateID: templateID,
		Questions:  questionData,
		Models:     []string{},
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal ai_answer_request event: %v", err)
		return
	}

	if err := s.mqPublisher.Publish(ctx, "ml.ai_answer_requests", eventJSON); err != nil {
		log.Printf("Failed to publish ai_answer_request event: %v", err)
	}
}

func (s *QuizService) publishQuizCreated(ctx context.Context, instance *repository.Instance) {
	if s.mqPublisher == nil {
		return
	}

	type QuizCreatedEvent struct {
		InstanceID string `json:"instance_id"`
		Title      string `json:"title"`
		GroupID    string `json:"group_id,omitempty"`
		CreatorID  string `json:"creator_id"`
		Deadline   string `json:"deadline,omitempty"`
	}

	event := QuizCreatedEvent{
		InstanceID: instance.ID,
		Title:      instance.Title,
		CreatorID:  instance.CreatedBy,
	}

	if instance.GroupID.Valid {
		event.GroupID = instance.GroupID.String
	}

	if instance.Deadline.Valid {
		event.Deadline = instance.Deadline.Time.Format(time.RFC3339)
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal quiz_created event: %v", err)
		return
	}

	if err := s.mqPublisher.Publish(ctx, "quiz.created", eventJSON); err != nil {
		log.Printf("Failed to publish quiz_created event: %v", err)
	}
}
