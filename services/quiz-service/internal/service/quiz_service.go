package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"quiz-service/internal/constants"
	"quiz-service/internal/model"
	"quiz-service/internal/repository"
	"quiz-service/pkg/database"
	"quiz-service/pkg/errors"
	"quiz-service/pkg/lang"
	pb "quiz-service/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RabbitMQPublisher interface {
	Publish(ctx context.Context, queueName string, body []byte) error
}

type UserClient interface {
	CheckGroupMembership(ctx context.Context, groupID, userID string) (bool, string, error)
	GetEmailsByIDs(ctx context.Context, userIDs []string) (map[string]string, error)
	GetUsersByIDs(ctx context.Context, userIDs []string) (map[string]*model.UserInfo, error)
	GetGroupMemberIDs(ctx context.Context, groupID string) ([]string, error)
	GetNotificationSettingsBatch(ctx context.Context, userIDs []string) (map[string]model.NotificationSettings, error)
}

type TemplateRepo interface {
	CreateTemplate(ctx context.Context, template *repository.Template) error
	GetTemplateByID(ctx context.Context, templateID string) (*repository.Template, error)
	GetTemplatesByOwner(ctx context.Context, ownerID string) ([]*repository.Template, error)
	UpdateTemplate(ctx context.Context, template *repository.Template) error
	DeleteTemplate(ctx context.Context, templateID, ownerID string) error
	CreateQuestion(ctx context.Context, question *repository.Question) error
	GetQuestionsByTemplateID(ctx context.Context, templateID string) ([]*repository.Question, error)
	DeleteQuestionsByTemplateID(ctx context.Context, templateID string) error
}

type InstanceRepo interface {
	CreateInstance(ctx context.Context, instance *repository.Instance) error
	GetInstanceWithQuestions(ctx context.Context, instanceID string) (*repository.InstanceWithQuestions, error)
	GetInstanceByAccessCode(ctx context.Context, accessCode string) (*repository.Instance, error)
	GetInstanceByID(ctx context.Context, instanceID string) (*repository.Instance, error)
	GetHostingInstances(ctx context.Context, userID, status string) ([]*repository.Instance, error)
	GetParticipatingInstances(ctx context.Context, userID, sessionStatus string) ([]*repository.ParticipatingInstance, error)
	GetInstanceParticipants(ctx context.Context, instanceID, excludeUserID string) ([]*repository.ParticipantSession, error)
	GetParticipantAnswers(ctx context.Context, instanceID, userID string) (*repository.ParticipantSession, error)
	GradeAnswer(ctx context.Context, instanceID, userID, questionID string, score int) (int, error)
	UpdateInstanceStatus(ctx context.Context, instanceID, status string) error
	DeleteInstance(ctx context.Context, instanceID, createdBy string) error
}

type DeleteRepo interface {
	DeleteAllByOwner(ctx context.Context, userID string) error
}

type QuizService struct {
	pb.UnimplementedQuizServiceServer
	templateRepo TemplateRepo
	instanceRepo InstanceRepo
	deleteRepo   DeleteRepo
	txMgr        database.TxManager
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
		deleteRepo:   repository.NewDeleteRepository(db),
		txMgr:        database.NewManager(db),
		mqPublisher:  mqPublisher,
		userClient:   userClient,
	}
}

func NewQuizServiceWithDeps(
	templateRepo TemplateRepo,
	instanceRepo InstanceRepo,
	deleteRepo DeleteRepo,
	txMgr database.TxManager,
	mqPublisher RabbitMQPublisher,
	userClient UserClient,
) *QuizService {
	return &QuizService{
		templateRepo: templateRepo,
		instanceRepo: instanceRepo,
		deleteRepo:   deleteRepo,
		txMgr:        txMgr,
		mqPublisher:  mqPublisher,
		userClient:   userClient,
	}
}

func (s *QuizService) DeleteAllByOwner(ctx context.Context, req *pb.DeleteAllByOwnerRequest) (*pb.DeleteAllByOwnerResponse, error) {
	if req.UserId == "" {
		return nil, errors.New(codes.InvalidArgument, errors.ReasonInvalidArgument, "User ID is required", nil)
	}

	if err := s.deleteRepo.DeleteAllByOwner(ctx, req.UserId); err != nil {
		log.Printf("DeleteAllByOwner: failed for user %s: %v", req.UserId, err)
		return nil, errors.New(codes.Internal, errors.ReasonDeleteAllByOwnerFailed, "Failed to delete quiz data", map[string]string{"user_id": req.UserId})
	}

	return &pb.DeleteAllByOwnerResponse{}, nil
}

