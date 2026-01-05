package database

import (
	"context"
	"database/sql"
	"fmt"

	"user-service/config"

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
	createNotificationSettingsTable := `
		CREATE TABLE IF NOT EXISTS user_notification_settings (
			user_id VARCHAR(255) PRIMARY KEY,
			new_quizzes BOOLEAN NOT NULL DEFAULT true,
			quiz_results BOOLEAN NOT NULL DEFAULT true,
			group_invites BOOLEAN NOT NULL DEFAULT true,
			deadline_reminder VARCHAR(50) NOT NULL DEFAULT '24h',
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_notification_settings_user_id ON user_notification_settings(user_id);
	`

	createGroupsTable := `
		CREATE TABLE IF NOT EXISTS groups (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			owner_id VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_groups_owner_id ON groups(owner_id);
	`

	createGroupMembersTable := `
		CREATE TABLE IF NOT EXISTS group_members (
			group_id VARCHAR(255) NOT NULL,
			user_id VARCHAR(255) NOT NULL,
			joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (group_id, user_id),
			FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_group_members_user_id ON group_members(user_id);
		CREATE INDEX IF NOT EXISTS idx_group_members_group_id ON group_members(group_id);
	`

	if _, err := c.db.ExecContext(ctx, createNotificationSettingsTable); err != nil {
		return fmt.Errorf("failed to create user_notification_settings table: %w", err)
	}

	if _, err := c.db.ExecContext(ctx, createGroupsTable); err != nil {
		return fmt.Errorf("failed to create groups table: %w", err)
	}

	if _, err := c.db.ExecContext(ctx, createGroupMembersTable); err != nil {
		return fmt.Errorf("failed to create group_members table: %w", err)
	}

	return nil
}