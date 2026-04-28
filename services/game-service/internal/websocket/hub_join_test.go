package websocket

import (
	"testing"
	"time"

	"game-service/internal/constants"
	pb "game-service/proto"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func makeQuizInstanceResponse(status, quizType, createdBy string) *pb.GetInstanceResponse {
	return &pb.GetInstanceResponse{
		Instance: &pb.QuizInstance{
			Id:        "inst-1",
			Status:    status,
			QuizType:  quizType,
			CreatedBy: createdBy,
			Title:     "Test Quiz",
			Settings:  &pb.QuizInstance_SyncSettings{SyncSettings: &pb.QuizSyncSettings{}},
			CreatedAt: timestamppb.Now(),
		},
		Questions: []*pb.Question{
			{
				Id:           "q-1",
				Text:         "What is 2+2?",
				OrderIndex:   0,
				MaxScore:     10,
				TimeLimitSec: 30,
				Answer: &pb.Question_SingleChoice{
					SingleChoice: &pb.SingleChoice{
						Options:       []string{"3", "4", "5"},
						CorrectOption: 1,
					},
				},
			},
		},
	}
}

func TestHandleJoin_Success(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:deleted").Return("", redis.Nil)
	resp := makeQuizInstanceResponse(constants.InstanceStatusWaiting, constants.QuizTypeSync, "creator-1")
	env.quizClient.EXPECT().GetInstance(gomock.Any(), "inst-1", "user-1").Return(resp, nil)
	env.redisClient.EXPECT().Set(gomock.Any(), "quiz:inst-1:data", gomock.Any(), gomock.Any()).Return(nil)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return("{}", nil).AnyTimes()
	env.sessionRepo.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(true, nil)
	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(&pb.User{Id: "user-1", FirstName: "John"}, nil).AnyTimes()

	registerClientToHub(env.hub, client)
	env.hub.handleJoin(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeConnected, msg.Type)

	time.Sleep(100 * time.Millisecond)
}

func TestHandleJoin_QuizFinished(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:deleted").Return("", redis.Nil)
	resp := makeQuizInstanceResponse(constants.InstanceStatusPendingReview, constants.QuizTypeSync, "creator-1")
	env.quizClient.EXPECT().GetInstance(gomock.Any(), "inst-1", "user-1").Return(resp, nil)

	registerClientToHub(env.hub, client)
	env.hub.handleJoin(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleJoin_CreatorAsyncRejected(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "creator-1", "inst-1", true)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:deleted").Return("", redis.Nil)
	resp := makeQuizInstanceResponse(constants.InstanceStatusActive, constants.QuizTypeAsync, "creator-1")
	env.quizClient.EXPECT().GetInstance(gomock.Any(), "inst-1", "creator-1").Return(resp, nil)
	env.redisClient.EXPECT().Set(gomock.Any(), "quiz:inst-1:data", gomock.Any(), gomock.Any()).Return(nil)

	registerClientToHub(env.hub, client)
	env.hub.handleJoin(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleJoin_LateSyncJoinRejected(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:deleted").Return("", redis.Nil)
	resp := makeQuizInstanceResponse(constants.InstanceStatusActive, constants.QuizTypeSync, "creator-1")
	env.quizClient.EXPECT().GetInstance(gomock.Any(), "inst-1", "user-1").Return(resp, nil)
	env.redisClient.EXPECT().Set(gomock.Any(), "quiz:inst-1:data", gomock.Any(), gomock.Any()).Return(nil)
	// CreateSession returns (true, nil) — newly created — so this is a late-join which should be rejected
	env.sessionRepo.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(true, nil)
	env.sessionRepo.EXPECT().DeleteSession(gomock.Any(), "inst-1", "user-1").Return(nil)

	registerClientToHub(env.hub, client)
	env.hub.handleJoin(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleJoin_GetInstanceFails(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:deleted").Return("", redis.Nil)
	env.quizClient.EXPECT().GetInstance(gomock.Any(), "inst-1", "user-1").Return(nil, assert.AnError)

	registerClientToHub(env.hub, client)
	env.hub.handleJoin(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeError, msg.Type)
}

func TestHandleJoin_ExistingSession(t *testing.T) {
	env := newTestHubWithMocks(t)
	client := newTestClient(env.hub, "user-1", "inst-1", false)

	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:deleted").Return("", redis.Nil)
	resp := makeQuizInstanceResponse(constants.InstanceStatusWaiting, constants.QuizTypeSync, "creator-1")
	env.quizClient.EXPECT().GetInstance(gomock.Any(), "inst-1", "user-1").Return(resp, nil)
	env.redisClient.EXPECT().Set(gomock.Any(), "quiz:inst-1:data", gomock.Any(), gomock.Any()).Return(nil)
	env.redisClient.EXPECT().Get(gomock.Any(), "quiz:inst-1:data").Return("{}", nil).AnyTimes()
	// CreateSession returns (false, nil) — session already existed (reconnect), no late-join rejection
	env.sessionRepo.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(false, nil)
	env.userClient.EXPECT().GetProfile(gomock.Any(), "user-1").Return(&pb.User{Id: "user-1"}, nil).AnyTimes()

	registerClientToHub(env.hub, client)
	env.hub.handleJoin(client)

	msg := readClientMessage(t, client)
	assert.Equal(t, MessageTypeConnected, msg.Type)

	time.Sleep(100 * time.Millisecond)
}