func (s *QuizService) CreateTemplate(ctx context.Context, req *pb.CreateTemplateRequest) (*pb.CreateTemplateResponse, error) {
	settingsJSON, err := s.marshalSettings(req.Settings)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonSettingsMarshalFailed, "Failed to marshal settings", nil)
	}

	template := &repository.Template{
		OwnerID:  req.UserId,
		Title:    req.Title,
		QuizType: req.QuizType,
		Settings: settingsJSON,
	}

	var questions []*repository.Question
	err = s.txMgr.InTransaction(ctx, func(ctx context.Context) error {
		if err := s.templateRepo.CreateTemplate(ctx, template); err != nil {
			return fmt.Errorf("create template: %w", err)
		}
		for i, q := range req.Questions {
			questionType, correctAnswerJSON, err := s.questionInputToDB(q)
			if err != nil {
				return fmt.Errorf("marshal answer for question %d: %w", i, err)
			}
			question := &repository.Question{
				TemplateID:    template.ID,
				Text:          q.Text,
				Type:          questionType,
				CorrectAnswer: correctAnswerJSON,
				OrderIndex:    i,
				MaxScore:      int(q.MaxScore),
				TimeLimitSec:  int(q.TimeLimitSec),
			}
			if err := s.templateRepo.CreateQuestion(ctx, question); err != nil {
				return fmt.Errorf("create question %d: %w", i, err)
			}
			questions = append(questions, question)
		}
		return nil
	})
	if err != nil {
		log.Printf("CreateTemplate tx failed for user %s: %v", req.UserId, err)
		return nil, errors.New(codes.Internal, errors.ReasonTemplateCreateFailed, "Failed to create template", map[string]string{"user_id": req.UserId})
	}

	return &pb.CreateTemplateResponse{
		Template:  s.templateToProto(template),
		Questions: s.questionsToProto(questions),
	}, nil
}

func (s *QuizService) GetTemplate(ctx context.Context, req *pb.GetTemplateRequest) (*pb.GetTemplateResponse, error) {
	template, err := s.templateRepo.GetTemplateByID(ctx, req.TemplateId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonTemplateNotFound, "Template not found", map[string]string{"template_id": req.TemplateId})
	}

	if template.OwnerID != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "User is not the template owner", map[string]string{"template_id": req.TemplateId, "user_id": req.UserId, "owner_id": template.OwnerID})
	}

	questions, err := s.templateRepo.GetQuestionsByTemplateID(ctx, req.TemplateId)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonTemplateNotFound, "Failed to get questions", map[string]string{"template_id": req.TemplateId})
	}

	return &pb.GetTemplateResponse{
		Template:  s.templateToProto(template),
		Questions: s.questionsToProto(questions),
	}, nil
}

func (s *QuizService) GetTemplates(ctx context.Context, req *pb.GetTemplatesRequest) (*pb.GetTemplatesResponse, error) {
	templates, err := s.templateRepo.GetTemplatesByOwner(ctx, req.UserId)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonTemplateNotFound, "Failed to get templates", map[string]string{"user_id": req.UserId})
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
		return nil, errors.New(codes.NotFound, errors.ReasonTemplateNotFound, "Template not found", map[string]string{"template_id": req.TemplateId})
	}

	if existing.OwnerID != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "User is not the template owner", map[string]string{"template_id": req.TemplateId, "user_id": req.UserId, "owner_id": existing.OwnerID})
	}

	settingsJSON, err := s.marshalSettings(req.Settings)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonSettingsMarshalFailed, "Failed to marshal settings", map[string]string{"template_id": req.TemplateId})
	}

	template := &repository.Template{
		ID:       req.TemplateId,
		OwnerID:  req.UserId,
		Title:    req.Title,
		QuizType: req.QuizType,
		Settings: settingsJSON,
	}

	var questions []*repository.Question
	err = s.txMgr.InTransaction(ctx, func(ctx context.Context) error {
		if err := s.templateRepo.UpdateTemplate(ctx, template); err != nil {
			return fmt.Errorf("update template: %w", err)
		}
		if err := s.templateRepo.DeleteQuestionsByTemplateID(ctx, req.TemplateId); err != nil {
			return fmt.Errorf("delete old questions: %w", err)
		}
		for i, q := range req.Questions {
			questionType, correctAnswerJSON, err := s.questionInputToDB(q)
			if err != nil {
				return fmt.Errorf("marshal answer for question %d: %w", i, err)
			}
			question := &repository.Question{
				TemplateID:    req.TemplateId,
				Text:          q.Text,
				Type:          questionType,
				CorrectAnswer: correctAnswerJSON,
				OrderIndex:    i,
				MaxScore:      int(q.MaxScore),
				TimeLimitSec:  int(q.TimeLimitSec),
			}
			if err := s.templateRepo.CreateQuestion(ctx, question); err != nil {
				return fmt.Errorf("create question %d: %w", i, err)
			}
			questions = append(questions, question)
		}
		return nil
	})
	if err != nil {
		log.Printf("UpdateTemplate tx failed: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonTemplateUpdateFailed, "Failed to update template", map[string]string{"template_id": req.TemplateId})
	}

	updatedTemplate, err := s.templateRepo.GetTemplateByID(ctx, req.TemplateId)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonTemplateNotFound, "Failed to get updated template", map[string]string{"template_id": req.TemplateId})
	}

	return &pb.UpdateTemplateResponse{
		Template:  s.templateToProto(updatedTemplate),
		Questions: s.questionsToProto(questions),
	}, nil
}

