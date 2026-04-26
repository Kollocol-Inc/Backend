package websocket

import (
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

func TestHandleStartQuiz_Async(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	asyncQuiz := &models.QuizData{
		QuizType:  constants.QuizTypeAsync,
		CreatedBy: "creator-1",
		Questions: []models.Question{
			{ID: "q-1", Text: "Q1", Type: "single", CorrectAnswer: "1", Options: []string{"a", "b"}, TimeLimitSec: 0, MaxScore: 10},
		},
	}
	data, _ := json.Marshal(asyncQuiz)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return(string(data), nil)
	env.sessionRepo.EXPECT().UpdateSessionStatus(gomock.Any(), "inst-1", "user-1", constants.SessionStatusInProgress).Return(nil)
	env.db.EXPECT().ExecContext(gomock.Any(), gomock.Any(), constants.InstanceStatusActive, "inst-1").Return(nil, nil)
	env.redisClient.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	env.hub.handleStartQuiz(client)

	msgs := drainClientMessages(client)
	types := make([]MessageType, len(msgs))
	for i, m := range msgs {
		types[i] = m.Type
	}
	assert.Contains(t, types, MessageTypeQuizStarted)
	assert.Contains(t, types, MessageTypeQuestion)
}

func TestHandleStartQuiz_Sync(t *testing.T) {
	env := newTestHubWithMocks(t)
	creator := newTestClient(env.hub, "creator-1", "inst-1", true)
	participant := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, creator)
	registerClientToHub(env.hub, participant)

	syncQuiz := &models.QuizData{
		QuizType:  constants.QuizTypeSync,
		CreatedBy: "creator-1",
		Questions: []models.Question{
			{ID: "q-1", Text: "Q1", Type: "single", CorrectAnswer: "1", Options: []string{"a", "b"}, TimeLimitSec: 30, MaxScore: 10},
		},
	}
	data, _ := json.Marshal(syncQuiz)
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return(string(data), nil).AnyTimes()
	env.sessionRepo.EXPECT().UpdateSessionStatus(gomock.Any(), "inst-1", gomock.Any(), constants.SessionStatusInProgress).Return(nil).AnyTimes()
	env.db.EXPECT().ExecContext(gomock.Any(), gomock.Any(), constants.InstanceStatusActive, "inst-1").Return(nil, nil)
	env.userClient.EXPECT().GetProfile(gomock.Any(), gomock.Any()).Return(&pb.User{Id: "user-1"}, nil).AnyTimes()
	env.redisClient.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	env.redisClient.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{}, nil).AnyTimes()

	env.hub.handleStartQuiz(creator)

	creatorMsgs := drainClientMessages(creator)
	participantMsgs := drainClientMessages(participant)

	creatorTypes := make([]MessageType, len(creatorMsgs))
	for i, m := range creatorMsgs {
		creatorTypes[i] = m.Type
	}
	assert.Contains(t, creatorTypes, MessageTypeQuizStarted)

	participantTypes := make([]MessageType, len(participantMsgs))
	for i, m := range participantMsgs {
		participantTypes[i] = m.Type
	}
	assert.Contains(t, participantTypes, MessageTypeQuizStarted)
	assert.Contains(t, participantTypes, MessageTypeQuestion)
}

func TestHandleStartQuiz_GetQuizDataFails(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "creator-1", "inst-1", true)
	registerClientToHub(env.hub, client)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return("", fmt.Errorf("not found"))

	env.hub.handleStartQuiz(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestSendQuestion_Normal(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	quizData := &models.QuizData{
		QuizType: constants.QuizTypeAsync,
		Questions: []models.Question{
			{ID: "q-1", Text: "What?", Type: "single", Options: []string{"A", "B"}, TimeLimitSec: 0, MaxScore: 10},
		},
	}

	env.redisClient.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	env.hub.sendQuestion(client, quizData, 0)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeQuestion, msg.Type)
}

