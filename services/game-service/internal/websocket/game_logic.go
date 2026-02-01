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
	log.Printf("Got quiz data for instance %s", client.InstanceID)

	if err := h.sessionRepo.UpdateSessionStatus(ctx, client.InstanceID, client.UserID, constants.SessionStatusInProgress); err != nil {
		log.Printf("Failed to update session status: %v", err)
	}
	log.Printf("Updated session status for creator %s", client.UserID)

	if err := h.updateInstanceStatus(ctx, client.InstanceID, constants.InstanceStatusActive); err != nil {
		log.Printf("Failed to update instance status: %v", err)
	}

	if quizData.QuizType == constants.QuizTypeSync {
		h.broadcastToInstance(client.InstanceID, MessageTypeQuizStarted, QuizStartedPayload{
			QuizType: quizData.QuizType,
		})

		h.mu.RLock()
		clients := h.clients[client.InstanceID]
		h.mu.RUnlock()

		for c := range clients {
			if !c.IsCreator {
				if err := h.sessionRepo.UpdateSessionStatus(ctx, client.InstanceID, c.UserID, constants.SessionStatusInProgress); err != nil {
					log.Printf("Failed to update session status for user %s: %v", c.UserID, err)
				}
				h.sendQuestion(c, quizData, 0)
			}
		}
		h.notifyCreatorProgress(ctx, client.InstanceID, 0)
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
	} else {
		var currentIndex int
		if h.redisClient != nil {
			indexKey := fmt.Sprintf("quiz:%s:current_index", client.InstanceID)
			indexStr, err := h.redisClient.Get(ctx, indexKey)
			if err == nil {
				fmt.Sscanf(indexStr, "%d", &currentIndex)
			}
		}

		startTimeKey := fmt.Sprintf("quiz:%s:question:%d:start", client.InstanceID, currentIndex)
		var startTime int64
		if h.redisClient != nil {
			startTimeStr, err := h.redisClient.Get(ctx, startTimeKey)
			if err == nil {
				fmt.Sscanf(startTimeStr, "%d", &startTime)
			}
		}

		isTimeExpired := false
		if startTime > 0 {
			question := quizData.Questions[currentIndex]
			if question.TimeLimitSec > 0 {
				elapsed := time.Now().UnixMilli() - startTime
				if elapsed > int64(question.TimeLimitSec)*1000 {
					isTimeExpired = true
				}
			}
		}

		allAnswered := h.checkAllParticipantsAnswered(ctx, client.InstanceID, currentIndex)

		if client.IsCreator {
			if isTimeExpired || allAnswered {
				h.sendLeaderboardToClient(client, currentIndex)
				client.SendMessage(MessageTypeWaitingForCreator, WaitingForCreatorPayload{
					QuestionIndex: currentIndex,
					Reason:        "Waiting for continue command",
				})
			} else {
				h.notifyCreatorProgress(ctx, client.InstanceID, currentIndex)
			}
			return
		}

		if session.CurrentQuestionIndex > currentIndex {
			h.sendLeaderboardToClient(client, currentIndex)
			client.SendMessage(MessageTypeWaitingForCreator, WaitingForCreatorPayload{
				QuestionIndex: currentIndex,
				Reason:        "Waiting for next question",
			})
		} else {
			if isTimeExpired {
				h.sendLeaderboardToClient(client, currentIndex)
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
	}
}

func (h *Hub) sendLeaderboardToClient(client *Client, questionIndex int) {
	ctx := context.Background()
	leaderboard := h.getLeaderboard(ctx, client.InstanceID)
	client.SendMessage(MessageTypeLeaderboard, LeaderboardPayload{
		Leaderboard: leaderboard,
	})
}

func (h *Hub) notifyCreatorProgress(ctx context.Context, instanceID string, questionIndex int) {
	h.mu.RLock()
	clients := h.clients[instanceID]
	h.mu.RUnlock()

	var creator *Client
	for c := range clients {
		if c.IsCreator {
			creator = c
			break
		}
	}

	if creator == nil {
		return
	}

	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, instanceID)
	if err != nil {
		return
	}

	participantCount := 0
	answeredCount := 0

	for _, session := range sessions {
		if session.UserID == creator.UserID {
			continue
		}

		if session.Status == constants.SessionStatusInProgress || session.Status == constants.SessionStatusFinished {
			participantCount++
			if session.CurrentQuestionIndex > questionIndex {
				answeredCount++
			}
		}
	}

	creator.SendMessage(MessageTypeWaitingForCreator, WaitingForCreatorPayload{
		QuestionIndex: questionIndex,
		Reason:        fmt.Sprintf("Question in progress: %d/%d answered", answeredCount, participantCount),
	})
}

