package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"game-service/internal/constants"
	"game-service/internal/models"
	pb "game-service/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func cachedQuizData() *models.QuizData {
	return &models.QuizData{
		QuizType:  constants.QuizTypeSync,
		CreatedBy: "creator-1",
		Title:     "Test Quiz",
		Questions: []models.Question{
			{
				ID:            "q-1",
				Text:          "What is 2+2?",
				Type:          constants.QuestionTypeSingle,
				Options:       []string{"3", "4", "5"},
				CorrectAnswer: "1",
				OrderIndex:    0,
				MaxScore:      10,
				TimeLimitSec:  30,
			},
		},
	}
}

func setupCachedQuiz(env *testHubEnv) {
	data, _ := json.Marshal(cachedQuizData())
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return(string(data), nil).AnyTimes()
}

func TestHandleAnswer_Correct(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	creator := newTestClient(env.hub, "creator-1", "inst-1", true)
	registerClientToHub(env.hub, client)
	registerClientToHub(env.hub, creator)

	setupCachedQuiz(env)

	startTime := fmt.Sprintf("%d", time.Now().UnixMilli())
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return(startTime, nil).AnyTimes()

	session := &models.GameSession{
		InstanceID: "inst-1", UserID: "user-1",
		Status: constants.SessionStatusInProgress, CurrentQuestionIndex: 0,
		Score: 0, Answers: "[]",
	}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil)
	env.sessionRepo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Return(nil)
	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{session}, nil).AnyTimes()
	env.userClient.EXPECT().GetProfile(gomock.Any(), gomock.Any()).Return(&pb.User{Id: "user-1", FirstName: "John"}, nil).AnyTimes()

	payload := AnswerPayload{QuestionID: "q-1", Answer: "1"}
	env.hub.handleAnswer(client, payload)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeAnswerResult, msg.Type)

	resultBytes, _ := json.Marshal(msg.Payload)
	var result AnswerResultPayload
	json.Unmarshal(resultBytes, &result)
	assert.True(t, result.IsCorrect)
	assert.Greater(t, result.Score, 0)
}

func TestHandleAnswer_Wrong(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	creator := newTestClient(env.hub, "creator-1", "inst-1", true)
	registerClientToHub(env.hub, client)
	registerClientToHub(env.hub, creator)

	setupCachedQuiz(env)

	startTime := fmt.Sprintf("%d", time.Now().UnixMilli())
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return(startTime, nil).AnyTimes()

	session := &models.GameSession{
		InstanceID: "inst-1", UserID: "user-1",
		Status: constants.SessionStatusInProgress, CurrentQuestionIndex: 0,
		Score: 0, Answers: "[]",
	}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil)
	env.sessionRepo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Return(nil)
	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{session}, nil).AnyTimes()
	env.userClient.EXPECT().GetProfile(gomock.Any(), gomock.Any()).Return(&pb.User{Id: "user-1"}, nil).AnyTimes()

	payload := AnswerPayload{QuestionID: "q-1", Answer: "0"} // wrong
	env.hub.handleAnswer(client, payload)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeAnswerResult, msg.Type)

	resultBytes, _ := json.Marshal(msg.Payload)
	var result AnswerResultPayload
	json.Unmarshal(resultBytes, &result)
	assert.False(t, result.IsCorrect)
	assert.Equal(t, 0, result.Score)
}

