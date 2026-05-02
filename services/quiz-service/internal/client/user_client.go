package client

import (
	"context"
	"fmt"
	"log"

	"quiz-service/internal/model"
	pb "quiz-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserClient struct {
	client pb.UserServiceClient
	conn   *grpc.ClientConn
}

func NewUserClient(host, port string) (*UserClient, error) {
	address := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user client for %s: %w", address, err)
	}

	log.Printf("User Service client initialized for %s", address)

	return &UserClient{
		client: pb.NewUserServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *UserClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *UserClient) CheckGroupMembership(ctx context.Context, groupID, userID string) (bool, string, error) {
	resp, err := c.client.CheckGroupMembership(ctx, &pb.CheckGroupMembershipRequest{
		GroupId: groupID,
		UserId:  userID,
	})
	if err != nil {
		return false, "", fmt.Errorf("failed to check group membership: %w", err)
	}

	return resp.IsMember, resp.Role, nil
}

func (c *UserClient) GetEmailsByIDs(ctx context.Context, userIDs []string) (map[string]string, error) {
	if len(userIDs) == 0 {
		return map[string]string{}, nil
	}
	resp, err := c.client.GetUsersByIDs(ctx, &pb.GetUsersByIDsRequest{UserIds: userIDs})
	if err != nil {
		return nil, fmt.Errorf("failed to get users by IDs: %w", err)
	}
	emails := make(map[string]string, len(resp.Users))
	for _, u := range resp.Users {
		if u.Email != "" {
			emails[u.Id] = u.Email
		}
	}
	return emails, nil
}

func (c *UserClient) GetUsersByIDs(ctx context.Context, userIDs []string) (map[string]*model.UserInfo, error) {
	if len(userIDs) == 0 {
		return map[string]*model.UserInfo{}, nil
	}
	resp, err := c.client.GetUsersByIDs(ctx, &pb.GetUsersByIDsRequest{UserIds: userIDs})
	if err != nil {
		return nil, fmt.Errorf("failed to get users by IDs: %w", err)
	}
	result := make(map[string]*model.UserInfo, len(resp.Users))
	for _, u := range resp.Users {
		result[u.Id] = &model.UserInfo{
			ID:           u.Id,
			Email:        u.Email,
			FirstName:    u.FirstName,
			LastName:     u.LastName,
			IsRegistered: u.IsRegistered,
			Language:     u.Language,
		}
	}
	return result, nil
}

func (c *UserClient) GetGroupMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	if groupID == "" {
		return nil, nil
	}
	resp, err := c.client.GetGroupMemberIDs(ctx, &pb.GetGroupMemberIDsRequest{GroupId: groupID})
	if err != nil {
		return nil, fmt.Errorf("failed to get group member IDs: %w", err)
	}
	return resp.UserIds, nil
}

func (c *UserClient) GetNotificationSettingsBatch(ctx context.Context, userIDs []string) (map[string]model.NotificationSettings, error) {
	if len(userIDs) == 0 {
		return map[string]model.NotificationSettings{}, nil
	}
	resp, err := c.client.GetNotificationSettingsBatch(ctx, &pb.GetNotificationSettingsBatchRequest{UserIds: userIDs})
	if err != nil {
		return nil, fmt.Errorf("failed to get notification settings batch: %w", err)
	}
	result := make(map[string]model.NotificationSettings, len(resp.Settings))
	for uid, s := range resp.Settings {
		settings := model.NotificationSettings{
			NewQuizzes:       s.NewQuizzes,
			QuizResults:      s.QuizResults,
			GroupInvites:     s.GroupInvites,
			DeadlineReminder: s.DeadlineReminder,
		}
		if l, ok := resp.Languages[uid]; ok {
			settings.Language = l
		}
		result[uid] = settings
	}
	return result, nil
}
