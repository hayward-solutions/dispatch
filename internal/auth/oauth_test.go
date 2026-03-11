package auth

import (
	"testing"

	githuboauth "golang.org/x/oauth2/github"
)

func TestNewOAuthConfig(t *testing.T) {
	cfg := NewOAuthConfig("my-client-id", "my-client-secret", "https://example.com")

	if cfg.ClientID != "my-client-id" {
		t.Errorf("ClientID: expected 'my-client-id', got %q", cfg.ClientID)
	}
	if cfg.ClientSecret != "my-client-secret" {
		t.Errorf("ClientSecret: expected 'my-client-secret', got %q", cfg.ClientSecret)
	}
	if cfg.RedirectURL != "https://example.com/auth/github/callback" {
		t.Errorf("RedirectURL: expected 'https://example.com/auth/github/callback', got %q", cfg.RedirectURL)
	}
	if len(cfg.Scopes) != 2 || cfg.Scopes[0] != "repo" || cfg.Scopes[1] != "workflow" {
		t.Errorf("Scopes: expected [repo, workflow], got %v", cfg.Scopes)
	}
	if cfg.Endpoint != githuboauth.Endpoint {
		t.Error("Endpoint: expected GitHub endpoint")
	}
}

func TestNewOAuthConfig_DifferentBaseURL(t *testing.T) {
	cfg := NewOAuthConfig("id", "secret", "http://localhost:8080")

	if cfg.RedirectURL != "http://localhost:8080/auth/github/callback" {
		t.Errorf("RedirectURL: got %q", cfg.RedirectURL)
	}
}
