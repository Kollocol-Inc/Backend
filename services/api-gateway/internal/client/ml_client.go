package client

import (
	"context"
	"fmt"
	"log"

	pb "api-gateway/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type MLClient struct {
	client pb.MLServiceClient
	conn   *grpc.ClientConn
}

func NewMLClient(host, port string) (*MLClient, error) {
	address := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ml client for %s: %w", address, err)
	}

	log.Printf("ML Service client initialized for %s", address)

	return &MLClient{
		client: pb.NewMLServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *MLClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *MLClient) Paraphrase(ctx context.Context, req *pb.ParaphraseRequest) (*pb.ParaphraseResponse, error) {
	return c.client.Paraphrase(ctx, req)
}

func (c *MLClient) GenerateTemplate(ctx context.Context, req *pb.GenerateTemplateRequest) (*pb.GenerateTemplateResponse, error) {
	return c.client.GenerateTemplate(ctx, req)
}

func (c *MLClient) GenerateQuestions(ctx context.Context, req *pb.GenerateQuestionsRequest) (*pb.GenerateQuestionsResponse, error) {
	return c.client.GenerateQuestions(ctx, req)
}

func (c *MLClient) ReviewAnswer(ctx context.Context, req *pb.ReviewAnswerRequest) (*pb.ReviewAnswerResponse, error) {
	return c.client.ReviewAnswer(ctx, req)
}