func (s *QuizService) DeleteTemplate(ctx context.Context, req *pb.DeleteTemplateRequest) (*pb.DeleteTemplateResponse, error) {
	err := s.txMgr.InTransaction(ctx, func(ctx context.Context) error {
		if err := s.templateRepo.DeleteQuestionsByTemplateID(ctx, req.TemplateId); err != nil {
			return fmt.Errorf("delete questions: %w", err)
		}
		if err := s.templateRepo.DeleteTemplate(ctx, req.TemplateId, req.UserId); err != nil {
			return fmt.Errorf("delete template: %w", err)
		}
		return nil
	})
	if err != nil {
		log.Printf("DeleteTemplate tx failed: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonTemplateDeleteFailed, "Failed to delete template", map[string]string{"template_id": req.TemplateId})
	}
	return &pb.DeleteTemplateResponse{}, nil
}

func (s *QuizService) CreateInstance(ctx context.Context, req *pb.CreateInstanceRequest) (*pb.CreateInstanceResponse, error) {
	template, err := s.templateRepo.GetTemplateByID(ctx, req.TemplateId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonTemplateNotFound, "Template not found", map[string]string{"template_id": req.TemplateId})
	}

	if template.OwnerID != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "User is not the template owner", map[string]string{"template_id": req.TemplateId, "user_id": req.UserId, "owner_id": template.OwnerID})
	}

	var totalTime uint64 = 0
	questions, err := s.templateRepo.GetQuestionsByTemplateID(ctx, req.TemplateId)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonTemplateNotFound, "Failed to get template questions", map[string]string{"template_id": req.TemplateId})
	}
	for _, question := range questions {
		totalTime += uint64(question.TimeLimitSec)
	}

	instance := &repository.Instance{
		TemplateID:     sql.NullString{String: req.TemplateId, Valid: true},
		Title:          req.Title,
		CreatedBy:      req.UserId,
		QuizType:       template.QuizType,
		Settings:       template.Settings,
		TotalTime:      uint64(totalTime),
		TotalQuestions: uint64(len(questions)),
	}

	if req.GroupId != "" {
		instance.GroupID = sql.NullString{String: req.GroupId, Valid: true}
	}

	if req.Deadline != nil {
		if template.QuizType != constants.QuizTypeAsync {
			return nil, errors.New(codes.InvalidArgument, errors.ReasonInvalidArgument,
				"Deadline can only be set for async quizzes",
				map[string]string{"quiz_type": template.QuizType})
		}
		instance.Deadline = sql.NullTime{Time: req.Deadline.AsTime(), Valid: true}
	}

	if err := s.instanceRepo.CreateInstance(ctx, instance); err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonInstanceCreateFailed, "Failed to create instance", map[string]string{"template_id": req.TemplateId})
	}

	s.publishQuizCreated(ctx, instance)

	return &pb.CreateInstanceResponse{
		Instance: s.instanceToProto(instance),
	}, nil
}

func (s *QuizService) DeleteInstance(ctx context.Context, req *pb.DeleteInstanceRequest) (*pb.DeleteInstanceResponse, error) {
	instance, err := s.instanceRepo.GetInstanceByID(ctx, req.InstanceId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonInstanceNotFound, "Instance not found", map[string]string{"instance_id": req.InstanceId})
	}

	if instance.CreatedBy != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "Only the quiz creator can delete the instance", map[string]string{"instance_id": req.InstanceId, "user_id": req.UserId})
	}

	if err := s.instanceRepo.DeleteInstance(ctx, req.InstanceId, req.UserId); err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonInstanceDeleteFailed, "Failed to delete instance", map[string]string{"instance_id": req.InstanceId})
	}

	return &pb.DeleteInstanceResponse{}, nil
}

func (s *QuizService) GetInstance(ctx context.Context, req *pb.GetInstanceRequest) (*pb.GetInstanceResponse, error) {
	instanceWithQuestions, err := s.instanceRepo.GetInstanceWithQuestions(ctx, req.InstanceId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonInstanceNotFound, "Instance not found", map[string]string{"instance_id": req.InstanceId})
	}

	resp := &pb.GetInstanceResponse{
		Instance:  s.instanceToProto(instanceWithQuestions.Instance),
		Questions: s.questionsToProto(instanceWithQuestions.Questions),
	}

	return resp, nil
}

