package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/hayward-solutions/dispatch.v2/internal/database"
)

type Session struct {
	ID        string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionStore struct {
	pool   database.Pool
	maxAge time.Duration
}

func NewSessionStore(pool database.Pool, maxAgeSeconds int) *SessionStore {
	return &SessionStore{
		pool:   pool,
		maxAge: time.Duration(maxAgeSeconds) * time.Second,
	}
}

func (s *SessionStore) Create(ctx context.Context, userID int64, ipAddress, userAgent string) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(s.maxAge)

	_, err = s.pool.Exec(ctx,
		`INSERT INTO sessions (id, user_id, expires_at, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4::inet, $5)`,
		id, userID, expiresAt, ipAddress, userAgent,
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return &Session{
		ID:        id,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *SessionStore) Get(ctx context.Context, id string) (*Session, error) {
	sess := &Session{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, created_at, expires_at FROM sessions
		 WHERE id = $1 AND expires_at > now()`,
		id,
	).Scan(&sess.ID, &sess.UserID, &sess.CreatedAt, &sess.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return sess, nil
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM sessions WHERE id = $1", id)
	return err
}

func (s *SessionStore) DeleteExpired(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM sessions WHERE expires_at < now()")
	return err
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
