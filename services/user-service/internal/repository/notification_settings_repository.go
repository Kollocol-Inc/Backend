package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

type NotificationSettings struct {
	UserID           string
	NewQuizzes       bool
	QuizResults      bool
	GroupInvites     bool
	DeadlineReminder string // "1h", "24h", "never"
	UpdatedAt        time.Time
}

type NotificationSettingsRepository struct {
	db *sql.DB
}

func NewNotificationSettingsRepository(db *sql.DB) *NotificationSettingsRepository {
	return &NotificationSettingsRepository{db: db}
}

func (r *NotificationSettingsRepository) GetSettings(ctx context.Context, userID string) (*NotificationSettings, error) {
	query := `
		SELECT user_id, new_quizzes, quiz_results, group_invites, deadline_reminder, updated_at
		FROM user_notification_settings
		WHERE user_id = $1
	`

	settings := &NotificationSettings{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&settings.UserID,
		&settings.NewQuizzes,
		&settings.QuizResults,
		&settings.GroupInvites,
		&settings.DeadlineReminder,
		&settings.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("settings not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}

	return settings, nil
}

func (r *NotificationSettingsRepository) CreateDefaultSettings(ctx context.Context, userID string) (*NotificationSettings, error) {
	settings := &NotificationSettings{
		UserID:           userID,
		NewQuizzes:       true,
		QuizResults:      true,
		GroupInvites:     true,
		DeadlineReminder: "24h",
		UpdatedAt:        time.Now(),
	}

	query := `
		INSERT INTO user_notification_settings (user_id, new_quizzes, quiz_results, group_invites, deadline_reminder, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET
			new_quizzes = EXCLUDED.new_quizzes,
			quiz_results = EXCLUDED.quiz_results,
			group_invites = EXCLUDED.group_invites,
			deadline_reminder = EXCLUDED.deadline_reminder,
			updated_at = EXCLUDED.updated_at
		RETURNING user_id, new_quizzes, quiz_results, group_invites, deadline_reminder, updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		settings.UserID,
		settings.NewQuizzes,
		settings.QuizResults,
		settings.GroupInvites,
		settings.DeadlineReminder,
		settings.UpdatedAt,
	).Scan(
		&settings.UserID,
		&settings.NewQuizzes,
		&settings.QuizResults,
		&settings.GroupInvites,
		&settings.DeadlineReminder,
		&settings.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create settings: %w", err)
	}

	return settings, nil
}

func (r *NotificationSettingsRepository) UpdateSettings(ctx context.Context, settings *NotificationSettings) error {
	settings.UpdatedAt = time.Now()

	query := `
		INSERT INTO user_notification_settings (user_id, new_quizzes, quiz_results, group_invites, deadline_reminder, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET
			new_quizzes = EXCLUDED.new_quizzes,
			quiz_results = EXCLUDED.quiz_results,
			group_invites = EXCLUDED.group_invites,
			deadline_reminder = EXCLUDED.deadline_reminder,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.ExecContext(ctx, query,
		settings.UserID,
		settings.NewQuizzes,
		settings.QuizResults,
		settings.GroupInvites,
		settings.DeadlineReminder,
		settings.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	return nil
}

func (r *NotificationSettingsRepository) GetOrCreateSettings(ctx context.Context, userID string) (*NotificationSettings, error) {
	log.Printf("GetOrCreateSettings for user %s", userID)
	settings, err := r.GetSettings(ctx, userID)
	if err == nil {
		log.Printf("Found existing settings: NewQuizzes=%v, QuizResults=%v, GroupInvites=%v, DeadlineReminder=%s",
			settings.NewQuizzes, settings.QuizResults, settings.GroupInvites, settings.DeadlineReminder)
		return settings, nil
	}

	log.Printf("Settings not found (error: %v), creating default ones", err)
	return r.CreateDefaultSettings(ctx, userID)
}