func (s *QuizService) GetInstanceByAccessCode(ctx context.Context, req *pb.GetInstanceByAccessCodeRequest) (*pb.GetInstanceByAccessCodeResponse, error) {
	instance, err := s.instanceRepo.GetInstanceByAccessCode(ctx, req.AccessCode)
	if err != nil {
		return &pb.GetInstanceByAccessCodeResponse{
			Instance:  nil,
			HasAccess: false,
		}, nil
	}

	hasAccess := instance.CreatedBy == req.UserId
	if !hasAccess && instance.GroupID.Valid {
		isMember, _, err := s.userClient.CheckGroupMembership(ctx, instance.GroupID.String, req.UserId)
		if err == nil && isMember {
			hasAccess = true
		}
	}

	if !instance.GroupID.Valid {
		hasAccess = true
	}

	return &pb.GetInstanceByAccessCodeResponse{
		Instance:  s.instanceToProto(instance),
		HasAccess: hasAccess,
	}, nil
}

func (s *QuizService) GetHostingInstances(ctx context.Context, req *pb.GetHostingInstancesRequest) (*pb.GetHostingInstancesResponse, error) {
	instances, err := s.instanceRepo.GetHostingInstances(ctx, req.UserId, req.Status)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonInstanceNotFound, "Failed to get hosting instances", map[string]string{"user_id": req.UserId})
	}

	var protoInstances []*pb.QuizInstance
	for _, inst := range instances {
		protoInstances = append(protoInstances, s.instanceToProto(inst))
	}

	return &pb.GetHostingInstancesResponse{
		Instances: protoInstances,
	}, nil
}

func (s *QuizService) GetParticipatingInstances(ctx context.Context, req *pb.GetParticipatingInstancesRequest) (*pb.GetParticipatingInstancesResponse, error) {
	instances, err := s.instanceRepo.GetParticipatingInstances(ctx, req.UserId, req.SessionStatus)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonInstanceNotFound, "Failed to get participating instances", map[string]string{"user_id": req.UserId})
	}

	var protoInstances []*pb.ParticipatingInstance
	for _, inst := range instances {
		protoInstances = append(protoInstances, &pb.ParticipatingInstance{
			Instance:      s.instanceToProto(inst.Instance),
			SessionStatus: inst.SessionStatus,
		})
	}

	return &pb.GetParticipatingInstancesResponse{
		Instances: protoInstances,
	}, nil
}

func (s *QuizService) templateToProto(t *repository.Template) *pb.QuizTemplate {
	tmpl := &pb.QuizTemplate{
		Id:        t.ID,
		OwnerId:   t.OwnerID,
		Title:     t.Title,
		QuizType:  t.QuizType,
		CreatedAt: timestamppb.New(t.CreatedAt),
		UpdatedAt: timestamppb.New(t.UpdatedAt),
	}

	switch t.QuizType {
	case "sync":
		tmpl.Settings = &pb.QuizTemplate_SyncSettings{SyncSettings: &pb.QuizSyncSettings{}}
	case "async":
		var asyncSettings pb.QuizAsyncSettings
		json.Unmarshal([]byte(t.Settings), &asyncSettings)
		tmpl.Settings = &pb.QuizTemplate_AsyncSettings{AsyncSettings: &asyncSettings}
	}

	return tmpl
}

func (s *QuizService) questionsToProto(questions []*repository.Question) []*pb.Question {
	var protoQuestions []*pb.Question
	for _, q := range questions {
		protoQuestion := &pb.Question{
			Id:           q.ID,
			TemplateId:   q.TemplateID,
			Text:         q.Text,
			OrderIndex:   int32(q.OrderIndex),
			MaxScore:     int32(q.MaxScore),
			TimeLimitSec: int32(q.TimeLimitSec),
		}

		switch q.Type {
		case "single":
			var data repository.SingleChoiceData
			json.Unmarshal([]byte(q.CorrectAnswer), &data)
			protoQuestion.Answer = &pb.Question_SingleChoice{
				SingleChoice: &pb.SingleChoice{Options: data.Options, CorrectOption: data.CorrectOption},
			}
		case "multiple":
			var data repository.MultipleChoiceData
			json.Unmarshal([]byte(q.CorrectAnswer), &data)
			protoQuestion.Answer = &pb.Question_MultipleChoice{
				MultipleChoice: &pb.MultipleChoice{Options: data.Options, CorrectOptions: data.CorrectOptions},
			}
		case "open":
			var data repository.OpenAnswerData
			json.Unmarshal([]byte(q.CorrectAnswer), &data)
			protoQuestion.Answer = &pb.Question_OpenAnswer{
				OpenAnswer: &pb.OpenAnswer{CorrectText: data.CorrectText},
			}
		}

		protoQuestions = append(protoQuestions, protoQuestion)
	}
	return protoQuestions
}

