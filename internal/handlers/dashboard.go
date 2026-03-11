package handlers

import (
	"net/http"

	"github.com/hayward-solutions/dispatch.v2/internal/auth"
	"github.com/hayward-solutions/dispatch.v2/internal/models"
)

type DashboardHandler struct {
	trackedRepos *models.TrackedRepoStore
}

func NewDashboardHandler(trackedRepos *models.TrackedRepoStore) *DashboardHandler {
	return &DashboardHandler{trackedRepos: trackedRepos}
}

func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	repos, err := h.trackedRepos.ListByUser(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "failed to load repos", http.StatusInternalServerError)
		return
	}

	renderer.Page(w, "dashboard", map[string]any{
		"User":       user,
		"Repos":      repos,
		"ActivePage": "dashboard",
	})
}
