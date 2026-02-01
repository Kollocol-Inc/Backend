package websocket

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"game-service/internal/client"
	"game-service/internal/constants"
	"game-service/internal/models"
	"game-service/internal/repository"
	"game-service/pkg/cache"
	pb "game-service/proto"
)

type ClientMessage struct {
	Client  *Client
	Message Message
}

type Hub struct {
	clients       map[string]map[*Client]bool
	Register      chan *Client
	Unregister    chan *Client
	HandleMessage chan *ClientMessage

	quizClient  *client.QuizClient
	redisClient *cache.RedisClient
	sessionRepo *repository.SessionRepository
	db          *sql.DB

	mu sync.RWMutex

	questionTimers map[string]*time.Timer
	timerMu        sync.Mutex
}

func NewHub(
	quizClient *client.QuizClient,
	redisClient *cache.RedisClient,
	sessionRepo *repository.SessionRepository,
	db *sql.DB,
) *Hub {
	return &Hub{
		clients:        make(map[string]map[*Client]bool),
		Register:       make(chan *Client),
		Unregister:     make(chan *Client),
		HandleMessage:  make(chan *ClientMessage),
		quizClient:     quizClient,
		redisClient:    redisClient,
		sessionRepo:    sessionRepo,
		db:             db,
		questionTimers: make(map[string]*time.Timer),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.registerClient(client)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case clientMsg := <-h.HandleMessage:
			h.handleClientMessage(clientMsg)
		}
	}
}

func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.InstanceID] == nil {
		h.clients[client.InstanceID] = make(map[*Client]bool)
	}
	h.clients[client.InstanceID][client] = true

	log.Printf("Client registered: user=%s, instance=%s, isCreator=%v",
		client.UserID, client.InstanceID, client.IsCreator)

	go h.handleJoin(client)
}

func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.InstanceID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.Send)

			if len(clients) == 0 {
				delete(h.clients, client.InstanceID)
				h.cancelAllTimersForInstance(client.InstanceID)
			} else {
				payload := ParticipantsUpdatePayload{
					Action: constants.ActionLeft,
					UserID: client.UserID,
					Count:  len(clients),
				}
				for c := range clients {
					c.SendMessage(MessageTypeParticipantsUpdate, payload)
				}
			}

			log.Printf("Client unregistered: user=%s, instance=%s", client.UserID, client.InstanceID)
		}
	}
}

func (h *Hub) handleClientMessage(clientMsg *ClientMessage) {
	client := clientMsg.Client
	msg := clientMsg.Message

	log.Printf("Received message: type=%s, user=%s, instance=%s", msg.Type, client.UserID, client.InstanceID)

	switch msg.Type {
	case MessageTypeStart:
		if client.IsCreator {
			h.handleStartQuiz(client)
		} else {
			client.SendError("Only the creator can start the quiz")
		}

	case MessageTypeAnswer:
		h.handleAnswer(client, msg.Payload)

	case MessageTypeContinue:
		if client.IsCreator {
			h.handleContinue(client)
		} else {
			client.SendError("Only the creator can continue")
		}

	case MessageTypePing:
		client.SendMessage(MessageTypePong, nil)

	default:
		client.SendError(fmt.Sprintf("Unknown message type: %s", msg.Type))
	}
}

func (h *Hub) handleJoin(client *Client) {
	log.Printf("Handling join for user %s in instance %s", client.UserID, client.InstanceID)
	ctx := context.Background()

	quizResp, err := h.quizClient.GetInstance(ctx, client.InstanceID, client.UserID)
	if err != nil {
		log.Printf("Failed to get quiz instance: %v", err)
		client.SendError("Failed to load quiz")
		return
	}
	log.Printf("Successfully retrieved quiz instance %s", client.InstanceID)

	if quizResp.Instance.Status == constants.InstanceStatusFinished {
		log.Printf("Quiz %s is finished, rejecting connection for user %s", client.InstanceID, client.UserID)
		client.SendError("Quiz has already finished")

		go func() {
			time.Sleep(500 * time.Millisecond)
			h.Unregister <- client
		}()
		return
	}

	quizData := h.convertToQuizData(quizResp)
	if err := h.cacheQuizData(ctx, client.InstanceID, quizData); err != nil {
		log.Printf("Failed to cache quiz data: %v", err)
	}

	exists, err := h.sessionRepo.SessionExists(ctx, client.InstanceID, client.UserID)
	if err != nil {
		log.Printf("Failed to check session existence: %v", err)
		client.SendError("Failed to join quiz")
		return
	}

	if !exists {
		session := &models.GameSession{
			InstanceID:           client.InstanceID,
			UserID:               client.UserID,
			Status:               constants.SessionStatusJoined,
			CurrentQuestionIndex: 0,
			Score:                0,
			Answers:              "[]",
			StartedAt:            time.Now(),
		}
		if err := h.sessionRepo.CreateSession(ctx, session); err != nil {
			log.Printf("Failed to create session: %v", err)
			client.SendError("Failed to join quiz")
			return
		}
	}

	log.Printf("Sending connected message to user %s (status=%s)", client.UserID, quizResp.Instance.Status)
	client.SendMessage(MessageTypeConnected, ConnectedPayload{
		SessionID:  fmt.Sprintf("%s:%s", client.InstanceID, client.UserID),
		QuizType:   quizData.QuizType,
		QuizStatus: quizResp.Instance.Status,
		IsCreator:  client.IsCreator,
	})

	h.mu.RLock()
	participantCount := len(h.clients[client.InstanceID])
	h.mu.RUnlock()

	h.broadcastToInstance(client.InstanceID, MessageTypeParticipantsUpdate, ParticipantsUpdatePayload{
		Action: constants.ActionJoined,
		UserID: client.UserID,
		Count:  participantCount,
	})

	if quizResp.Instance.Status == constants.InstanceStatusActive {
		if !client.IsCreator {
			if err := h.sessionRepo.UpdateSessionStatus(ctx, client.InstanceID, client.UserID, constants.SessionStatusInProgress); err != nil {
				log.Printf("Failed to update session status for late joiner %s: %v", client.UserID, err)
			}
		}

		go func() {
			time.Sleep(100 * time.Millisecond)
			h.handleResumeQuiz(client, quizData)
		}()
	}
}

