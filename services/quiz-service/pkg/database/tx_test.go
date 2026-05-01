package database

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestInTransaction_Commit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO foo").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mgr := NewManager(db)
	err = mgr.InTransaction(context.Background(), func(ctx context.Context) error {
		tx := TxFromContext(ctx)
		if tx == nil {
			t.Fatal("expected tx in ctx")
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO foo VALUES (1)")
		return err
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInTransaction_Rollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO foo").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	sentinel := errors.New("nope")
	mgr := NewManager(db)
	err = mgr.InTransaction(context.Background(), func(ctx context.Context) error {
		_, _ = TxFromContext(ctx).ExecContext(ctx, "INSERT INTO foo VALUES (1)")
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInTransaction_Reentrant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO foo").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mgr := NewManager(db)
	err = mgr.InTransaction(context.Background(), func(ctx context.Context) error {
		return mgr.InTransaction(ctx, func(ctx context.Context) error {
			_, err := TxFromContext(ctx).ExecContext(ctx, "INSERT INTO foo VALUES (1)")
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQuerier_FallbackWhenNoTx(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	q := Querier(context.Background(), db)
	if q != db {
		t.Fatal("expected fallback to be the *sql.DB")
	}
}
