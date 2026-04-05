package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"game-service/internal/constants"
	"game-service/internal/models"
	pb "game-service/proto"

	"github.com/stretchr/testify/assert"
)

func TestIsQuestionTimeExpired_NotExpired(t *testing.T) {
	h := newTestHub()
	q := models.Question{TimeLimitSec: 30}
	startTime := time.Now().UnixMilli() - 5000

	assert.False(t, h.isQuestionTimeExpired(q, startTime))
}

func TestIsQuestionTimeExpired_Expired(t *testing.T) {
	h := newTestHub()
	q := models.Question{TimeLimitSec: 10}
	startTime := time.Now().UnixMilli() - 15000

	assert.True(t, h.isQuestionTimeExpired(q, startTime))
}

func TestIsQuestionTimeExpired_ZeroStartTime(t *testing.T) {
	h := newTestHub()
	q := models.Question{TimeLimitSec: 30}

	assert.False(t, h.isQuestionTimeExpired(q, 0))
}

func TestIsQuestionTimeExpired_ZeroTimeLimit(t *testing.T) {
	h := newTestHub()
	q := models.Question{TimeLimitSec: 0}
	startTime := time.Now().UnixMilli() - 99999

	assert.False(t, h.isQuestionTimeExpired(q, startTime))
}

func TestIsQuestionTimeExpired_NegativeStartTime(t *testing.T) {
	h := newTestHub()
	q := models.Question{TimeLimitSec: 30}

	assert.False(t, h.isQuestionTimeExpired(q, -1))
}

func TestIsQuestionTimeExpired_ExactBoundary(t *testing.T) {
	h := newTestHub()
	q := models.Question{TimeLimitSec: 10}
	startTime := time.Now().UnixMilli() - 10000

	_ = h.isQuestionTimeExpired(q, startTime)
}


func TestBuildQuestionStats_SingleChoice(t *testing.T) {
	h := newTestHub()

	quizData := &models.QuizData{
		CreatedBy: "creator-1",
		Questions: []models.Question{
			{
				ID:      "q-1",
				Type:    constants.QuestionTypeSingle,
				Options: []string{"A", "B", "C"},
			},
		},
	}

	sessions := []*models.GameSession{
		{UserID: "creator-1", Answers: `[{"question_id":"q-1","answer":"0"}]`},
		{UserID: "user-1", Answers: `[{"question_id":"q-1","answer":"0"}]`},
		{UserID: "user-2", Answers: `[{"question_id":"q-1","answer":"1"}]`},
		{UserID: "user-3", Answers: `[{"question_id":"q-1","answer":"0"}]`},
	}

	stats := h.buildQuestionStats(sessions, quizData, 0)
	assert.Len(t, stats, 3)
	assert.Equal(t, 2, stats[0].Count)
	assert.Equal(t, 1, stats[1].Count)
	assert.Equal(t, 0, stats[2].Count)
}

func TestBuildQuestionStats_MultipleChoice(t *testing.T) {
	h := newTestHub()

	quizData := &models.QuizData{
		CreatedBy: "creator-1",
		Questions: []models.Question{
			{
				ID:      "q-1",
				Type:    constants.QuestionTypeMultiple,
				Options: []string{"A", "B", "C"},
			},
		},
	}

	sessions := []*models.GameSession{
		{UserID: "user-1", Answers: `[{"question_id":"q-1","answer":"[0,2]"}]`},
		{UserID: "user-2", Answers: `[{"question_id":"q-1","answer":"[1]"}]`},
	}

	stats := h.buildQuestionStats(sessions, quizData, 0)
	assert.Len(t, stats, 3)
	assert.Equal(t, 1, stats[0].Count)
	assert.Equal(t, 1, stats[1].Count)
	assert.Equal(t, 1, stats[2].Count)
}

func TestBuildQuestionStats_OpenQuestion_ReturnsNil(t *testing.T) {
	h := newTestHub()

	quizData := &models.QuizData{
		Questions: []models.Question{
			{ID: "q-1", Type: constants.QuestionTypeOpen},
		},
	}

	stats := h.buildQuestionStats(nil, quizData, 0)
	assert.Nil(t, stats)
}

