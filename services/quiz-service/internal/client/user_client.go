package client

import (
	"context"
	"fmt"
	"log"

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