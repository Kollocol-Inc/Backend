package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"game-service/internal/constants"
	"game-service/internal/models"
)

func (h *Hub) handleStartQuiz(client *Client) {
	log.Printf("Starting quiz for instance %s", client.InstanceID)
	ctx := context.Background()

	quizData, err := h.getQuizData(ctx, client.InstanceID)
	if err != nil {
		log.Printf("Failed to get quiz data: %v", err)
		client.SendError("Failed to start quiz")
		return
	}

	if err := h.sessionRepo.UpdateSessionStatus(ctx, client.InstanceID, client.UserID, constants.SessionStatusInProgress); err != nil {
		log.Printf("Failed to update session status: %v", err)
	}

	if err := h.updateInstanceStatus(ctx, client.InstanceID, constants.InstanceStatusActive); err != nil {
		log.Printf("Failed to update instance status: %v", err)
	}

	if quizData.QuizType == constants.QuizTypeSync {
		h.freezeParticipants(ctx, client.InstanceID)

		h.broadcastToInstance(client.InstanceID, MessageTypeQuizStarted, QuizStartedPayload{
			QuizType: quizData.QuizType,
		})

		h.mu.RLock()
		clients := h.clients[client.InstanceID]
		h.mu.RUnlock()

		participantCount := 0
		for c := range clients {
			if err := h.sessionRepo.UpdateSessionStatus(ctx, client.InstanceID, c.UserID, constants.SessionStatusInProgress); err != nil {
				log.Printf("Failed to update session status for user %s: %v", c.UserID, err)
			}
			if !c.IsCreator {
				participantCount++
			}
		}
		h.saveTotalParticipants(ctx, client.InstanceID, participantCount)
		h.sendQuestionToAll(clients, quizData, 0)
		h.sendAnswerProgressToAll(ctx, client.InstanceID, quizData, 0)
	} else {
		client.SendMessage(MessageTypeQuizStarted, QuizStartedPayload{
			QuizType: quizData.QuizType,
		})
		h.sendQuestion(client, quizData, 0)
	}
}

func (h *Hub) handleResumeQuiz(client *Client, quizData *models.QuizData) {
	ctx := context.Background()

	session, err := h.sessionRepo.GetSession(ctx, client.InstanceID, client.UserID)
	if err != nil {
		log.Printf("Failed to get session for resume: %v", err)
		return
	}

	if quizData.QuizType == constants.QuizTypeAsync {
		h.sendQuestion(client, quizData, session.CurrentQuestionIndex)
		return
	}

	currentIndex := h.getCurrentQuestionIndex(ctx, client.InstanceID)
	if currentIndex >= len(quizData.Questions) {
		h.finishQuiz(client)
		return
	}

	stKey := redisStartTimeKey(quizData.QuizType, client.InstanceID, client.UserID, currentIndex)
	startTime := h.getStartTimeMs(ctx, stKey)
	isTimeExpired := h.isQuestionTimeExpired(quizData.Questions[currentIndex], startTime)

	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, client.InstanceID)
	if err != nil {
		log.Printf("Failed to get sessions for resume: %v", err)
		return
	}
	total, answered := h.countAnswerProgress(ctx, client.InstanceID, sessions, quizData.CreatedBy, currentIndex)
	allAnswered := total > 0 && answered >= total

	qd := questionPayloadFromModel(quizData.Questions[currentIndex], currentIndex, len(quizData.Questions))

	if client.IsCreator {
		if isTimeExpired || allAnswered {
			h.sendLeaderboardToClient(ctx, client, quizData, currentIndex, qd)
			client.SendMessage(MessageTypeWaitingForCreator, WaitingForCreatorPayload{
				QuestionIndex: currentIndex,
				Reason:        "Waiting for continue command",
			})
		} else {
			h.sendQuestion(client, quizData, currentIndex)
			h.sendAnswerProgressToAll(ctx, client.InstanceID, quizData, currentIndex)
		}
		return
	}

	if session.CurrentQuestionIndex > currentIndex {
		h.sendLeaderboardToClient(ctx, client, quizData, currentIndex, qd)
		client.SendMessage(MessageTypeWaitingForCreator, WaitingForCreatorPayload{
			QuestionIndex: currentIndex,
			Reason:        "Waiting for next question",
		})
		return
	}

	if isTimeExpired {
		h.sendLeaderboardToClient(ctx, client, quizData, currentIndex, qd)
		client.SendMessage(MessageTypeTimeExpired, TimeExpiredPayload{
			QuestionIndex: currentIndex,
		})
		client.SendMessage(MessageTypeWaitingForCreator, WaitingForCreatorPayload{
			QuestionIndex: currentIndex,
			Reason:        "Time expired",
		})
		return
	}

	h.sendQuestion(client, quizData, currentIndex)
}

