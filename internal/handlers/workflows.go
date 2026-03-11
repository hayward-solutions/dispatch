package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/hayward-solutions/dispatch.v2/internal/auth"
	gh "github.com/hayward-solutions/dispatch.v2/internal/github"
)

type WorkflowsHandler struct{}

func NewWorkflowsHandler() *WorkflowsHandler {
	return &WorkflowsHandler{}
}

func (h *WorkflowsHandler) ListWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	runs, err := gh.ListWorkflowRuns(r.Context(), client, owner, name, 20)
	if err != nil {
		slog.Error("list workflow runs", "repo", owner+"/"+name, "error", err)
		renderer.Partial(w, "workflow_runs", map[string]any{
			"Error": "Failed to load workflow runs",
		})
		return
	}

	renderer.Partial(w, "workflow_runs", map[string]any{
		"Runs":  runs,
		"Owner": owner,
		"Name":  name,
	})
}

func (h *WorkflowsHandler) GetRunJobs(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	runIDStr := r.PathValue("runID")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	jobs, err := gh.ListWorkflowJobs(r.Context(), client, owner, name, runID)
	if err != nil {
		slog.Error("list workflow jobs", "runID", runID, "error", err)
		renderer.Partial(w, "run_jobs", map[string]any{"Error": "Failed to load jobs"})
		return
	}

	inProgress := false
	for _, j := range jobs {
		if j.Status == "in_progress" || j.Status == "queued" {
			inProgress = true
			break
		}
	}

	renderer.Partial(w, "run_jobs", map[string]any{
		"Jobs":       jobs,
		"Owner":      owner,
		"Name":       name,
		"RunID":      runID,
		"InProgress": inProgress,
	})
}

func (h *WorkflowsHandler) GetJobLog(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	jobIDStr := r.PathValue("jobID")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	jobID, err := strconv.ParseInt(jobIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid job ID", http.StatusBadRequest)
		return
	}

	log, truncated, err := gh.GetWorkflowJobLog(r.Context(), client, owner, name, jobID, 2000)
	if err != nil {
		slog.Error("get job log", "jobID", jobID, "error", err)
		renderer.Partial(w, "job_log", map[string]any{"Error": "Failed to load logs"})
		return
	}

	renderer.Partial(w, "job_log", map[string]any{
		"Log":       log,
		"Truncated": truncated,
	})
}
