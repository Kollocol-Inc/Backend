package websocket

import (
	"context"
	"fmt"
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

func timerKey(quizType, instanceID, userID string, questionIndex int) string {
	if quizType == constants.QuizTypeSync {
		return fmt.Sprintf("%s:%d", instanceID, questionIndex)
	}
	return fmt.Sprintf("%s:%s:%d", instanceID, userID, questionIndex)
}

func userFromProfile(p *pb.User, isCreator bool) User {
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
