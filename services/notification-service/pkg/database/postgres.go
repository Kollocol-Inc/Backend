package database

import (
	"context"
	"database/sql"
	"fmt"

	"notification-service/config"

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
	createNotificationsTable := `
		CREATE TABLE IF NOT EXISTS notifications (
			id VARCHAR(255) PRIMARY KEY,
			user_id VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			title VARCHAR(255) NOT NULL,
			content TEXT NOT NULL,
			is_read BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_notifications_user_read ON notifications(user_id, is_read);
	`

	if _, err := c.db.ExecContext(ctx, createNotificationsTable); err != nil {
		return fmt.Errorf("failed to create notifications table: %w", err)
	}

	return nil
}