func (s *QuizService) questionInputToDB(q *pb.QuestionInput) (questionType string, correctAnswerJSON string, err error) {
	switch a := q.Answer.(type) {
	case *pb.QuestionInput_SingleChoice:
		questionType = "single"
		correctAnswerJSON, err = repository.MarshalAnswerJSON(repository.SingleChoiceData{
			Options:       a.SingleChoice.Options,
			CorrectOption: a.SingleChoice.CorrectOption,
		})
	case *pb.QuestionInput_MultipleChoice:
		questionType = "multiple"
		correctAnswerJSON, err = repository.MarshalAnswerJSON(repository.MultipleChoiceData{
			Options:        a.MultipleChoice.Options,
			CorrectOptions: a.MultipleChoice.CorrectOptions,
		})
	case *pb.QuestionInput_OpenAnswer:
		questionType = "open"
		correctAnswerJSON, err = repository.MarshalAnswerJSON(repository.OpenAnswerData{
			CorrectText: a.OpenAnswer.CorrectText,
		})
	}
	return
}

func (s *QuizService) instanceToProto(i *repository.Instance) *pb.QuizInstance {
	instance := &pb.QuizInstance{
		Id:             i.ID,
		Title:          i.Title,
		AccessCode:     i.AccessCode,
		Status:         i.Status,
		CreatedBy:      i.CreatedBy,
		CreatedAt:      timestamppb.New(i.CreatedAt),
		QuizType:       i.QuizType,
		TotalTime:      i.TotalTime,
		TotalQuestions: i.TotalQuestions,
	}

	switch i.QuizType {
	case "sync":
		instance.Settings = &pb.QuizInstance_SyncSettings{SyncSettings: &pb.QuizSyncSettings{}}
	case "async":
		var asyncSettings pb.QuizAsyncSettings
		json.Unmarshal([]byte(i.Settings), &asyncSettings)
		instance.Settings = &pb.QuizInstance_AsyncSettings{AsyncSettings: &asyncSettings}
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

func (s *QuizService) marshalSettings(settings any) (string, error) {
	switch st := settings.(type) {
	case *pb.CreateTemplateRequest_AsyncSettings:
		b, err := json.Marshal(st.AsyncSettings)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case *pb.UpdateTemplateRequest_AsyncSettings:
		b, err := json.Marshal(st.AsyncSettings)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "{}", nil
	}
}

func (s *QuizService) GetInstanceParticipants(ctx context.Context, req *pb.GetInstanceParticipantsRequest) (*pb.GetInstanceParticipantsResponse, error) {
	instance, err := s.instanceRepo.GetInstanceByID(ctx, req.InstanceId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonInstanceNotFound, "Instance not found", map[string]string{"instance_id": req.InstanceId})
	}

	if instance.CreatedBy != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "Only the quiz creator can view participants", map[string]string{"instance_id": req.InstanceId})
	}

	sessions, err := s.instanceRepo.GetInstanceParticipants(ctx, req.InstanceId, instance.CreatedBy)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonInstanceNotFound, "Failed to get participants", map[string]string{"instance_id": req.InstanceId})
	}

	instanceWithQuestions, err := s.instanceRepo.GetInstanceWithQuestions(ctx, req.InstanceId)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonInstanceNotFound, "Failed to get instance questions", map[string]string{"instance_id": req.InstanceId})
	}

	openQuestionIDs := make(map[string]bool)
	var maxPossibleScore int32
	for _, q := range instanceWithQuestions.Questions {
		maxPossibleScore += int32(q.MaxScore)
		if q.Type == "open" {
			openQuestionIDs[q.ID] = true
		}
	}

	var participants []*pb.ParticipantInfo
	for _, session := range sessions {
		var answers []repository.SessionAnswer
		json.Unmarshal([]byte(session.Answers), &answers)

		var totalScore int32
		for _, a := range answers {
			totalScore += int32(a.Score)
		}

		reviewStatus := "reviewed"
		if session.Status != "finished" {
			reviewStatus = "not_finished"
		} else if len(openQuestionIDs) > 0 {
			for _, a := range answers {
				if openQuestionIDs[a.QuestionID] && !a.IsReviewed {
					reviewStatus = "pending_review"
					break
				}
			}
		}

		participants = append(participants, &pb.ParticipantInfo{
			UserId:           session.UserID,
			SessionStatus:    session.Status,
			ReviewStatus:     reviewStatus,
			TotalScore:       totalScore,
			MaxPossibleScore: maxPossibleScore,
		})
	}

	return &pb.GetInstanceParticipantsResponse{
		Participants: participants,
	}, nil
}