func TestSendQuestion_WithTimeLimit(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	quizData := &models.QuizData{
		QuizType: constants.QuizTypeAsync,
		Questions: []models.Question{
			{ID: "q-1", Text: "What?", Type: "single", TimeLimitSec: 30, MaxScore: 10},
		},
	}

	env.redisClient.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	env.hub.sendQuestion(client, quizData, 0)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeQuestion, msg.Type)

	payloadBytes, _ := json.Marshal(msg.Payload)
	var qp QuestionPayload
	json.Unmarshal(payloadBytes, &qp)
	assert.Greater(t, qp.TimeLimitMs, int64(0))

	assert.Len(t, env.hub.questionTimers, 1)
}

func TestSendQuestion_IndexOutOfRange_FinishesQuiz(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	quizData := cachedQuizData()
	setupCachedQuiz(env)

	session := &models.GameSession{
		InstanceID: "inst-1", UserID: "user-1", Score: 100, Answers: "[]",
	}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil)
	env.sessionRepo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Return(nil)
	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{session}, nil).AnyTimes()
	env.userClient.EXPECT().GetProfile(gomock.Any(), gomock.Any()).Return(&pb.User{Id: "user-1"}, nil).AnyTimes()

	env.hub.sendQuestion(client, quizData, 999)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeQuizFinished, msg.Type)
}


func TestRemainingDuration_ZeroTimeLimit(t *testing.T) {
	env := newTestHubWithMocks(t)
	q := models.Question{TimeLimitSec: 0}
	d := env.hub.remainingDuration(t.Context(), q, "key")
	assert.Equal(t, time.Duration(0), d)
}

func TestRemainingDuration_NoStartTime(t *testing.T) {
	env := newTestHubWithMocks(t)
	q := models.Question{TimeLimitSec: 30}
	env.redisClient.EXPECT().Get(gomock.Any(), "key").Return("", fmt.Errorf("nil"))

	d := env.hub.remainingDuration(t.Context(), q, "key")
	assert.Equal(t, 30*time.Second, d)
}

func TestRemainingDuration_Partial(t *testing.T) {
	env := newTestHubWithMocks(t)
	q := models.Question{TimeLimitSec: 30}
	startTime := fmt.Sprintf("%d", time.Now().Add(-10*time.Second).UnixMilli())
	env.redisClient.EXPECT().Get(gomock.Any(), "key").Return(startTime, nil)

	d := env.hub.remainingDuration(t.Context(), q, "key")
	assert.InDelta(t, 20*time.Second, d, float64(500*time.Millisecond))
}

func TestRemainingDuration_Expired(t *testing.T) {
	env := newTestHubWithMocks(t)
	q := models.Question{TimeLimitSec: 10}
	startTime := fmt.Sprintf("%d", time.Now().Add(-30*time.Second).UnixMilli())
	env.redisClient.EXPECT().Get(gomock.Any(), "key").Return(startTime, nil)

	d := env.hub.remainingDuration(t.Context(), q, "key")
	assert.Equal(t, time.Duration(0), d)
}

func TestStartQuestionTimer_CreatesTimer(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	quizData := &models.QuizData{QuizType: constants.QuizTypeAsync}

	env.hub.startQuestionTimer(client, quizData, 0, 10*time.Second)

	tk := timerKey(constants.QuizTypeAsync, "inst-1", "user-1", 0)
	assert.Contains(t, env.hub.questionTimers, tk)
	env.hub.questionTimers[tk].Stop()
}

func TestStartQuestionTimer_ReplacesExisting(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	quizData := &models.QuizData{QuizType: constants.QuizTypeAsync}

	env.hub.startQuestionTimer(client, quizData, 0, 10*time.Second)
	env.hub.startQuestionTimer(client, quizData, 0, 20*time.Second)

	tk := timerKey(constants.QuizTypeAsync, "inst-1", "user-1", 0)
	assert.Len(t, env.hub.questionTimers, 1)
	env.hub.questionTimers[tk].Stop()
}

func TestHandleQuestionTimeout_Sync(t *testing.T) {
	env := newTestHubWithMocks(t)
	creator := newTestClient(env.hub, "creator-1", "inst-1", true)
	participant := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, creator)
	registerClientToHub(env.hub, participant)

	quizData := cachedQuizData()

	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{}, nil).AnyTimes()
	env.userClient.EXPECT().GetProfile(gomock.Any(), gomock.Any()).Return(&pb.User{Id: "user-1"}, nil).AnyTimes()
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	env.hub.handleQuestionTimeout(creator, quizData, 0)

	participantMsgs := drainClientMessages(participant)
	types := make([]MessageType, len(participantMsgs))
	for i, m := range participantMsgs {
		types[i] = m.Type
	}
	assert.Contains(t, types, MessageTypeTimeExpired)
	assert.Contains(t, types, MessageTypeLeaderboard)
}

