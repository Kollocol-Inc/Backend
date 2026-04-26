package sweeper

import (
	"context"
	"database/sql"
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
	if n, err := s.markPerSessionExpired(ctx); err != nil {
		log.Printf("sweeper: per-session expiry failed: %v", err)
	} else if n > 0 {
		log.Printf("sweeper: marked %d expired async sessions as finished", n)
	}

	if n, err := s.finalizeExpiredInstances(ctx); err != nil {
		log.Printf("sweeper: instance expiry failed: %v", err)
	} else if n > 0 {
		log.Printf("sweeper: finalized %d async instances past deadline", n)
	}
}

func (s *DeadlineSweeper) markPerSessionExpired(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE game_sessions gs
		SET status = 'finished', finished_at = NOW()
		FROM quiz_instances i
		WHERE gs.instance_id = i.id
		  AND i.quiz_type = 'async'
		  AND gs.status != 'finished'
		  AND i.total_time > 0
		  AND gs.started_at + (i.total_time || ' seconds')::interval < NOW()
	`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *DeadlineSweeper) finalizeExpiredInstances(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id FROM quiz_instances
		WHERE quiz_type = 'async'
		  AND deadline IS NOT NULL
		  AND deadline < NOW()
		  AND status NOT IN ('pending_review', 'reviewed')
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, id := range ids {
		if err := s.finalizeInstance(ctx, id); err != nil {
			log.Printf("sweeper: failed to finalize instance %s: %v", id, err)
		}
	}
	return len(ids), nil
}

func (s *DeadlineSweeper) finalizeInstance(ctx context.Context, instanceID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE game_sessions
		SET status = 'finished', finished_at = NOW()
		WHERE instance_id = $1 AND status != 'finished'
	`, instanceID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE quiz_instances
		SET status = 'pending_review'
		WHERE id = $1
	`, instanceID); err != nil {
		return err
	}

	return tx.Commit()
}
