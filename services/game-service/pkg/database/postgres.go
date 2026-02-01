package database

import (
	"context"
	"database/sql"
	"fmt"

	"game-service/config"

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
	createGameSessionsTable := `
		CREATE TABLE IF NOT EXISTS game_sessions (
			instance_id VARCHAR(255) NOT NULL,
			user_id VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'joined',
			current_question_index INTEGER NOT NULL DEFAULT 0,
			score INTEGER NOT NULL DEFAULT 0,
			answers JSONB NOT NULL DEFAULT '[]',
			started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			finished_at TIMESTAMP,
			PRIMARY KEY (instance_id, user_id)
		);
		CREATE INDEX IF NOT EXISTS idx_game_sessions_instance_id ON game_sessions(instance_id);
		CREATE INDEX IF NOT EXISTS idx_game_sessions_user_id ON game_sessions(user_id);
		CREATE INDEX IF NOT EXISTS idx_game_sessions_status ON game_sessions(status);
	`

	if _, err := c.db.ExecContext(ctx, createGameSessionsTable); err != nil {
		return fmt.Errorf("failed to create game_sessions table: %w", err)
	}

	return nil
}