func (h *Hub) isQuestionTimeExpired(question models.Question, startTime int64) bool {
	if startTime <= 0 || question.TimeLimitSec <= 0 {
		return false
	}
	elapsed := time.Now().UnixMilli() - startTime
	return elapsed > int64(question.TimeLimitSec)*1000
}

func (h *Hub) sendLeaderboardToClient(ctx context.Context, client *Client, quizData *models.QuizData, questionIndex int, question *QuestionPayload) {
	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, client.InstanceID)
	if err != nil {
		log.Printf("Failed to get sessions for leaderboard: %v", err)
		return
	}

	leaderboard := h.buildLeaderboard(ctx, sessions, quizData, questionIndex)
	questionStats := h.buildQuestionStats(sessions, quizData, questionIndex)

	canContinue := false
	if client.IsCreator {
		canContinue = h.canCreatorContinue(ctx, client.InstanceID, questionIndex, quizData)
	}

	client.SendMessage(MessageTypeLeaderboard, LeaderboardPayload{
		Leaderboard:       leaderboard,
		AnswerOptionStats: questionStats,
		CanContinue:       canContinue,
		Question:          question,
	})
}

func (h *Hub) sendLeaderboardToAll(ctx context.Context, instanceID string, quizData *models.QuizData, questionIndex int, question *QuestionPayload) {
	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, instanceID)
	if err != nil {
		log.Printf("Failed to get sessions for leaderboard: %v", err)
		return
	}

	leaderboard := h.buildLeaderboard(ctx, sessions, quizData, questionIndex)
	questionStats := h.buildQuestionStats(sessions, quizData, questionIndex)
	canContinue := h.canCreatorContinue(ctx, instanceID, questionIndex, quizData)

	h.mu.RLock()
	clients := h.clients[instanceID]
	h.mu.RUnlock()

	for client := range clients {
		payload := LeaderboardPayload{
			Leaderboard:       leaderboard,
			AnswerOptionStats: questionStats,
			Question:          question,
		}
		if client.IsCreator {
			payload.CanContinue = canContinue
		}
		client.SendMessage(MessageTypeLeaderboard, payload)
	}
}

func (h *Hub) sendAnswerProgressToAll(ctx context.Context, instanceID string, quizData *models.QuizData, questionIndex int) {
	h.mu.RLock()
	clients := h.clients[instanceID]
	h.mu.RUnlock()

	if clients == nil {
		return
	}

	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, instanceID)
	if err != nil {
		return
	}

	total, answered := h.countAnswerProgress(ctx, instanceID, sessions, quizData.CreatedBy, questionIndex)

	progress := AnswerProgressPayload{
		ParticipantsAnswered: answered,
		TotalParticipants:    total,
	}

	for client := range clients {
		client.SendMessage(MessageTypeAnswerProgress, progress)
	}
}

func (h *Hub) sendQuestionToAll(clients map[*Client]bool, quizData *models.QuizData, questionIndex int) {
	for c := range clients {
		h.sendQuestion(c, quizData, questionIndex)
	}
}

func (h *Hub) sendQuestion(client *Client, quizData *models.QuizData, questionIndex int) {
	if questionIndex >= len(quizData.Questions) {
		h.finishQuiz(client)
		return
	}

	question := quizData.Questions[questionIndex]
	ctx := context.Background()

	stKey := redisStartTimeKey(quizData.QuizType, client.InstanceID, client.UserID, questionIndex)
	if h.redisClient != nil {
		h.redisClient.GetClient().SetNX(ctx, stKey, time.Now().UnixMilli(), 1*time.Hour)
	}

	duration := h.remainingDuration(ctx, question, stKey)

	payload := QuestionPayload{
		Question: QuestionData{
			ID:           question.ID,
			Text:         question.Text,
			Type:         question.Type,
			Options:      question.Options,
			OrderIndex:   question.OrderIndex,
			MaxScore:     question.MaxScore,
			TimeLimitSec: question.TimeLimitSec,
		},
		QuestionIndex:  questionIndex,
		TotalQuestions: len(quizData.Questions),
		ServerTime:     time.Now().UnixMilli(),
	}

	if question.TimeLimitSec > 0 {
		payload.TimeLimitMs = duration.Milliseconds()
	}

	client.SendMessage(MessageTypeQuestion, payload)

	if question.TimeLimitSec > 0 {
		h.startQuestionTimer(client, quizData, questionIndex, duration)
	}
}