func TestBuildQuestionStats_IndexOutOfRange(t *testing.T) {
	h := newTestHub()

	quizData := &models.QuizData{
		Questions: []models.Question{
			{ID: "q-1", Type: constants.QuestionTypeSingle, Options: []string{"A"}},
		},
	}

	stats := h.buildQuestionStats(nil, quizData, 5)
	assert.Nil(t, stats)
}

func TestBuildQuestionStats_InvalidAnswerJSON(t *testing.T) {
	h := newTestHub()

	quizData := &models.QuizData{
		CreatedBy: "creator-1",
		Questions: []models.Question{
			{ID: "q-1", Type: constants.QuestionTypeSingle, Options: []string{"A", "B"}},
		},
	}

	sessions := []*models.GameSession{
		{UserID: "user-1", Answers: `invalid json`},
		{UserID: "user-2", Answers: `[{"question_id":"q-1","answer":"0"}]`},
	}

	stats := h.buildQuestionStats(sessions, quizData, 0)
	assert.Len(t, stats, 2)
	assert.Equal(t, 1, stats[0].Count)
}

func TestBuildQuestionStats_CreatorExcluded(t *testing.T) {
	h := newTestHub()

	quizData := &models.QuizData{
		CreatedBy: "creator-1",
		Questions: []models.Question{
			{ID: "q-1", Type: constants.QuestionTypeSingle, Options: []string{"A", "B"}},
		},
	}

	sessions := []*models.GameSession{
		{UserID: "creator-1", Answers: `[{"question_id":"q-1","answer":"0"}]`},
	}

	stats := h.buildQuestionStats(sessions, quizData, 0)
	assert.Equal(t, 0, stats[0].Count)
}

func TestCountAnswerProgress_WithFrozenTotal(t *testing.T) {
	h := &Hub{}

	sessions := []*models.GameSession{
		{UserID: "creator-1", Status: constants.SessionStatusInProgress, CurrentQuestionIndex: 0},
		{UserID: "user-1", Status: constants.SessionStatusInProgress, CurrentQuestionIndex: 1},
		{UserID: "user-2", Status: constants.SessionStatusInProgress, CurrentQuestionIndex: 0},
		{UserID: "user-3", Status: constants.SessionStatusFinished, CurrentQuestionIndex: 1},
	}

	total, answered := h.countAnswerProgress(context.TODO(), "inst-1", sessions, "creator-1", 0)

	assert.Equal(t, 3, total)
	assert.Equal(t, 2, answered)
}

func TestCountAnswerProgress_CreatorExcluded(t *testing.T) {
	h := &Hub{}

	sessions := []*models.GameSession{
		{UserID: "creator-1", Status: constants.SessionStatusInProgress, CurrentQuestionIndex: 5},
	}

	total, answered := h.countAnswerProgress(context.TODO(), "inst-1", sessions, "creator-1", 0)
	assert.Equal(t, 0, total)
	assert.Equal(t, 0, answered)
}

func TestCountAnswerProgress_NoSessions(t *testing.T) {
	h := &Hub{}

	total, answered := h.countAnswerProgress(context.TODO(), "inst-1", []*models.GameSession{}, "creator-1", 0)
	assert.Equal(t, 0, total)
	assert.Equal(t, 0, answered)
}

func TestCountAnswerProgress_JoinedSessionsNotCounted(t *testing.T) {
	h := &Hub{}

	sessions := []*models.GameSession{
		{UserID: "user-1", Status: constants.SessionStatusJoined, CurrentQuestionIndex: 0},
		{UserID: "user-2", Status: constants.SessionStatusInProgress, CurrentQuestionIndex: 0},
	}

	total, answered := h.countAnswerProgress(context.TODO(), "inst-1", sessions, "creator", 0)
	assert.Equal(t, 1, total)
	assert.Equal(t, 0, answered)
}

func TestRedisStartTimeKey_Sync(t *testing.T) {
	key := redisStartTimeKey(constants.QuizTypeSync, "inst-1", "user-1", 3)
	assert.Equal(t, "quiz:inst-1:question:3:start", key)
	assert.NotContains(t, key, "user-1")
}

