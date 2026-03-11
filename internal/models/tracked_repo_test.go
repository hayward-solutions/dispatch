package models

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestTrackedRepoStore_Add(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewTrackedRepoStore(mock)

	mock.ExpectExec("INSERT INTO tracked_repos").
		WithArgs(int64(1), "owner", "repo", "owner/repo").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = store.Add(context.Background(), 1, "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTrackedRepoStore_Add_Error(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewTrackedRepoStore(mock)

	mock.ExpectExec("INSERT INTO tracked_repos").
		WithArgs(int64(1), "owner", "repo", "owner/repo").
		WillReturnError(pgx.ErrTxClosed)

	err = store.Add(context.Background(), 1, "owner", "repo")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTrackedRepoStore_Remove(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewTrackedRepoStore(mock)

	mock.ExpectExec("DELETE FROM tracked_repos").
		WithArgs(int64(1), "owner", "repo").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = store.Remove(context.Background(), 1, "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTrackedRepoStore_ListByUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewTrackedRepoStore(mock)
	now := time.Now().Truncate(time.Second)

	rows := pgxmock.NewRows([]string{"id", "user_id", "repo_owner", "repo_name", "repo_full_name", "added_at"}).
		AddRow(int64(1), int64(42), "octocat", "hello-world", "octocat/hello-world", now).
		AddRow(int64(2), int64(42), "octocat", "spoon-knife", "octocat/spoon-knife", now)

	mock.ExpectQuery("SELECT (.+) FROM tracked_repos WHERE user_id").
		WithArgs(int64(42)).
		WillReturnRows(rows)

	repos, err := store.ListByUser(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].RepoName != "hello-world" {
		t.Errorf("expected 'hello-world', got %q", repos[0].RepoName)
	}
	if repos[1].RepoName != "spoon-knife" {
		t.Errorf("expected 'spoon-knife', got %q", repos[1].RepoName)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestTrackedRepoStore_ListByUser_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewTrackedRepoStore(mock)

	rows := pgxmock.NewRows([]string{"id", "user_id", "repo_owner", "repo_name", "repo_full_name", "added_at"})

	mock.ExpectQuery("SELECT (.+) FROM tracked_repos WHERE user_id").
		WithArgs(int64(42)).
		WillReturnRows(rows)

	repos, err := store.ListByUser(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

func TestTrackedRepoStore_IsTracked_True(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewTrackedRepoStore(mock)

	rows := pgxmock.NewRows([]string{"exists"}).AddRow(true)

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(int64(1), "owner", "repo").
		WillReturnRows(rows)

	tracked, err := store.IsTracked(context.Background(), 1, "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tracked {
		t.Error("expected tracked=true")
	}
}

func TestTrackedRepoStore_IsTracked_False(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("create mock: %v", err)
	}
	defer mock.Close()

	store := NewTrackedRepoStore(mock)

	rows := pgxmock.NewRows([]string{"exists"}).AddRow(false)

	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(int64(1), "owner", "repo").
		WillReturnRows(rows)

	tracked, err := store.IsTracked(context.Background(), 1, "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tracked {
		t.Error("expected tracked=false")
	}
}
