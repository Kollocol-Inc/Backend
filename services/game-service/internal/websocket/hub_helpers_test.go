package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"game-service/internal/websocket/mocks"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testHubEnv struct {
	hub         *Hub
	quizClient  *mocks.MockQuizClientInterface
	userClient  *mocks.MockUserClientInterface
	redisClient *mocks.MockRedisClientInterface
	sessionRepo *mocks.MockSessionRepoInterface
	db          *mocks.MockDBExecer
}

func newTestHubWithMocks(t *testing.T) *testHubEnv {
	ctrl := gomock.NewController(t)
	qc := mocks.NewMockQuizClientInterface(ctrl)
	uc := mocks.NewMockUserClientInterface(ctrl)
	rc := mocks.NewMockRedisClientInterface(ctrl)
	sr := mocks.NewMockSessionRepoInterface(ctrl)
	db := mocks.NewMockDBExecer(ctrl)

	hub := NewHubWithDeps(qc, uc, rc, sr, db)
	return &testHubEnv{
		hub:         hub,
		quizClient:  qc,
		userClient:  uc,
		redisClient: rc,
		sessionRepo: sr,
		db:          db,
	}
}

func newTestClient(hub *Hub, userID, instanceID string, isCreator bool) *Client {
	return &Client{
		Hub:        hub,
		Send:       make(chan []byte, 256),
		UserID:     userID,
		InstanceID: instanceID,
		IsCreator:  isCreator,
	}
}

func readClientMessage(t *testing.T, client *Client) Message {
	t.Helper()
	select {
	case data := <-client.Send:
		var msg Message
		require.NoError(t, json.Unmarshal(data, &msg))
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timeout reading client message")
		return Message{}
	}
}

func readClientMessageType(t *testing.T, client *Client) MessageType {
	t.Helper()
	return readClientMessage(t, client).Type
}

func drainClientMessages(client *Client) []Message {
	var msgs []Message
	for {
		select {
		case data := <-client.Send:
			var msg Message
			json.Unmarshal(data, &msg)
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

func registerClientToHub(hub *Hub, client *Client) {
	hub.mu.Lock()
	if hub.clients[client.InstanceID] == nil {
		hub.clients[client.InstanceID] = make(map[*Client]bool)
	}
	hub.clients[client.InstanceID][client] = true
	hub.mu.Unlock()
}
