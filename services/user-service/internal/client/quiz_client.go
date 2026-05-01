package client

import (
	"context"
	"fmt"
	"log"

	pb "user-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type QuizClient struct {
	client pb.QuizServiceClient
	conn   *grpc.ClientConn
}

func NewQuizClient(host, port string) (*QuizClient, error) {
	address := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create quiz client for %s: %w", address, err)
	}

	log.Printf("Quiz Service client initialized for %s", address)

	return &QuizClient{
		client: pb.NewQuizServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *QuizClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *QuizClient) DeleteAllByOwner(ctx context.Context, userID string) error {
	if _, err := c.client.DeleteAllByOwner(ctx, &pb.DeleteAllByOwnerRequest{UserId: userID}); err != nil {
		return fmt.Errorf("failed to delete quiz data: %w", err)
	}
	return nil
}
