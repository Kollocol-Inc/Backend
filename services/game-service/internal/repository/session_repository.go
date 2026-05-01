package repository

import (
	"context"
	"fmt"

	"game-service/internal/models"
	"game-service/pkg/database"
)

type SessionRepository struct {
	db database.DBTX
}

func NewSessionRepository(db database.DBTX) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) q(ctx context.Context) database.DBTX {
	return database.Querier(ctx, r.db)
}

func (r *SessionRepository) CreateSession(ctx context.Context, session *models.GameSession) (bool, error) {
	const query = `
		INSERT INTO game_sessions (instance_id, user_id, status, current_question_index, score, answers, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (instance_id, user_id) DO NOTHING
	`
	res, err := r.q(ctx).ExecContext(ctx, query,
		session.InstanceID,
		session.UserID,
		session.Status,
		session.CurrentQuestionIndex,
		session.Score,
		session.Answers,
		session.StartedAt,
	)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}

func (r *SessionRepository) GetSession(ctx context.Context, instanceID, userID string) (*models.GameSession, error) {
	query := `
		SELECT instance_id, user_id, status, current_question_index, score, answers, started_at, finished_at
		FROM game_sessions
		WHERE instance_id = $1 AND user_id = $2
	`
	session := &models.GameSession{}
	err := r.q(ctx).QueryRowContext(ctx, query, instanceID, userID).Scan(
		&session.InstanceID,
		&session.UserID,
		&session.Status,
		&session.CurrentQuestionIndex,
		&session.Score,
		&session.Answers,
		&session.StartedAt,
		&session.FinishedAt,
	)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (r *SessionRepository) UpdateSession(ctx context.Context, session *models.GameSession) error {
	query := `
		UPDATE game_sessions
		SET status = $1, current_question_index = $2, score = $3, answers = $4, finished_at = $5
		WHERE instance_id = $6 AND user_id = $7
	`
	_, err := r.q(ctx).ExecContext(ctx, query,
		session.Status,
		session.CurrentQuestionIndex,
		session.Score,
		session.Answers,
		session.FinishedAt,
		session.InstanceID,
		session.UserID,
	)
	return err
}

func (r *SessionRepository) GetSessionsByInstance(ctx context.Context, instanceID string) ([]*models.GameSession, error) {
	query := `
		SELECT instance_id, user_id, status, current_question_index, score, answers, started_at, finished_at
		FROM game_sessions
		WHERE instance_id = $1
		ORDER BY score DESC, started_at ASC
	`
	rows, err := r.q(ctx).QueryContext(ctx, query, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.GameSession
	for rows.Next() {
		session := &models.GameSession{}
		err := rows.Scan(
			&session.InstanceID,
			&session.UserID,
			&session.Status,
			&session.CurrentQuestionIndex,
			&session.Score,
			&session.Answers,
			&session.StartedAt,
			&session.FinishedAt,
		)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (r *SessionRepository) SessionExists(ctx context.Context, instanceID, userID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM game_sessions WHERE instance_id = $1 AND user_id = $2)`
	var exists bool
	err := r.q(ctx).QueryRowContext(ctx, query, instanceID, userID).Scan(&exists)
	return exists, err
}

func (r *SessionRepository) GetActiveSessionsCount(ctx context.Context, instanceID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM game_sessions
		WHERE instance_id = $1 AND status IN ('joined', 'in_progress')
	`
	var count int
	err := r.q(ctx).QueryRowContext(ctx, query, instanceID).Scan(&count)
	return count, err
}

func (r *SessionRepository) DeleteSession(ctx context.Context, instanceID, userID string) error {
	query := `DELETE FROM game_sessions WHERE instance_id = $1 AND user_id = $2`
	_, err := r.q(ctx).ExecContext(ctx, query, instanceID, userID)
	return err
}

func (r *SessionRepository) BulkFinishInProgress(ctx context.Context, instanceID string) (int64, error) {
	query := `
		UPDATE game_sessions
		SET status = 'finished', finished_at = NOW()
		WHERE instance_id = $1 AND status != 'finished'
	`
	result, err := r.q(ctx).ExecContext(ctx, query, instanceID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *SessionRepository) UpdateSessionStatus(ctx context.Context, instanceID, userID, status string) error {
	query := `UPDATE game_sessions SET status = $1 WHERE instance_id = $2 AND user_id = $3`
	result, err := r.q(ctx).ExecContext(ctx, query, status, instanceID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("session not found")
	}
	return nil
}
