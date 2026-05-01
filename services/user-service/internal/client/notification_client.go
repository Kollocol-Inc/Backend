package client

import (
	"context"
	"fmt"
	"log"

	pb "user-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NotificationClient struct {
	client pb.NotificationServiceClient
	conn   *grpc.ClientConn
}

func NewNotificationClient(host, port string) (*NotificationClient, error) {
	address := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create notification client for %s: %w", address, err)
	}

	log.Printf("Notification Service client initialized for %s", address)

	return &NotificationClient{
		client: pb.NewNotificationServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *NotificationClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *NotificationClient) DeleteAllForUser(ctx context.Context, userID string) error {
	if _, err := c.client.DeleteAllForUser(ctx, &pb.DeleteAllForUserRequest{UserId: userID}); err != nil {
		return fmt.Errorf("failed to delete notifications: %w", err)
	}
	return nil
}
