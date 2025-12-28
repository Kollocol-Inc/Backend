package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"log"
	"math/big"
	"time"

	pb "auth-service/proto"
	"auth-service/internal/repository"
	"auth-service/pkg/cache"
	"auth-service/pkg/jwt"
	"auth-service/pkg/messaging"
	"auth-service/pkg/validator"
)

type AuthService struct {
	pb.UnimplementedAuthServiceServer
	authRepo   *repository.AuthRepository
	userRepo   *repository.UserRepository
	rabbitMQ   *messaging.RabbitMQClient
	jwtSecret  string
}

func NewAuthService(redis *cache.RedisClient, db *sql.DB, rabbitMQ *messaging.RabbitMQClient, jwtSecret string) *AuthService {
	return &AuthService{
		authRepo:  repository.NewAuthRepository(redis, db),
		userRepo:  repository.NewUserRepository(db),
		rabbitMQ:  rabbitMQ,
		jwtSecret: jwtSecret,
	}
}

func (s *AuthService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	email := validator.NormalizeEmail(req.Email)
	if err := validator.ValidateEmail(email); err != nil {
		return &pb.LoginResponse{
			Success: false,
			Message: "Invalid email address",
		}, nil
	}

	code, err := generateCode(4)
	if err != nil {
		log.Printf("Failed to generate code: %v", err)
		return &pb.LoginResponse{
			Success: false,
			Message: "Failed to generate verification code",
		}, nil
	}

	log.Printf("Generated code %s for %s", code, email)

	if err := s.authRepo.SaveAuthCode(ctx, email, code); err != nil {
		log.Printf("Failed to save auth code: %v", err)
		return &pb.LoginResponse{
			Success: false,
			Message: "Failed to save verification code",
		}, nil
	}

	event := map[string]string{
		"email": email,
		"code":  code,
	}
	eventData, _ := json.Marshal(event)

	if err := s.rabbitMQ.Publish(ctx, "auth.send_code", eventData); err != nil {
		log.Printf("Failed to publish send_auth_code event: %v", err)
	}

	return &pb.LoginResponse{
		Success: true,
		Message: "Verification code sent to your email",
	}, nil
}

func (s *AuthService) VerifyCode(ctx context.Context, req *pb.VerifyCodeRequest) (*pb.VerifyCodeResponse, error) {
	email := validator.NormalizeEmail(req.Email)

	authCode, err := s.authRepo.GetAuthCode(ctx, email)
	if err != nil {
		return &pb.VerifyCodeResponse{
			Success: false,
			Message: "Verification code not found or expired",
		}, nil
	}

	if authCode.Attempts >= repository.MaxAttempts {
		s.authRepo.DeleteAuthCode(ctx, email)
		return &pb.VerifyCodeResponse{
			Success: false,
			Message: "Too many failed attempts. Please request a new code",
		}, nil
	}

	if authCode.Code != req.Code {
		s.authRepo.IncrementAuthCodeAttempts(ctx, email)
		return &pb.VerifyCodeResponse{
			Success: false,
			Message: "Invalid verification code",
		}, nil
	}

	if time.Now().After(authCode.ExpiresAt) {
		s.authRepo.DeleteAuthCode(ctx, email)
		return &pb.VerifyCodeResponse{
			Success: false,
			Message: "Verification code expired",
		}, nil
	}

	user, err := s.userRepo.GetOrCreateUser(ctx, email)
	if err != nil {
		log.Printf("Failed to get or create user: %v", err)
		return &pb.VerifyCodeResponse{
			Success: false,
			Message: "Failed to process user",
		}, nil
	}

	tokens, err := jwt.GenerateTokenPair(user.ID, user.Email, s.jwtSecret)
	if err != nil {
		log.Printf("Failed to generate tokens: %v", err)
		return &pb.VerifyCodeResponse{
			Success: false,
			Message: "Failed to generate tokens",
		}, nil
	}

	refreshToken := repository.NewRefreshToken(tokens.RefreshToken, user.ID, time.Now().Add(jwt.RefreshTokenDuration), time.Now())
	if err := s.authRepo.SaveRefreshToken(ctx, refreshToken); err != nil {
		log.Printf("Failed to save refresh token: %v", err)
		return &pb.VerifyCodeResponse{
			Success: false,
			Message: "Failed to save refresh token",
		}, nil
	}

	s.authRepo.DeleteAuthCode(ctx, email)

	return &pb.VerifyCodeResponse{
		Success:      true,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		IsRegistered: user.IsRegistered,
		UserId:       user.ID,
		Message:      "Login successful",
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	claims, err := jwt.ValidateRefreshToken(req.RefreshToken, s.jwtSecret)
	if err != nil {
		return &pb.RefreshTokenResponse{
			Success: false,
			Message: "Invalid refresh token",
		}, nil
	}

	storedToken, err := s.authRepo.GetRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return &pb.RefreshTokenResponse{
			Success: false,
			Message: "Refresh token not found",
		}, nil
	}

	if time.Now().After(storedToken.ExpiresAt) {
		s.authRepo.DeleteRefreshToken(ctx, req.RefreshToken)
		return &pb.RefreshTokenResponse{
			Success: false,
			Message: "Refresh token expired",
		}, nil
	}

	newTokens, err := jwt.GenerateTokenPair(claims.UserID, claims.Email, s.jwtSecret)
	if err != nil {
		log.Printf("Failed to generate new tokens: %v", err)
		return &pb.RefreshTokenResponse{
			Success: false,
			Message: "Failed to generate new tokens",
		}, nil
	}

	if err := s.authRepo.DeleteRefreshToken(ctx, req.RefreshToken); err != nil {
		log.Printf("Failed to delete old refresh token: %v", err)
	}

	newRefreshToken := repository.NewRefreshToken(newTokens.RefreshToken, claims.UserID, time.Now().Add(jwt.RefreshTokenDuration), time.Now())
	if err := s.authRepo.SaveRefreshToken(ctx, newRefreshToken); err != nil {
		log.Printf("Failed to save new refresh token: %v", err)
		return &pb.RefreshTokenResponse{
			Success: false,
			Message: "Failed to save new refresh token",
		}, nil
	}

	return &pb.RefreshTokenResponse{
		Success:      true,
		AccessToken:  newTokens.AccessToken,
		RefreshToken: newTokens.RefreshToken,
		Message:      "Tokens refreshed successfully",
	}, nil
}

