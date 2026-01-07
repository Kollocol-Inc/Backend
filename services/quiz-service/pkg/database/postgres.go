package database

import (
	"context"
	"database/sql"
	"fmt"

	"quiz-service/config"

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
	createQuizTemplatesTable := `
		CREATE TABLE IF NOT EXISTS quiz_templates (
			id VARCHAR(255) PRIMARY KEY,
			owner_id VARCHAR(255) NOT NULL,
			title VARCHAR(255) NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			quiz_type VARCHAR(50) NOT NULL,
			settings JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_quiz_templates_owner_id ON quiz_templates(owner_id);
	`

	createQuestionsTable := `
		CREATE TABLE IF NOT EXISTS questions (
			id VARCHAR(255) PRIMARY KEY,
			text TEXT NOT NULL,
			type VARCHAR(50) NOT NULL,
			options JSONB NOT NULL DEFAULT '[]',
			correct_answer JSONB NOT NULL DEFAULT '{}',
			max_score INTEGER NOT NULL DEFAULT 0,
			time_limit_sec INTEGER NOT NULL DEFAULT 0,
			ai_answer JSONB
		);
	`

	createTemplateQuestionsTable := `
		CREATE TABLE IF NOT EXISTS template_questions (
			template_id VARCHAR(255) NOT NULL,
			question_id VARCHAR(255) NOT NULL,
			order_index INTEGER NOT NULL,
			PRIMARY KEY (template_id, question_id),
			FOREIGN KEY (template_id) REFERENCES quiz_templates(id) ON DELETE CASCADE,
			FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_template_questions_template_id ON template_questions(template_id);
	`

	createQuizInstancesTable := `
		CREATE TABLE IF NOT EXISTS quiz_instances (
			id VARCHAR(255) PRIMARY KEY,
			template_id VARCHAR(255),
			title VARCHAR(255) NOT NULL,
			access_code VARCHAR(50) NOT NULL,
			status VARCHAR(50) NOT NULL,
			group_id VARCHAR(255),
			created_by VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			start_time TIMESTAMP,
			deadline TIMESTAMP,
			quiz_type VARCHAR(50) NOT NULL DEFAULT 'sync',
			settings JSONB NOT NULL DEFAULT '{}',
			FOREIGN KEY (template_id) REFERENCES quiz_templates(id) ON DELETE SET NULL
		);
		CREATE INDEX IF NOT EXISTS idx_quiz_instances_template_id ON quiz_instances(template_id);
		CREATE INDEX IF NOT EXISTS idx_quiz_instances_access_code ON quiz_instances(access_code);
		CREATE INDEX IF NOT EXISTS idx_quiz_instances_group_id ON quiz_instances(group_id);
	`

	createInstanceQuestionsTable := `
		CREATE TABLE IF NOT EXISTS instance_questions (
			instance_id VARCHAR(255) NOT NULL,
			question_id VARCHAR(255) NOT NULL,
			order_index INTEGER NOT NULL,
			PRIMARY KEY (instance_id, question_id),
			FOREIGN KEY (instance_id) REFERENCES quiz_instances(id) ON DELETE CASCADE,
			FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_instance_questions_instance_id ON instance_questions(instance_id);
	`

	if _, err := c.db.ExecContext(ctx, createQuizTemplatesTable); err != nil {
		return fmt.Errorf("failed to create quiz_templates table: %w", err)
	}

	if _, err := c.db.ExecContext(ctx, createQuestionsTable); err != nil {
		return fmt.Errorf("failed to create questions table: %w", err)
	}

	if _, err := c.db.ExecContext(ctx, createTemplateQuestionsTable); err != nil {
		return fmt.Errorf("failed to create template_questions table: %w", err)
	}

	if _, err := c.db.ExecContext(ctx, createQuizInstancesTable); err != nil {
		return fmt.Errorf("failed to create quiz_instances table: %w", err)
	}

	if _, err := c.db.ExecContext(ctx, createInstanceQuestionsTable); err != nil {
		return fmt.Errorf("failed to create instance_questions table: %w", err)
	}

	return nil
}