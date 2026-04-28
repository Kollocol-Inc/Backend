package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"quiz-service/internal/model"
	"quiz-service/internal/repository"
	"quiz-service/internal/service/mocks"
	pb "quiz-service/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func setupTest(t *testing.T) (*QuizService, *mocks.MockTemplateRepo, *mocks.MockInstanceRepo, *mocks.MockRabbitMQPublisher, *mocks.MockUserClient, *mocks.MockDeleteRepo) {
	ctrl := gomock.NewController(t)
	templateRepo := mocks.NewMockTemplateRepo(ctrl)
	instanceRepo := mocks.NewMockInstanceRepo(ctrl)
	deleteRepo := mocks.NewMockDeleteRepo(ctrl)
	publisher := mocks.NewMockRabbitMQPublisher(ctrl)
	userClient := mocks.NewMockUserClient(ctrl)

	svc := NewQuizServiceWithDeps(templateRepo, instanceRepo, deleteRepo, publisher, userClient)
	return svc, templateRepo, instanceRepo, publisher, userClient, deleteRepo
}

func TestCreateTemplate_Success(t *testing.T) {
	svc, templateRepo, _, _, _, _ := setupTest(t)
	ctx := context.Background()

	templateRepo.EXPECT().CreateTemplate(ctx, gomock.Any()).Return(nil)
	templateRepo.EXPECT().CreateQuestion(ctx, gomock.Any()).Return(nil)

	resp, err := svc.CreateTemplate(ctx, &pb.CreateTemplateRequest{
		UserId:   "user-1",
		Title:    "My Quiz",
		QuizType: "sync",
		Questions: []*pb.QuestionInput{
			{
				Text:         "What is 2+2?",
				MaxScore:     10,
				TimeLimitSec: 30,
				Answer:       &pb.QuestionInput_SingleChoice{SingleChoice: &pb.SingleChoice{Options: []string{"3", "4", "5"}, CorrectOption: 1}},
			},
		},
	})

	require.NoError(t, err)
	assert.NotNil(t, resp.Template)
	assert.Equal(t, "My Quiz", resp.Template.Title)
	assert.Len(t, resp.Questions, 1)
}

