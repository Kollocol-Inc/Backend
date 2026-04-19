package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type AIBan struct {
	UserID    string
	Reason    string
	BannedBy  string
	CreatedAt time.Time
}

type AIBanRepository struct {
	db *sql.DB
}

func NewAIBanRepository(db *sql.DB) *AIBanRepository {
	return &AIBanRepository{db: db}
}

func (r *AIBanRepository) CreateAIBan(ctx context.Context, ban *AIBan) (*AIBan, error) {
	query := `
		INSERT INTO ai_bans (user_id, reason, banned_by, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			reason = EXCLUDED.reason,
			banned_by = EXCLUDED.banned_by,
			created_at = EXCLUDED.created_at
		RETURNING user_id, reason, banned_by, created_at
	`

	result := &AIBan{}
	err := r.db.QueryRowContext(ctx, query,
		ban.UserID,
		ban.Reason,
		ban.BannedBy,
		ban.CreatedAt,
	).Scan(
		&result.UserID,
		&result.Reason,
		&result.BannedBy,
		&result.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI ban: %w", err)
	}

	return result, nil
}

func (r *AIBanRepository) DeleteAIBan(ctx context.Context, userID string) error {
	query := `DELETE FROM ai_bans WHERE user_id = $1`

	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete AI ban: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *AIBanRepository) GetAIBan(ctx context.Context, userID string) (*AIBan, error) {
	query := `
		SELECT user_id, reason, banned_by, created_at
		FROM ai_bans
		WHERE user_id = $1
	`

	ban := &AIBan{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&ban.UserID,
		&ban.Reason,
		&ban.BannedBy,
		&ban.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get AI ban: %w", err)
	}

	return ban, nil
}

func (r *AIBanRepository) ListAIBans(ctx context.Context) ([]*AIBan, error) {
	query := `
		SELECT user_id, reason, banned_by, created_at
		FROM ai_bans
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list AI bans: %w", err)
	}
	defer rows.Close()

	var bans []*AIBan
	for rows.Next() {
		ban := &AIBan{}
		if err := rows.Scan(&ban.UserID, &ban.Reason, &ban.BannedBy, &ban.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan AI ban: %w", err)
		}
		bans = append(bans, ban)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate AI bans: %w", err)
	}

	return bans, nil
}

func (r *AIBanRepository) IsAIBanned(ctx context.Context, userID string) (bool, string, error) {
	query := `SELECT reason FROM ai_bans WHERE user_id = $1`

	var reason string
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&reason)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("failed to check AI ban: %w", err)
	}

	return true, reason, nil
}
