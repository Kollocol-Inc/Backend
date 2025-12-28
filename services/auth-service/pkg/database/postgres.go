package database

import (
	"context"
	"database/sql"
	"fmt"

	"auth-service/config"

	_ "github.com/lib/pq"
)

type PostgresClient struct {
	db     *sql.DB
	config *config.DBConfig
}

func NewPostgresClient(cfg *config.DBConfig) (*PostgresClient, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresClient{
		db:     db,
		config: cfg,
	}, nil
}

func (c *PostgresClient) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *PostgresClient) GetDB() *sql.DB {
	return c.db
}

func (c *PostgresClient) InitSchema(ctx context.Context) error {
	createUsersTable := `
		CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(255) PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			first_name VARCHAR(255) NOT NULL DEFAULT '',
			last_name VARCHAR(255) NOT NULL DEFAULT '',
			avatar_url TEXT NOT NULL DEFAULT '',
			is_registered BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	`

	createRefreshTokensTable := `
		CREATE TABLE IF NOT EXISTS refresh_tokens (
			token_hash VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
	`

	if _, err := c.db.ExecContext(ctx, createUsersTable); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	if _, err := c.db.ExecContext(ctx, createRefreshTokensTable); err != nil {
		return fmt.Errorf("failed to create refresh_tokens table: %w", err)
	}

	return nil
}