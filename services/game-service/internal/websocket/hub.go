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
	userClient  *client.UserClient
	redisClient *cache.RedisClient
	sessionRepo *repository.SessionRepository
	db          *sql.DB

	mu sync.RWMutex

	questionTimers map[string]*time.Timer
	timerMu        sync.Mutex

	frozenParticipants map[string][]User
}

func NewHub(
	quizClient *client.QuizClient,
	userClient *client.UserClient,
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
		userClient:     userClient,
		redisClient:    redisClient,
		sessionRepo:    sessionRepo,
		db:             db,
		questionTimers:     make(map[string]*time.Timer),
		frozenParticipants: make(map[string][]User),
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

	if h.clients[client.InstanceID] == nil {
		h.clients[client.InstanceID] = make(map[*Client]bool)
	}

	for existing := range h.clients[client.InstanceID] {
		if existing.UserID == client.UserID {
			delete(h.clients[client.InstanceID], existing)
			close(existing.Send)
			log.Printf("Closing duplicate connection: user=%s, instance=%s", client.UserID, client.InstanceID)
			break
		}
	}

	h.clients[client.InstanceID][client] = true
	h.mu.Unlock()

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
				delete(h.frozenParticipants, client.InstanceID)
				h.cancelAllTimersForInstance(client.InstanceID)
			} else {
				userProfile, err := h.userClient.GetProfile(context.Background(), client.UserID)
				if err != nil {
					log.Printf("Failed to get user profile for participant update: %v", err)
				}

				leftUser := userFromProfile(userProfile, client.IsCreator)
				leftUser.IsOnline = false
				payload := ParticipantsUpdatePayload{
					Action: constants.ActionLeft,
					User:   leftUser,
					Count:  len(clients),
				}
				for c := range clients {
					c.SendMessage(MessageTypeParticipantsUpdate, payload)
				}
				go h.sendParticipantsList(client.InstanceID)
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

	case MessageTypeKick:
		if client.IsCreator {
			h.handleKick(client, msg.Payload)
		} else {
			client.SendError("Only the creator can kick participants")
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

	if quizResp.Instance.Status == constants.InstanceStatusPendingReview {
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

	if client.IsCreator && quizData.QuizType == constants.QuizTypeAsync {
		log.Printf("Rejecting creator %s from async quiz %s", client.UserID, client.InstanceID)
		client.SendError("Creator cannot join async quizzes")

		go func() {
			time.Sleep(500 * time.Millisecond)
			h.Unregister <- client
		}()
		return
	}

	exists, err := h.sessionRepo.SessionExists(ctx, client.InstanceID, client.UserID)
	if err != nil {
		log.Printf("Failed to check session existence: %v", err)
		client.SendError("Failed to join quiz")
		return
	}

	if !exists && !client.IsCreator && quizResp.Instance.Status == constants.InstanceStatusActive && quizData.QuizType == constants.QuizTypeSync {
		log.Printf("Rejecting late join for user %s in active sync quiz %s", client.UserID, client.InstanceID)
		client.SendError("Quiz has already started")

		go func() {
			time.Sleep(500 * time.Millisecond)
			h.Unregister <- client
		}()
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

	userProfile, err := h.userClient.GetProfile(ctx, client.UserID)
	if err != nil {
		log.Printf("Failed to get user profile for participant update: %v", err)
	}

	h.broadcastToInstance(client.InstanceID, MessageTypeParticipantsUpdate, ParticipantsUpdatePayload{
		Action: constants.ActionJoined,
		User:   userFromProfile(userProfile, client.IsCreator),
		Count:  participantCount,
	})

	go h.sendParticipantsList(client.InstanceID)

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
		var questionType string
		var options []string
		var correctAnswer string

		switch a := q.Answer.(type) {
		case *pb.Question_SingleChoice:
			questionType = constants.QuestionTypeSingle
			options = a.SingleChoice.Options
			correctAnswer = fmt.Sprintf("%d", a.SingleChoice.CorrectOption)
		case *pb.Question_MultipleChoice:
			questionType = constants.QuestionTypeMultiple
			options = a.MultipleChoice.Options
			b, _ := json.Marshal(a.MultipleChoice.CorrectOptions)
			correctAnswer = string(b)
		case *pb.Question_OpenAnswer:
			questionType = constants.QuestionTypeOpen
			correctAnswer = a.OpenAnswer.CorrectText
		}

		questions[i] = models.Question{
			ID:            q.Id,
			Text:          q.Text,
			Type:          questionType,
			Options:       options,
			CorrectAnswer: correctAnswer,
			OrderIndex:    int(q.OrderIndex),
			MaxScore:      int(q.MaxScore),
			TimeLimitSec:  int(q.TimeLimitSec),
		}
	}

	settings := models.Settings{}
	if asyncSettings := resp.Instance.GetAsyncSettings(); asyncSettings != nil {
		settings.QuestionsRandomOrder = asyncSettings.QuestionsRandomOrder
	}

	return &models.QuizData{
		QuizType:   resp.Instance.QuizType,
		CreatedBy:  resp.Instance.CreatedBy,
		Questions:  questions,
		TemplateID: resp.Instance.TemplateId,
		Title:      resp.Instance.Title,
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
	return h.redisClient.Set(ctx, redisQuizDataKey(instanceID), string(jsonData), 24*time.Hour)
}

func (h *Hub) getQuizData(ctx context.Context, instanceID string) (*models.QuizData, error) {
	if h.redisClient == nil {
		return nil, fmt.Errorf("redis client is not initialized")
	}
	jsonData, err := h.redisClient.Get(ctx, redisQuizDataKey(instanceID))
	if err != nil {
		return nil, err
	}

	var data models.QuizData
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (h *Hub) handleKick(client *Client, payload any) {
	ctx := context.Background()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		client.SendError("Invalid kick format")
		return
	}

	var kickPayload KickPayload
	if err := json.Unmarshal(payloadBytes, &kickPayload); err != nil {
		client.SendError("Invalid kick format")
		return
	}

	if kickPayload.Email == "" {
		client.SendError("Email is required")
		return
	}

	userProfile, err := h.userClient.GetByEmail(ctx, kickPayload.Email)
	if err != nil {
		log.Printf("Failed to get user profile by email %s: %v", kickPayload.Email, err)
		client.SendError("User not found")
		return
	}

	targetUserID := userProfile.Id

	if targetUserID == client.UserID {
		client.SendError("You cannot kick yourself")
		return
	}

	quizData, err := h.getQuizData(ctx, client.InstanceID)
	if err != nil {
		log.Printf("Failed to get quiz data: %v", err)
		client.SendError("Failed to kick participant")
		return
	}

	if quizData.QuizType != constants.QuizTypeSync {
		client.SendError("Kick is only available in sync quizzes")
		return
	}

	h.mu.Lock()

	clients, ok := h.clients[client.InstanceID]
	if !ok {
		h.mu.Unlock()
		client.SendError("No participants found")
		return
	}

	var targetClient *Client
	for c := range clients {
		if c.UserID == targetUserID {
			targetClient = c
			break
		}
	}

	if targetClient == nil {
		h.mu.Unlock()
		client.SendError("Participant not found")
		return
	}

	if targetClient.IsCreator {
		h.mu.Unlock()
		client.SendError("Cannot kick the creator")
		return
	}

	close(targetClient.Send)
	targetClient.Conn.Close()

	delete(clients, targetClient)
	clientCount := len(clients)
	h.mu.Unlock()

	if err := h.sessionRepo.DeleteSession(ctx, client.InstanceID, targetUserID); err != nil {
		log.Printf("Failed to delete session for kicked user %s: %v", targetUserID, err)
	}

	log.Printf("User %s kicked user %s (email: %s) from instance %s", client.UserID, targetUserID, kickPayload.Email, client.InstanceID)

	kickedUser := userFromProfile(userProfile, targetClient.IsCreator)
	kickedUser.IsOnline = false
	h.broadcastToInstance(client.InstanceID, MessageTypeParticipantsUpdate, ParticipantsUpdatePayload{
		Action: constants.ActionLeft,
		User:   kickedUser,
		Count:  clientCount,
	})

	go h.sendParticipantsList(client.InstanceID)
}

func (h *Hub) broadcastToInstance(instanceID string, msgType MessageType, payload any) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.clients[instanceID]
	for client := range clients {
		client.SendMessage(msgType, payload)
	}
}

func (h *Hub) sendParticipantsList(instanceID string) {
	ctx := context.Background()

	quizData, err := h.getQuizData(ctx, instanceID)
	if err != nil {
		log.Printf("Failed to get quiz data for participants list: %v", err)
		return
	}

	h.mu.RLock()
	frozen := h.frozenParticipants[instanceID]
	clients := h.clients[instanceID]
	h.mu.RUnlock()

	var participants []User

	if len(frozen) > 0 {
		participants = make([]User, len(frozen))
		copy(participants, frozen)
		for i := range participants {
			participants[i].IsOnline = h.isUserOnline(instanceID, participants[i].UserID)
		}
	} else {
		participants = make([]User, 0, len(clients))
		for client := range clients {
			userProfile, err := h.userClient.GetProfile(ctx, client.UserID)
			if err != nil {
				log.Printf("Failed to get user profile %s: %v", client.UserID, err)
				continue
			}
			participants = append(participants, userFromProfile(userProfile, client.IsCreator))
		}
	}

	h.broadcastToInstance(instanceID, MessageTypeParticipantsList, ParticipantsListPayload{
		Participants: participants,
		Quiz: Quiz{
			Title: quizData.Title,
		},
	})
}

func (h *Hub) freezeParticipants(ctx context.Context, instanceID string) {
	h.mu.RLock()
	liveClients := h.clients[instanceID]
	snapshot := make([]*Client, 0, len(liveClients))
	for c := range liveClients {
		snapshot = append(snapshot, c)
	}
	h.mu.RUnlock()

	participants := make([]User, 0, len(snapshot))
	for _, c := range snapshot {
		userProfile, err := h.userClient.GetProfile(ctx, c.UserID)
		if err != nil {
			log.Printf("Failed to get user profile %s for freeze: %v", c.UserID, err)
			continue
		}
		participants = append(participants, userFromProfile(userProfile, c.IsCreator))
	}

	h.mu.Lock()
	h.frozenParticipants[instanceID] = participants
	h.mu.Unlock()

	log.Printf("Froze %d participants for instance %s", len(participants), instanceID)
}

func (h *Hub) isUserOnline(instanceID, userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients[instanceID] {
		if c.UserID == userID {
			return true
		}
	}
	return false
}

func (h *Hub) broadcastToParticipants(instanceID string, msgType MessageType, payload any) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.clients[instanceID]
	for client := range clients {
		if !client.IsCreator {
			client.SendMessage(msgType, payload)
		}
	}
}

func (h *Hub) sendLeaderboardToCreator(ctx context.Context, instanceID string, quizData *models.QuizData) {
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

	questionIndex := h.getCurrentQuestionIndex(ctx, instanceID)

	sessions, err := h.sessionRepo.GetSessionsByInstance(ctx, instanceID)
	if err != nil {
		log.Printf("Failed to get sessions for creator leaderboard: %v", err)
		return
	}

	leaderboard := h.buildLeaderboard(ctx, sessions, quizData, questionIndex)
	questionStats := h.buildQuestionStats(sessions, quizData, questionIndex)

	var qp *QuestionPayload
	if questionIndex < len(quizData.Questions) {
		qp = questionPayloadFromModel(quizData.Questions[questionIndex], questionIndex, len(quizData.Questions))
	}

	creator.SendMessage(MessageTypeLeaderboard, LeaderboardPayload{
		Leaderboard:       leaderboard,
		AnswerOptionStats: questionStats,
		Question:          qp,
	})
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

func (h *Hub) validateAnswer(answer, correctAnswerJSON, questionType string) bool {
	var correctAnswer string
	if err := json.Unmarshal([]byte(correctAnswerJSON), &correctAnswer); err != nil {
		correctAnswer = correctAnswerJSON
	}

	switch questionType {
	case constants.QuestionTypeSingle:
		return strings.TrimSpace(answer) == strings.TrimSpace(correctAnswer)

	case constants.QuestionTypeMultiple:
		var userIndices, correctIndices []int
		if err := json.Unmarshal([]byte(answer), &userIndices); err != nil {
			return false
		}
		if err := json.Unmarshal([]byte(correctAnswer), &correctIndices); err != nil {
			return false
		}
		if len(userIndices) != len(correctIndices) {
			return false
		}
		correctSet := make(map[int]struct{}, len(correctIndices))
		for _, idx := range correctIndices {
			correctSet[idx] = struct{}{}
		}
		for _, idx := range userIndices {
			if _, ok := correctSet[idx]; !ok {
				return false
			}
		}
		return true

	case constants.QuestionTypeOpen:
		return strings.TrimSpace(strings.ToLower(answer)) == strings.TrimSpace(strings.ToLower(correctAnswer))

	default:
		return strings.TrimSpace(strings.ToLower(answer)) == strings.TrimSpace(strings.ToLower(correctAnswer))
	}
}

func (h *Hub) calculateScore(maxScore int, timeSpentMs, timeLimitMs int64) int {
	maxScore = 100 * maxScore
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