func (h *Hub) remainingDuration(ctx context.Context, question models.Question, stKey string) time.Duration {
	if question.TimeLimitSec <= 0 {
		return 0
	}

	fullDuration := time.Duration(question.TimeLimitSec) * time.Second
	startTime := h.getStartTimeMs(ctx, stKey)
	if startTime <= 0 {
		return fullDuration
	}

	elapsed := time.Now().UnixMilli() - startTime
	remainingMs := int64(question.TimeLimitSec)*1000 - elapsed
	if remainingMs <= 0 {
		return 0
	}
	return time.Duration(remainingMs) * time.Millisecond
}

func (h *Hub) startQuestionTimer(client *Client, quizData *models.QuizData, questionIndex int, duration time.Duration) {
	tk := timerKey(quizData.QuizType, client.InstanceID, client.UserID, questionIndex)

	h.timerMu.Lock()
	if timer, ok := h.questionTimers[tk]; ok {
		timer.Stop()
	}

	timer := time.AfterFunc(duration, func() {
		h.handleQuestionTimeout(client, quizData, questionIndex)
	})
	h.questionTimers[tk] = timer
	h.timerMu.Unlock()
}

func (h *Hub) handleQuestionTimeout(client *Client, quizData *models.QuizData, questionIndex int) {
	log.Printf("Question timeout: instance=%s, user=%s, question=%d",
		client.InstanceID, client.UserID, questionIndex)

	ctx := context.Background()

	if quizData.QuizType == constants.QuizTypeSync {
		h.broadcastToParticipants(client.InstanceID, MessageTypeTimeExpired, TimeExpiredPayload{
			QuestionIndex: questionIndex,
		})
		qd := questionPayloadFromModel(quizData.Questions[questionIndex], questionIndex, len(quizData.Questions))
		h.sendLeaderboardToAll(ctx, client.InstanceID, quizData, questionIndex, qd)
	} else {
		client.SendMessage(MessageTypeTimeExpired, TimeExpiredPayload{
			QuestionIndex: questionIndex,
		})
		time.AfterFunc(200*time.Millisecond, func() {
			h.sendQuestion(client, quizData, questionIndex+1)
		})
	}
}