func TestHandleQuestionTimeout_Async(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	quizData := &models.QuizData{
		QuizType:  constants.QuizTypeAsync,
		CreatedBy: "creator-1",
		Questions: []models.Question{
			{ID: "q-1", Text: "Q1", Type: "single", TimeLimitSec: 10, MaxScore: 10},
			{ID: "q-2", Text: "Q2", Type: "single", TimeLimitSec: 10, MaxScore: 10},
		},
	}

	env.redisClient.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	env.hub.handleQuestionTimeout(client, quizData, 0)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeTimeExpired, msg.Type)

	time.Sleep(400 * time.Millisecond)
	msgs := drainClientMessages(client)
	found := false
	for _, m := range msgs {
		if m.Type == MessageTypeQuestion {
			found = true
		}
	}
	assert.True(t, found, "expected next question after timeout")
}

func TestHandleContinue_NextQuestion(t *testing.T) {
	env := newTestHubWithMocks(t)
	creator := newTestClient(env.hub, "creator-1", "inst-1", true)
	participant := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, creator)
	registerClientToHub(env.hub, participant)

	quizData := &models.QuizData{
		QuizType:  constants.QuizTypeSync,
		CreatedBy: "creator-1",
		Questions: []models.Question{
			{ID: "q-1", Text: "Q1", Type: "single", TimeLimitSec: 30, MaxScore: 10},
			{ID: "q-2", Text: "Q2", Type: "single", TimeLimitSec: 30, MaxScore: 10},
		},
	}
	data, _ := json.Marshal(quizData)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return(string(data), nil)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:current_index").Return("0", nil)
	env.redisClient.EXPECT().Set(gomock.Any(), "quiz:inst-1:current_index", 1, gomock.Any()).Return(nil)
	env.redisClient.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()
	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{}, nil).AnyTimes()

	env.hub.handleContinue(creator)

	participantMsgs := drainClientMessages(participant)
	found := false
	for _, m := range participantMsgs {
		if m.Type == MessageTypeQuestion {
			found = true
		}
	}
	assert.True(t, found)
}

func TestHandleContinue_LastQuestion_FinishesQuiz(t *testing.T) {
	env := newTestHubWithMocks(t)
	creator := newTestClient(env.hub, "creator-1", "inst-1", true)
	participant := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, creator)
	registerClientToHub(env.hub, participant)

	quizData := cachedQuizData()
	data, _ := json.Marshal(quizData)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return(string(data), nil).AnyTimes()
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:current_index").Return("0", nil)
	env.db.EXPECT().ExecContext(gomock.Any(), gomock.Any(), constants.InstanceStatusPendingReview, "inst-1").Return(nil, nil)

	creatorSession := &models.GameSession{InstanceID: "inst-1", UserID: "creator-1", Score: 0, Answers: "[]"}
	participantSession := &models.GameSession{InstanceID: "inst-1", UserID: "user-1", Score: 500, Answers: "[]"}
	env.sessionRepo.EXPECT().BulkFinishInProgress(gomock.Any(), "inst-1").Return(int64(2), nil)
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "creator-1").Return(creatorSession, nil)
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(participantSession, nil)
	env.sessionRepo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{creatorSession, participantSession}, nil).AnyTimes()
	env.userClient.EXPECT().GetProfile(gomock.Any(), gomock.Any()).Return(&pb.User{Id: "user-1"}, nil).AnyTimes()

	env.hub.handleContinue(creator)

	creatorMsgs := drainClientMessages(creator)
	participantMsgs := drainClientMessages(participant)

	creatorFinished := false
	for _, m := range creatorMsgs {
		if m.Type == MessageTypeQuizFinished {
			creatorFinished = true
		}
	}
	assert.True(t, creatorFinished)

	participantFinished := false
	for _, m := range participantMsgs {
		if m.Type == MessageTypeQuizFinished {
			participantFinished = true
		}
	}
	assert.True(t, participantFinished)
}

