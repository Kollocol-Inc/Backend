package websocket

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"time"

	"game-service/internal/constants"
	"game-service/internal/models"
	pb "game-service/proto"
)

func redisQuizDataKey(instanceID string) string {
	return fmt.Sprintf("quiz:%s:data", instanceID)
}

func redisCurrentIndexKey(instanceID string) string {
	return fmt.Sprintf("quiz:%s:current_index", instanceID)
}

func redisTotalParticipantsKey(instanceID string) string {
	return fmt.Sprintf("quiz:%s:total_participants", instanceID)
}

func redisStartTimeKey(quizType, instanceID, userID string, questionIndex int) string {
	if quizType == constants.QuizTypeSync {
		return fmt.Sprintf("quiz:%s:question:%d:start", instanceID, questionIndex)
	}
	return fmt.Sprintf("quiz:%s:user:%s:question:%d:start", instanceID, userID, questionIndex)
}

func redisDeletedKey(instanceID string) string {
	return fmt.Sprintf("quiz:%s:deleted", instanceID)
}

func timerKey(quizType, instanceID, userID string, questionIndex int) string {
	if quizType == constants.QuizTypeSync {
		return fmt.Sprintf("%s:%d", instanceID, questionIndex)
	}
	return fmt.Sprintf("%s:%s:%d", instanceID, userID, questionIndex)
}

func userFromProfile(p *pb.User, isCreator bool) User {
	if p == nil {
		return User{
			IsCreator: isCreator,
			IsOnline:  true,
		}
	}
	return User{
		UserID:    p.Id,
		FirstName: p.FirstName,
		LastName:  p.LastName,
		Email:     p.Email,
		AvatarURL: p.AvatarUrl,
		IsCreator: isCreator,
		IsOnline:  true,
	}
}

func (h *Hub) getCurrentQuestionIndex(ctx context.Context, instanceID string) int {
	var idx int
	if h.redisClient != nil {
		key := redisCurrentIndexKey(instanceID)
		if s, err := h.redisClient.Get(ctx, key); err == nil {
			fmt.Sscanf(s, "%d", &idx)
		}
	}
	return idx
}

func (h *Hub) getStartTimeMs(ctx context.Context, key string) int64 {
	if h.redisClient == nil {
		return 0
	}
	s, err := h.redisClient.Get(ctx, key)
	if err != nil || s == "" {
		return 0
	}
	var ts int64
	fmt.Sscanf(s, "%d", &ts)
	return ts
}

func questionDataFromModel(q models.Question) *QuestionData {
	return &QuestionData{
		ID:           q.ID,
		Text:         q.Text,
		Type:         q.Type,
		Options:      q.Options,
		OrderIndex:   q.OrderIndex,
		MaxScore:     q.MaxScore,
		TimeLimitSec: q.TimeLimitSec,
	}
}

func questionPayloadFromModel(q models.Question, questionIndex int, totalQuestions int) *QuestionPayload {
	return &QuestionPayload{
		Question:       *questionDataFromModel(q),
		QuestionIndex:  questionIndex,
		TotalQuestions: totalQuestions,
		ServerTime:     time.Now().UnixMilli(),
	}
}

func (h *Hub) saveTotalParticipants(ctx context.Context, instanceID string, count int) {
	if h.redisClient != nil {
		h.redisClient.Set(ctx, redisTotalParticipantsKey(instanceID), count, 24*time.Hour)
	}
}

func (h *Hub) getTotalParticipants(ctx context.Context, instanceID string) int {
	if h.redisClient == nil {
		return 0
	}
	s, err := h.redisClient.Get(ctx, redisTotalParticipantsKey(instanceID))
	if err != nil || s == "" {
		return 0
	}
	var count int
	fmt.Sscanf(s, "%d", &count)
	return count
}

func (h *Hub) isInstanceDeleted(ctx context.Context, instanceID string) bool {
	if h.redisClient == nil {
		return false
	}
	val, err := h.redisClient.Get(ctx, redisDeletedKey(instanceID))
	return err == nil && val == "true"
}

func (h *Hub) cleanRedisKeys(ctx context.Context, instanceID string) {
	h.redisClient.Delete(ctx,
		redisQuizDataKey(instanceID),
		redisCurrentIndexKey(instanceID),
		redisTotalParticipantsKey(instanceID),
	)
}

func shuffledIndices(instanceID, userID string, n int) []int {
	hasher := fnv.New64a()
	hasher.Write([]byte(instanceID))
	hasher.Write([]byte{0})
	hasher.Write([]byte(userID))
	r := rand.New(rand.NewSource(int64(hasher.Sum64())))

	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	r.Shuffle(n, func(i, j int) { perm[i], perm[j] = perm[j], perm[i] })
	return perm
}

func questionsForClient(client *Client, quizData *models.QuizData) []models.Question {
	if quizData.QuizType != constants.QuizTypeAsync || !quizData.Settings.QuestionsRandomOrder {
		return quizData.Questions
	}

	perm := shuffledIndices(client.InstanceID, client.UserID, len(quizData.Questions))
	out := make([]models.Question, len(perm))
	for i, src := range perm {
		out[i] = quizData.Questions[src]
	}
	return out
}

func asyncDeadlineExpired(quizData *models.QuizData, session *models.GameSession) bool {
	now := time.Now()

	if quizData.DeadlineMs > 0 && now.UnixMilli() > quizData.DeadlineMs {
		return true
	}

	budgetSec := 0
	for _, q := range quizData.Questions {
		budgetSec += q.TimeLimitSec
	}
	if budgetSec > 0 && !session.StartedAt.IsZero() {
		if now.After(session.StartedAt.Add(time.Duration(budgetSec) * time.Second)) {
			return true
		}
	}
	return false
}

func (h *Hub) countAnswerProgress(ctx context.Context, instanceID string, sessions []*models.GameSession, creatorID string, questionIndex int) (total, answered int) {
	total = h.getTotalParticipants(ctx, instanceID)

	for _, session := range sessions {
		if session.UserID == creatorID {
			continue
		}
		if session.CurrentQuestionIndex > questionIndex {
			answered++
		}
	}

	if total == 0 {
		for _, session := range sessions {
			if session.UserID == creatorID {
				continue
			}
			if session.Status == constants.SessionStatusInProgress || session.Status == constants.SessionStatusFinished {
				total++
			}
		}
	}
	return
}
