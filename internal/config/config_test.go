package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"PORT", "BASE_URL", "DATABASE_URL", "GITHUB_CLIENT_ID",
		"GITHUB_CLIENT_SECRET", "ENCRYPTION_KEY", "SESSION_SECRET",
		"LOG_LEVEL", "SESSION_MAX_AGE",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("GITHUB_CLIENT_ID", "test-client-id")
	t.Setenv("GITHUB_CLIENT_SECRET", "test-client-secret")
}

func TestLoad_Success(t *testing.T) {
	clearEnv(t)
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("unexpected DatabaseURL: %s", cfg.DatabaseURL)
	}
	if cfg.GitHubClientID != "test-client-id" {
		t.Errorf("unexpected GitHubClientID: %s", cfg.GitHubClientID)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level 'info', got %s", cfg.LogLevel)
	}
	if cfg.SessionMaxAge != 2592000 {
		t.Errorf("expected default session max age 2592000, got %d", cfg.SessionMaxAge)
	}
	if len(cfg.EncryptionKey) != 32 {
		t.Errorf("expected 32-byte encryption key, got %d", len(cfg.EncryptionKey))
	}
	if len(cfg.SessionSecret) != 32 {
		t.Errorf("expected 32-byte session secret, got %d", len(cfg.SessionSecret))
	}
}

func TestLoad_CustomPort(t *testing.T) {
	clearEnv(t)
	setRequiredEnv(t)
	t.Setenv("PORT", "3000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 3000 {
		t.Errorf("expected port 3000, got %d", cfg.Port)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	clearEnv(t)
	setRequiredEnv(t)
	t.Setenv("PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid PORT")
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	clearEnv(t)
	t.Setenv("GITHUB_CLIENT_ID", "id")
	t.Setenv("GITHUB_CLIENT_SECRET", "secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL")
	}
}

func TestLoad_MissingGitHubClientID(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("GITHUB_CLIENT_SECRET", "secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing GITHUB_CLIENT_ID")
	}
}

func TestLoad_MissingGitHubClientSecret(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("GITHUB_CLIENT_ID", "id")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing GITHUB_CLIENT_SECRET")
	}
}

func TestLoad_CustomSessionMaxAge(t *testing.T) {
	clearEnv(t)
	setRequiredEnv(t)
	t.Setenv("SESSION_MAX_AGE", "3600")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SessionMaxAge != 3600 {
		t.Errorf("expected session max age 3600, got %d", cfg.SessionMaxAge)
	}
}

func TestLoad_InvalidSessionMaxAge(t *testing.T) {
	clearEnv(t)
	setRequiredEnv(t)
	t.Setenv("SESSION_MAX_AGE", "invalid")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid SESSION_MAX_AGE")
	}
}

func TestDecodeKeyOrGenerate_ValidBase64(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	t.Setenv("TEST_KEY", encoded)

	result, err := decodeKeyOrGenerate("TEST_KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(result))
	}
	for i, b := range result {
		if b != byte(i) {
			t.Errorf("byte %d: expected %d, got %d", i, i, b)
			break
		}
	}
}

func TestDecodeKeyOrGenerate_InvalidBase64(t *testing.T) {
	t.Setenv("TEST_KEY", "not-valid-base64!!!")

	_, err := decodeKeyOrGenerate("TEST_KEY")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecodeKeyOrGenerate_WrongLength(t *testing.T) {
	key := make([]byte, 16) // 16 bytes instead of 32
	encoded := base64.StdEncoding.EncodeToString(key)
	t.Setenv("TEST_KEY", encoded)

	_, err := decodeKeyOrGenerate("TEST_KEY")
	if err == nil {
		t.Fatal("expected error for wrong key length")
	}
}

func TestDecodeKeyOrGenerate_Empty(t *testing.T) {
	os.Unsetenv("TEST_KEY_EMPTY")

	result, err := decodeKeyOrGenerate("TEST_KEY_EMPTY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 32 {
		t.Errorf("expected generated 32-byte key, got %d bytes", len(result))
	}
}

func TestGetenv(t *testing.T) {
	t.Setenv("TEST_GETENV_SET", "value")

	if v := getenv("TEST_GETENV_SET", "fallback"); v != "value" {
		t.Errorf("expected 'value', got '%s'", v)
	}

	os.Unsetenv("TEST_GETENV_UNSET")
	if v := getenv("TEST_GETENV_UNSET", "fallback"); v != "fallback" {
		t.Errorf("expected 'fallback', got '%s'", v)
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")

	content := `# comment
KEY1=value1
KEY2=value2

KEY3 = value3
invalid_line_no_equals
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	// Ensure keys are not set
	os.Unsetenv("KEY1")
	os.Unsetenv("KEY2")
	os.Unsetenv("KEY3")

	loadDotEnv(envFile)

	if v := os.Getenv("KEY1"); v != "value1" {
		t.Errorf("KEY1: expected 'value1', got '%s'", v)
	}
	if v := os.Getenv("KEY2"); v != "value2" {
		t.Errorf("KEY2: expected 'value2', got '%s'", v)
	}
	if v := os.Getenv("KEY3"); v != "value3" {
		t.Errorf("KEY3: expected 'value3', got '%s'", v)
	}

	// Clean up
	os.Unsetenv("KEY1")
	os.Unsetenv("KEY2")
	os.Unsetenv("KEY3")
}

func TestLoadDotEnv_DoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")

	if err := os.WriteFile(envFile, []byte("EXISTING_KEY=new_value\n"), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("EXISTING_KEY", "original_value")
	loadDotEnv(envFile)

	if v := os.Getenv("EXISTING_KEY"); v != "original_value" {
		t.Errorf("expected 'original_value', got '%s'", v)
	}
}

func TestLoadDotEnv_MissingFile(t *testing.T) {
	// Should not panic or error
	loadDotEnv("/nonexistent/.env")
}
