package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           string
	Email        string
	FirstName    string
	LastName     string
	AvatarURL    string
	IsRegistered bool
	CreatedAt    time.Time
}

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, first_name, last_name, avatar_url, is_registered, created_at
		FROM users
		WHERE email = $1
	`

	user := &User{}
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.AvatarURL,
		&user.IsRegistered,
		&user.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, email string) (*User, error) {
	user := &User{
		ID:           uuid.New().String(),
		Email:        email,
		FirstName:    "",
		LastName:     "",
		AvatarURL:    "",
		IsRegistered: false,
		CreatedAt:    time.Now(),
	}

	query := `
		INSERT INTO users (id, email, first_name, last_name, avatar_url, is_registered, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (email) DO UPDATE SET
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			avatar_url = EXCLUDED.avatar_url,
			is_registered = EXCLUDED.is_registered
		RETURNING id, email, first_name, last_name, avatar_url, is_registered, created_at
	`

	err := r.db.QueryRowContext(ctx, query,
		user.ID,
		user.Email,
		user.FirstName,
		user.LastName,
		user.AvatarURL,
		user.IsRegistered,
		user.CreatedAt,
	).Scan(
		&user.ID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.AvatarURL,
		&user.IsRegistered,
		&user.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (r *UserRepository) GetOrCreateUser(ctx context.Context, email string) (*User, error) {
	user, err := r.GetUserByEmail(ctx, email)
	if err == nil {
		return user, nil
	}

	return r.CreateUser(ctx, email)
}