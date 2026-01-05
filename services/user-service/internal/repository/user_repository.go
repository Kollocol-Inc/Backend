package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
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
	return &UserRepository{db: db}
}

func (r *UserRepository) GetUserByID(ctx context.Context, userID string) (*User, error) {
	query := `
		SELECT id, email, first_name, last_name, avatar_url, is_registered, created_at
		FROM users
		WHERE id = $1
	`

	user := &User{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
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

func (r *UserRepository) UpdateUser(ctx context.Context, user *User) error {
	query := `
		UPDATE users
		SET first_name = $2,
		    last_name = $3,
		    avatar_url = $4,
		    is_registered = $5
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.FirstName,
		user.LastName,
		user.AvatarURL,
		user.IsRegistered,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

func (r *UserRepository) GetOrCreateUser(ctx context.Context, email string) (*User, error) {
	user, err := r.GetUserByEmail(ctx, email)
	if err == nil {
		return user, nil
	}

	return r.CreateUser(ctx, email)
}

func (r *UserRepository) GetUsersByEmails(ctx context.Context, emails []string) ([]*User, error) {
	if len(emails) == 0 {
		return []*User{}, nil
	}

	query := `
		SELECT id, email, first_name, last_name, avatar_url, is_registered, created_at
		FROM users
		WHERE email = ANY($1)
	`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(emails))
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.AvatarURL, &user.IsRegistered, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return users, nil
}

func (r *UserRepository) GetUsersByEmailsMap(ctx context.Context, emails []string) (map[string]*User, error) {
	users, err := r.GetUsersByEmails(ctx, emails)
	if err != nil {
		return nil, err
	}

	userMap := make(map[string]*User, len(users))
	for _, user := range users {
		userMap[user.Email] = user
	}

	return userMap, nil
}

func (r *UserRepository) GetUsersByIDs(ctx context.Context, userIDs []string) ([]*User, error) {
	if len(userIDs) == 0 {
		return []*User{}, nil
	}

	query := `
		SELECT id, email, first_name, last_name, avatar_url, is_registered, created_at
		FROM users
		WHERE id = ANY($1)
	`

	rows, err := r.db.QueryContext(ctx, query, pq.Array(userIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.AvatarURL, &user.IsRegistered, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return users, nil
}