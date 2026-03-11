package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hayward-solutions/dispatch.v2/internal/auth"
	"github.com/hayward-solutions/dispatch.v2/internal/config"
	"github.com/hayward-solutions/dispatch.v2/internal/database"
	"github.com/hayward-solutions/dispatch.v2/internal/handlers"
	"github.com/hayward-solutions/dispatch.v2/internal/models"
	"github.com/hayward-solutions/dispatch.v2/internal/tmpl"
	"github.com/hayward-solutions/dispatch.v2/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	setupLogging(cfg.LogLevel)

	// Database
	db, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	// Stores
	userStore := models.NewUserStore(db.Pool)
	trackedRepoStore := models.NewTrackedRepoStore(db.Pool)
	sessionStore := auth.NewSessionStore(db.Pool, cfg.SessionMaxAge)

	// Auth
	oauthCfg := auth.NewOAuthConfig(cfg.GitHubClientID, cfg.GitHubClientSecret, cfg.BaseURL)

	getUserFunc := func(ctx context.Context, id int64) (*auth.ContextUser, error) {
		u, err := userStore.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		token, err := auth.DecryptToken(u.OAuthToken, cfg.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt token: %w", err)
		}
		return &auth.ContextUser{
			ID:         u.ID,
			Login:      u.Login,
			Name:       u.Name,
			AvatarURL:  u.AvatarURL,
			OAuthToken: token,
		}, nil
	}

	authMiddleware := auth.NewMiddleware(sessionStore, cfg.SessionSecret, cfg.EncryptionKey, getUserFunc)

	// Templates
	templateFS, err := fs.Sub(web.TemplateFS, "templates")
	if err != nil {
		return fmt.Errorf("template fs: %w", err)
	}

	dev := cfg.LogLevel == "debug"
	renderer, err := tmpl.New(templateFS, dev)
	if err != nil {
		return fmt.Errorf("init templates: %w", err)
	}
	handlers.SetRenderer(renderer)

	// Handlers
	authHandler := handlers.NewAuthHandler(oauthCfg, sessionStore, userStore, authMiddleware, cfg.EncryptionKey)
	dashboardHandler := handlers.NewDashboardHandler(trackedRepoStore)
	reposHandler := handlers.NewReposHandler(trackedRepoStore)
	workflowsHandler := handlers.NewWorkflowsHandler()
	envsHandler := handlers.NewEnvironmentsHandler()

	// Router
	mux := http.NewServeMux()

	// Static files
	staticFS, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		return fmt.Errorf("static fs: %w", err)
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Pool.Ping(r.Context()); err != nil {
			http.Error(w, "db unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Public routes
	mux.HandleFunc("GET /login", authHandler.LoginPage)
	mux.HandleFunc("GET /auth/github", authHandler.BeginOAuth)
	mux.HandleFunc("GET /auth/github/callback", authHandler.Callback)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)

	// Redirect root to repos
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/repos", http.StatusSeeOther)
	})

	// Protected routes - helper to wrap handlers with auth middleware
	protect := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, authMiddleware.RequireAuth(http.HandlerFunc(handler)))
	}

	protect("GET /dashboard", dashboardHandler.Dashboard)

	protect("GET /repos", reposHandler.ReposPage)
	protect("GET /repos/search", reposHandler.SearchRepos)
	protect("GET /repos/{owner}/{name}", reposHandler.RepoDetail)
	protect("POST /repos/{owner}/{name}/track", reposHandler.TrackRepo)
	protect("DELETE /repos/{owner}/{name}/track", reposHandler.UntrackRepo)
	protect("GET /sidebar/repos", reposHandler.SidebarRepos)

	protect("GET /repos/{owner}/{name}/runs", workflowsHandler.ListWorkflowRuns)
	protect("GET /repos/{owner}/{name}/runs/{runID}/jobs", workflowsHandler.GetRunJobs)
	protect("GET /repos/{owner}/{name}/jobs/{jobID}/log", workflowsHandler.GetJobLog)

	protect("GET /repos/{owner}/{name}/environments", envsHandler.ListEnvironments)
	protect("GET /repos/{owner}/{name}/environments/new", envsHandler.NewEnvironmentPage)
	protect("POST /repos/{owner}/{name}/environments", envsHandler.CreateEnvironment)
	protect("GET /repos/{owner}/{name}/environments/{env}", envsHandler.EnvDetail)
	protect("DELETE /repos/{owner}/{name}/environments/{env}", envsHandler.DeleteEnvironment)
	protect("GET /repos/{owner}/{name}/environments/{env}/export", envsHandler.ExportEnvConfig)

	protect("GET /repos/{owner}/{name}/environments/{env}/variables", envsHandler.ListEnvVariables)
	protect("POST /repos/{owner}/{name}/environments/{env}/variables", envsHandler.CreateEnvVariable)
	protect("PATCH /repos/{owner}/{name}/environments/{env}/variables/{varName}", envsHandler.UpdateEnvVariable)
	protect("DELETE /repos/{owner}/{name}/environments/{env}/variables/{varName}", envsHandler.DeleteEnvVariable)

	protect("GET /repos/{owner}/{name}/environments/{env}/secrets", envsHandler.ListEnvSecrets)
	protect("POST /repos/{owner}/{name}/environments/{env}/secrets", envsHandler.CreateEnvSecret)
	protect("DELETE /repos/{owner}/{name}/environments/{env}/secrets/{secretName}", envsHandler.DeleteEnvSecret)

	protect("GET /repos/{owner}/{name}/environments/{env}/deployments", envsHandler.ListEnvDeployments)
	protect("GET /repos/{owner}/{name}/environments/{env}/dispatch", envsHandler.DispatchPage)
	protect("POST /repos/{owner}/{name}/dispatch", envsHandler.DispatchWorkflow)
	protect("GET /repos/{owner}/{name}/environments/{env}/workflows", envsHandler.ListDispatchWorkflows)
	protect("GET /repos/{owner}/{name}/refs", envsHandler.ListRepoRefs)

	// Server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		slog.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	slog.Info("server starting", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func setupLogging(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
}