func (h *Hub) sendQuestion(client *Client, quizData *models.QuizData, questionIndex int) {
	log.Printf("Sending question %d to user %s (creator=%v, type=%s)", questionIndex, client.UserID, client.IsCreator, quizData.QuizType)

	if questionIndex >= len(quizData.Questions) {
		h.finishQuiz(client)
		return
	}

	question := quizData.Questions[questionIndex]
	ctx := context.Background()

	var startTimeKey string
	if quizData.QuizType == constants.QuizTypeSync {
		startTimeKey = fmt.Sprintf("quiz:%s:question:%d:start", client.InstanceID, questionIndex)
	} else {
		startTimeKey = fmt.Sprintf("quiz:%s:user:%s:question:%d:start", client.InstanceID, client.UserID, questionIndex)
	}

	if h.redisClient != nil {
		h.redisClient.GetClient().SetNX(ctx, startTimeKey, time.Now().UnixMilli(), 1*time.Hour)
	}

	duration := time.Duration(0)
	if question.TimeLimitSec > 0 {
		duration = time.Duration(question.TimeLimitSec) * time.Second

		if h.redisClient != nil {
			startTimeStr, err := h.redisClient.Get(ctx, startTimeKey)
			if err == nil {
				var startTime int64
				fmt.Sscanf(startTimeStr, "%d", &startTime)
				if startTime > 0 {
					elapsed := time.Now().UnixMilli() - startTime
					remainingMs := int64(question.TimeLimitSec)*1000 - elapsed
					if remainingMs <= 0 {
						duration = 0
					} else {
						duration = time.Duration(remainingMs) * time.Millisecond
					}
				}
			}
		}
	}

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

	if quizData.QuizType == constants.QuizTypeSync && !client.IsCreator {
		log.Printf("Sending question payload to participant %s", client.UserID)
		client.SendMessage(MessageTypeQuestion, payload)
	} else if quizData.QuizType == constants.QuizTypeAsync {
		client.SendMessage(MessageTypeQuestion, payload)
	} else {
		log.Printf("Skipping question send for user %s (sync creator)", client.UserID)
	}

	if question.TimeLimitSec > 0 {
		h.startQuestionTimer(client, quizData, questionIndex, duration)
	}
}

func (h *Hub) startQuestionTimer(client *Client, quizData *models.QuizData, questionIndex int, duration time.Duration) {
	var timerKey string
	if quizData.QuizType == constants.QuizTypeSync {
		timerKey = fmt.Sprintf("%s:%d", client.InstanceID, questionIndex)
	} else {
		timerKey = fmt.Sprintf("%s:%s:%d", client.InstanceID, client.UserID, questionIndex)
	}

	h.timerMu.Lock()
	if timer, ok := h.questionTimers[timerKey]; ok {
		timer.Stop()
	}

	timer := time.AfterFunc(duration, func() {
		h.handleQuestionTimeout(client, quizData, questionIndex)
	})
	h.questionTimers[timerKey] = timer
	h.timerMu.Unlock()
}

