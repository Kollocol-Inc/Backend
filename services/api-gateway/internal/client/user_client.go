package client

import (
	"context"
	"fmt"
	"log"

	pb "api-gateway/proto"

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
		return nil, fmt.Errorf("failed to create auth client for %s: %w", address, err)
	}

	log.Printf("Auth Service client initialized for %s", address)

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

func (c *UserClient) Register(ctx context.Context, userID, firstName, lastName string, avatarData []byte, avatarFilename string) (*pb.RegisterResponse, error) {
	return c.client.Register(ctx, &pb.RegisterRequest{
		UserId:         userID,
		FirstName:      firstName,
		LastName:       lastName,
		AvatarData:     avatarData,
		AvatarFilename: avatarFilename,
	})
}

func (c *UserClient) GetProfile(ctx context.Context, userID string) (*pb.GetProfileResponse, error) {
	return c.client.GetProfile(ctx, &pb.GetProfileRequest{
		UserId: userID,
	})
}

func (c *UserClient) GetProfileByEmail(ctx context.Context, email, requesterID string) (*pb.GetProfileByEmailResponse, error) {
	return c.client.GetProfileByEmail(ctx, &pb.GetProfileByEmailRequest{
		Email:       email,
		RequesterId: requesterID,
	})
}

func (c *UserClient) UpdateProfile(ctx context.Context, userID, firstName, lastName string, avatarData []byte, avatarFilename string) (*pb.UpdateProfileResponse, error) {
	return c.client.UpdateProfile(ctx, &pb.UpdateProfileRequest{
		UserId:         userID,
		FirstName:      firstName,
		LastName:       lastName,
		AvatarData:     avatarData,
		AvatarFilename: avatarFilename,
	})
}

func (c *UserClient) GetNotificationSettings(ctx context.Context, userID string) (*pb.GetNotificationSettingsResponse, error) {
	return c.client.GetNotificationSettings(ctx, &pb.GetNotificationSettingsRequest{
		UserId: userID,
	})
}

func (c *UserClient) UpdateNotificationSettings(ctx context.Context, req *pb.UpdateNotificationSettingsRequest) (*pb.UpdateNotificationSettingsResponse, error) {
	return c.client.UpdateNotificationSettings(ctx, req)
}

func (c *UserClient) CreateGroup(ctx context.Context, ownerID, name string, memberEmails []string) (*pb.CreateGroupResponse, error) {
	return c.client.CreateGroup(ctx, &pb.CreateGroupRequest{
		OwnerId:      ownerID,
		Name:         name,
		MemberEmails: memberEmails,
	})
}

func (c *UserClient) GetGroups(ctx context.Context, userID, filter string) (*pb.GetGroupsResponse, error) {
	return c.client.GetGroups(ctx, &pb.GetGroupsRequest{
		UserId: userID,
		Filter: filter,
	})
}

func (c *UserClient) GetGroup(ctx context.Context, groupID, userID string) (*pb.GetGroupResponse, error) {
	return c.client.GetGroup(ctx, &pb.GetGroupRequest{
		GroupId: groupID,
		UserId:  userID,
	})
}

func (c *UserClient) UpdateGroup(ctx context.Context, groupID, userID, name string, memberEmails []string) (*pb.UpdateGroupResponse, error) {
	return c.client.UpdateGroup(ctx, &pb.UpdateGroupRequest{
		GroupId:      groupID,
		UserId:       userID,
		Name:         name,
		MemberEmails: memberEmails,
	})
}

func (c *UserClient) DeleteGroup(ctx context.Context, groupID, userID string) (*pb.DeleteGroupResponse, error) {
	return c.client.DeleteGroup(ctx, &pb.DeleteGroupRequest{
		GroupId: groupID,
		UserId:  userID,
	})
}