func (h *Hub) handleAnswer(client *Client, payload any) {
	ctx := context.Background()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		client.SendError("Invalid answer format")
		return
	}

	var answerPayload AnswerPayload
	if err := json.Unmarshal(payloadBytes, &answerPayload); err != nil {
		client.SendError("Invalid answer format")
		return
	}

	quizData, err := h.getQuizData(ctx, client.InstanceID)
	if err != nil {
		log.Printf("Failed to get quiz data: %v", err)
		client.SendError("Failed to process answer")
		return
	}

	var question *models.Question
	var questionIndex int
	for i, q := range quizData.Questions {
		if q.ID == answerPayload.QuestionID {
			question = &q
			questionIndex = i
			break
		}
	}

	if question == nil {
		client.SendError("Question not found")
		return
	}

	stKey := redisStartTimeKey(quizData.QuizType, client.InstanceID, client.UserID, questionIndex)
	startTime := h.getStartTimeMs(ctx, stKey)
	if startTime == 0 {
		startTime = time.Now().UnixMilli()
	}

	timeSpentMs := max(time.Now().UnixMilli()-startTime, 0)

	if question.TimeLimitSec > 0 {
		if timeSpentMs > int64(question.TimeLimitSec)*1000 {
			client.SendError("Time limit exceeded")
			return
		}
	}

	isCorrect := h.validateAnswer(answerPayload.Answer, question.CorrectAnswer, question.Type)

	score := 0
	if isCorrect {
		score = h.calculateScore(question.MaxScore, timeSpentMs, int64(question.TimeLimitSec)*1000)
	}

	session, err := h.sessionRepo.GetSession(ctx, client.InstanceID, client.UserID)
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		client.SendError("Failed to save answer")
		return
	}

	var answers []models.Answer
	if err := json.Unmarshal([]byte(session.Answers), &answers); err != nil {
		log.Printf("Failed to parse answers: %v", err)
		answers = []models.Answer{}
	}

	answers = append(answers, models.Answer{
		QuestionID:  answerPayload.QuestionID,
		Answer:      answerPayload.Answer,
		IsCorrect:   isCorrect,
		Score:       score,
		TimeSpentMs: timeSpentMs,
	})

	answersJSON, _ := json.Marshal(answers)
	session.Answers = string(answersJSON)
	session.Score += score
	session.CurrentQuestionIndex = questionIndex + 1

	if err := h.sessionRepo.UpdateSession(ctx, session); err != nil {
		log.Printf("Failed to update session: %v", err)
		client.SendError("Failed to save answer")
		return
	}

	client.SendMessage(MessageTypeAnswerResult, AnswerResultPayload{
		IsCorrect:   isCorrect,
		Score:       score,
		TimeSpentMs: timeSpentMs,
		TotalScore:  session.Score,
	})

	if quizData.QuizType == constants.QuizTypeSync {
		h.sendLeaderboardToCreator(ctx, client.InstanceID, quizData)

		sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, client.InstanceID)
		if err != nil {
			log.Printf("Failed to get sessions: %v", err)
			return
		}
		total, answered := h.countAnswerProgress(ctx, client.InstanceID, sessions, quizData.CreatedBy, questionIndex)
		allAnswered := total > 0 && answered >= total

		if allAnswered {
			h.cancelQuestionTimer(timerKey(quizData.QuizType, client.InstanceID, client.UserID, questionIndex))
			qd := questionPayloadFromModel(quizData.Questions[questionIndex], questionIndex, len(quizData.Questions))
			h.sendLeaderboardToAll(ctx, client.InstanceID, quizData, questionIndex, qd)
		} else {
			h.sendAnswerProgressToAll(ctx, client.InstanceID, quizData, questionIndex)
		}
	} else {
		h.cancelQuestionTimer(timerKey(quizData.QuizType, client.InstanceID, client.UserID, questionIndex))
		time.AfterFunc(200*time.Millisecond, func() {
			h.sendQuestion(client, quizData, questionIndex+1)
		})
	}
}

func (h *Hub) handleContinue(client *Client) {
	ctx := context.Background()

	quizData, err := h.getQuizData(ctx, client.InstanceID)
	if err != nil {
		log.Printf("Failed to get quiz data: %v", err)
		client.SendError("Failed to continue")
		return
	}

	nextQuestionIndex := h.getCurrentQuestionIndex(ctx, client.InstanceID) + 1

	h.mu.RLock()
	clients := h.clients[client.InstanceID]
	h.mu.RUnlock()

	if nextQuestionIndex >= len(quizData.Questions) {
		log.Printf("Quiz %s finished, updating status", client.InstanceID)
		if err := h.updateInstanceStatus(ctx, client.InstanceID, constants.InstanceStatusPendingReview); err != nil {
			log.Printf("Failed to update instance status: %v", err)
		}

		for c := range clients {
			h.finishQuiz(c)
		}
		return
	}

	if quizData.QuizType == constants.QuizTypeSync && h.redisClient != nil {
		h.redisClient.Set(ctx, redisCurrentIndexKey(client.InstanceID), nextQuestionIndex, 24*time.Hour)
	}

	h.sendQuestionToAll(clients, quizData, nextQuestionIndex)
	h.sendAnswerProgressToAll(ctx, client.InstanceID, quizData, nextQuestionIndex)
}

func (h *Hub) canCreatorContinue(ctx context.Context, instanceID string, questionIndex int, quizData *models.QuizData) bool {
	if questionIndex >= len(quizData.Questions) || quizData.QuizType != constants.QuizTypeSync {
		return false
	}

	stKey := redisStartTimeKey(quizData.QuizType, instanceID, "", questionIndex)
	startTime := h.getStartTimeMs(ctx, stKey)
	if h.isQuestionTimeExpired(quizData.Questions[questionIndex], startTime) {
		return true
	}

	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, instanceID)
	if err != nil {
		return false
	}
	total, answered := h.countAnswerProgress(ctx, instanceID, sessions, quizData.CreatedBy, questionIndex)
	return total > 0 && answered >= total
}