func (h *Hub) handleQuestionTimeout(client *Client, quizData *models.QuizData, questionIndex int) {
	log.Printf("Question timeout: instance=%s, user=%s, question=%d",
		client.InstanceID, client.UserID, questionIndex)

	if quizData.QuizType == constants.QuizTypeSync {
		h.broadcastToParticipants(client.InstanceID, MessageTypeTimeExpired, TimeExpiredPayload{
			QuestionIndex: questionIndex,
		})

		h.showLeaderboardAndWait(client.InstanceID, questionIndex)
	} else {
		client.SendMessage(MessageTypeTimeExpired, TimeExpiredPayload{
			QuestionIndex: questionIndex,
		})

		time.Sleep(200 * time.Millisecond)
		h.sendQuestion(client, quizData, questionIndex+1)
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

	var startTimeKey string
	if quizData.QuizType == constants.QuizTypeSync {
		startTimeKey = fmt.Sprintf("quiz:%s:question:%d:start", client.InstanceID, questionIndex)
	} else {
		startTimeKey = fmt.Sprintf("quiz:%s:user:%s:question:%d:start", client.InstanceID, client.UserID, questionIndex)
	}

	var startTime int64
	if h.redisClient != nil {
		startTimeStr, err := h.redisClient.Get(ctx, startTimeKey)
		if err != nil {
			log.Printf("Failed to get start time: %v", err)
			startTime = time.Now().UnixMilli()
		} else {
			fmt.Sscanf(startTimeStr, "%d", &startTime)
		}
	} else {
		startTime = time.Now().UnixMilli()
	}

	timeSpentMs := max(time.Now().UnixMilli() - startTime, 0)

	if question.TimeLimitSec > 0 {
		timeLimitMs := int64(question.TimeLimitSec) * 1000
		if timeSpentMs > timeLimitMs {
			client.SendError("Time limit exceeded")
			return
		}
	}

	isCorrect := h.validateAnswer(answerPayload.Answer, question.CorrectAnswer)

	score := 0
	if isCorrect {
		timeLimitMs := int64(question.TimeLimitSec) * 1000
		score = h.calculateScore(question.MaxScore, timeSpentMs, timeLimitMs)
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
		h.updateLeaderboard(ctx, client.InstanceID)
	}

	if quizData.QuizType == constants.QuizTypeSync {
		allAnswered := h.checkAllParticipantsAnswered(ctx, client.InstanceID, questionIndex)
		if allAnswered {
			timerKey := fmt.Sprintf("%s:%d", client.InstanceID, questionIndex)
			h.cancelQuestionTimer(timerKey)

			h.showLeaderboardAndWait(client.InstanceID, questionIndex)
		} else {
			h.notifyCreatorProgress(ctx, client.InstanceID, questionIndex)
		}
	} else {
		timerKey := fmt.Sprintf("%s:%s:%d", client.InstanceID, client.UserID, questionIndex)
		h.cancelQuestionTimer(timerKey)

		time.Sleep(200 * time.Millisecond)
		h.sendQuestion(client, quizData, questionIndex+1)
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

	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, client.InstanceID)
	if err != nil || len(sessions) == 0 {
		client.SendError("No active sessions")
		return
	}

	nextQuestionIndex := 0
	for _, s := range sessions {
		if s.UserID != quizData.CreatedBy {
			nextQuestionIndex = s.CurrentQuestionIndex
			break
		}
	}

	h.mu.RLock()
	clients := h.clients[client.InstanceID]
	h.mu.RUnlock()

	if nextQuestionIndex >= len(quizData.Questions) {
		log.Printf("Quiz %s finished, updating status", client.InstanceID)
		if err := h.updateInstanceStatus(ctx, client.InstanceID, constants.InstanceStatusFinished); err != nil {
			log.Printf("Failed to update instance status: %v", err)
		}

		for c := range clients {
			h.finishQuiz(c)
		}
		return
	}

	if quizData.QuizType == constants.QuizTypeSync && h.redisClient != nil {
		indexKey := fmt.Sprintf("quiz:%s:current_index", client.InstanceID)
		h.redisClient.Set(ctx, indexKey, nextQuestionIndex, 24*time.Hour)
	}

	for c := range clients {
		if !c.IsCreator {
			h.sendQuestion(c, quizData, nextQuestionIndex)
		}
	}
	h.notifyCreatorProgress(ctx, client.InstanceID, nextQuestionIndex)
}

func (h *Hub) showLeaderboardAndWait(instanceID string, questionIndex int) {
	ctx := context.Background()

	leaderboard := h.getLeaderboard(ctx, instanceID)

	h.broadcastToInstance(instanceID, MessageTypeLeaderboard, LeaderboardPayload{
		Leaderboard: leaderboard,
	})

	h.broadcastToParticipants(instanceID, MessageTypeWaitingForCreator, WaitingForCreatorPayload{
		QuestionIndex: questionIndex,
		Reason:        "All participants answered",
	})
}

func (h *Hub) checkAllParticipantsAnswered(ctx context.Context, instanceID string, questionIndex int) bool {
	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, instanceID)
	if err != nil {
		return false
	}

	quizData, err := h.getQuizData(ctx, instanceID)
	if err != nil {
		log.Printf("Failed to get quiz data in checkAllParticipantsAnswered: %v", err)
		return false
	}

	participantCount := 0
	answeredCount := 0

	for _, session := range sessions {
		if session.UserID == quizData.CreatedBy {
			continue
		}

		if session.Status == constants.SessionStatusInProgress || session.Status == constants.SessionStatusFinished {
			participantCount++
			if session.CurrentQuestionIndex > questionIndex {
				answeredCount++
			}
		}
	}

	if participantCount == 0 {
		return false
	}

	return answeredCount >= participantCount
}

func (h *Hub) updateLeaderboard(ctx context.Context, instanceID string) {
	leaderboard := h.getLeaderboard(ctx, instanceID)
	h.broadcastToInstance(instanceID, MessageTypeLeaderboard, LeaderboardPayload{
		Leaderboard: leaderboard,
	})
}

func (h *Hub) getLeaderboard(ctx context.Context, instanceID string) []LeaderboardEntry {
	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, instanceID)
	if err != nil {
		log.Printf("Failed to get sessions for leaderboard: %v", err)
		return []LeaderboardEntry{}
	}

	quizData, err := h.getQuizData(ctx, instanceID)
	if err != nil {
		log.Printf("Failed to get quiz data: %v", err)
		return []LeaderboardEntry{}
	}

	var leaderboard []LeaderboardEntry
	rank := 1
	for _, session := range sessions {
		if session.UserID == quizData.CreatedBy {
			continue
		}

		leaderboard = append(leaderboard, LeaderboardEntry{
			Rank:   rank,
			UserID: session.UserID,
			Score:  session.Score,
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

	leaderboard := h.getLeaderboard(ctx, client.InstanceID)
	rank := 0
	for _, entry := range leaderboard {
		if entry.UserID == client.UserID {
			rank = entry.Rank
			break
		}
	}

	client.SendMessage(MessageTypeQuizFinished, QuizFinishedPayload{
		FinalScore: session.Score,
		Rank:       rank,
	})

	log.Printf("Quiz finished: user=%s, instance=%s, score=%d, rank=%d", client.UserID, client.InstanceID, session.Score, rank)
}
