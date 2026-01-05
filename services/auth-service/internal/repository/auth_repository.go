package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"auth-service/pkg/cache"
	"auth-service/pkg/jwt"
)

const (
	CodeTTL       = 5 * time.Minute
	BlacklistTTL  = jwt.AccessTokenDuration
	MaxAttempts   = 5
)

type AuthCode struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
}

type RefreshToken struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func NewRefreshToken(token, userID string, expiresAt, createdAt time.Time) *RefreshToken {
	return &RefreshToken{
		TokenHash: hashToken(token),
		UserID: userID,
		ExpiresAt: expiresAt,
		CreatedAt: createdAt,
	}
}

type AuthRepository struct {
	redis *cache.RedisClient
	db    *sql.DB
}

func NewAuthRepository(redis *cache.RedisClient, db *sql.DB) *AuthRepository {
	return &AuthRepository{
		redis: redis,
		db:    db,
	}
}

func (r *AuthRepository) SaveAuthCode(ctx context.Context, email, code string) error {
	key := fmt.Sprintf("auth:code:%s", email)

	authCode := AuthCode{
		Code:      code,
		ExpiresAt: time.Now().Add(CodeTTL),
		Attempts:  0,
	}

	data, err := json.Marshal(authCode)
	if err != nil {
		return fmt.Errorf("failed to marshal auth code: %w", err)
	}

	return r.redis.Set(ctx, key, data, CodeTTL)
}

func (r *AuthRepository) GetAuthCode(ctx context.Context, email string) (*AuthCode, error) {
	key := fmt.Sprintf("auth:code:%s", email)

	data, err := r.redis.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("auth code not found or expired")
	}

	var authCode AuthCode
	if err := json.Unmarshal([]byte(data), &authCode); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth code: %w", err)
	}

	return &authCode, nil
}

func (r *AuthRepository) IncrementAuthCodeAttempts(ctx context.Context, email string) error {
	authCode, err := r.GetAuthCode(ctx, email)
	if err != nil {
		return err
	}

	authCode.Attempts++

	key := fmt.Sprintf("auth:code:%s", email)
	data, err := json.Marshal(authCode)
	if err != nil {
		return fmt.Errorf("failed to marshal auth code: %w", err)
	}

	ttl := time.Until(authCode.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("auth code expired")
	}

	return r.redis.Set(ctx, key, data, ttl)
}

func (r *AuthRepository) DeleteAuthCode(ctx context.Context, email string) error {
	key := fmt.Sprintf("auth:code:%s", email)
	return r.redis.Delete(ctx, key)
}

func (r *AuthRepository) AddToBlacklist(ctx context.Context, jti string) error {
	key := fmt.Sprintf("auth:blacklist:%s", jti)
	return r.redis.Set(ctx, key, "revoked", BlacklistTTL)
}

func (r *AuthRepository) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	key := fmt.Sprintf("auth:blacklist:%s", jti)
	count, err := r.redis.Exists(ctx, key)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *AuthRepository) SaveRefreshToken(ctx context.Context, token *RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (token_hash, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (token_hash) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			expires_at = EXCLUDED.expires_at,
			created_at = EXCLUDED.created_at
	`

	_, err := r.db.ExecContext(ctx, query,
		token.TokenHash,
		token.UserID,
		token.ExpiresAt,
		token.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}

	return nil
}

func (r *AuthRepository) GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error) {
	query := `
		SELECT token_hash, user_id, expires_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	hashedToken := hashToken(token)
	storedToken := &RefreshToken{}
	err := r.db.QueryRowContext(ctx, query, hashedToken).Scan(
		&storedToken.TokenHash,
		&storedToken.UserID,
		&storedToken.ExpiresAt,
		&storedToken.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("refresh token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	return storedToken, nil
}

func (r *AuthRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	query := `DELETE FROM refresh_tokens WHERE token_hash = $1`

	hashedToken := hashToken(token)
	_, err := r.db.ExecContext(ctx, query, hashedToken)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}

	return nil
}

func (r *AuthRepository) DeleteAllUserRefreshTokens(ctx context.Context, userID string) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`

	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user refresh tokens: %w", err)
	}

	return nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}