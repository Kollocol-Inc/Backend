package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"game-service/internal/constants"
	"game-service/internal/models"
	"game-service/internal/websocket/mocks"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type noopTxManager struct{}

func (noopTxManager) InTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

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

	hub := NewHubWithDeps(qc, uc, rc, sr, db, noopTxManager{})
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

func TestShuffledIndices_DeterministicPerUser(t *testing.T) {
	t.Helper()
	a1 := shuffledIndices("inst-1", "user-1", 8)
	a2 := shuffledIndices("inst-1", "user-1", 8)
	if len(a1) != len(a2) {
		t.Fatal("length mismatch")
	}
	for i := range a1 {
		if a1[i] != a2[i] {
			t.Fatalf("not deterministic at %d: %v vs %v", i, a1, a2)
		}
	}
}

func TestShuffledIndices_DiffersAcrossUsers(t *testing.T) {
	t.Helper()
	a := shuffledIndices("inst-1", "user-1", 12)
	b := shuffledIndices("inst-1", "user-2", 12)
	same := true
	for i := range a {
		if a[i] != b[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatalf("expected different permutations for different users; got %v == %v", a, b)
	}
}

func TestQuestionsForClient_SyncReturnsOriginal(t *testing.T) {
	hub := newTestHub()
	client := newTestClient(hub, "user-1", "inst-1", false)
	quizData := &models.QuizData{
		QuizType: constants.QuizTypeSync,
		Settings: models.Settings{QuestionsRandomOrder: true},
		Questions: []models.Question{
			{ID: "q-1"}, {ID: "q-2"}, {ID: "q-3"},
		},
	}
	got := questionsForClient(client, quizData)
	if len(got) != 3 || got[0].ID != "q-1" || got[2].ID != "q-3" {
		t.Fatalf("sync should not be shuffled, got %+v", got)
	}
}

func TestQuestionsForClient_AsyncWithoutFlagReturnsOriginal(t *testing.T) {
	hub := newTestHub()
	client := newTestClient(hub, "user-1", "inst-1", false)
	quizData := &models.QuizData{
		QuizType: constants.QuizTypeAsync,
		Settings: models.Settings{QuestionsRandomOrder: false},
		Questions: []models.Question{
			{ID: "q-1"}, {ID: "q-2"}, {ID: "q-3"},
		},
	}
	got := questionsForClient(client, quizData)
	if got[0].ID != "q-1" || got[2].ID != "q-3" {
		t.Fatalf("expected original order without flag, got %+v", got)
	}
}

func TestQuestionsForClient_AsyncShuffledIsDeterministic(t *testing.T) {
	hub := newTestHub()
	quizData := &models.QuizData{
		QuizType: constants.QuizTypeAsync,
		Settings: models.Settings{QuestionsRandomOrder: true},
		Questions: []models.Question{
			{ID: "q-1"}, {ID: "q-2"}, {ID: "q-3"}, {ID: "q-4"}, {ID: "q-5"},
		},
	}
	c1 := newTestClient(hub, "user-1", "inst-1", false)
	first := questionsForClient(c1, quizData)
	second := questionsForClient(c1, quizData)
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Fatalf("not deterministic for same client: %v vs %v", first, second)
		}
	}

	c2 := newTestClient(hub, "user-2", "inst-1", false)
	other := questionsForClient(c2, quizData)
	differs := false
	for i := range first {
		if first[i].ID != other[i].ID {
			differs = true
			break
		}
	}
	if !differs {
		t.Fatalf("expected different permutation for different user; both got %v", first)
	}
}

func TestShuffledIndices_IsPermutation(t *testing.T) {
	t.Helper()
	const n = 10
	perm := shuffledIndices("inst-1", "user-1", n)
	seen := make(map[int]bool, n)
	for _, v := range perm {
		if v < 0 || v >= n {
			t.Fatalf("out of range %d", v)
		}
		if seen[v] {
			t.Fatalf("duplicate %d in %v", v, perm)
		}
		seen[v] = true
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