func TestHandleContinue_GetQuizDataFails(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "creator-1", "inst-1", true)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return("", fmt.Errorf("not found"))

	env.hub.handleContinue(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}


func TestFinishQuiz_Success(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	session := &models.GameSession{
		InstanceID: "inst-1", UserID: "user-1", Score: 750, Answers: "[]",
	}
	setupCachedQuiz(env)
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil)
	env.sessionRepo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).DoAndReturn(func(_ interface{}, s *models.GameSession) error {
		assert.Equal(t, constants.SessionStatusFinished, s.Status)
		assert.True(t, s.FinishedAt.Valid)
		return nil
	})
	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{session}, nil)
	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(&pb.User{Id: "user-1"}, nil)

	env.hub.finishQuiz(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeQuizFinished, msg.Type)

	payloadBytes, _ := json.Marshal(msg.Payload)
	var payload QuizFinishedPayload
	json.Unmarshal(payloadBytes, &payload)
	assert.Equal(t, 750, payload.FinalScore)
	assert.Equal(t, 1, payload.Rank)
}

func TestFinishQuiz_SessionNotFound(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(nil, fmt.Errorf("not found"))

	env.hub.finishQuiz(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestFinishQuiz_QuizDataNotCached(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	session := &models.GameSession{InstanceID: "inst-1", UserID: "user-1", Score: 100, Answers: "[]"}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil)
	env.sessionRepo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Return(nil)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return("", fmt.Errorf("nil"))

	env.hub.finishQuiz(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeQuizFinished, msg.Type)

	payloadBytes, _ := json.Marshal(msg.Payload)
	var payload QuizFinishedPayload
	json.Unmarshal(payloadBytes, &payload)
	assert.Equal(t, 100, payload.FinalScore)
}


func TestSendLeaderboardToClient_AsCreator(t *testing.T) {
	env := newTestHubWithMocks(t)
	creator := newTestClient(env.hub, "creator-1", "inst-1", true)
	registerClientToHub(env.hub, creator)

	quizData := cachedQuizData()
	sessions := []*models.GameSession{
		{InstanceID: "inst-1", UserID: "user-1", Score: 500, CurrentQuestionIndex: 1},
	}

	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return(sessions, nil).Times(2)
	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(&pb.User{Id: "user-1"}, nil).AnyTimes()
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	qd := questionPayloadFromModel(quizData.Questions[0], 0, 1)
	env.hub.sendLeaderboardToClient(t.Context(), creator, quizData, 0, qd)

	msg := readClientMessage(t, creator)
	assert.Equal(t, MessageTypeLeaderboard, msg.Type)

	payloadBytes, _ := json.Marshal(msg.Payload)
	var lb LeaderboardPayload
	json.Unmarshal(payloadBytes, &lb)
	require.Len(t, lb.Leaderboard, 1)
	assert.NotNil(t, lb.Question)
}

func TestSendLeaderboardToClient_AsParticipant(t *testing.T) {
	env := newTestHubWithMocks(t)
	participant := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, participant)

	quizData := cachedQuizData()
	sessions := []*models.GameSession{
		{InstanceID: "inst-1", UserID: "user-1", Score: 300, CurrentQuestionIndex: 1},
	}

	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return(sessions, nil)
	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(&pb.User{Id: "user-1"}, nil)

	qd := questionPayloadFromModel(quizData.Questions[0], 0, 1)
	env.hub.sendLeaderboardToClient(t.Context(), participant, quizData, 0, qd)

	msg := readClientMessage(t, participant)
	assert.Equal(t, MessageTypeLeaderboard, msg.Type)
}