func (h *Hub) buildQuestionStats(sessions []*models.GameSession, quizData *models.QuizData, questionIndex int) []AnswerOptionStats {
	if questionIndex >= len(quizData.Questions) {
		return nil
	}

	question := quizData.Questions[questionIndex]
	if question.Type == constants.QuestionTypeOpen {
		return nil
	}

	if question.Type != constants.QuestionTypeSingle && question.Type != constants.QuestionTypeMultiple {
		return nil
	}

	optionStats := make([]AnswerOptionStats, len(question.Options))
	for i, option := range question.Options {
		optionStats[i] = AnswerOptionStats{Option: option}
	}

	for _, session := range sessions {
		if session.UserID == quizData.CreatedBy {
			continue
		}

		var answers []models.Answer
		if err := json.Unmarshal([]byte(session.Answers), &answers); err != nil {
			continue
		}

		for _, answer := range answers {
			if answer.QuestionID != question.ID {
				continue
			}
			switch question.Type {
			case constants.QuestionTypeSingle:
				var idx int
				if _, err := fmt.Sscanf(answer.Answer, "%d", &idx); err == nil && idx >= 0 && idx < len(optionStats) {
					optionStats[idx].Count++
				}
			case constants.QuestionTypeMultiple:
				var indices []int
				if err := json.Unmarshal([]byte(answer.Answer), &indices); err == nil {
					for _, idx := range indices {
						if idx >= 0 && idx < len(optionStats) {
							optionStats[idx].Count++
						}
					}
				}
			}
			break
		}
	}

	return optionStats
}

func (h *Hub) buildLeaderboard(ctx context.Context, sessions []*models.GameSession, quizData *models.QuizData, questionIndex int) []LeaderboardEntry {
	var leaderboard []LeaderboardEntry
	rank := 1
	for _, session := range sessions {
		if session.UserID == quizData.CreatedBy {
			continue
		}

		userProfile, err := h.userClient.GetProfile(ctx, session.UserID)
		if err != nil {
			log.Printf("Failed to get user profile %s: %v", session.UserID, err)
			continue
		}

		user := userFromProfile(userProfile, false)
		user.IsOnline = h.isUserOnline(session.InstanceID, session.UserID)

		leaderboard = append(leaderboard, LeaderboardEntry{
			Rank:       rank,
			User:       user,
			Score:      session.Score,
			IsAnswered: session.CurrentQuestionIndex > questionIndex,
		})
		rank++
	}

	return leaderboard
}

func (h *Hub) finishQuiz(client *Client) {
	ctx := context.Background()

	session, err := h.sessionRepo.GetSession(ctx, client.InstanceID, client.UserID)
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		client.SendError("Failed to finish quiz")
		return
	}

	session.Status = constants.SessionStatusFinished
	session.FinishedAt.Valid = true
	session.FinishedAt.Time = time.Now()

	if err := h.sessionRepo.UpdateSession(ctx, session); err != nil {
		log.Printf("Failed to update session: %v", err)
	}

	log.Printf("Quiz finished: user=%s, instance=%s, score=%d", client.UserID, client.InstanceID, session.Score)

	quizData, err := h.getQuizData(ctx, client.InstanceID)
	if err != nil {
		log.Printf("Failed to get quiz data for finish: %v", err)
		client.SendMessage(MessageTypeQuizFinished, QuizFinishedPayload{
			FinalScore: session.Score,
		})
		return
	}

	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, client.InstanceID)
	if err != nil {
		log.Printf("Failed to get sessions for finish: %v", err)
		client.SendMessage(MessageTypeQuizFinished, QuizFinishedPayload{
			FinalScore: session.Score,
		})
		return
	}

	leaderboard := h.buildLeaderboard(ctx, sessions, quizData, len(quizData.Questions)-1)
	rank := 0
	score := 0
	for _, entry := range leaderboard {
		if entry.User.UserID == client.UserID {
			rank = entry.Rank
			score = entry.Score
			break
		}
	}

	client.SendMessage(MessageTypeQuizFinished, QuizFinishedPayload{
		FinalScore: score,
		Rank:       rank,
	})
}
