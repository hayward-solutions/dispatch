package auth

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestNewSessionStore(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewSessionStore(mock, 3600)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.maxAge != time.Hour {
		t.Errorf("expected maxAge 1h, got %v", store.maxAge)
	}
}

func TestSessionStore_Create(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewSessionStore(mock, 3600)

	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(pgxmock.AnyArg(), int64(42), pgxmock.AnyArg(), "127.0.0.1", "TestAgent").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	sess, err := store.Create(context.Background(), 42, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if sess.UserID != 42 {
		t.Errorf("expected UserID 42, got %d", sess.UserID)
	}
	if sess.ExpiresAt.Before(time.Now()) {
		t.Error("expected ExpiresAt in the future")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionStore_Create_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewSessionStore(mock, 3600)

	mock.ExpectExec("INSERT INTO sessions").
		WithArgs(pgxmock.AnyArg(), int64(1), pgxmock.AnyArg(), "127.0.0.1", "agent").
		WillReturnError(pgx.ErrTxClosed)

	_, err = store.Create(context.Background(), 1, "127.0.0.1", "agent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStore_Get(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewSessionStore(mock, 3600)
	now := time.Now().Truncate(time.Second)
	expires := now.Add(time.Hour)

	rows := pgxmock.NewRows([]string{"id", "user_id", "created_at", "expires_at"}).
		AddRow("session-abc", int64(42), now, expires)

	mock.ExpectQuery("SELECT (.+) FROM sessions WHERE id").
		WithArgs("session-abc").
		WillReturnRows(rows)

	sess, err := store.Get(context.Background(), "session-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sess.ID != "session-abc" {
		t.Errorf("ID: expected 'session-abc', got %q", sess.ID)
	}
	if sess.UserID != 42 {
		t.Errorf("UserID: expected 42, got %d", sess.UserID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionStore_Get_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewSessionStore(mock, 3600)

	mock.ExpectQuery("SELECT (.+) FROM sessions WHERE id").
		WithArgs("nonexistent").
		WillReturnError(pgx.ErrNoRows)

	_, err = store.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewSessionStore(mock, 3600)

	mock.ExpectExec("DELETE FROM sessions WHERE id").
		WithArgs("session-to-delete").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = store.Delete(context.Background(), "session-to-delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionStore_DeleteExpired(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewSessionStore(mock, 3600)

	mock.ExpectExec("DELETE FROM sessions WHERE expires_at").
		WillReturnResult(pgxmock.NewResult("DELETE", 5))

	err = store.DeleteExpired(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGenerateSessionID(t *testing.T) {
	id1, err := generateSessionID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(id1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("expected 64-char hex string, got %d chars", len(id1))
	}

	id2, err := generateSessionID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id1 == id2 {
		t.Error("expected different IDs on each call")
	}
}
