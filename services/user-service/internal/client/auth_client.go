package client

import (
	"context"
	"fmt"
	"log"

	pb "user-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthClient struct {
	client pb.AuthServiceClient
	conn   *grpc.ClientConn
}

func NewAuthClient(host, port string) (*AuthClient, error) {
	address := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth client for %s: %w", address, err)
	}

	log.Printf("Auth Service client initialized for %s", address)

	return &AuthClient{
		client: pb.NewAuthServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *AuthClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *AuthClient) RevokeUser(ctx context.Context, userID string) error {
	if _, err := c.client.RevokeUser(ctx, &pb.RevokeUserRequest{UserId: userID}); err != nil {
		return fmt.Errorf("failed to revoke user: %w", err)
	}
	return nil
}