func TestRedisStartTimeKey_Async(t *testing.T) {
	key := redisStartTimeKey(constants.QuizTypeAsync, "inst-1", "user-1", 3)
	assert.Equal(t, "quiz:inst-1:user:user-1:question:3:start", key)
}

func TestTimerKey_Sync(t *testing.T) {
	key := timerKey(constants.QuizTypeSync, "inst-1", "user-1", 2)
	assert.Equal(t, "inst-1:2", key)
}

func TestTimerKey_Async(t *testing.T) {
	key := timerKey(constants.QuizTypeAsync, "inst-1", "user-1", 2)
	assert.Equal(t, "inst-1:user-1:2", key)
}

func TestRedisQuizDataKey(t *testing.T) {
	assert.Equal(t, "quiz:inst-1:data", redisQuizDataKey("inst-1"))
}

func TestRedisCurrentIndexKey(t *testing.T) {
	assert.Equal(t, "quiz:inst-1:current_index", redisCurrentIndexKey("inst-1"))
}

func TestRedisTotalParticipantsKey(t *testing.T) {
	assert.Equal(t, "quiz:inst-1:total_participants", redisTotalParticipantsKey("inst-1"))
}

func TestQuestionDataFromModel(t *testing.T) {
	q := models.Question{
		ID: "q-1", Text: "What?", Type: "single",
		Options: []string{"A", "B"}, OrderIndex: 0, MaxScore: 10, TimeLimitSec: 30,
	}
	qd := questionDataFromModel(q)
	assert.Equal(t, "q-1", qd.ID)
	assert.Equal(t, "What?", qd.Text)
	assert.Equal(t, []string{"A", "B"}, qd.Options)
	assert.Equal(t, 10, qd.MaxScore)
}

func TestQuestionPayloadFromModel(t *testing.T) {
	q := models.Question{ID: "q-1", Text: "What?", Type: "open"}
	qp := questionPayloadFromModel(q, 2, 5)
	assert.Equal(t, 2, qp.QuestionIndex)
	assert.Equal(t, 5, qp.TotalQuestions)
	assert.Greater(t, qp.ServerTime, int64(0))
}

func TestUserFromProfile(t *testing.T) {
	profile := &pb.User{
		Id: "user-1", FirstName: "John", LastName: "Doe",
		Email: "john@example.com", AvatarUrl: "http://avatar.png",
	}

	user := userFromProfile(profile, true)
	assert.Equal(t, "user-1", user.UserID)
	assert.Equal(t, "John", user.FirstName)
	assert.True(t, user.IsCreator)
	assert.True(t, user.IsOnline)
}

func TestUserFromProfile_NotCreator(t *testing.T) {
	profile := &pb.User{Id: "user-2", Email: "jane@example.com"}
	user := userFromProfile(profile, false)
	assert.False(t, user.IsCreator)
}

func TestValidateAnswer_SingleChoice_JSONWrapped(t *testing.T) {
	h := newTestHub()
	correctJSON, _ := json.Marshal("2")
	assert.True(t, h.validateAnswer("2", string(correctJSON), "single"))
	assert.False(t, h.validateAnswer("1", string(correctJSON), "single"))
}

func TestValidateAnswer_MultipleChoice_OrderIndependent(t *testing.T) {
	h := newTestHub()
	correct, _ := json.Marshal([]int{0, 1, 3})
	user, _ := json.Marshal([]int{3, 0, 1})
	assert.True(t, h.validateAnswer(string(user), string(correct), "multiple"))
}

func TestValidateAnswer_MultipleChoice_Duplicate(t *testing.T) {
	h := newTestHub()
	correct, _ := json.Marshal([]int{0, 1})
	user, _ := json.Marshal([]int{0, 0})
	assert.True(t, h.validateAnswer(string(user), string(correct), "multiple"))
}

func TestValidateAnswer_Open_Unicode(t *testing.T) {
	h := newTestHub()
	assert.True(t, h.validateAnswer("Привет", `"привет"`, "open"))
}
