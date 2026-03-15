package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/hayward-solutions/dispatch.v2/internal/auth"
	gh "github.com/hayward-solutions/dispatch.v2/internal/github"
)

type ObservabilityHandler struct{}

func NewObservabilityHandler() *ObservabilityHandler {
	return &ObservabilityHandler{}
}

func (h *ObservabilityHandler) RepoObservabilityPage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	repo, err := gh.GetRepo(r.Context(), client, owner, name)
	if err != nil {
		slog.Error("get repo for observability", "repo", owner+"/"+name, "error", err)
		renderer.Page(w, "observability", map[string]any{
			"User":       user,
			"Owner":      owner,
			"Name":       name,
			"Error":      "Failed to load repository details",
			"ActivePage": "repos",
		})
		return
	}

	obs, err := gh.GetRepoObservability(r.Context(), client, owner, name)
	if err != nil {
		slog.Error("get repo observability", "repo", owner+"/"+name, "error", err)
		renderer.Page(w, "observability", map[string]any{
			"User":       user,
			"Owner":      owner,
			"Name":       name,
			"Repo":       repo,
			"Error":      "Failed to load observability data. You may not have access to Actions data for this repository.",
			"ActivePage": "repos",
		})
		return
	}

	renderer.Page(w, "observability", map[string]any{
		"User":          user,
		"Owner":         owner,
		"Name":          name,
		"Repo":          repo,
		"Observability": obs,
		"ActivePage":    "repos",
	})
}

func (h *ObservabilityHandler) ObservabilityHistory(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	q := r.URL.Query()
	opts := gh.WorkflowHistoryOptions{
		Status: q.Get("status"),
		Branch: q.Get("branch"),
	}

	if wfID := q.Get("workflow_id"); wfID != "" {
		if id, err := strconv.ParseInt(wfID, 10, 64); err == nil {
			opts.WorkflowID = id
		}
	}
	if page := q.Get("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil {
			opts.Page = p
		}
	}

	runs, total, err := gh.GetWorkflowRunHistory(r.Context(), client, owner, name, opts)
	if err != nil {
		slog.Error("get workflow run history", "repo", owner+"/"+name, "error", err)
		renderer.Partial(w, "obs_history", map[string]any{
			"Error": "Failed to load workflow run history",
		})
		return
	}

	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = 25
	}

	renderer.Partial(w, "obs_history", map[string]any{
		"Runs":       runs,
		"Total":      total,
		"Page":       opts.Page,
		"PerPage":    perPage,
		"Owner":      owner,
		"Name":       name,
		"WorkflowID": opts.WorkflowID,
		"Status":     opts.Status,
		"Branch":     opts.Branch,
	})
}
