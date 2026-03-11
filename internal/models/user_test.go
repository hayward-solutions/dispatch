package models

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestNewUserStore(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewUserStore(mock)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestUserStore_Upsert(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewUserStore(mock)
	user := &User{
		ID:         123,
		Login:      "testuser",
		Name:       "Test User",
		AvatarURL:  "https://example.com/avatar.png",
		OAuthToken: "encrypted-token",
		TokenScope: "repo,workflow",
	}

	mock.ExpectExec("INSERT INTO users").
		WithArgs(user.ID, user.Login, user.Name, user.AvatarURL, user.OAuthToken, user.TokenScope).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = store.Upsert(context.Background(), user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUserStore_Upsert_Error(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewUserStore(mock)
	user := &User{ID: 1}

	mock.ExpectExec("INSERT INTO users").
		WithArgs(user.ID, user.Login, user.Name, user.AvatarURL, user.OAuthToken, user.TokenScope).
		WillReturnError(pgx.ErrTxClosed)

	err = store.Upsert(context.Background(), user)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserStore_GetByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewUserStore(mock)
	now := time.Now().Truncate(time.Second)

	rows := pgxmock.NewRows([]string{"id", "login", "name", "avatar_url", "oauth_token", "token_scope", "created_at", "updated_at"}).
		AddRow(int64(42), "octocat", "Octo Cat", "https://avatars.com/42", "enc-token", "repo", now, now)

	mock.ExpectQuery("SELECT (.+) FROM users WHERE id").
		WithArgs(int64(42)).
		WillReturnRows(rows)

	user, err := store.GetByID(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.ID != 42 {
		t.Errorf("ID: expected 42, got %d", user.ID)
	}
	if user.Login != "octocat" {
		t.Errorf("Login: expected 'octocat', got %q", user.Login)
	}
	if user.Name != "Octo Cat" {
		t.Errorf("Name: expected 'Octo Cat', got %q", user.Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUserStore_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewUserStore(mock)

	mock.ExpectQuery("SELECT (.+) FROM users WHERE id").
		WithArgs(int64(999)).
		WillReturnError(pgx.ErrNoRows)

	_, err = store.GetByID(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}