func TestSendLeaderboardToAll(t *testing.T) {
	env := newTestHubWithMocks(t)
	creator := newTestClient(env.hub, "creator-1", "inst-1", true)
	participant := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, creator)
	registerClientToHub(env.hub, participant)

	quizData := cachedQuizData()
	sessions := []*models.GameSession{
		{InstanceID: "inst-1", UserID: "user-1", Score: 500, CurrentQuestionIndex: 1},
	}

	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return(sessions, nil).AnyTimes()
	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(&pb.User{Id: "user-1"}, nil).AnyTimes()
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	qd := questionPayloadFromModel(quizData.Questions[0], 0, 1)
	env.hub.sendLeaderboardToAll(t.Context(), "inst-1", quizData, 0, qd)

	creatorMsg := readClientMessage(t, creator)
	assert.Equal(t, MessageTypeLeaderboard, creatorMsg.Type)

	participantMsg := readClientMessage(t, participant)
	assert.Equal(t, MessageTypeLeaderboard, participantMsg.Type)
}

func TestHandleResumeQuiz_Async(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	quizData := &models.QuizData{
		QuizType:  constants.QuizTypeAsync,
		CreatedBy: "creator-1",
		Questions: []models.Question{
			{ID: "q-1", Text: "Q1", Type: "single", TimeLimitSec: 0, MaxScore: 10},
			{ID: "q-2", Text: "Q2", Type: "single", TimeLimitSec: 0, MaxScore: 10},
		},
	}

	session := &models.GameSession{InstanceID: "inst-1", UserID: "user-1", CurrentQuestionIndex: 1}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil)
	env.redisClient.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	env.hub.handleResumeQuiz(client, quizData)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeQuestion, msg.Type)

	payloadBytes, _ := json.Marshal(msg.Payload)
	var qp QuestionPayload
	json.Unmarshal(payloadBytes, &qp)
	assert.Equal(t, 1, qp.QuestionIndex)
}

func TestHandleResumeQuiz_Sync_AllQuestionsAnswered(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	quizData := cachedQuizData()

	session := &models.GameSession{InstanceID: "inst-1", UserID: "user-1", CurrentQuestionIndex: 0, Score: 100, Answers: "[]"}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil).AnyTimes()

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:current_index").Return("1", nil)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return(func() string {
		d, _ := json.Marshal(quizData)
		return string(d)
	}(), nil).AnyTimes()

	env.sessionRepo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Return(nil)
	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{session}, nil)
	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(&pb.User{Id: "user-1"}, nil)

	env.hub.handleResumeQuiz(client, quizData)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeQuizFinished, msg.Type)
}

func TestHandleResumeQuiz_SessionNotFound(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	quizData := cachedQuizData()
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(nil, fmt.Errorf("not found"))

	env.hub.handleResumeQuiz(client, quizData)

	msgs := drainClientMessages(client)
	assert.Empty(t, msgs)
}

func TestHandleResumeQuiz_Sync_ParticipantAlreadyAnswered(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, client)

	quizData := cachedQuizData()

	session := &models.GameSession{InstanceID: "inst-1", UserID: "user-1", CurrentQuestionIndex: 1}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:current_index").Return("0", nil)
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()
	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{session}, nil).AnyTimes()
	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(&pb.User{Id: "user-1"}, nil).AnyTimes()

	env.hub.handleResumeQuiz(client, quizData)

	msgs := drainClientMessages(client)
	types := make([]MessageType, len(msgs))
	for i, m := range msgs {
		types[i] = m.Type
	}
	assert.Contains(t, types, MessageTypeLeaderboard)
	assert.Contains(t, types, MessageTypeWaitingForCreator)
}

func TestHandleResumeQuiz_Sync_CreatorTimeExpired(t *testing.T) {
	env := newTestHubWithMocks(t)
	creator := newTestClient(env.hub, "creator-1", "inst-1", true)
	registerClientToHub(env.hub, creator)

	quizData := cachedQuizData()

	session := &models.GameSession{InstanceID: "inst-1", UserID: "creator-1", CurrentQuestionIndex: 0}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "creator-1").Return(session, nil)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:current_index").Return("0", nil)
	expiredStart := fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).UnixMilli())
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Regex("question.*start")).Return(expiredStart, nil)
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{}, nil).AnyTimes()

	env.hub.handleResumeQuiz(creator, quizData)

	msgs := drainClientMessages(creator)
	types := make([]MessageType, len(msgs))
	for i, m := range msgs {
		types[i] = m.Type
	}
	assert.Contains(t, types, MessageTypeLeaderboard)
	assert.Contains(t, types, MessageTypeWaitingForCreator)
	assert.NotContains(t, types, MessageTypeTimeExpired)
}

