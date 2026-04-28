package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type DeleteRepository struct {
	db *sql.DB
}

func NewDeleteRepository(db *sql.DB) *DeleteRepository {
	return &DeleteRepository{db: db}
}

func (r *DeleteRepository) DeleteAllByOwner(ctx context.Context, userID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	statements := []struct {
		name  string
		query string
	}{
		{"game_sessions as participant",
			`DELETE FROM game_sessions WHERE user_id = $1`},
		{"game_sessions in owned instances",
			`DELETE FROM game_sessions WHERE instance_id IN
			 (SELECT id FROM quiz_instances WHERE created_by = $1)`},
		{"instance_questions of owned instances",
			`DELETE FROM instance_questions WHERE instance_id IN
			 (SELECT id FROM quiz_instances WHERE created_by = $1)`},
		{"quiz_instances",
			`DELETE FROM quiz_instances WHERE created_by = $1`},
		{"template_questions of owned templates",
			`DELETE FROM template_questions WHERE template_id IN
			 (SELECT id FROM quiz_templates WHERE owner_id = $1)`},
		{"quiz_templates",
			`DELETE FROM quiz_templates WHERE owner_id = $1`},
		{"quiz_deadline_reminders_sent",
			`DELETE FROM quiz_deadline_reminders_sent WHERE user_id = $1`},
	}

	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt.query, userID); err != nil {
			return fmt.Errorf("delete %s: %w", stmt.name, err)
		}
	}

	return tx.Commit()
}
