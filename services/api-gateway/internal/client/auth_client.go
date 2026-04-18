package client

import (
	"context"
	"fmt"
	"log"

	pb "api-gateway/proto"

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

func (c *AuthClient) Login(ctx context.Context, email string) (*pb.LoginResponse, error) {
	return c.client.Login(ctx, &pb.LoginRequest{
		Email: email,
	})
}

func (c *AuthClient) VerifyCode(ctx context.Context, email, code string) (*pb.VerifyCodeResponse, error) {
	return c.client.VerifyCode(ctx, &pb.VerifyCodeRequest{
		Email: email,
		Code:  code,
	})
}

func (c *AuthClient) RefreshToken(ctx context.Context, refreshToken string) (*pb.RefreshTokenResponse, error) {
	return c.client.RefreshToken(ctx, &pb.RefreshTokenRequest{
		RefreshToken: refreshToken,
	})
}

func (c *AuthClient) Logout(ctx context.Context, accessToken, refreshToken string) (*pb.LogoutResponse, error) {
	return c.client.Logout(ctx, &pb.LogoutRequest{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func (c *AuthClient) ValidateToken(ctx context.Context, accessToken string) (*pb.ValidateTokenResponse, error) {
	return c.client.ValidateToken(ctx, &pb.ValidateTokenRequest{
		AccessToken: accessToken,
	})
}

func (c *AuthClient) CreateAIBan(ctx context.Context, req *pb.CreateAIBanRequest) (*pb.CreateAIBanResponse, error) {
	return c.client.CreateAIBan(ctx, req)
}

func (c *AuthClient) DeleteAIBan(ctx context.Context, userID string) (*pb.DeleteAIBanResponse, error) {
	return c.client.DeleteAIBan(ctx, &pb.DeleteAIBanRequest{UserId: userID})
}

func (c *AuthClient) GetAIBan(ctx context.Context, userID string) (*pb.GetAIBanResponse, error) {
	return c.client.GetAIBan(ctx, &pb.GetAIBanRequest{UserId: userID})
}

func (c *AuthClient) ListAIBans(ctx context.Context) (*pb.ListAIBansResponse, error) {
	return c.client.ListAIBans(ctx, &pb.ListAIBansRequest{})
}

func (c *AuthClient) CheckAIBan(ctx context.Context, userID string) (*pb.CheckAIBanResponse, error) {
	return c.client.CheckAIBan(ctx, &pb.CheckAIBanRequest{UserId: userID})
}
