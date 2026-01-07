package client

import (
	"context"
	"fmt"
	"log"

	pb "api-gateway/proto"

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

func (c *QuizClient) CreateTemplate(ctx context.Context, req *pb.CreateTemplateRequest) (*pb.CreateTemplateResponse, error) {
	return c.client.CreateTemplate(ctx, req)
}

func (c *QuizClient) GetTemplate(ctx context.Context, req *pb.GetTemplateRequest) (*pb.GetTemplateResponse, error) {
	return c.client.GetTemplate(ctx, req)
}

func (c *QuizClient) GetTemplates(ctx context.Context, req *pb.GetTemplatesRequest) (*pb.GetTemplatesResponse, error) {
	return c.client.GetTemplates(ctx, req)
}

func (c *QuizClient) UpdateTemplate(ctx context.Context, req *pb.UpdateTemplateRequest) (*pb.UpdateTemplateResponse, error) {
	return c.client.UpdateTemplate(ctx, req)
}

func (c *QuizClient) DeleteTemplate(ctx context.Context, req *pb.DeleteTemplateRequest) (*pb.DeleteTemplateResponse, error) {
	return c.client.DeleteTemplate(ctx, req)
}

func (c *QuizClient) CreateInstance(ctx context.Context, req *pb.CreateInstanceRequest) (*pb.CreateInstanceResponse, error) {
	return c.client.CreateInstance(ctx, req)
}

func (c *QuizClient) GetInstance(ctx context.Context, req *pb.GetInstanceRequest) (*pb.GetInstanceResponse, error) {
	return c.client.GetInstance(ctx, req)
}

func (c *QuizClient) GetHostingInstances(ctx context.Context, req *pb.GetHostingInstancesRequest) (*pb.GetHostingInstancesResponse, error) {
	return c.client.GetHostingInstances(ctx, req)
}
