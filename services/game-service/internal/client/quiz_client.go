package client

import (
	"context"
	"fmt"

	pb "game-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type QuizClient struct {
	conn   *grpc.ClientConn
	client pb.QuizServiceClient
}

func NewQuizClient(host, port string) (*QuizClient, error) {
	addr := fmt.Sprintf("%s:%s", host, port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to quiz service: %w", err)
	}

	return &QuizClient{
		conn:   conn,
		client: pb.NewQuizServiceClient(conn),
	}, nil
}

func (c *QuizClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *QuizClient) GetInstance(ctx context.Context, instanceID, userID string) (*pb.GetInstanceResponse, error) {
	return c.client.GetInstance(ctx, &pb.GetInstanceRequest{
		InstanceId: instanceID,
		UserId:     userID,
	})
}

func (c *QuizClient) GetInstanceByAccessCode(ctx context.Context, accessCode, userID string) (*pb.GetInstanceByAccessCodeResponse, error) {
	return c.client.GetInstanceByAccessCode(ctx, &pb.GetInstanceByAccessCodeRequest{
		AccessCode: accessCode,
		UserId:     userID,
	})
}