func TestHandleResumeQuiz_Sync_ParticipantTimeExpired(t *testing.T) {
	env := newTestHubWithMocks(t)
	participant := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, participant)

	quizData := cachedQuizData()

	session := &models.GameSession{InstanceID: "inst-1", UserID: "user-1", CurrentQuestionIndex: 0}
	env.sessionRepo.EXPECT().GetSession(gomock.Any(), "inst-1", "user-1").Return(session, nil)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:current_index").Return("0", nil)
	expiredStart := fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).UnixMilli())
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Regex("question.*start")).Return(expiredStart, nil)
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return([]*models.GameSession{session}, nil).AnyTimes()
	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(&pb.User{Id: "user-1"}, nil).AnyTimes()

	env.hub.handleResumeQuiz(participant, quizData)

	msgs := drainClientMessages(participant)
	types := make([]MessageType, len(msgs))
	for i, m := range msgs {
		types[i] = m.Type
	}
	assert.Contains(t, types, MessageTypeLeaderboard)
	assert.Contains(t, types, MessageTypeTimeExpired)
	assert.Contains(t, types, MessageTypeWaitingForCreator)
}

func TestCanCreatorContinue_AsyncReturnsFalse(t *testing.T) {
	env := newTestHubWithMocks(t)
	quizData := &models.QuizData{
		QuizType:  constants.QuizTypeAsync,
		Questions: []models.Question{{ID: "q-1"}},
	}

	result := env.hub.canCreatorContinue(t.Context(), "inst-1", 0, quizData)
	assert.False(t, result)
}

func TestCanCreatorContinue_IndexOutOfRange(t *testing.T) {
	env := newTestHubWithMocks(t)
	quizData := cachedQuizData()

	result := env.hub.canCreatorContinue(t.Context(), "inst-1", 999, quizData)
	assert.False(t, result)
}

func TestCanCreatorContinue_TimeExpired(t *testing.T) {
	env := newTestHubWithMocks(t)
	quizData := cachedQuizData()

	startTime := fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).UnixMilli())
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return(startTime, nil)

	result := env.hub.canCreatorContinue(t.Context(), "inst-1", 0, quizData)
	assert.True(t, result)
}

func TestCanCreatorContinue_AllAnswered(t *testing.T) {
	env := newTestHubWithMocks(t)
	quizData := cachedQuizData()

	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Regex("question.*start")).Return(fmt.Sprintf("%d", time.Now().UnixMilli()), nil)
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Regex("total_participants")).Return("", fmt.Errorf("nil"))
	sessions := []*models.GameSession{
		{UserID: "user-1", Status: constants.SessionStatusInProgress, CurrentQuestionIndex: 1},
	}
	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return(sessions, nil)

	result := env.hub.canCreatorContinue(t.Context(), "inst-1", 0, quizData)
	assert.True(t, result)
}

func TestSendAnswerProgressToAll(t *testing.T) {
	env := newTestHubWithMocks(t)
	creator := newTestClient(env.hub, "creator-1", "inst-1", true)
	participant := newTestClient(env.hub, "user-1", "inst-1", false)
	registerClientToHub(env.hub, creator)
	registerClientToHub(env.hub, participant)

	quizData := cachedQuizData()
	sessions := []*models.GameSession{
		{UserID: "user-1", Status: constants.SessionStatusInProgress, CurrentQuestionIndex: 1},
	}

	env.sessionRepo.EXPECT().GetSessionsByInstance(gomock.Any(), "inst-1").Return(sessions, nil)
	env.redisClient.EXPECT().Get(gomock.Any(), gomock.Any()).Return("", fmt.Errorf("nil")).AnyTimes()

	env.hub.sendAnswerProgressToAll(t.Context(), "inst-1", quizData, 0)

	creatorMsg := readClientMessage(t, creator)
	assert.Equal(t, MessageTypeAnswerProgress, creatorMsg.Type)

	participantMsg := readClientMessage(t, participant)
	assert.Equal(t, MessageTypeAnswerProgress, participantMsg.Type)
}
