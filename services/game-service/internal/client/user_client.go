package client

import (
	"context"
	"fmt"

	pb "game-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type UserClient struct {
	conn   *grpc.ClientConn
	client pb.UserServiceClient
}

func NewUserClient(host, port string) (*UserClient, error) {
	addr := fmt.Sprintf("%s:%s", host, port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to user service: %w", err)
	}

	return &UserClient{
		conn:   conn,
		client: pb.NewUserServiceClient(conn),
	}, nil
}

func (c *UserClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *UserClient) GetProfile(ctx context.Context, userID string) (*pb.User, error) {
	resp, err := c.client.GetProfile(ctx, &pb.GetProfileRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, err
	}
	return resp.User, nil
}

func (c *UserClient) GetByEmail(ctx context.Context, email string) (*pb.User, error) {
	resp, err := c.client.GetProfileByEmail(ctx, &pb.GetProfileByEmailRequest{
		Email: email,
	})
	if err != nil {
		return nil, err
	}
	return resp.User, nil
}
