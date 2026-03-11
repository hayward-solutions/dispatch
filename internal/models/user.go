package models

import (
	"context"
	"fmt"
	"time"

	"github.com/hayward-solutions/dispatch.v2/internal/database"
)

type User struct {
	ID         int64
	Login      string
	Name       string
	AvatarURL  string
	OAuthToken string // encrypted
	TokenScope string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type UserStore struct {
	pool database.Pool
}

func NewUserStore(pool database.Pool) *UserStore {
	return &UserStore{pool: pool}
}

func (s *UserStore) Upsert(ctx context.Context, u *User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, login, name, avatar_url, oauth_token, token_scope, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
		ON CONFLICT (id) DO UPDATE SET
			login = EXCLUDED.login,
			name = EXCLUDED.name,
			avatar_url = EXCLUDED.avatar_url,
			oauth_token = EXCLUDED.oauth_token,
			token_scope = EXCLUDED.token_scope,
			updated_at = now()
	`, u.ID, u.Login, u.Name, u.AvatarURL, u.OAuthToken, u.TokenScope)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

func (s *UserStore) GetByID(ctx context.Context, id int64) (*User, error) {
	u := &User{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, login, name, avatar_url, oauth_token, token_scope, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Login, &u.Name, &u.AvatarURL, &u.OAuthToken, &u.TokenScope, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}