func TestHandleAnswer_AsyncAfterDeadline_FinalizesSession(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	expiredQuiz := &models.QuizData{
		QuizType:   constants.QuizTypeAsync,
		CreatedBy:  "creator-1",
		Title:      "Async Quiz",
		DeadlineMs: time.Now().Add(-1 * time.Hour).UnixMilli(),
		Questions: []models.Question{
			{ID: "q-1", Text: "Q1", Type: constants.QuestionTypeSingle, Options: []string{"A", "B"}, CorrectAnswer: "0", MaxScore: 10, TimeLimitSec: 30},
		},
	}
	data, _ := json.Marshal(expiredQuiz)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return(string(data), nil)

	session := &models.GameSession{
		InstanceID: "inst-1", UserID: "user-1",
		Status: constants.SessionStatusInProgress, Score: 42, Answers: "[]",
		StartedAt: time.Now().Add(-2 * time.Hour),
	}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil)
	env.sessionRepo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, s *models.GameSession) error {
		assert.Equal(t, constants.SessionStatusFinished, s.Status)
		assert.True(t, s.FinishedAt.Valid)
		return nil
	})

	env.hub.handleAnswer(client, AnswerPayload{QuestionID: "q-1", Answer: "0"})

	msgs := drainClientMessages(client)
	var sawError, sawFinished bool
	for _, m := range msgs {
		if m.Type == MessageTypeError {
			sawError = true
		}
		if m.Type == MessageTypeQuizFinished {
			sawFinished = true
		}
	}
	assert.True(t, sawError, "expected error message")
	assert.True(t, sawFinished, "expected quiz_finished message")
}

func TestHandleAnswer_AsyncSessionBudgetExceeded_FinalizesSession(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	quiz := &models.QuizData{
		QuizType:  constants.QuizTypeAsync,
		CreatedBy: "creator-1",
		Title:     "Async Quiz",
		Questions: []models.Question{
			{ID: "q-1", Type: constants.QuestionTypeSingle, Options: []string{"A"}, CorrectAnswer: "0", MaxScore: 10, TimeLimitSec: 60},
		},
	}
	data, _ := json.Marshal(quiz)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return(string(data), nil)

	session := &models.GameSession{
		InstanceID: "inst-1", UserID: "user-1",
		Status: constants.SessionStatusInProgress, Score: 0, Answers: "[]",
		StartedAt: time.Now().Add(-10 * time.Minute),
	}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil)
	env.sessionRepo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Return(nil)

	env.hub.handleAnswer(client, AnswerPayload{QuestionID: "q-1", Answer: "0"})

	msgs := drainClientMessages(client)
	var sawFinished bool
	for _, m := range msgs {
		if m.Type == MessageTypeQuizFinished {
			sawFinished = true
		}
	}
	assert.True(t, sawFinished, "expected quiz_finished after exceeding per-session budget")
}