func (s *AuthService) ResendCode(ctx context.Context, req *pb.ResendCodeRequest) (*pb.ResendCodeResponse, error) {
	email := validator.NormalizeEmail(req.Email)
	if err := validator.ValidateEmail(email); err != nil {
		return &pb.ResendCodeResponse{
			Success: false,
			Message: "Invalid email address",
		}, nil
	}

	code, err := generateCode(4)
	if err != nil {
		log.Printf("Failed to generate code: %v", err)
		return &pb.ResendCodeResponse{
			Success: false,
			Message: "Failed to generate verification code",
		}, nil
	}

	log.Printf("Generated code %s for %s", code, email)

	if err := s.authRepo.SaveAuthCode(ctx, email, code); err != nil {
		log.Printf("Failed to save auth code: %v", err)
		return &pb.ResendCodeResponse{
			Success: false,
			Message: "Failed to save verification code",
		}, nil
	}

	event := map[string]string{
		"email": email,
		"code":  code,
	}
	eventData, _ := json.Marshal(event)

	if err := s.rabbitMQ.Publish(ctx, "auth.send_code", eventData); err != nil {
		log.Printf("Failed to publish send_auth_code event: %v", err)
	}

	return &pb.ResendCodeResponse{
		Success: true,
		Message: "Verification code resent to your email",
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	jti, err := jwt.ExtractJTI(req.AccessToken)
	if err != nil {
		log.Printf("Failed to extract JTI: %v", err)
		return &pb.LogoutResponse{
			Success: false,
			Message: "Invalid access token",
		}, nil
	}

	if err := s.authRepo.AddToBlacklist(ctx, jti); err != nil {
		log.Printf("Failed to add token to blacklist: %v", err)
	}

	if req.RefreshToken != "" {
		if err := s.authRepo.DeleteRefreshToken(ctx, req.RefreshToken); err != nil {
			log.Printf("Failed to delete refresh token: %v", err)
		}
	}

	return &pb.LogoutResponse{
		Success: true,
		Message: "Logged out successfully",
	}, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	claims, err := jwt.ValidateAccessToken(req.AccessToken, s.jwtSecret)
	if err != nil {
		return &pb.ValidateTokenResponse{
			Valid:   false,
			Message: "Invalid token",
		}, nil
	}

	isBlacklisted, err := s.authRepo.IsBlacklisted(ctx, claims.JTI)
	if err != nil {
		log.Printf("Failed to check blacklist: %v", err)
		return &pb.ValidateTokenResponse{
			Valid:   false,
			Message: "Failed to validate token",
		}, nil
	}

	if isBlacklisted {
		return &pb.ValidateTokenResponse{
			Valid:   false,
			Message: "Token has been revoked",
		}, nil
	}

	return &pb.ValidateTokenResponse{
		Valid:   true,
		UserId:  claims.UserID,
		Email:   claims.Email,
		Message: "Token is valid",
	}, nil
}

func generateCode(length int) (string, error) {
	const digits = "0123456789"
	code := make([]byte, length)

	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[num.Int64()]
	}

	return string(code), nil
}