func (s *QuizService) GetParticipantAnswers(ctx context.Context, req *pb.GetParticipantAnswersRequest) (*pb.GetParticipantAnswersResponse, error) {
	instanceWithQuestions, err := s.instanceRepo.GetInstanceWithQuestions(ctx, req.InstanceId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonInstanceNotFound, "Instance not found", map[string]string{"instance_id": req.InstanceId})
	}

	if instanceWithQuestions.Instance.CreatedBy != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "Only the quiz creator can view answers", map[string]string{"instance_id": req.InstanceId})
	}

	session, err := s.instanceRepo.GetParticipantAnswers(ctx, req.InstanceId, req.ParticipantId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonParticipantNotFound, "Participant not found", map[string]string{"instance_id": req.InstanceId, "participant_id": req.ParticipantId})
	}

	if session.Status != constants.SessionStatusFinished {
		return nil, errors.New(codes.NotFound, errors.ReasonSessionNotFinished, "Participant has not finished the quiz", map[string]string{"instance_id": req.InstanceId, "participant_id": req.ParticipantId, "status": session.Status})
	}

	var answers []repository.SessionAnswer
	json.Unmarshal([]byte(session.Answers), &answers)

	answersByID := make(map[string]repository.SessionAnswer, len(answers))
	for _, a := range answers {
		answersByID[a.QuestionID] = a
	}

	answerInfos := make([]*pb.AnswerInfo, 0, len(instanceWithQuestions.Questions))
	for _, q := range instanceWithQuestions.Questions {
		if a, ok := answersByID[q.ID]; ok {
			answerInfos = append(answerInfos, &pb.AnswerInfo{
				QuestionId:  a.QuestionID,
				Answer:      a.Answer,
				IsCorrect:   a.IsCorrect,
				Score:       int32(a.Score),
				TimeSpentMs: a.TimeSpentMs,
				IsReviewed:  a.IsReviewed,
			})
			continue
		}
		answerInfos = append(answerInfos, &pb.AnswerInfo{
			QuestionId:    q.ID,
			IsReviewed:    true,
			IsTimeExpired: true,
		})
	}

	return &pb.GetParticipantAnswersResponse{
		Instance:  s.instanceToProto(instanceWithQuestions.Instance),
		Questions: s.questionsToProto(instanceWithQuestions.Questions),
		Answers:   answerInfos,
	}, nil
}

func (s *QuizService) GradeAnswer(ctx context.Context, req *pb.GradeAnswerRequest) (*pb.GradeAnswerResponse, error) {
	instance, err := s.instanceRepo.GetInstanceByID(ctx, req.InstanceId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonInstanceNotFound, "Instance not found", map[string]string{"instance_id": req.InstanceId})
	}

	if instance.CreatedBy != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "Only the quiz creator can grade answers", map[string]string{"instance_id": req.InstanceId})
	}

	instanceWithQuestions, err := s.instanceRepo.GetInstanceWithQuestions(ctx, req.InstanceId)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonInstanceNotFound, "Failed to get instance questions", map[string]string{"instance_id": req.InstanceId})
	}

	var maxScore int32
	questionFound := false
	for _, q := range instanceWithQuestions.Questions {
		if q.ID == req.QuestionId {
			maxScore = int32(q.MaxScore)
			questionFound = true
			break
		}
	}
	if !questionFound {
		return nil, errors.New(codes.NotFound, errors.ReasonQuestionNotFound, "Question not found in instance", map[string]string{"question_id": req.QuestionId})
	}

	if req.Score < 0 || req.Score > maxScore {
		return nil, errors.New(codes.InvalidArgument, errors.ReasonInvalidScore, "Score must be between 0 and max_score", map[string]string{"max_score": fmt.Sprintf("%d", maxScore)})
	}

	oldScore, err := s.instanceRepo.GradeAnswer(ctx, req.InstanceId, req.ParticipantId, req.QuestionId, int(req.Score))
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonGradeFailed, "Failed to grade answer", map[string]string{"instance_id": req.InstanceId})
	}

	if instance.Status == constants.InstanceStatusPublishedResults && oldScore != int(req.Score) {
		s.publishGradeChanged(ctx, instance, instanceWithQuestions, req.ParticipantId)
	}

	return &pb.GradeAnswerResponse{}, nil
}

