package config

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port               int
	BaseURL            string
	DatabaseURL        string
	GitHubClientID     string
	GitHubClientSecret string
	EncryptionKey      []byte // 32 bytes for AES-256
	SessionSecret      []byte // 32 bytes for securecookie
	LogLevel           string
	SessionMaxAge      int  // seconds
	DevPreview         bool // when true, serves mock data without auth or database
}

// buildDatabaseURL returns DATABASE_URL if set, otherwise constructs one
// from individual DB_* environment variables.
func buildDatabaseURL() (string, error) {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v, nil
	}

	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")

	if host == "" || user == "" || name == "" {
		return "", fmt.Errorf("either DATABASE_URL or DB_HOST, DB_USER, and DB_NAME are required")
	}

	port := getenv("DB_PORT", "5432")
	sslmode := getenv("DB_SSLMODE", "disable")

	if password != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			user, password, host, port, name, sslmode), nil
	}
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s",
		user, host, port, name, sslmode), nil
}

func Load() (*Config, error) {
	loadDotEnv(".env")

	port := 8080
	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT: %w", err)
		}
		port = p
	}

	// Dev preview mode: skip all external dependencies (DB, GitHub OAuth).
	// Refuses to activate if PRODUCTION=true is set.
	if strings.EqualFold(os.Getenv("DEV_PREVIEW"), "true") {
		if strings.EqualFold(os.Getenv("PRODUCTION"), "true") {
			return nil, fmt.Errorf("DEV_PREVIEW cannot be enabled when PRODUCTION=true")
		}
		return &Config{
			Port:       port,
			BaseURL:    getenv("BASE_URL", fmt.Sprintf("http://localhost:%d", port)),
			LogLevel:   getenv("LOG_LEVEL", "debug"),
			DevPreview: true,
		}, nil
	}

	sessionMaxAge := 2592000 // 30 days
	if v := os.Getenv("SESSION_MAX_AGE"); v != "" {
		s, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid SESSION_MAX_AGE: %w", err)
		}
		sessionMaxAge = s
	}

	encKey, err := decodeKeyOrGenerate("ENCRYPTION_KEY")
	if err != nil {
		return nil, err
	}

	sessKey, err := decodeKeyOrGenerate("SESSION_SECRET")
	if err != nil {
		return nil, err
	}

	dbURL, err := buildDatabaseURL()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:               port,
		BaseURL:            getenv("BASE_URL", fmt.Sprintf("http://localhost:%d", port)),
		DatabaseURL:        dbURL,
		GitHubClientID:     requireEnv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: requireEnv("GITHUB_CLIENT_SECRET"),
		EncryptionKey:      encKey,
		SessionSecret:      sessKey,
		LogLevel:           getenv("LOG_LEVEL", "info"),
		SessionMaxAge:      sessionMaxAge,
	}
	if cfg.GitHubClientID == "" {
		return nil, fmt.Errorf("GITHUB_CLIENT_ID is required")
	}
	if cfg.GitHubClientSecret == "" {
		return nil, fmt.Errorf("GITHUB_CLIENT_SECRET is required")
	}

	return cfg, nil
}

func decodeKeyOrGenerate(envVar string) ([]byte, error) {
	v := os.Getenv(envVar)
	if v == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate %s: %w", envVar, err)
		}
		slog.Warn("auto-generated key (sessions will not survive restarts)", "key", envVar)
		return key, nil
	}
	key, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: must be base64-encoded: %w", envVar, err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid %s: must be 32 bytes (got %d)", envVar, len(key))
	}
	return key, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	return os.Getenv(key)
}

// loadDotEnv reads a .env file and sets any variables not already in the environment.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env is optional
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Don't override existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
