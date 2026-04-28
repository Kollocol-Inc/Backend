package database

import (
	"context"
	"database/sql"
	"fmt"
)

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type ctxKey struct{}

func ContextWithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, ctxKey{}, tx)
}

func TxFromContext(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(ctxKey{}).(*sql.Tx)
	return tx
}

func Querier(ctx context.Context, fallback DBTX) DBTX {
	if tx := TxFromContext(ctx); tx != nil {
		return tx
	}
	return fallback
}

type TxManager interface {
	InTransaction(ctx context.Context, fn func(context.Context) error) error
}

type Manager struct {
	db *sql.DB
}

func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

func (m *Manager) InTransaction(ctx context.Context, fn func(context.Context) error) (err error) {
	if TxFromContext(ctx) != nil {
		return fn(ctx)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := fn(ContextWithTx(ctx, tx)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}