func TestCreateTemplate_CreateFails(t *testing.T) {
	svc, templateRepo, _, _, _, _ := setupTest(t)
	ctx := context.Background()

	templateRepo.EXPECT().CreateTemplate(ctx, gomock.Any()).Return(fmt.Errorf("db error"))

	_, err := svc.CreateTemplate(ctx, &pb.CreateTemplateRequest{
		UserId:   "user-1",
		Title:    "My Quiz",
		QuizType: "sync",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to create template")
}

func TestCreateTemplate_WithAsyncSettings(t *testing.T) {
	svc, templateRepo, _, _, _, _ := setupTest(t)
	ctx := context.Background()

	templateRepo.EXPECT().CreateTemplate(ctx, gomock.Any()).Return(nil)

	resp, err := svc.CreateTemplate(ctx, &pb.CreateTemplateRequest{
		UserId:   "user-1",
		Title:    "Async Quiz",
		QuizType: "async",
		Settings: &pb.CreateTemplateRequest_AsyncSettings{
			AsyncSettings: &pb.QuizAsyncSettings{
				QuestionsRandomOrder: true,
			},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "Async Quiz", resp.Template.Title)
}

func TestGetTemplate_Success(t *testing.T) {
	svc, templateRepo, _, _, _, _ := setupTest(t)
	ctx := context.Background()

	template := &repository.Template{
		ID: "tmpl-1", OwnerID: "user-1", Title: "Test", QuizType: "sync", Settings: "{}",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	questions := []*repository.Question{
		{ID: "q-1", TemplateID: "tmpl-1", Text: "Q1", Type: "single", CorrectAnswer: `{"options":["a","b"],"correct_option":0}`, OrderIndex: 0, MaxScore: 10, TimeLimitSec: 30},
	}

	templateRepo.EXPECT().GetTemplateByID(ctx, "tmpl-1").Return(template, nil)
	templateRepo.EXPECT().GetQuestionsByTemplateID(ctx, "tmpl-1").Return(questions, nil)

	resp, err := svc.GetTemplate(ctx, &pb.GetTemplateRequest{TemplateId: "tmpl-1", UserId: "user-1"})
	require.NoError(t, err)
	assert.Equal(t, "Test", resp.Template.Title)
	assert.Len(t, resp.Questions, 1)
}

func TestGetTemplate_NotFound(t *testing.T) {
	svc, templateRepo, _, _, _, _ := setupTest(t)
	ctx := context.Background()

	templateRepo.EXPECT().GetTemplateByID(ctx, "tmpl-999").Return(nil, fmt.Errorf("not found"))

	_, err := svc.GetTemplate(ctx, &pb.GetTemplateRequest{TemplateId: "tmpl-999", UserId: "user-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetTemplate_NotOwner(t *testing.T) {
	svc, templateRepo, _, _, _, _ := setupTest(t)
	ctx := context.Background()

	template := &repository.Template{ID: "tmpl-1", OwnerID: "user-1", Title: "Test", QuizType: "sync", Settings: "{}"}
	templateRepo.EXPECT().GetTemplateByID(ctx, "tmpl-1").Return(template, nil)

	_, err := svc.GetTemplate(ctx, &pb.GetTemplateRequest{TemplateId: "tmpl-1", UserId: "user-2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not the template owner")
}

func TestDeleteTemplate_Success(t *testing.T) {
	svc, templateRepo, _, _, _, _ := setupTest(t)
	ctx := context.Background()

	templateRepo.EXPECT().DeleteQuestionsByTemplateID(ctx, "tmpl-1").Return(nil)
	templateRepo.EXPECT().DeleteTemplate(ctx, "tmpl-1", "user-1").Return(nil)

	resp, err := svc.DeleteTemplate(ctx, &pb.DeleteTemplateRequest{TemplateId: "tmpl-1", UserId: "user-1"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestDeleteTemplate_DeleteQuestionsFails(t *testing.T) {
	svc, templateRepo, _, _, _, _ := setupTest(t)
	ctx := context.Background()

	templateRepo.EXPECT().DeleteQuestionsByTemplateID(ctx, "tmpl-1").Return(fmt.Errorf("db error"))

	_, err := svc.DeleteTemplate(ctx, &pb.DeleteTemplateRequest{TemplateId: "tmpl-1", UserId: "user-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to delete questions")
}

func TestCreateInstance_Success(t *testing.T) {
	svc, templateRepo, instanceRepo, _, _, _ := setupTest(t)
	ctx := context.Background()

	template := &repository.Template{ID: "tmpl-1", OwnerID: "user-1", Title: "Test", QuizType: "async", Settings: `{"time_limit_minutes":60}`}
	questions := []*repository.Question{
		{ID: "q-1", TimeLimitSec: 30},
		{ID: "q-2", TimeLimitSec: 45},
	}

	templateRepo.EXPECT().GetTemplateByID(ctx, "tmpl-1").Return(template, nil)
	templateRepo.EXPECT().GetQuestionsByTemplateID(ctx, "tmpl-1").Return(questions, nil)
	instanceRepo.EXPECT().CreateInstance(ctx, gomock.Any()).Return(nil)

	resp, err := svc.CreateInstance(ctx, &pb.CreateInstanceRequest{
		TemplateId: "tmpl-1",
		UserId:     "user-1",
		Title:      "Instance 1",
	})

	require.NoError(t, err)
	assert.NotNil(t, resp.Instance)
	assert.Equal(t, "Instance 1", resp.Instance.Title)
}

func TestCreateInstance_NotOwner(t *testing.T) {
	svc, templateRepo, _, _, _, _ := setupTest(t)
	ctx := context.Background()

	template := &repository.Template{ID: "tmpl-1", OwnerID: "user-1"}
	templateRepo.EXPECT().GetTemplateByID(ctx, "tmpl-1").Return(template, nil)

	_, err := svc.CreateInstance(ctx, &pb.CreateInstanceRequest{
		TemplateId: "tmpl-1",
		UserId:     "user-2",
		Title:      "Instance",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not the template owner")
}

func TestCreateInstance_WithGroupAndDeadline(t *testing.T) {
	svc, templateRepo, instanceRepo, publisher, userClient, _ := setupTest(t)
	ctx := context.Background()

	template := &repository.Template{ID: "tmpl-1", OwnerID: "user-1", QuizType: "async", Settings: "{}"}
	templateRepo.EXPECT().GetTemplateByID(ctx, "tmpl-1").Return(template, nil)
	templateRepo.EXPECT().GetQuestionsByTemplateID(ctx, "tmpl-1").Return([]*repository.Question{}, nil)
	instanceRepo.EXPECT().CreateInstance(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, inst *repository.Instance) error {
		assert.True(t, inst.GroupID.Valid)
		assert.Equal(t, "group-1", inst.GroupID.String)
		assert.True(t, inst.Deadline.Valid)
		return nil
	})

	userClient.EXPECT().GetGroupMemberIDs(ctx, "group-1").Return([]string{"user-1", "user-2"}, nil)
	userClient.EXPECT().GetUsersByIDs(ctx, gomock.Any()).Return(map[string]*model.UserInfo{
		"user-1": {ID: "user-1", FirstName: "Alice", LastName: "Smith", Email: "alice@example.com", IsRegistered: true},
		"user-2": {ID: "user-2", Email: "bob@example.com", IsRegistered: true},
	}, nil)
	userClient.EXPECT().GetNotificationSettingsBatch(ctx, []string{"user-2"}).Return(map[string]model.NotificationSettings{
		"user-2": {NewQuizzes: true, DeadlineReminder: "24h"},
	}, nil)
	publisher.EXPECT().Publish(ctx, "quiz.created", gomock.Any()).Return(nil)

	deadline := timestamppb.New(time.Now().Add(48 * time.Hour))
	_, err := svc.CreateInstance(ctx, &pb.CreateInstanceRequest{
		TemplateId: "tmpl-1",
		UserId:     "user-1",
		Title:      "Group Quiz",
		GroupId:    "group-1",
		Deadline:   deadline,
	})
	require.NoError(t, err)
}

func TestCreateInstance_DeadlineRejectedForSync(t *testing.T) {
	svc, templateRepo, _, _, _, _ := setupTest(t)
	ctx := context.Background()

	template := &repository.Template{ID: "tmpl-1", OwnerID: "user-1", QuizType: "sync", Settings: "{}"}
	templateRepo.EXPECT().GetTemplateByID(ctx, "tmpl-1").Return(template, nil)
	templateRepo.EXPECT().GetQuestionsByTemplateID(ctx, "tmpl-1").Return([]*repository.Question{}, nil)

	deadline := timestamppb.New(time.Now().Add(48 * time.Hour))
	_, err := svc.CreateInstance(ctx, &pb.CreateInstanceRequest{
		TemplateId: "tmpl-1",
		UserId:     "user-1",
		Title:      "Sync With Deadline",
		Deadline:   deadline,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Deadline can only be set for async quizzes")
}

func TestGetParticipantAnswers_SessionNotFinished_Returns404(t *testing.T) {
	svc, _, instanceRepo, _, _, _ := setupTest(t)
	ctx := context.Background()

	iwq := &repository.InstanceWithQuestions{
		Instance:  &repository.Instance{ID: "inst-1", CreatedBy: "creator-1", QuizType: "async"},
		Questions: []*repository.Question{{ID: "q-1"}},
	}
	instanceRepo.EXPECT().GetInstanceWithQuestions(ctx, "inst-1").Return(iwq, nil)
	instanceRepo.EXPECT().GetParticipantAnswers(ctx, "inst-1", "user-1").Return(&repository.ParticipantSession{
		UserID: "user-1", Status: "in_progress", Answers: "[]",
	}, nil)

	_, err := svc.GetParticipantAnswers(ctx, &pb.GetParticipantAnswersRequest{
		InstanceId:    "inst-1",
		UserId:        "creator-1",
		ParticipantId: "user-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has not finished")
}

func TestGetParticipantAnswers_FinishedSession_Returns200(t *testing.T) {
	svc, _, instanceRepo, _, _, _ := setupTest(t)
	ctx := context.Background()

	iwq := &repository.InstanceWithQuestions{
		Instance:  &repository.Instance{ID: "inst-1", CreatedBy: "creator-1", QuizType: "async"},
		Questions: []*repository.Question{{ID: "q-1"}},
	}
	instanceRepo.EXPECT().GetInstanceWithQuestions(ctx, "inst-1").Return(iwq, nil)
	instanceRepo.EXPECT().GetParticipantAnswers(ctx, "inst-1", "user-1").Return(&repository.ParticipantSession{
		UserID: "user-1", Status: "finished", Answers: "[]",
	}, nil)

	resp, err := svc.GetParticipantAnswers(ctx, &pb.GetParticipantAnswersRequest{
		InstanceId:    "inst-1",
		UserId:        "creator-1",
		ParticipantId: "user-1",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp.Instance)
}

func TestGetInstanceByAccessCode_CreatorHasAccess(t *testing.T) {
	svc, _, instanceRepo, _, _, _ := setupTest(t)
	ctx := context.Background()

	instance := &repository.Instance{
		ID: "inst-1", CreatedBy: "user-1", AccessCode: "123456",
		QuizType: "sync", Settings: "{}", Status: "waiting",
		GroupID: sql.NullString{Valid: false},
	}
	instanceRepo.EXPECT().GetInstanceByAccessCode(ctx, "123456").Return(instance, nil)

	resp, err := svc.GetInstanceByAccessCode(ctx, &pb.GetInstanceByAccessCodeRequest{
		AccessCode: "123456",
		UserId:     "user-1",
	})
	require.NoError(t, err)
	assert.True(t, resp.HasAccess)
}

func TestGetInstanceByAccessCode_PublicQuiz(t *testing.T) {
	svc, _, instanceRepo, _, _, _ := setupTest(t)
	ctx := context.Background()

	instance := &repository.Instance{
		ID: "inst-1", CreatedBy: "user-1", AccessCode: "123456",
		QuizType: "sync", Settings: "{}", Status: "waiting",
		GroupID: sql.NullString{Valid: false},
	}
	instanceRepo.EXPECT().GetInstanceByAccessCode(ctx, "123456").Return(instance, nil)

	resp, err := svc.GetInstanceByAccessCode(ctx, &pb.GetInstanceByAccessCodeRequest{
		AccessCode: "123456",
		UserId:     "user-2",
	})
	require.NoError(t, err)
	assert.True(t, resp.HasAccess)
}

func TestGetInstanceByAccessCode_GroupMember(t *testing.T) {
	svc, _, instanceRepo, _, userClient, _ := setupTest(t)
	ctx := context.Background()

	instance := &repository.Instance{
		ID: "inst-1", CreatedBy: "user-1", AccessCode: "123456",
		QuizType: "sync", Settings: "{}", Status: "waiting",
		GroupID: sql.NullString{String: "group-1", Valid: true},
	}
	instanceRepo.EXPECT().GetInstanceByAccessCode(ctx, "123456").Return(instance, nil)
	userClient.EXPECT().CheckGroupMembership(ctx, "group-1", "user-2").Return(true, "member", nil)

	resp, err := svc.GetInstanceByAccessCode(ctx, &pb.GetInstanceByAccessCodeRequest{
		AccessCode: "123456",
		UserId:     "user-2",
	})
	require.NoError(t, err)
	assert.True(t, resp.HasAccess)
}

func TestGetInstanceByAccessCode_GroupNonMember(t *testing.T) {
	svc, _, instanceRepo, _, userClient, _ := setupTest(t)
	ctx := context.Background()

	instance := &repository.Instance{
		ID: "inst-1", CreatedBy: "user-1", AccessCode: "123456",
		QuizType: "sync", Settings: "{}", Status: "waiting",
		GroupID: sql.NullString{String: "group-1", Valid: true},
	}
	instanceRepo.EXPECT().GetInstanceByAccessCode(ctx, "123456").Return(instance, nil)
	userClient.EXPECT().CheckGroupMembership(ctx, "group-1", "user-3").Return(false, "", nil)

	resp, err := svc.GetInstanceByAccessCode(ctx, &pb.GetInstanceByAccessCodeRequest{
		AccessCode: "123456",
		UserId:     "user-3",
	})
	require.NoError(t, err)
	assert.False(t, resp.HasAccess)
}

func TestGetInstanceByAccessCode_NotFound(t *testing.T) {
	svc, _, instanceRepo, _, _, _ := setupTest(t)
	ctx := context.Background()

	instanceRepo.EXPECT().GetInstanceByAccessCode(ctx, "999999").Return(nil, fmt.Errorf("not found"))

	resp, err := svc.GetInstanceByAccessCode(ctx, &pb.GetInstanceByAccessCodeRequest{
		AccessCode: "999999",
		UserId:     "user-1",
	})
	require.NoError(t, err)
	assert.False(t, resp.HasAccess)
	assert.Nil(t, resp.Instance)
}

func TestGetInstance_Success(t *testing.T) {
	svc, _, instanceRepo, _, _, _ := setupTest(t)
	ctx := context.Background()

	instWithQ := &repository.InstanceWithQuestions{
		Instance: &repository.Instance{
			ID: "inst-1", Title: "Test", CreatedBy: "user-1",
			QuizType: "sync", Settings: "{}", Status: "waiting",
		},
		Questions: []*repository.Question{
			{ID: "q-1", Text: "Q1", Type: "open", CorrectAnswer: `{"correct_text":"hello"}`},
		},
	}
	instanceRepo.EXPECT().GetInstanceWithQuestions(ctx, "inst-1").Return(instWithQ, nil)

	resp, err := svc.GetInstance(ctx, &pb.GetInstanceRequest{InstanceId: "inst-1"})
	require.NoError(t, err)
	assert.Equal(t, "Test", resp.Instance.Title)
	assert.Len(t, resp.Questions, 1)
}

func TestQuestionInputToDB_SingleChoice(t *testing.T) {
	svc, _, _, _, _, _ := setupTest(t)
	input := &pb.QuestionInput{
		Answer: &pb.QuestionInput_SingleChoice{
			SingleChoice: &pb.SingleChoice{Options: []string{"a", "b", "c"}, CorrectOption: 1},
		},
	}

	qType, jsonStr, err := svc.questionInputToDB(input)
	require.NoError(t, err)
	assert.Equal(t, "single", qType)
	assert.Contains(t, jsonStr, `"correct_option":1`)
}

func TestQuestionInputToDB_MultipleChoice(t *testing.T) {
	svc, _, _, _, _, _ := setupTest(t)
	input := &pb.QuestionInput{
		Answer: &pb.QuestionInput_MultipleChoice{
			MultipleChoice: &pb.MultipleChoice{Options: []string{"a", "b", "c"}, CorrectOptions: []int32{0, 2}},
		},
	}

	qType, jsonStr, err := svc.questionInputToDB(input)
	require.NoError(t, err)
	assert.Equal(t, "multiple", qType)
	assert.Contains(t, jsonStr, `"correct_options":[0,2]`)
}

func TestQuestionInputToDB_OpenAnswer(t *testing.T) {
	svc, _, _, _, _, _ := setupTest(t)
	input := &pb.QuestionInput{
		Answer: &pb.QuestionInput_OpenAnswer{
			OpenAnswer: &pb.OpenAnswer{CorrectText: "hello world"},
		},
	}

	qType, jsonStr, err := svc.questionInputToDB(input)
	require.NoError(t, err)
	assert.Equal(t, "open", qType)
	assert.Contains(t, jsonStr, `"correct_text":"hello world"`)
}

func TestMarshalSettings_Sync(t *testing.T) {
	svc, _, _, _, _, _ := setupTest(t)
	result, err := svc.marshalSettings(nil)
	require.NoError(t, err)
	assert.Equal(t, "{}", result)
}

func TestMarshalSettings_Async(t *testing.T) {
	svc, _, _, _, _, _ := setupTest(t)
	settings := &pb.CreateTemplateRequest_AsyncSettings{
		AsyncSettings: &pb.QuizAsyncSettings{QuestionsRandomOrder: true},
	}
	result, err := svc.marshalSettings(settings)
	require.NoError(t, err)
	assert.Contains(t, result, "questions_random_order")
}

func TestPublishGradeChanged_SkipsWhenQuizResultsDisabled(t *testing.T) {
	svc, _, _, _, userClient, _ := setupTest(t)
	ctx := context.Background()

	inst := &repository.Instance{ID: "inst-1", Title: "Math Quiz"}
	iwq := &repository.InstanceWithQuestions{
		Instance:  inst,
		Questions: []*repository.Question{{ID: "q-1", MaxScore: 10}},
	}

	userClient.EXPECT().
		GetNotificationSettingsBatch(ctx, []string{"user-2"}).
		Return(map[string]model.NotificationSettings{
			"user-2": {QuizResults: false},
		}, nil)

	svc.publishGradeChanged(ctx, inst, iwq, "user-2")
}

func TestPublishGradeChanged_PublishesWhenQuizResultsEnabled(t *testing.T) {
	svc, _, instanceRepo, publisher, userClient, _ := setupTest(t)
	ctx := context.Background()

	inst := &repository.Instance{ID: "inst-1", Title: "Math Quiz"}
	iwq := &repository.InstanceWithQuestions{
		Instance:  inst,
		Questions: []*repository.Question{{ID: "q-1", MaxScore: 10}},
	}

	userClient.EXPECT().
		GetNotificationSettingsBatch(ctx, []string{"user-2"}).
		Return(map[string]model.NotificationSettings{
			"user-2": {QuizResults: true},
		}, nil)
	userClient.EXPECT().
		GetEmailsByIDs(ctx, []string{"user-2"}).
		Return(map[string]string{"user-2": "u2@example.com"}, nil)
	instanceRepo.EXPECT().
		GetParticipantAnswers(ctx, "inst-1", "user-2").
		Return(&repository.ParticipantSession{
			UserID:  "user-2",
			Status:  "finished",
			Answers: `[{"question_id":"q-1","score":7}]`,
		}, nil)
	publisher.EXPECT().
		Publish(ctx, "quiz.grade_changed", gomock.Any()).
		Return(nil)

	svc.publishGradeChanged(ctx, inst, iwq, "user-2")
}