func (s *QuizService) PublishResults(ctx context.Context, req *pb.PublishResultsRequest) (*pb.PublishResultsResponse, error) {
	instance, err := s.instanceRepo.GetInstanceByID(ctx, req.InstanceId)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonInstanceNotFound, "Instance not found", map[string]string{"instance_id": req.InstanceId})
	}

	if instance.CreatedBy != req.UserId {
		return nil, errors.New(codes.PermissionDenied, errors.ReasonUnauthorized, "Only the quiz creator can publish results", map[string]string{"instance_id": req.InstanceId})
	}

	sessions, err := s.instanceRepo.GetInstanceParticipants(ctx, req.InstanceId, instance.CreatedBy)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonPublishFailed, "Failed to get participants", map[string]string{"instance_id": req.InstanceId})
	}

	instanceWithQuestions, err := s.instanceRepo.GetInstanceWithQuestions(ctx, req.InstanceId)
	if err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonPublishFailed, "Failed to get instance questions", map[string]string{"instance_id": req.InstanceId})
	}

	openQuestionIDs := make(map[string]bool)
	for _, q := range instanceWithQuestions.Questions {
		if q.Type == "open" {
			openQuestionIDs[q.ID] = true
		}
	}

	if len(openQuestionIDs) > 0 {
		for _, session := range sessions {
			if session.Status != "finished" {
				continue
			}
			var answers []repository.SessionAnswer
			json.Unmarshal([]byte(session.Answers), &answers)
			for _, a := range answers {
				if openQuestionIDs[a.QuestionID] && !a.IsReviewed {
					return nil, errors.New(codes.FailedPrecondition, errors.ReasonNotAllReviewed, "Not all open answers have been reviewed", map[string]string{"participant_id": session.UserID, "question_id": a.QuestionID})
				}
			}
		}
	}

	if err := s.instanceRepo.UpdateInstanceStatus(ctx, req.InstanceId, constants.InstanceStatusPublishedResults); err != nil {
		return nil, errors.New(codes.Internal, errors.ReasonPublishFailed, "Failed to update instance status", map[string]string{"instance_id": req.InstanceId})
	}

	s.publishQuizResults(ctx, instance, instanceWithQuestions, sessions)

	return &pb.PublishResultsResponse{}, nil
}

func (s *QuizService) publishQuizResults(ctx context.Context, instance *repository.Instance, instanceWithQuestions *repository.InstanceWithQuestions, sessions []*repository.ParticipantSession) {
	if s.mqPublisher == nil {
		return
	}

	type ParticipantResult struct {
		UserID   string `json:"user_id"`
		Email    string `json:"email"`
		Score    int    `json:"score"`
		MaxScore int    `json:"max_score"`
		Language string `json:"language"`
	}

	type QuizResultsEvent struct {
		InstanceID   string              `json:"instance_id"`
		Title        string              `json:"title"`
		Participants []ParticipantResult `json:"participants"`
	}

	maxScore := 0
	for _, q := range instanceWithQuestions.Questions {
		maxScore += q.MaxScore
	}

	userIDs := make([]string, 0, len(sessions))
	for _, session := range sessions {
		userIDs = append(userIDs, session.UserID)
	}
	emailsByID, err := s.userClient.GetEmailsByIDs(ctx, userIDs)
	if err != nil {
		log.Printf("Failed to resolve participant emails for quiz_results_ready: %v", err)
		emailsByID = map[string]string{}
	}

	settingsBatch, err := s.userClient.GetNotificationSettingsBatch(ctx, userIDs)
	if err != nil {
		log.Printf("Failed to fetch notification settings for quiz_results_ready: %v", err)
		settingsBatch = map[string]model.NotificationSettings{}
	}

	participants := make([]ParticipantResult, 0, len(sessions))
	for _, session := range sessions {
		if settings, ok := settingsBatch[session.UserID]; ok && !settings.QuizResults {
			continue
		}

		var answers []repository.SessionAnswer
		json.Unmarshal([]byte(session.Answers), &answers)
		total := 0
		for _, a := range answers {
			total += a.Score
		}
		language := string(lang.Default)
		if settings, ok := settingsBatch[session.UserID]; ok && settings.Language != "" {
			language = settings.Language
		}
		participants = append(participants, ParticipantResult{
			UserID:   session.UserID,
			Email:    emailsByID[session.UserID],
			Score:    total,
			MaxScore: maxScore,
			Language: language,
		})
	}

	event := QuizResultsEvent{
		InstanceID:   instance.ID,
		Title:        instance.Title,
		Participants: participants,
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal quiz_results_ready event: %v", err)
		return
	}

	if err := s.mqPublisher.Publish(ctx, "quiz.results_ready", eventJSON); err != nil {
		log.Printf("Failed to publish quiz_results_ready event: %v", err)
	}
}

