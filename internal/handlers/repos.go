package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/hayward-solutions/dispatch.v2/internal/auth"
	"github.com/hayward-solutions/dispatch.v2/internal/dispatch"
	"github.com/hayward-solutions/dispatch.v2/internal/engine"
	gh "github.com/hayward-solutions/dispatch.v2/internal/github"
	"github.com/hayward-solutions/dispatch.v2/internal/models"
)

type ReposHandler struct {
	trackedRepos *models.TrackedRepoStore
}

func NewReposHandler(trackedRepos *models.TrackedRepoStore) *ReposHandler {
	return &ReposHandler{trackedRepos: trackedRepos}
}

func (h *ReposHandler) ReposPage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	client := gh.NewClient(r.Context(), user.OAuthToken)

	repos, _, err := gh.ListUserRepos(r.Context(), client, 1, 30)
	if err != nil {
		slog.Error("list repos", "error", err)
		http.Error(w, "failed to load repositories", http.StatusInternalServerError)
		return
	}

	// Check which repos are tracked
	tracked, err := h.trackedRepos.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("list tracked repos", "error", err)
	}
	trackedSet := make(map[string]bool, len(tracked))
	for _, t := range tracked {
		trackedSet[t.RepoFullName] = true
	}

	renderer.Page(w, "repos", map[string]any{
		"User":       user,
		"Repos":      repos,
		"TrackedSet": trackedSet,
		"ActivePage": "repos",
	})
}

func (h *ReposHandler) SearchRepos(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	client := gh.NewClient(r.Context(), user.OAuthToken)
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	repos, _, err := gh.ListUserRepos(r.Context(), client, 1, 100)
	if err != nil {
		slog.Error("search repos", "error", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	if query != "" {
		q := strings.ToLower(query)
		filtered := repos[:0]
		for _, repo := range repos {
			if strings.Contains(strings.ToLower(repo.Name), q) ||
				strings.Contains(strings.ToLower(repo.Description), q) {
				filtered = append(filtered, repo)
			}
		}
		repos = filtered
	}

	tracked, _ := h.trackedRepos.ListByUser(r.Context(), user.ID)
	trackedSet := make(map[string]bool, len(tracked))
	for _, t := range tracked {
		trackedSet[t.RepoFullName] = true
	}

	renderer.Partial(w, "repo_list", map[string]any{
		"Repos":      repos,
		"TrackedSet": trackedSet,
	})
}

func (h *ReposHandler) TrackRepo(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")

	if err := h.trackedRepos.Add(r.Context(), user.ID, owner, name); err != nil {
		slog.Error("track repo", "error", err)
		http.Error(w, "failed to track", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Repository tracked", "type": "success"}, "refreshSidebar": true}`)
	renderer.Partial(w, "repo_card", map[string]any{
		"Repo": gh.Repo{
			Owner:    owner,
			Name:     name,
			FullName: owner + "/" + name,
		},
		"Tracked": true,
	})
}

func (h *ReposHandler) UntrackRepo(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")

	if err := h.trackedRepos.Remove(r.Context(), user.ID, owner, name); err != nil {
		slog.Error("untrack repo", "error", err)
		http.Error(w, "failed to untrack", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Repository untracked", "type": "info"}, "refreshSidebar": true}`)
	renderer.Partial(w, "repo_card", map[string]any{
		"Repo": gh.Repo{
			Owner:    owner,
			Name:     name,
			FullName: owner + "/" + name,
		},
		"Tracked": false,
	})
}

func (h *ReposHandler) SidebarRepos(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	repos, err := h.trackedRepos.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("sidebar repos", "error", err)
		http.Error(w, "failed to load repos", http.StatusInternalServerError)
		return
	}

	renderer.Partial(w, "sidebar_repos", map[string]any{
		"Repos": repos,
	})
}

func (h *ReposHandler) RepoDetail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	repo, err := gh.GetRepo(r.Context(), client, owner, name)
	if err != nil {
		slog.Error("get repo", "error", err)
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	tracked, _ := h.trackedRepos.IsTracked(r.Context(), user.ID, owner, name)

	data := map[string]any{
		"User":       user,
		"Repo":       repo,
		"Tracked":    tracked,
		"ActivePage": "repos",
	}

	// Check for advanced mode via .dispatch.yaml
	configBytes, err := gh.GetFileContent(r.Context(), client, owner, name, ".dispatch.yaml")
	if err == nil {
		cfg, err := dispatch.Parse(configBytes)
		if err == nil {
			data["AdvancedMode"] = true
			data["DispatchConfig"] = cfg

			// Fetch and parse variables
			varsBytes, err := gh.GetFileContent(r.Context(), client, owner, name, cfg.VariablesPath)
			if err == nil {
				eng, err := engine.GetEngine(cfg.Mode)
				if err == nil {
					variables, err := eng.ParseVariables(varsBytes)
					if err == nil {
						data["Variables"] = variables
					} else {
						slog.Error("parse variables", "error", err)
					}
				}
			} else {
				slog.Error("get variables file", "path", cfg.VariablesPath, "error", err)
			}
		} else {
			slog.Warn("parse dispatch config", "error", err)
		}
	}

	renderer.Page(w, "repo_detail", data)
}
