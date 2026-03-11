package auth

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

type contextKey string

const (
	userContextKey    contextKey = "user"
	cookieName                   = "dispatch_session"
)

type ContextUser struct {
	ID         int64
	Login      string
	Name       string
	AvatarURL  string
	OAuthToken string // decrypted
}

func UserFromContext(ctx context.Context) *ContextUser {
	u, _ := ctx.Value(userContextKey).(*ContextUser)
	return u
}

type Middleware struct {
	sessions     *SessionStore
	secureCookie *securecookie.SecureCookie
	encryptionKey []byte
	getUserFunc  func(ctx context.Context, id int64) (*ContextUser, error)
}

func NewMiddleware(sessions *SessionStore, sessionSecret, encryptionKey []byte, getUserFunc func(ctx context.Context, id int64) (*ContextUser, error)) *Middleware {
	sc := securecookie.New(sessionSecret, nil)
	sc.MaxAge(0) // no max age on the encoding side; we handle expiry in DB

	return &Middleware{
		sessions:      sessions,
		secureCookie:  sc,
		encryptionKey: encryptionKey,
		getUserFunc:   getUserFunc,
	}
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			m.handleUnauth(w, r)
			return
		}

		var sessionID string
		if err := m.secureCookie.Decode(cookieName, cookie.Value, &sessionID); err != nil {
			m.handleUnauth(w, r)
			return
		}

		session, err := m.sessions.Get(r.Context(), sessionID)
		if err != nil {
			m.handleUnauth(w, r)
			return
		}

		if session.ExpiresAt.Before(time.Now()) {
			m.sessions.Delete(r.Context(), sessionID)
			m.handleUnauth(w, r)
			return
		}

		user, err := m.getUserFunc(r.Context(), session.UserID)
		if err != nil {
			slog.Error("failed to get user for session", "user_id", session.UserID, "error", err)
			m.handleUnauth(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) handleUnauth(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (m *Middleware) SetSessionCookie(w http.ResponseWriter, sessionID string, maxAge int) error {
	encoded, err := m.secureCookie.Encode(cookieName, sessionID)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false, // set to true in production behind TLS
	})
	return nil
}

func (m *Middleware) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}