func (s *QuizService) publishGradeChanged(ctx context.Context, instance *repository.Instance, instanceWithQuestions *repository.InstanceWithQuestions, participantID string) {
	if s.mqPublisher == nil {
		return
	}

	settingsBatch, err := s.userClient.GetNotificationSettingsBatch(ctx, []string{participantID})
	if err != nil {
		log.Printf("Failed to fetch notification settings for quiz.grade_changed: %v", err)
	} else if settings, ok := settingsBatch[participantID]; ok && !settings.QuizResults {
		log.Printf("publishGradeChanged: user %s opted out of quiz_results, skipping", participantID)
		return
	}

	type GradeChangedEvent struct {
		InstanceID       string `json:"instance_id"`
		ParticipantID    string `json:"participant_id"`
		ParticipantEmail string `json:"participant_email"`
		Title            string `json:"title"`
		Score            int    `json:"score"`
		MaxScore         int    `json:"max_score"`
		Language         string `json:"language"`
	}

	emailsByID, err := s.userClient.GetEmailsByIDs(ctx, []string{participantID})
	if err != nil {
		log.Printf("Failed to resolve participant email for quiz.grade_changed: %v", err)
		emailsByID = map[string]string{}
	}

	maxScore := 0
	for _, q := range instanceWithQuestions.Questions {
		maxScore += q.MaxScore
	}

	total := 0
	if session, err := s.instanceRepo.GetParticipantAnswers(ctx, instance.ID, participantID); err == nil {
		var answers []repository.SessionAnswer
		json.Unmarshal([]byte(session.Answers), &answers)
		for _, a := range answers {
			total += a.Score
		}
	} else {
		log.Printf("Failed to load participant session for quiz.grade_changed: %v", err)
	}

	language := string(lang.Default)
	if settings, ok := settingsBatch[participantID]; ok && settings.Language != "" {
		language = settings.Language
	}

	event := GradeChangedEvent{
		InstanceID:       instance.ID,
		ParticipantID:    participantID,
		ParticipantEmail: emailsByID[participantID],
		Title:            instance.Title,
		Score:            total,
		MaxScore:         maxScore,
		Language:         language,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal quiz.grade_changed event: %v", err)
		return
	}

	if err := s.mqPublisher.Publish(ctx, "quiz.grade_changed", eventBytes); err != nil {
		log.Printf("Failed to publish quiz.grade_changed event: %v", err)
	}
}

func (s *QuizService) publishQuizCreated(ctx context.Context, instance *repository.Instance) {
	if s.mqPublisher == nil {
		return
	}

	if !instance.GroupID.Valid {
		return
	}
	groupID := instance.GroupID.String

	type Participant struct {
		UserID   string `json:"user_id"`
		Email    string `json:"email"`
		Language string `json:"language"`
	}

	type QuizCreatedEvent struct {
		InstanceID   string        `json:"instance_id"`
		Title        string        `json:"title"`
		GroupID      string        `json:"group_id"`
		CreatorID    string        `json:"creator_id"`
		CreatorName  string        `json:"creator_name"`
		Deadline     string        `json:"deadline,omitempty"`
		Participants []Participant `json:"participants"`
	}

	memberIDs, err := s.userClient.GetGroupMemberIDs(ctx, groupID)
	if err != nil {
		log.Printf("publishQuizCreated: failed to get member IDs for group %s: %v", groupID, err)
		return
	}

	filtered := make([]string, 0, len(memberIDs))
	for _, id := range memberIDs {
		if id != instance.CreatedBy {
			filtered = append(filtered, id)
		}
	}

	allIDs := append([]string{}, filtered...)
	allIDs = append(allIDs, instance.CreatedBy)
	usersMap, err := s.userClient.GetUsersByIDs(ctx, allIDs)
	if err != nil {
		log.Printf("publishQuizCreated: failed to fetch user details: %v", err)
		usersMap = map[string]*model.UserInfo{}
	}

	creatorName := instance.CreatedBy
	if creator, ok := usersMap[instance.CreatedBy]; ok {
		fullName := creator.FirstName + " " + creator.LastName
		if fullName != " " {
			creatorName = fullName
		} else if creator.Email != "" {
			creatorName = creator.Email
		}
	}

	settingsBatch, err := s.userClient.GetNotificationSettingsBatch(ctx, filtered)
	if err != nil {
		log.Printf("publishQuizCreated: failed to fetch notification settings: %v", err)
		settingsBatch = map[string]model.NotificationSettings{}
	}

	participants := make([]Participant, 0, len(filtered))
	for _, uid := range filtered {
		if settings, ok := settingsBatch[uid]; ok && !settings.NewQuizzes {
			continue
		}

		email := ""
		language := string(lang.Default)
		if info, ok := usersMap[uid]; ok {
			if info.IsRegistered {
				email = info.Email
			}
			if info.Language != "" {
				language = info.Language
			}
		}
		participants = append(participants, Participant{UserID: uid, Email: email, Language: language})
	}

	if len(participants) == 0 {
		return
	}

	event := QuizCreatedEvent{
		InstanceID:   instance.ID,
		Title:        instance.Title,
		GroupID:      groupID,
		CreatorID:    instance.CreatedBy,
		CreatorName:  creatorName,
		Participants: participants,
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
