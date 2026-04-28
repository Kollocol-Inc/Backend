package sweeper

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

type DeadlineSweeper struct {
	db       *sql.DB
	interval time.Duration
}

func New(db *sql.DB, interval time.Duration) *DeadlineSweeper {
	return &DeadlineSweeper{db: db, interval: interval}
}

func (s *DeadlineSweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep(ctx)
		}
	}
}

func (s *DeadlineSweeper) sweep(ctx context.Context) {
	ids, err := s.findInstancesNeedingSweep(ctx)
	if err != nil {
		log.Printf("sweeper: findInstancesNeedingSweep failed: %v", err)
		return
	}
	for _, id := range ids {
		if err := s.sweepInstance(ctx, id); err != nil {
			log.Printf("sweeper: sweep instance %s failed: %v", id, err)
		}
	}
	if len(ids) > 0 {
		log.Printf("sweeper: swept %d instances", len(ids))
	}
}

func (s *DeadlineSweeper) findInstancesNeedingSweep(ctx context.Context) ([]string, error) {
	const q = `
		SELECT DISTINCT i.id
		FROM quiz_instances i
		LEFT JOIN game_sessions gs ON gs.instance_id = i.id
		WHERE i.quiz_type = 'async'
		  AND (
		    (i.deadline IS NOT NULL AND i.deadline < NOW() AND i.status NOT IN ('pending_review', 'reviewed'))
		    OR (i.total_time > 0 AND gs.status != 'finished'
		        AND gs.started_at + (i.total_time || ' seconds')::interval < NOW())
		  )
	`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *DeadlineSweeper) sweepInstance(ctx context.Context, instanceID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_sessions gs
		SET status = 'finished', finished_at = NOW()
		FROM quiz_instances i
		WHERE gs.instance_id = $1
		  AND i.id = gs.instance_id
		  AND i.total_time > 0
		  AND gs.status != 'finished'
		  AND gs.started_at + (i.total_time || ' seconds')::interval < NOW()
	`, instanceID); err != nil {
		return fmt.Errorf("per-session expiry: %w", err)
	}

	var deadlineExpired bool
	if err := tx.QueryRowContext(ctx, `
		SELECT (deadline IS NOT NULL AND deadline < NOW() AND status NOT IN ('pending_review', 'reviewed'))
		FROM quiz_instances WHERE id = $1
	`, instanceID).Scan(&deadlineExpired); err != nil {
		return fmt.Errorf("check deadline: %w", err)
	}

	if deadlineExpired {
		if _, err := tx.ExecContext(ctx, `
			UPDATE game_sessions SET status = 'finished', finished_at = NOW()
			WHERE instance_id = $1 AND status != 'finished'
		`, instanceID); err != nil {
			return fmt.Errorf("force-finish remaining sessions: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE quiz_instances SET status = 'pending_review' WHERE id = $1
		`, instanceID); err != nil {
			return fmt.Errorf("flip instance status: %w", err)
		}
	}

	return tx.Commit()
}
