package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"auth-service/internal/repository"
	"auth-service/internal/service/mocks"
	"auth-service/pkg/jwt"
	pb "auth-service/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const testJWTSecret = "test-secret-key-for-unit-tests"

func setupTest(t *testing.T) (*AuthService, *mocks.MockAuthRepository, *mocks.MockUserRepository, *mocks.MockMessagePublisher) {
	ctrl := gomock.NewController(t)
	authRepo := mocks.NewMockAuthRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	publisher := mocks.NewMockMessagePublisher(ctrl)

	svc := NewAuthServiceWithDeps(authRepo, userRepo, nil, publisher, testJWTSecret)
	return svc, authRepo, userRepo, publisher
}

func TestLogin_Success(t *testing.T) {
	svc, authRepo, _, publisher := setupTest(t)
	ctx := context.Background()

	authRepo.EXPECT().SaveAuthCode(ctx, "test@example.com", gomock.Any()).Return(nil)
	publisher.EXPECT().Publish(ctx, "auth.send_code", gomock.Any()).Return(nil)

	resp, err := svc.Login(ctx, &pb.LoginRequest{Email: "test@example.com"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestLogin_InvalidEmail(t *testing.T) {
	svc, _, _, _ := setupTest(t)
	ctx := context.Background()

	_, err := svc.Login(ctx, &pb.LoginRequest{Email: "not-an-email"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid email")
}

func TestLogin_NormalizesEmail(t *testing.T) {
	svc, authRepo, _, publisher := setupTest(t)
	ctx := context.Background()

	authRepo.EXPECT().SaveAuthCode(ctx, "test@example.com", gomock.Any()).Return(nil)
	publisher.EXPECT().Publish(ctx, "auth.send_code", gomock.Any()).Return(nil)

	_, err := svc.Login(ctx, &pb.LoginRequest{Email: "  Test@Example.COM  "})
	require.NoError(t, err)
}

func TestLogin_SaveCodeFails(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	authRepo.EXPECT().SaveAuthCode(ctx, "test@example.com", gomock.Any()).Return(fmt.Errorf("redis error"))

	_, err := svc.Login(ctx, &pb.LoginRequest{Email: "test@example.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to save verification code")
}

func TestLogin_PublishFails_StillSucceeds(t *testing.T) {
	svc, authRepo, _, publisher := setupTest(t)
	ctx := context.Background()

	authRepo.EXPECT().SaveAuthCode(ctx, "test@example.com", gomock.Any()).Return(nil)
	publisher.EXPECT().Publish(ctx, "auth.send_code", gomock.Any()).Return(fmt.Errorf("rabbitmq down"))

	resp, err := svc.Login(ctx, &pb.LoginRequest{Email: "test@example.com"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestLogin_AppleReviewEmail_UsesHardcodedCodeAndSkipsPublish(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	authRepo.EXPECT().SaveAuthCode(ctx, "apple@review.com", "1234").Return(nil)

	resp, err := svc.Login(ctx, &pb.LoginRequest{Email: "apple@review.com"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestLogin_AppleReviewEmail_CaseInsensitive(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	authRepo.EXPECT().SaveAuthCode(ctx, "apple@review.com", "1234").Return(nil)

	resp, err := svc.Login(ctx, &pb.LoginRequest{Email: "  Apple@Review.COM  "})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestVerifyCode_Success(t *testing.T) {
	svc, authRepo, userRepo, _ := setupTest(t)
	ctx := context.Background()

	authCode := &repository.AuthCode{
		Code:      "1234",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Attempts:  0,
	}
	user := &repository.User{
		ID:           "user-123",
		Email:        "test@example.com",
		IsRegistered: true,
	}

	authRepo.EXPECT().GetAuthCode(ctx, "test@example.com").Return(authCode, nil)
	userRepo.EXPECT().GetOrCreateUser(ctx, "test@example.com").Return(user, nil)
	authRepo.EXPECT().SaveRefreshToken(ctx, gomock.Any()).Return(nil)
	authRepo.EXPECT().DeleteAuthCode(ctx, "test@example.com").Return(nil)

	resp, err := svc.VerifyCode(ctx, &pb.VerifyCodeRequest{
		Email: "test@example.com",
		Code:  "1234",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.True(t, resp.IsRegistered)
	assert.Equal(t, "user-123", resp.UserId)
}

func TestVerifyCode_CodeNotFound(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	authRepo.EXPECT().GetAuthCode(ctx, "test@example.com").Return(nil, fmt.Errorf("not found"))

	_, err := svc.VerifyCode(ctx, &pb.VerifyCodeRequest{
		Email: "test@example.com",
		Code:  "1234",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found or expired")
}

func TestVerifyCode_TooManyAttempts(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	authCode := &repository.AuthCode{
		Code:      "1234",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Attempts:  5,
	}

	authRepo.EXPECT().GetAuthCode(ctx, "test@example.com").Return(authCode, nil)
	authRepo.EXPECT().DeleteAuthCode(ctx, "test@example.com").Return(nil)

	_, err := svc.VerifyCode(ctx, &pb.VerifyCodeRequest{
		Email: "test@example.com",
		Code:  "1234",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Too many failed attempts")
}

func TestVerifyCode_WrongCode(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	authCode := &repository.AuthCode{
		Code:      "1234",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Attempts:  0,
	}

	authRepo.EXPECT().GetAuthCode(ctx, "test@example.com").Return(authCode, nil)
	authRepo.EXPECT().IncrementAuthCodeAttempts(ctx, "test@example.com").Return(nil)

	_, err := svc.VerifyCode(ctx, &pb.VerifyCodeRequest{
		Email: "test@example.com",
		Code:  "9999",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid verification code")
}

func TestVerifyCode_ExpiredCode(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	authCode := &repository.AuthCode{
		Code:      "1234",
		ExpiresAt: time.Now().Add(-1 * time.Minute),
		Attempts:  0,
	}

	authRepo.EXPECT().GetAuthCode(ctx, "test@example.com").Return(authCode, nil)
	authRepo.EXPECT().DeleteAuthCode(ctx, "test@example.com").Return(nil)

	_, err := svc.VerifyCode(ctx, &pb.VerifyCodeRequest{
		Email: "test@example.com",
		Code:  "1234",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestVerifyCode_UserCreationFails(t *testing.T) {
	svc, authRepo, userRepo, _ := setupTest(t)
	ctx := context.Background()

	authCode := &repository.AuthCode{
		Code:      "1234",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Attempts:  0,
	}

	authRepo.EXPECT().GetAuthCode(ctx, "test@example.com").Return(authCode, nil)
	userRepo.EXPECT().GetOrCreateUser(ctx, "test@example.com").Return(nil, fmt.Errorf("db error"))

	_, err := svc.VerifyCode(ctx, &pb.VerifyCodeRequest{
		Email: "test@example.com",
		Code:  "1234",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to process user")
}

func TestRefreshToken_Success(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, err := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)
	require.NoError(t, err)

	storedToken := &repository.RefreshToken{
		TokenHash: "hash",
		UserID:    "user-123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	authRepo.EXPECT().GetRefreshToken(ctx, pair.RefreshToken).Return(storedToken, nil)
	authRepo.EXPECT().IsUserRevoked(ctx, "user-123").Return(false, nil)
	authRepo.EXPECT().DeleteRefreshToken(ctx, pair.RefreshToken).Return(nil)
	authRepo.EXPECT().SaveRefreshToken(ctx, gomock.Any()).Return(nil)

	resp, err := svc.RefreshToken(ctx, &pb.RefreshTokenRequest{
		RefreshToken: pair.RefreshToken,
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
}

func TestRefreshToken_UserRevoked(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	storedToken := &repository.RefreshToken{
		TokenHash: "hash",
		UserID:    "user-123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	authRepo.EXPECT().GetRefreshToken(ctx, pair.RefreshToken).Return(storedToken, nil)
	authRepo.EXPECT().IsUserRevoked(ctx, "user-123").Return(true, nil)
	// no token rotation must happen — DeleteRefreshToken / SaveRefreshToken must NOT be called

	_, err := svc.RefreshToken(ctx, &pb.RefreshTokenRequest{
		RefreshToken: pair.RefreshToken,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deleted")
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	svc, _, _, _ := setupTest(t)
	ctx := context.Background()

	_, err := svc.RefreshToken(ctx, &pb.RefreshTokenRequest{
		RefreshToken: "invalid-token",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid refresh token")
}

func TestRefreshToken_NotFoundInDB(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	authRepo.EXPECT().GetRefreshToken(ctx, pair.RefreshToken).Return(nil, fmt.Errorf("not found"))

	_, err := svc.RefreshToken(ctx, &pb.RefreshTokenRequest{
		RefreshToken: pair.RefreshToken,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Refresh token not found")
}

func TestRefreshToken_Expired(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	storedToken := &repository.RefreshToken{
		TokenHash: "hash",
		UserID:    "user-123",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedAt: time.Now().Add(-48 * time.Hour),
	}

	authRepo.EXPECT().GetRefreshToken(ctx, pair.RefreshToken).Return(storedToken, nil)
	authRepo.EXPECT().DeleteRefreshToken(ctx, pair.RefreshToken).Return(nil)

	_, err := svc.RefreshToken(ctx, &pb.RefreshTokenRequest{
		RefreshToken: pair.RefreshToken,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestLogout_Success(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	authRepo.EXPECT().AddToBlacklist(ctx, gomock.Any()).Return(nil)
	authRepo.EXPECT().DeleteRefreshToken(ctx, pair.RefreshToken).Return(nil)

	resp, err := svc.Logout(ctx, &pb.LogoutRequest{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestLogout_WithoutRefreshToken(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	authRepo.EXPECT().AddToBlacklist(ctx, gomock.Any()).Return(nil)

	resp, err := svc.Logout(ctx, &pb.LogoutRequest{
		AccessToken:  pair.AccessToken,
		RefreshToken: "",
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestLogout_InvalidAccessToken(t *testing.T) {
	svc, _, _, _ := setupTest(t)
	ctx := context.Background()

	_, err := svc.Logout(ctx, &pb.LogoutRequest{
		AccessToken: "garbage",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid access token")
}

func TestLogout_BlacklistFails(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	authRepo.EXPECT().AddToBlacklist(ctx, gomock.Any()).Return(fmt.Errorf("redis error"))

	_, err := svc.Logout(ctx, &pb.LogoutRequest{
		AccessToken: pair.AccessToken,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to blacklist token")
}

func TestValidateToken_Success(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	authRepo.EXPECT().IsBlacklisted(ctx, gomock.Any()).Return(false, nil)
	authRepo.EXPECT().IsUserRevoked(ctx, "user-123").Return(false, nil)

	resp, err := svc.ValidateToken(ctx, &pb.ValidateTokenRequest{
		AccessToken: pair.AccessToken,
	})
	require.NoError(t, err)
	assert.Equal(t, "user-123", resp.UserId)
	assert.Equal(t, "test@example.com", resp.Email)
}

func TestValidateToken_UserRevoked(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	authRepo.EXPECT().IsBlacklisted(ctx, gomock.Any()).Return(false, nil)
	authRepo.EXPECT().IsUserRevoked(ctx, "user-123").Return(true, nil)

	_, err := svc.ValidateToken(ctx, &pb.ValidateTokenRequest{
		AccessToken: pair.AccessToken,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "deleted")
}

func TestValidateToken_UserRevokedCheckFails(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	authRepo.EXPECT().IsBlacklisted(ctx, gomock.Any()).Return(false, nil)
	authRepo.EXPECT().IsUserRevoked(ctx, "user-123").Return(false, fmt.Errorf("redis error"))

	_, err := svc.ValidateToken(ctx, &pb.ValidateTokenRequest{
		AccessToken: pair.AccessToken,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to validate token")
}

func TestRevokeUser_Success(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	authRepo.EXPECT().RevokeUser(ctx, "user-123").Return(nil)

	resp, err := svc.RevokeUser(ctx, &pb.RevokeUserRequest{UserId: "user-123"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestRevokeUser_EmptyUserID(t *testing.T) {
	svc, _, _, _ := setupTest(t)
	ctx := context.Background()

	_, err := svc.RevokeUser(ctx, &pb.RevokeUserRequest{UserId: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "User ID is required")
}

func TestRevokeUser_RepoFails(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	authRepo.EXPECT().RevokeUser(ctx, "user-123").Return(fmt.Errorf("redis down"))

	_, err := svc.RevokeUser(ctx, &pb.RevokeUserRequest{UserId: "user-123"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to revoke user")
}

func TestValidateToken_InvalidToken(t *testing.T) {
	svc, _, _, _ := setupTest(t)
	ctx := context.Background()

	_, err := svc.ValidateToken(ctx, &pb.ValidateTokenRequest{
		AccessToken: "invalid",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid token")
}

func TestValidateToken_Blacklisted(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	authRepo.EXPECT().IsBlacklisted(ctx, gomock.Any()).Return(true, nil)

	_, err := svc.ValidateToken(ctx, &pb.ValidateTokenRequest{
		AccessToken: pair.AccessToken,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "revoked")
}

func TestValidateToken_BlacklistCheckFails(t *testing.T) {
	svc, authRepo, _, _ := setupTest(t)
	ctx := context.Background()

	pair, _ := jwt.GenerateTokenPair("user-123", "test@example.com", "user", testJWTSecret)

	authRepo.EXPECT().IsBlacklisted(ctx, gomock.Any()).Return(false, fmt.Errorf("redis error"))

	_, err := svc.ValidateToken(ctx, &pb.ValidateTokenRequest{
		AccessToken: pair.AccessToken,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Failed to validate token")
}
