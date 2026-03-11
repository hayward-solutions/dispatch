package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/hayward-solutions/dispatch.v2/internal/auth"
	"github.com/hayward-solutions/dispatch.v2/internal/models"
	"golang.org/x/oauth2"
)

type AuthHandler struct {
	oauthConfig   *oauth2.Config
	sessions      *auth.SessionStore
	users         *models.UserStore
	authMiddleware *auth.Middleware
	encryptionKey []byte
}

func NewAuthHandler(oauthConfig *oauth2.Config, sessions *auth.SessionStore, users *models.UserStore, authMiddleware *auth.Middleware, encryptionKey []byte) *AuthHandler {
	return &AuthHandler{
		oauthConfig:    oauthConfig,
		sessions:       sessions,
		users:          users,
		authMiddleware: authMiddleware,
		encryptionKey:  encryptionKey,
	}
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect to dashboard
	if user := auth.UserFromContext(r.Context()); user != nil {
		http.Redirect(w, r, "/repos", http.StatusSeeOther)
		return
	}

	renderer.Page(w, "login", nil)
}

func (h *AuthHandler) BeginOAuth(w http.ResponseWriter, r *http.Request) {
	state, err := generateState()
	if err != nil {
		slog.Error("generate oauth state", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Store state in a short-lived cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	url := h.oauthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	// Validate state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		http.Error(w, "missing state cookie", http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Exchange code for token
	code := r.URL.Query().Get("code")
	token, err := h.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("oauth token exchange", "error", err)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	// Fetch GitHub user info
	ghUser, err := auth.FetchGitHubUser(r.Context(), token)
	if err != nil {
		slog.Error("fetch github user", "error", err)
		http.Error(w, "failed to fetch user info", http.StatusInternalServerError)
		return
	}

	// Encrypt token for storage
	encryptedToken, err := auth.EncryptToken(token.AccessToken, h.encryptionKey)
	if err != nil {
		slog.Error("encrypt token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Upsert user
	user := &models.User{
		ID:         ghUser.ID,
		Login:      ghUser.Login,
		Name:       ghUser.Name,
		AvatarURL:  ghUser.AvatarURL,
		OAuthToken: encryptedToken,
		TokenScope: strings.Join(h.oauthConfig.Scopes, ","),
	}
	if err := h.users.Upsert(r.Context(), user); err != nil {
		slog.Error("upsert user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create session
	ipAddress := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ipAddress = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	// Strip port from address for PostgreSQL inet compatibility
	if host, _, err := net.SplitHostPort(ipAddress); err == nil {
		ipAddress = host
	}

	session, err := h.sessions.Create(r.Context(), ghUser.ID, ipAddress, r.UserAgent())
	if err != nil {
		slog.Error("create session", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	if err := h.authMiddleware.SetSessionCookie(w, session.ID, int(session.ExpiresAt.Sub(session.CreatedAt).Seconds())); err != nil {
		slog.Error("set session cookie", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	slog.Info("user logged in", "user", ghUser.Login, "id", ghUser.ID)
	http.Redirect(w, r, "/repos", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("dispatch_session")
	if err == nil {
		var sessionID string
		// Best effort to delete session from DB
		if err := h.authMiddleware.SetSessionCookie(w, "", -1); err == nil {
			_ = sessionID // avoid unused
		}
		_ = cookie
	}

	h.authMiddleware.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
