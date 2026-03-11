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

	// Public routes
	mux.HandleFunc("GET /login", authHandler.LoginPage)
	mux.HandleFunc("GET /auth/github", authHandler.BeginOAuth)
	mux.HandleFunc("GET /auth/github/callback", authHandler.Callback)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)

	// Redirect root to repos
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/repos", http.StatusSeeOther)
	})

	// Protected routes
	protected := http.NewServeMux()

	protected.HandleFunc("GET /dashboard", dashboardHandler.Dashboard)

	protected.HandleFunc("GET /repos", reposHandler.ReposPage)
	protected.HandleFunc("GET /repos/search", reposHandler.SearchRepos)
	protected.HandleFunc("GET /repos/{owner}/{name}", reposHandler.RepoDetail)
	protected.HandleFunc("POST /repos/{owner}/{name}/track", reposHandler.TrackRepo)
	protected.HandleFunc("DELETE /repos/{owner}/{name}/track", reposHandler.UntrackRepo)
	protected.HandleFunc("GET /sidebar/repos", reposHandler.SidebarRepos)

	protected.HandleFunc("GET /repos/{owner}/{name}/runs", workflowsHandler.ListWorkflowRuns)
	protected.HandleFunc("GET /repos/{owner}/{name}/runs/{runID}/jobs", workflowsHandler.GetRunJobs)
	protected.HandleFunc("GET /repos/{owner}/{name}/jobs/{jobID}/log", workflowsHandler.GetJobLog)

	protected.HandleFunc("GET /repos/{owner}/{name}/environments", envsHandler.ListEnvironments)
	protected.HandleFunc("GET /repos/{owner}/{name}/environments/new", envsHandler.NewEnvironmentPage)
	protected.HandleFunc("POST /repos/{owner}/{name}/environments", envsHandler.CreateEnvironment)
	protected.HandleFunc("GET /repos/{owner}/{name}/environments/{env}", envsHandler.EnvDetail)
	protected.HandleFunc("DELETE /repos/{owner}/{name}/environments/{env}", envsHandler.DeleteEnvironment)
	protected.HandleFunc("GET /repos/{owner}/{name}/environments/{env}/export", envsHandler.ExportEnvConfig)

	protected.HandleFunc("GET /repos/{owner}/{name}/environments/{env}/variables", envsHandler.ListEnvVariables)
	protected.HandleFunc("POST /repos/{owner}/{name}/environments/{env}/variables", envsHandler.CreateEnvVariable)
	protected.HandleFunc("PATCH /repos/{owner}/{name}/environments/{env}/variables/{varName}", envsHandler.UpdateEnvVariable)
	protected.HandleFunc("DELETE /repos/{owner}/{name}/environments/{env}/variables/{varName}", envsHandler.DeleteEnvVariable)

	protected.HandleFunc("GET /repos/{owner}/{name}/environments/{env}/secrets", envsHandler.ListEnvSecrets)
	protected.HandleFunc("POST /repos/{owner}/{name}/environments/{env}/secrets", envsHandler.CreateEnvSecret)
	protected.HandleFunc("DELETE /repos/{owner}/{name}/environments/{env}/secrets/{secretName}", envsHandler.DeleteEnvSecret)

	protected.HandleFunc("GET /repos/{owner}/{name}/environments/{env}/deployments", envsHandler.ListEnvDeployments)
	protected.HandleFunc("GET /repos/{owner}/{name}/environments/{env}/dispatch", envsHandler.DispatchPage)
	protected.HandleFunc("POST /repos/{owner}/{name}/dispatch", envsHandler.DispatchWorkflow)
	protected.HandleFunc("GET /repos/{owner}/{name}/environments/{env}/workflows", envsHandler.ListDispatchWorkflows)
	protected.HandleFunc("GET /repos/{owner}/{name}/refs", envsHandler.ListRepoRefs)

	mux.Handle("/", authMiddleware.RequireAuth(protected))

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
