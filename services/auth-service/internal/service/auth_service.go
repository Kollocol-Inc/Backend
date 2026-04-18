package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"log"
	"math/big"
	"time"

	"auth-service/internal/repository"
	"auth-service/pkg/cache"
	"auth-service/pkg/errors"
	"auth-service/pkg/jwt"
	"auth-service/pkg/messaging"
	"auth-service/pkg/validator"
	pb "auth-service/proto"

	"google.golang.org/grpc/codes"
)

type AuthRepository interface {
	SaveAuthCode(ctx context.Context, email, code string) error
	GetAuthCode(ctx context.Context, email string) (*repository.AuthCode, error)
	IncrementAuthCodeAttempts(ctx context.Context, email string) error
	DeleteAuthCode(ctx context.Context, email string) error
	AddToBlacklist(ctx context.Context, jti string) error
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
	SaveRefreshToken(ctx context.Context, token *repository.RefreshToken) error
	GetRefreshToken(ctx context.Context, token string) (*repository.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
}

type UserRepository interface {
	GetOrCreateUser(ctx context.Context, email string) (*repository.User, error)
}

type MessagePublisher interface {
	Publish(ctx context.Context, queueName string, body []byte) error
}

type AIBanRepository interface {
	CreateAIBan(ctx context.Context, ban *repository.AIBan) (*repository.AIBan, error)
	DeleteAIBan(ctx context.Context, userID string) error
	GetAIBan(ctx context.Context, userID string) (*repository.AIBan, error)
	ListAIBans(ctx context.Context) ([]*repository.AIBan, error)
	IsAIBanned(ctx context.Context, userID string) (bool, string, error)
}

type AuthService struct {
	pb.UnimplementedAuthServiceServer
	authRepo  AuthRepository
	userRepo  UserRepository
	aiBanRepo AIBanRepository
	rabbitMQ  MessagePublisher
	jwtSecret string
}

func NewAuthService(redis *cache.RedisClient, db *sql.DB, rabbitMQ *messaging.RabbitMQClient, jwtSecret string) *AuthService {
	return &AuthService{
		authRepo:  repository.NewAuthRepository(redis, db),
		userRepo:  repository.NewUserRepository(db),
		aiBanRepo: repository.NewAIBanRepository(db),
		rabbitMQ:  rabbitMQ,
		jwtSecret: jwtSecret,
	}
}

func NewAuthServiceWithDeps(authRepo AuthRepository, userRepo UserRepository, aiBanRepo AIBanRepository, rabbitMQ MessagePublisher, jwtSecret string) *AuthService {
	return &AuthService{
		authRepo:  authRepo,
		userRepo:  userRepo,
		aiBanRepo: aiBanRepo,
		rabbitMQ:  rabbitMQ,
		jwtSecret: jwtSecret,
	}
}

func (s *AuthService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	email := validator.NormalizeEmail(req.Email)
	if err := validator.ValidateEmail(email); err != nil {
		return nil, errors.New(codes.InvalidArgument, errors.ReasonInvalidEmail, "Invalid email address", map[string]string{"email": email})
	}

	code, err := generateCode(4)
	if err != nil {
		log.Printf("Failed to generate code: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonCodeGenerationFailed, "Failed to generate verification code", nil)
	}

	log.Printf("Generated code %s for %s", code, email)

	if err := s.authRepo.SaveAuthCode(ctx, email, code); err != nil {
		log.Printf("Failed to save auth code: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonCodeSaveFailed, "Failed to save verification code", nil)
	}

	event := map[string]string{
		"email": email,
		"code":  code,
	}
	eventData, _ := json.Marshal(event)

	if err := s.rabbitMQ.Publish(ctx, "auth.send_code", eventData); err != nil {
		log.Printf("Failed to publish send_auth_code event: %v", err)
	}

	return &pb.LoginResponse{}, nil
}

func (s *AuthService) VerifyCode(ctx context.Context, req *pb.VerifyCodeRequest) (*pb.VerifyCodeResponse, error) {
	email := validator.NormalizeEmail(req.Email)

	authCode, err := s.authRepo.GetAuthCode(ctx, email)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonCodeNotFound, "Verification code not found or expired", map[string]string{"email": email})
	}

	if authCode.Attempts >= repository.MaxAttempts {
		s.authRepo.DeleteAuthCode(ctx, email)
		return nil, errors.New(codes.FailedPrecondition, errors.ReasonTooManyAttempts, "Too many failed attempts. Please request a new code", map[string]string{"email": email})
	}

	if authCode.Code != req.Code {
		s.authRepo.IncrementAuthCodeAttempts(ctx, email)
		return nil, errors.New(codes.InvalidArgument, errors.ReasonInvalidCode, "Invalid verification code", map[string]string{"email": email})
	}

	if time.Now().After(authCode.ExpiresAt) {
		s.authRepo.DeleteAuthCode(ctx, email)
		return nil, errors.New(codes.DeadlineExceeded, errors.ReasonCodeExpired, "Verification code expired", map[string]string{"email": email})
	}

	user, err := s.userRepo.GetOrCreateUser(ctx, email)
	if err != nil {
		log.Printf("Failed to get or create user: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonUserProcessFailed, "Failed to process user", map[string]string{"email": email})
	}

	tokens, err := jwt.GenerateTokenPair(user.ID, user.Email, user.Role, s.jwtSecret)
	if err != nil {
		log.Printf("Failed to generate tokens: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonTokenGenerationFailed, "Failed to generate tokens", map[string]string{"email": email, "user_id": user.ID})
	}

	refreshToken := repository.NewRefreshToken(tokens.RefreshToken, user.ID, time.Now().Add(jwt.RefreshTokenDuration), time.Now())
	if err := s.authRepo.SaveRefreshToken(ctx, refreshToken); err != nil {
		log.Printf("Failed to save refresh token: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonTokenSaveFailed, "Failed to save refresh token", map[string]string{"email": email, "user_id": user.ID})
	}

	s.authRepo.DeleteAuthCode(ctx, email)

	return &pb.VerifyCodeResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		IsRegistered: user.IsRegistered,
		UserId:       user.ID,
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	claims, err := jwt.ValidateRefreshToken(req.RefreshToken, s.jwtSecret)
	if err != nil {
		return nil, errors.New(codes.Unauthenticated, errors.ReasonInvalidToken, "Invalid refresh token", nil)
	}

	storedToken, err := s.authRepo.GetRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, errors.New(codes.NotFound, errors.ReasonTokenNotFound, "Refresh token not found", nil)
	}

	if time.Now().After(storedToken.ExpiresAt) {
		s.authRepo.DeleteRefreshToken(ctx, req.RefreshToken)
		return nil, errors.New(codes.Unauthenticated, errors.ReasonTokenExpired, "Refresh token expired", nil)
	}

	newTokens, err := jwt.GenerateTokenPair(claims.UserID, claims.Email, claims.Role, s.jwtSecret)
	if err != nil {
		log.Printf("Failed to generate new tokens: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonTokenGenerationFailed, "Failed to generate new tokens", map[string]string{"email": claims.Email, "user_id": claims.UserID})
	}

	if err := s.authRepo.DeleteRefreshToken(ctx, req.RefreshToken); err != nil {
		log.Printf("Failed to delete old refresh token: %v", err)
	}

	newRefreshToken := repository.NewRefreshToken(newTokens.RefreshToken, claims.UserID, time.Now().Add(jwt.RefreshTokenDuration), time.Now())
	if err := s.authRepo.SaveRefreshToken(ctx, newRefreshToken); err != nil {
		log.Printf("Failed to save new refresh token: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonTokenSaveFailed, "Failed to save new refresh token", map[string]string{"email": claims.Email, "user_id": claims.UserID})
	}

	return &pb.RefreshTokenResponse{
		AccessToken:  newTokens.AccessToken,
		RefreshToken: newTokens.RefreshToken,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	jti, err := jwt.ExtractJTI(req.AccessToken)
	if err != nil {
		log.Printf("Failed to extract JTI: %v", err)
		return nil, errors.New(codes.InvalidArgument, errors.ReasonInvalidToken, "Invalid access token", nil)
	}

	if err := s.authRepo.AddToBlacklist(ctx, jti); err != nil {
		log.Printf("Failed to add token to blacklist: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonBlacklistAddFailed, "Failed to blacklist token", nil)
	}

	if req.RefreshToken != "" {
		if err := s.authRepo.DeleteRefreshToken(ctx, req.RefreshToken); err != nil {
			log.Printf("Failed to delete refresh token: %v", err)
		}
	}

	return &pb.LogoutResponse{}, nil
}

func (s *AuthService) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	claims, err := jwt.ValidateAccessToken(req.AccessToken, s.jwtSecret)
	if err != nil {
		return nil, errors.New(codes.Unauthenticated, errors.ReasonInvalidToken, "Invalid token", nil)
	}

	isBlacklisted, err := s.authRepo.IsBlacklisted(ctx, claims.JTI)
	if err != nil {
		log.Printf("Failed to check blacklist: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonTokenValidationFailed, "Failed to validate token", nil)
	}

	if isBlacklisted {
		return nil, errors.New(codes.Unauthenticated, errors.ReasonTokenRevoked, "Token has been revoked", nil)
	}

	return &pb.ValidateTokenResponse{
		UserId: claims.UserID,
		Email:  claims.Email,
		Role:   claims.Role,
	}, nil
}

func (s *AuthService) CreateAIBan(ctx context.Context, req *pb.CreateAIBanRequest) (*pb.CreateAIBanResponse, error) {
	if req.UserId == "" {
		return nil, errors.New(codes.InvalidArgument, errors.ReasonUserIDRequired, "User ID is required", nil)
	}

	ban, err := s.aiBanRepo.CreateAIBan(ctx, &repository.AIBan{
		UserID:    req.UserId,
		Reason:    req.Reason,
		BannedBy:  req.BannedBy,
		CreatedAt: time.Now(),
	})
	if err != nil {
		log.Printf("Failed to create AI ban: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonAIBanCreateFailed, "Failed to create AI ban", nil)
	}

	return &pb.CreateAIBanResponse{
		Ban: &pb.AIBan{
			UserId:    ban.UserID,
			Reason:    ban.Reason,
			BannedBy:  ban.BannedBy,
			CreatedAt: ban.CreatedAt.Unix(),
		},
	}, nil
}

func (s *AuthService) DeleteAIBan(ctx context.Context, req *pb.DeleteAIBanRequest) (*pb.DeleteAIBanResponse, error) {
	if req.UserId == "" {
		return nil, errors.New(codes.InvalidArgument, errors.ReasonUserIDRequired, "User ID is required", nil)
	}

	if err := s.aiBanRepo.DeleteAIBan(ctx, req.UserId); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(codes.NotFound, errors.ReasonAIBanNotFound, "AI ban not found", map[string]string{"user_id": req.UserId})
		}
		log.Printf("Failed to delete AI ban: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonAIBanDeleteFailed, "Failed to delete AI ban", nil)
	}

	return &pb.DeleteAIBanResponse{}, nil
}

func (s *AuthService) GetAIBan(ctx context.Context, req *pb.GetAIBanRequest) (*pb.GetAIBanResponse, error) {
	if req.UserId == "" {
		return nil, errors.New(codes.InvalidArgument, errors.ReasonUserIDRequired, "User ID is required", nil)
	}

	ban, err := s.aiBanRepo.GetAIBan(ctx, req.UserId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(codes.NotFound, errors.ReasonAIBanNotFound, "AI ban not found", map[string]string{"user_id": req.UserId})
		}
		log.Printf("Failed to get AI ban: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonAIBanCheckFailed, "Failed to get AI ban", nil)
	}

	return &pb.GetAIBanResponse{
		Ban: &pb.AIBan{
			UserId:    ban.UserID,
			Reason:    ban.Reason,
			BannedBy:  ban.BannedBy,
			CreatedAt: ban.CreatedAt.Unix(),
		},
	}, nil
}

func (s *AuthService) ListAIBans(ctx context.Context, req *pb.ListAIBansRequest) (*pb.ListAIBansResponse, error) {
	bans, err := s.aiBanRepo.ListAIBans(ctx)
	if err != nil {
		log.Printf("Failed to list AI bans: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonAIBanListFailed, "Failed to list AI bans", nil)
	}

	pbBans := make([]*pb.AIBan, len(bans))
	for i, ban := range bans {
		pbBans[i] = &pb.AIBan{
			UserId:    ban.UserID,
			Reason:    ban.Reason,
			BannedBy:  ban.BannedBy,
			CreatedAt: ban.CreatedAt.Unix(),
		}
	}

	return &pb.ListAIBansResponse{Bans: pbBans}, nil
}

func (s *AuthService) CheckAIBan(ctx context.Context, req *pb.CheckAIBanRequest) (*pb.CheckAIBanResponse, error) {
	if req.UserId == "" {
		return nil, errors.New(codes.InvalidArgument, errors.ReasonUserIDRequired, "User ID is required", nil)
	}

	isBanned, reason, err := s.aiBanRepo.IsAIBanned(ctx, req.UserId)
	if err != nil {
		log.Printf("Failed to check AI ban: %v", err)
		return nil, errors.New(codes.Internal, errors.ReasonAIBanCheckFailed, "Failed to check AI ban status", nil)
	}

	return &pb.CheckAIBanResponse{
		IsBanned: isBanned,
		Reason:   reason,
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
