package models

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TrackedRepo struct {
	ID           int64
	UserID       int64
	RepoOwner    string
	RepoName     string
	RepoFullName string
	AddedAt      time.Time
}

type TrackedRepoStore struct {
	pool *pgxpool.Pool
}

func NewTrackedRepoStore(pool *pgxpool.Pool) *TrackedRepoStore {
	return &TrackedRepoStore{pool: pool}
}

func (s *TrackedRepoStore) Add(ctx context.Context, userID int64, owner, name string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO tracked_repos (user_id, repo_owner, repo_name, repo_full_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, repo_owner, repo_name) DO NOTHING
	`, userID, owner, name, owner+"/"+name)
	if err != nil {
		return fmt.Errorf("track repo: %w", err)
	}
	return nil
}

func (s *TrackedRepoStore) Remove(ctx context.Context, userID int64, owner, name string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM tracked_repos WHERE user_id = $1 AND repo_owner = $2 AND repo_name = $3
	`, userID, owner, name)
	if err != nil {
		return fmt.Errorf("untrack repo: %w", err)
	}
	return nil
}

func (s *TrackedRepoStore) ListByUser(ctx context.Context, userID int64) ([]TrackedRepo, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, repo_owner, repo_name, repo_full_name, added_at
		FROM tracked_repos WHERE user_id = $1 ORDER BY added_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tracked repos: %w", err)
	}
	defer rows.Close()

	var repos []TrackedRepo
	for rows.Next() {
		var r TrackedRepo
		if err := rows.Scan(&r.ID, &r.UserID, &r.RepoOwner, &r.RepoName, &r.RepoFullName, &r.AddedAt); err != nil {
			return nil, fmt.Errorf("scan tracked repo: %w", err)
		}
		repos = append(repos, r)
	}
	return repos, nil
}

func (s *TrackedRepoStore) IsTracked(ctx context.Context, userID int64, owner, name string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM tracked_repos WHERE user_id = $1 AND repo_owner = $2 AND repo_name = $3)
	`, userID, owner, name).Scan(&exists)
	return exists, err
}