func TestHandleAnswer_QuestionNotFound(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	setupCachedQuiz(env)

	payload := AnswerPayload{QuestionID: "nonexistent", Answer: "1"}
	env.hub.handleAnswer(client, payload)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleAnswer_InvalidPayload(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	env.hub.handleAnswer(client, "not a struct")

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleClientMessage_Ping(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.hub.handleClientMessage(&ClientMessage{
		Client:  client,
		Message: Message{Type: MessageTypePing},
	})

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypePong, msg.Type)
}

func TestHandleClientMessage_UnknownType(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.hub.handleClientMessage(&ClientMessage{
		Client:  client,
		Message: Message{Type: "unknown"},
	})

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleClientMessage_NonCreatorCantStart(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.hub.handleClientMessage(&ClientMessage{
		Client:  client,
		Message: Message{Type: MessageTypeStart},
	})

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleClientMessage_NonCreatorCantContinue(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.hub.handleClientMessage(&ClientMessage{
		Client:  client,
		Message: Message{Type: MessageTypeContinue},
	})

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleClientMessage_NonCreatorCantKick(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.hub.handleClientMessage(&ClientMessage{
		Client:  client,
		Message: Message{Type: MessageTypeKick},
	})

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestConvertToQuizData_SingleChoice(t *testing.T) {
	env := newTestHubWithMocks(t)

	resp := makeQuizInstanceResponse(constants.InstanceStatusWaiting, constants.QuizTypeSync, "creator-1")
	data := env.hub.convertToQuizData(resp)

	require.Len(t, data.Questions, 1)
	assert.Equal(t, constants.QuestionTypeSingle, data.Questions[0].Type)
	assert.Equal(t, "1", data.Questions[0].CorrectAnswer)
	assert.Equal(t, []string{"3", "4", "5"}, data.Questions[0].Options)
}

func TestConvertToQuizData_MultipleChoice(t *testing.T) {
	env := newTestHubWithMocks(t)

	resp := &pb.GetInstanceResponse{
		Instance: &pb.QuizInstance{
			QuizType:  constants.QuizTypeSync,
			CreatedBy: "creator-1",
			Title:     "Quiz",
			Settings:  &pb.QuizInstance_SyncSettings{SyncSettings: &pb.QuizSyncSettings{}},
			CreatedAt: nil,
		},
		Questions: []*pb.Question{
			{
				Id: "q-1", Text: "Select all", OrderIndex: 0, MaxScore: 10, TimeLimitSec: 30,
				Answer: &pb.Question_MultipleChoice{
					MultipleChoice: &pb.MultipleChoice{
						Options:        []string{"A", "B", "C"},
						CorrectOptions: []int32{0, 2},
					},
				},
			},
		},
	}

	data := env.hub.convertToQuizData(resp)
	assert.Equal(t, constants.QuestionTypeMultiple, data.Questions[0].Type)
	assert.Contains(t, data.Questions[0].CorrectAnswer, "0")
	assert.Contains(t, data.Questions[0].CorrectAnswer, "2")
}

func TestConvertToQuizData_OpenAnswer(t *testing.T) {
	env := newTestHubWithMocks(t)

	resp := &pb.GetInstanceResponse{
		Instance: &pb.QuizInstance{
			QuizType: constants.QuizTypeSync, CreatedBy: "creator-1", Title: "Quiz",
			Settings: &pb.QuizInstance_SyncSettings{SyncSettings: &pb.QuizSyncSettings{}},
		},
		Questions: []*pb.Question{
			{
				Id: "q-1", Text: "Type answer", OrderIndex: 0, MaxScore: 10, TimeLimitSec: 30,
				Answer: &pb.Question_OpenAnswer{
					OpenAnswer: &pb.OpenAnswer{CorrectText: "hello"},
				},
			},
		},
	}

	data := env.hub.convertToQuizData(resp)
	assert.Equal(t, constants.QuestionTypeOpen, data.Questions[0].Type)
	assert.Equal(t, "hello", data.Questions[0].CorrectAnswer)
}

func TestCacheAndGetQuizData(t *testing.T) {
	env := newTestHubWithMocks(t)

	quizData := cachedQuizData()
	expectedJSON, _ := json.Marshal(quizData)

	env.redisClient.EXPECT().Set(gomock.Any(), "quiz:inst-1:data", string(expectedJSON), gomock.Any()).Return(nil)
	err := env.hub.cacheQuizData(t.Context(), "inst-1", quizData)
	require.NoError(t, err)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return(string(expectedJSON), nil)
	result, err := env.hub.getQuizData(t.Context(), "inst-1")
	require.NoError(t, err)
	assert.Equal(t, quizData.Title, result.Title)
	assert.Len(t, result.Questions, 1)
}

func TestGetQuizData_NotCached(t *testing.T) {
	env := newTestHubWithMocks(t)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return("", fmt.Errorf("redis: nil"))
	_, err := env.hub.getQuizData(t.Context(), "inst-1")
	assert.Error(t, err)
}

func TestBuildLeaderboard_CreatorExcluded(t *testing.T) {
	env := newTestHubWithMocks(t)

	quizData := cachedQuizData()
	sessions := []*models.GameSession{
		{InstanceID: "inst-1", UserID: "creator-1", Score: 999, CurrentQuestionIndex: 1},
		{InstanceID: "inst-1", UserID: "user-1", Score: 500, CurrentQuestionIndex: 1},
		{InstanceID: "inst-1", UserID: "user-2", Score: 300, CurrentQuestionIndex: 0},
	}

	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(&pb.User{Id: "user-1", FirstName: "John"}, nil)
	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-2").Return(&pb.User{Id: "user-2", FirstName: "Jane"}, nil)

	leaderboard := env.hub.buildLeaderboard(t.Context(), sessions, quizData, 0)
	assert.Len(t, leaderboard, 2)
	assert.Equal(t, "user-1", leaderboard[0].User.UserID)
	assert.Equal(t, 1, leaderboard[0].Rank)
	assert.True(t, leaderboard[0].IsAnswered)
	assert.False(t, leaderboard[1].IsAnswered)
}

func TestBuildLeaderboard_EmptySessions(t *testing.T) {
	env := newTestHubWithMocks(t)
	quizData := cachedQuizData()

	leaderboard := env.hub.buildLeaderboard(t.Context(), []*models.GameSession{}, quizData, 0)
	assert.Empty(t, leaderboard)
}

func TestBuildLeaderboard_ProfileFetchFails(t *testing.T) {
	env := newTestHubWithMocks(t)
	quizData := cachedQuizData()

	sessions := []*models.GameSession{
		{InstanceID: "inst-1", UserID: "user-1", Score: 100, CurrentQuestionIndex: 1},
	}

	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(nil, assert.AnError)

	leaderboard := env.hub.buildLeaderboard(t.Context(), sessions, quizData, 0)
	assert.Empty(t, leaderboard)
}

func TestHandleKick_EmptyEmail(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "creator-1", "inst-1", true)

	payload := KickPayload{Email: ""}
	env.hub.handleKick(client, payload)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleKick_KickSelf(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "creator-1", "inst-1", true)
	registerClientToHub(env.hub, client)

	env.userClient.EXPECT().GetByEmail(gomock.Any(), "creator@example.com").Return(&pb.User{Id: "creator-1"}, nil)

	payload := KickPayload{Email: "creator@example.com"}
	env.hub.handleKick(client, payload)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleKick_AsyncNotAllowed(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "creator-1", "inst-1", true)
	registerClientToHub(env.hub, client)

	env.userClient.EXPECT().GetByEmail(gomock.Any(), "user@example.com").Return(&pb.User{Id: "user-1"}, nil)

	asyncQuiz := &models.QuizData{QuizType: constants.QuizTypeAsync, CreatedBy: "creator-1"}
	data, _ := json.Marshal(asyncQuiz)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return(string(data), nil)

	payload := KickPayload{Email: "user@example.com"}
	env.hub.handleKick(client, payload)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestIsUserOnline_True(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	assert.True(t, env.hub.isUserOnline("inst-1", "user-1"))
}

func TestIsUserOnline_False(t *testing.T) {
	env := newTestHubWithMocks(t)
	assert.False(t, env.hub.isUserOnline("inst-1", "user-999"))
}

func TestCancelQuestionTimer(t *testing.T) {
	env := newTestHubWithMocks(t)
	timer := time.AfterFunc(time.Hour, func() {})
	env.hub.questionTimers["inst-1:0"] = timer

	env.hub.cancelQuestionTimer("inst-1:0")
	assert.Empty(t, env.hub.questionTimers)
}

func TestCancelAllTimersForInstance(t *testing.T) {
	env := newTestHubWithMocks(t)
	env.hub.questionTimers["inst-1:0"] = time.AfterFunc(time.Hour, func() {})
	env.hub.questionTimers["inst-1:1"] = time.AfterFunc(time.Hour, func() {})
	env.hub.questionTimers["inst-2:0"] = time.AfterFunc(time.Hour, func() {})

	env.hub.cancelAllTimersForInstance("inst-1")
	assert.Len(t, env.hub.questionTimers, 1)
	assert.Contains(t, env.hub.questionTimers, "inst-2:0")
}