func (h *Hub) convertToQuizData(resp *pb.GetInstanceResponse) *models.QuizData {
	questions := make([]models.Question, len(resp.Questions))
	for i, q := range resp.Questions {
		questions[i] = models.Question{
			ID:            q.Id,
			Text:          q.Text,
			Type:          q.Type,
			Options:       q.Options,
			CorrectAnswer: q.CorrectAnswer,
			OrderIndex:    int(q.OrderIndex),
			MaxScore:      int(q.MaxScore),
			TimeLimitSec:  int(q.TimeLimitSec),
		}
	}

	settings := models.Settings{}
	if resp.Instance.Settings != nil {
		settings.RandomOrder = resp.Instance.Settings.RandomOrder
		settings.TimeLimitTotal = int(resp.Instance.Settings.TimeLimitTotal)
		settings.ShowCorrectAnswers = resp.Instance.Settings.ShowCorrectAnswers
		settings.AllowReview = resp.Instance.Settings.AllowReview
	}

	return &models.QuizData{
		QuizType:   resp.Instance.QuizType,
		CreatedBy:  resp.Instance.CreatedBy,
		Questions:  questions,
		TemplateID: resp.Instance.TemplateId,
		Settings:   settings,
	}
}

func (h *Hub) cacheQuizData(ctx context.Context, instanceID string, data *models.QuizData) error {
	if h.redisClient == nil {
		return nil
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("quiz:%s:data", instanceID)
	return h.redisClient.Set(ctx, key, string(jsonData), 24*time.Hour)
}

func (h *Hub) getQuizData(ctx context.Context, instanceID string) (*models.QuizData, error) {
	if h.redisClient == nil {
		return nil, fmt.Errorf("redis client is not initialized")
	}
	key := fmt.Sprintf("quiz:%s:data", instanceID)
	jsonData, err := h.redisClient.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var data models.QuizData
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (h *Hub) broadcastToInstance(instanceID string, msgType MessageType, payload interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.clients[instanceID]
	for client := range clients {
		client.SendMessage(msgType, payload)
	}
}

func (h *Hub) broadcastToParticipants(instanceID string, msgType MessageType, payload interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.clients[instanceID]
	for client := range clients {
		if !client.IsCreator {
			client.SendMessage(msgType, payload)
		}
	}
}

func (h *Hub) cancelQuestionTimer(timerKey string) {
	h.timerMu.Lock()
	defer h.timerMu.Unlock()

	if timer, ok := h.questionTimers[timerKey]; ok {
		timer.Stop()
		delete(h.questionTimers, timerKey)
	}
}

func (h *Hub) cancelAllTimersForInstance(instanceID string) {
	h.timerMu.Lock()
	defer h.timerMu.Unlock()

	for key, timer := range h.questionTimers {
		if len(key) >= len(instanceID) && key[:len(instanceID)] == instanceID {
			timer.Stop()
			delete(h.questionTimers, key)
		}
	}
}

func (h *Hub) validateAnswer(answer, correctAnswerJSON string) bool {
	var correctAnswer any
	if err := json.Unmarshal([]byte(correctAnswerJSON), &correctAnswer); err != nil {
		return strings.TrimSpace(strings.ToLower(answer)) == strings.TrimSpace(strings.ToLower(correctAnswerJSON))
	}

	correctAnswerStr := fmt.Sprintf("%v", correctAnswer)
	return strings.TrimSpace(strings.ToLower(answer)) == strings.TrimSpace(strings.ToLower(correctAnswerStr))
}

func (h *Hub) calculateScore(maxScore int, timeSpentMs, timeLimitMs int64) int {
	if timeLimitMs == 0 {
		return maxScore
	}

	timeRatio := float64(timeSpentMs) / float64(timeLimitMs)
	if timeRatio > 1.0 {
		timeRatio = 1.0
	}

	score := float64(maxScore) * (1.0 - 0.5*timeRatio)
	return int(score)
}

func (h *Hub) updateInstanceStatus(ctx context.Context, instanceID, status string) error {
	query := `UPDATE quiz_instances SET status = $1 WHERE id = $2`
	_, err := h.db.ExecContext(ctx, query, status, instanceID)
	if err != nil {
		log.Printf("Failed to update instance status: %v", err)
		return err
	}
	log.Printf("Updated instance %s status to %s", instanceID, status)
	return nil
}
