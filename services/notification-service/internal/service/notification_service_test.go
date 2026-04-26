package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"notification-service/internal/repository"
	"notification-service/internal/service/mocks"
	"notification-service/pkg/email"
	pb "notification-service/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func setupTest(t *testing.T) (*NotificationService, *mocks.MockNotificationRepo, *mocks.MockEmailSender) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNotificationRepo(ctrl)
	smtp := mocks.NewMockEmailSender(ctrl)

	svc := NewNotificationServiceWithDeps(repo, smtp)
	return svc, repo, smtp
}

func TestGetNotifications_Success(t *testing.T) {
	svc, repo, _ := setupTest(t)
	ctx := context.Background()

	notifications := []*repository.Notification{
		{ID: "n-1", UserID: "user-1", Type: "quiz_created", Title: "New Quiz", Content: "Math Quiz", IsRead: false, CreatedAt: time.Now()},
	}
	repo.EXPECT().GetNotifications(ctx, "user-1", 10, 0).Return(notifications, 1, nil)

	resp, err := svc.GetNotifications(ctx, &pb.GetNotificationsRequest{UserId: "user-1", Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Len(t, resp.Notifications, 1)
	assert.Equal(t, int32(1), resp.Total)
	assert.Equal(t, "New Quiz", resp.Notifications[0].Title)
}

func TestGetNotifications_ZeroLimit(t *testing.T) {
	svc, repo, _ := setupTest(t)
	ctx := context.Background()

	repo.EXPECT().GetNotifications(ctx, "user-1", 1, 0).Return([]*repository.Notification{}, 0, nil)

	resp, err := svc.GetNotifications(ctx, &pb.GetNotificationsRequest{UserId: "user-1", Limit: 0, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, int32(0), resp.Total)
}

func TestGetNotifications_RepoError(t *testing.T) {
	svc, repo, _ := setupTest(t)
	ctx := context.Background()

	repo.EXPECT().GetNotifications(ctx, "user-1", 10, 0).Return(nil, 0, fmt.Errorf("db error"))

	resp, err := svc.GetNotifications(ctx, &pb.GetNotificationsRequest{UserId: "user-1", Limit: 10, Offset: 0})
	require.NoError(t, err) // returns empty, not error
	assert.Empty(t, resp.Notifications)
}

func TestMarkAsRead_Success(t *testing.T) {
	svc, repo, _ := setupTest(t)
	ctx := context.Background()

	ids := []string{"n-1", "n-2"}
	repo.EXPECT().MarkAsRead(ctx, ids, "user-1").Return(nil)

	_, err := svc.MarkAsRead(ctx, &pb.MarkAsReadRequest{NotificationIds: ids, UserId: "user-1"})
	require.NoError(t, err)
}

func TestMarkAsRead_RepoError(t *testing.T) {
	svc, repo, _ := setupTest(t)
	ctx := context.Background()

	ids := []string{"n-999"}
	repo.EXPECT().MarkAsRead(ctx, ids, "user-1").Return(fmt.Errorf("db error"))

	_, err := svc.MarkAsRead(ctx, &pb.MarkAsReadRequest{NotificationIds: ids, UserId: "user-1"})
	assert.Error(t, err)
}

func TestDeleteNotification_Success(t *testing.T) {
	svc, repo, _ := setupTest(t)
	ctx := context.Background()

	ids := []string{"n-1", "n-2"}
	repo.EXPECT().DeleteNotification(ctx, ids, "user-1").Return(nil)

	_, err := svc.DeleteNotification(ctx, &pb.DeleteNotificationRequest{NotificationIds: ids, UserId: "user-1"})
	require.NoError(t, err)
}

func TestHandleSendAuthCode_Success(t *testing.T) {
	svc, _, smtp := setupTest(t)
	ctx := context.Background()

	data, _ := json.Marshal(map[string]string{"email": "test@example.com", "code": "1234"})
	smtp.EXPECT().SendAuthCode("test@example.com", "1234").Return(nil)

	err := svc.HandleSendAuthCode(ctx, data)
	require.NoError(t, err)
}

func TestHandleSendAuthCode_InvalidJSON(t *testing.T) {
	svc, _, _ := setupTest(t)
	ctx := context.Background()

	err := svc.HandleSendAuthCode(ctx, []byte("not json"))
	assert.Error(t, err)
}

func TestHandleGroupInvite_Success(t *testing.T) {
	svc, _, smtp := setupTest(t)
	ctx := context.Background()

	data, _ := json.Marshal(map[string]string{
		"group_id": "grp-1", "group_name": "Test Group",
		"inviter_name": "John", "invitee_email": "jane@example.com",
	})
	smtp.EXPECT().SendGroupInvite("jane@example.com", "Test Group", "John").Return(nil)

	err := svc.HandleGroupInvite(ctx, data)
	require.NoError(t, err)
}

func TestHandleQuizCreated_Success(t *testing.T) {
	svc, repo, _ := setupTest(t)
	ctx := context.Background()

	event := map[string]any{
		"instance_id":  "inst-1",
		"title":        "Math Quiz",
		"participants": []string{"user-1", "user-2"},
	}
	data, _ := json.Marshal(event)

	repo.EXPECT().CreateNotification(ctx, gomock.Any()).Times(2).Return(nil)

	err := svc.HandleQuizCreated(ctx, data)
	require.NoError(t, err)
}

func TestHandleQuizCreated_PartialFailure(t *testing.T) {
	svc, repo, _ := setupTest(t)
	ctx := context.Background()

	event := map[string]any{
		"instance_id":  "inst-1",
		"title":        "Quiz",
		"participants": []string{"user-1", "user-2"},
	}
	data, _ := json.Marshal(event)

	repo.EXPECT().CreateNotification(ctx, gomock.Any()).Return(fmt.Errorf("db error"))
	repo.EXPECT().CreateNotification(ctx, gomock.Any()).Return(nil)

	err := svc.HandleQuizCreated(ctx, data)
	require.NoError(t, err)
}

func TestHandleQuizResultsReady_Success(t *testing.T) {
	svc, repo, _ := setupTest(t)
	ctx := context.Background()

	event := map[string]any{
		"instance_id": "inst-1",
		"title":       "Math Quiz",
		"participants": []map[string]any{
			{"user_id": "user-1", "email": "", "score": 7, "max_score": 10},
		},
	}
	data, _ := json.Marshal(event)

	repo.EXPECT().CreateNotification(ctx, gomock.Any()).Return(nil)

	err := svc.HandleQuizResultsReady(ctx, data)
	require.NoError(t, err)
}

func TestHandleSendEmail_Success(t *testing.T) {
	svc, _, smtp := setupTest(t)
	ctx := context.Background()

	event := map[string]string{"to": "test@example.com", "subject": "Hello", "body": "World"}
	data, _ := json.Marshal(event)

	smtp.EXPECT().SendEmail(email.EmailData{To: "test@example.com", Subject: "Hello", Body: "World"}).Return(nil)

	err := svc.HandleSendEmail(ctx, data)
	require.NoError(t, err)
}

func TestHandleCreateNotification_Success(t *testing.T) {
	svc, repo, _ := setupTest(t)
	ctx := context.Background()

	event := map[string]string{"user_id": "user-1", "type": "info", "title": "Test", "content": "Hello"}
	data, _ := json.Marshal(event)

	repo.EXPECT().CreateNotification(ctx, gomock.Any()).Return(nil)

	err := svc.HandleCreateNotification(ctx, data)
	require.NoError(t, err)
}
