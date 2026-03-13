package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/hayward-solutions/dispatch.v2/internal/auth"
	gh "github.com/hayward-solutions/dispatch.v2/internal/github"
	"github.com/hayward-solutions/dispatch.v2/internal/models"
)

// DevPreviewHandler serves mock data for all routes when DEV_PREVIEW=true.
// This allows LLMs to preview UI components without GitHub OAuth or a database.
type DevPreviewHandler struct{}

// Mock data

var mockRepos = []gh.Repo{
	{ID: 101, Owner: "acme-corp", Name: "api-gateway", FullName: "acme-corp/api-gateway", Description: "Central API gateway with rate limiting and auth", Private: false, HTMLURL: "https://github.com/acme-corp/api-gateway"},
	{ID: 102, Owner: "acme-corp", Name: "frontend", FullName: "acme-corp/frontend", Description: "React dashboard for internal tooling", Private: false, HTMLURL: "https://github.com/acme-corp/frontend"},
	{ID: 103, Owner: "acme-corp", Name: "infra-terraform", FullName: "acme-corp/infra-terraform", Description: "Terraform modules for AWS infrastructure", Private: true, HTMLURL: "https://github.com/acme-corp/infra-terraform"},
	{ID: 104, Owner: "acme-corp", Name: "billing-service", FullName: "acme-corp/billing-service", Description: "Stripe integration and invoice processing", Private: true, HTMLURL: "https://github.com/acme-corp/billing-service"},
	{ID: 105, Owner: "acme-corp", Name: "docs", FullName: "acme-corp/docs", Description: "Public documentation site", Private: false, HTMLURL: "https://github.com/acme-corp/docs"},
	{ID: 106, Owner: "acme-corp", Name: "ml-pipeline", FullName: "acme-corp/ml-pipeline", Description: "Data processing and ML model training pipeline", Private: true, HTMLURL: "https://github.com/acme-corp/ml-pipeline"},
}

var mockTrackedSet = map[string]bool{
	"acme-corp/api-gateway":     true,
	"acme-corp/infra-terraform": true,
	"acme-corp/billing-service": true,
}

var mockTrackedRepos = []models.TrackedRepo{
	{ID: 1, UserID: 1, RepoOwner: "acme-corp", RepoName: "api-gateway", RepoFullName: "acme-corp/api-gateway", AddedAt: time.Now().Add(-48 * time.Hour)},
	{ID: 2, UserID: 1, RepoOwner: "acme-corp", RepoName: "infra-terraform", RepoFullName: "acme-corp/infra-terraform", AddedAt: time.Now().Add(-24 * time.Hour)},
	{ID: 3, UserID: 1, RepoOwner: "acme-corp", RepoName: "billing-service", RepoFullName: "acme-corp/billing-service", AddedAt: time.Now().Add(-2 * time.Hour)},
}

var mockWorkflowRuns = []gh.WorkflowRun{
	{ID: 9001, Name: "CI", DisplayTitle: "fix: resolve race condition in auth middleware", Status: "completed", Conclusion: "success", Event: "push", HeadBranch: "main", RunNumber: 142, HTMLURL: "https://github.com/acme-corp/api-gateway/actions/runs/9001", CreatedAt: time.Now().Add(-15 * time.Minute), Actor: "jsmith", ActorAvatar: "https://github.com/ghost.png"},
	{ID: 9002, Name: "Deploy", DisplayTitle: "deploy: staging release v2.4.1", Status: "completed", Conclusion: "failure", Event: "workflow_dispatch", HeadBranch: "release/v2.4.1", RunNumber: 88, HTMLURL: "https://github.com/acme-corp/api-gateway/actions/runs/9002", CreatedAt: time.Now().Add(-45 * time.Minute), Actor: "agarcia", ActorAvatar: "https://github.com/ghost.png"},
	{ID: 9003, Name: "CI", DisplayTitle: "feat: add request tracing headers", Status: "in_progress", Conclusion: "", Event: "pull_request", HeadBranch: "feature/tracing", RunNumber: 143, HTMLURL: "https://github.com/acme-corp/api-gateway/actions/runs/9003", CreatedAt: time.Now().Add(-5 * time.Minute), Actor: "mwong", ActorAvatar: "https://github.com/ghost.png"},
	{ID: 9004, Name: "Security Scan", DisplayTitle: "chore: weekly dependency audit", Status: "completed", Conclusion: "success", Event: "schedule", HeadBranch: "main", RunNumber: 31, HTMLURL: "https://github.com/acme-corp/api-gateway/actions/runs/9004", CreatedAt: time.Now().Add(-3 * time.Hour), Actor: "github-actions", ActorAvatar: "https://github.com/ghost.png"},
	{ID: 9005, Name: "CI", DisplayTitle: "chore: update Go to 1.22", Status: "queued", Conclusion: "", Event: "push", HeadBranch: "deps/go-1.22", RunNumber: 144, HTMLURL: "https://github.com/acme-corp/api-gateway/actions/runs/9005", CreatedAt: time.Now().Add(-1 * time.Minute), Actor: "jsmith", ActorAvatar: "https://github.com/ghost.png"},
}

var mockWorkflowJobs = []gh.WorkflowJob{
	{
		ID: 8001, RunID: 9001, Name: "build", Status: "completed", Conclusion: "success",
		StartedAt: time.Now().Add(-14 * time.Minute), CompletedAt: time.Now().Add(-12 * time.Minute),
		Steps: []gh.JobStep{
			{Name: "Checkout", Status: "completed", Conclusion: "success", Number: 1, StartedAt: time.Now().Add(-14 * time.Minute), CompletedAt: time.Now().Add(-14*time.Minute + 5*time.Second)},
			{Name: "Setup Go", Status: "completed", Conclusion: "success", Number: 2, StartedAt: time.Now().Add(-14*time.Minute + 5*time.Second), CompletedAt: time.Now().Add(-14*time.Minute + 20*time.Second)},
			{Name: "Build", Status: "completed", Conclusion: "success", Number: 3, StartedAt: time.Now().Add(-14*time.Minute + 20*time.Second), CompletedAt: time.Now().Add(-13 * time.Minute)},
			{Name: "Test", Status: "completed", Conclusion: "success", Number: 4, StartedAt: time.Now().Add(-13 * time.Minute), CompletedAt: time.Now().Add(-12 * time.Minute)},
		},
	},
	{
		ID: 8002, RunID: 9001, Name: "lint", Status: "completed", Conclusion: "success",
		StartedAt: time.Now().Add(-14 * time.Minute), CompletedAt: time.Now().Add(-13 * time.Minute),
		Steps: []gh.JobStep{
			{Name: "Checkout", Status: "completed", Conclusion: "success", Number: 1, StartedAt: time.Now().Add(-14 * time.Minute), CompletedAt: time.Now().Add(-14*time.Minute + 5*time.Second)},
			{Name: "Run golangci-lint", Status: "completed", Conclusion: "success", Number: 2, StartedAt: time.Now().Add(-14*time.Minute + 5*time.Second), CompletedAt: time.Now().Add(-13 * time.Minute)},
		},
	},
	{
		ID: 8003, RunID: 9001, Name: "deploy", Status: "completed", Conclusion: "success",
		StartedAt: time.Now().Add(-12 * time.Minute), CompletedAt: time.Now().Add(-10 * time.Minute),
		Steps: []gh.JobStep{
			{Name: "Checkout", Status: "completed", Conclusion: "success", Number: 1, StartedAt: time.Now().Add(-12 * time.Minute), CompletedAt: time.Now().Add(-12*time.Minute + 5*time.Second)},
			{Name: "Deploy to staging", Status: "completed", Conclusion: "success", Number: 2, StartedAt: time.Now().Add(-12*time.Minute + 5*time.Second), CompletedAt: time.Now().Add(-10 * time.Minute)},
		},
	},
}

var mockJobLog = `2024-01-15T10:30:01Z Run actions/checkout@v4
2024-01-15T10:30:03Z Syncing repository: acme-corp/api-gateway
2024-01-15T10:30:05Z Setting up Go 1.22...
2024-01-15T10:30:12Z go: downloading dependencies...
2024-01-15T10:30:18Z go: all modules verified
2024-01-15T10:30:19Z Running: go build ./...
2024-01-15T10:30:25Z Build successful
2024-01-15T10:30:26Z Running: go test ./... -race -coverprofile=coverage.out
2024-01-15T10:30:27Z ok  	github.com/acme-corp/api-gateway/internal/auth	0.245s	coverage: 89.2% of statements
2024-01-15T10:30:28Z ok  	github.com/acme-corp/api-gateway/internal/handlers	0.512s	coverage: 76.8% of statements
2024-01-15T10:30:29Z ok  	github.com/acme-corp/api-gateway/internal/middleware	0.189s	coverage: 92.1% of statements
2024-01-15T10:30:30Z PASS
2024-01-15T10:30:30Z Total coverage: 84.3%
2024-01-15T10:30:31Z Uploading coverage report...
2024-01-15T10:30:32Z Done.`

var mockEnvironments = []gh.Environment{
	{ID: 201, Name: "production"},
	{ID: 202, Name: "staging"},
	{ID: 203, Name: "development"},
}

var mockEnvVariables = []gh.EnvVariable{
	{Name: "API_URL", Value: "https://api.acme-corp.com", UpdatedAt: time.Now().Add(-72 * time.Hour)},
	{Name: "LOG_LEVEL", Value: "info", UpdatedAt: time.Now().Add(-48 * time.Hour)},
	{Name: "REGION", Value: "us-east-1", UpdatedAt: time.Now().Add(-168 * time.Hour)},
	{Name: "APP_NAME", Value: "api-gateway", UpdatedAt: time.Now().Add(-720 * time.Hour)},
}

var mockEnvSecrets = []gh.EnvSecret{
	{Name: "DATABASE_PASSWORD", UpdatedAt: time.Now().Add(-168 * time.Hour)},
	{Name: "API_KEY", UpdatedAt: time.Now().Add(-48 * time.Hour)},
	{Name: "JWT_SECRET", UpdatedAt: time.Now().Add(-720 * time.Hour)},
}

var mockDeployments = []gh.EnvironmentDeployment{
	{ID: 3001, SHA: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", Ref: "main", Environment: "production", Description: "Deploy v2.4.0", Creator: "jsmith", CreatedAt: time.Now().Add(-2 * time.Hour), State: "success", RunID: 9001, WorkflowName: "Deploy"},
	{ID: 3002, SHA: "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3", Ref: "release/v2.4.1", Environment: "staging", Description: "Deploy v2.4.1-rc1", Creator: "agarcia", CreatedAt: time.Now().Add(-45 * time.Minute), State: "failure", RunID: 9002, WorkflowName: "Deploy"},
	{ID: 3003, SHA: "c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4", Ref: "main", Environment: "staging", Description: "Deploy v2.4.0", Creator: "jsmith", CreatedAt: time.Now().Add(-5 * time.Hour), State: "success", RunID: 8999, WorkflowName: "Deploy"},
	{ID: 3004, SHA: "d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5", Ref: "feature/tracing", Environment: "development", Description: "Deploy feature branch", Creator: "mwong", CreatedAt: time.Now().Add(-10 * time.Minute), State: "in_progress", RunID: 9003, WorkflowName: "CI"},
}

var mockDispatchWorkflows = []gh.Workflow{
	{
		ID: 5001, Name: "Deploy", Path: ".github/workflows/deploy.yml", State: "active",
		Inputs: []gh.WorkflowInput{
			{Name: "environment", Description: "Target environment", Required: true, Default: "staging", Type: "choice", Options: []string{"production", "staging", "development"}},
			{Name: "version", Description: "Version tag to deploy", Required: true, Default: "", Type: "string"},
			{Name: "dry_run", Description: "Run in dry-run mode without applying changes", Required: false, Default: "false", Type: "boolean"},
		},
	},
	{
		ID: 5002, Name: "Database Migration", Path: ".github/workflows/migrate.yml", State: "active",
		Inputs: []gh.WorkflowInput{
			{Name: "direction", Description: "Migration direction", Required: true, Default: "up", Type: "choice", Options: []string{"up", "down"}},
			{Name: "steps", Description: "Number of migration steps (0 = all)", Required: false, Default: "0", Type: "string"},
		},
	},
}

var mockRepoRefs = &gh.RepoRefs{
	Branches: []string{"main", "release/v2.4.1", "feature/tracing"},
	Tags:     []string{"v2.4.0", "v2.3.1"},
}

// Page handlers

func (h *DevPreviewHandler) ReposPage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	renderer.Page(w, "repos", map[string]any{
		"User":       user,
		"Repos":      mockRepos,
		"TrackedSet": mockTrackedSet,
		"ActivePage": "repos",
	})
}

func (h *DevPreviewHandler) SearchRepos(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	repos := mockRepos
	if query != "" {
		q := strings.ToLower(query)
		filtered := make([]gh.Repo, 0)
		for _, repo := range repos {
			if strings.Contains(strings.ToLower(repo.Name), q) ||
				strings.Contains(strings.ToLower(repo.Description), q) {
				filtered = append(filtered, repo)
			}
		}
		repos = filtered
	}
	renderer.Partial(w, "repo_list", map[string]any{
		"Repos":      repos,
		"TrackedSet": mockTrackedSet,
	})
}

func (h *DevPreviewHandler) RepoDetail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	repo := &gh.Repo{
		ID:          101,
		Owner:       owner,
		Name:        name,
		FullName:    owner + "/" + name,
		Description: "A sample repository for preview mode",
		HTMLURL:     "https://github.com/" + owner + "/" + name,
	}
	renderer.Page(w, "repo_detail", map[string]any{
		"User":       user,
		"Repo":       repo,
		"Tracked":    mockTrackedSet[owner+"/"+name],
		"ActivePage": "repos",
	})
}

func (h *DevPreviewHandler) TrackRepo(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Repository tracked", "type": "success"}, "refreshSidebar": true}`)
	renderer.Partial(w, "repo_card", map[string]any{
		"Repo":    gh.Repo{Owner: owner, Name: name, FullName: owner + "/" + name},
		"Tracked": true,
	})
}

func (h *DevPreviewHandler) UntrackRepo(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Repository untracked", "type": "info"}, "refreshSidebar": true}`)
	renderer.Partial(w, "repo_card", map[string]any{
		"Repo":    gh.Repo{Owner: owner, Name: name, FullName: owner + "/" + name},
		"Tracked": false,
	})
}

func (h *DevPreviewHandler) SidebarRepos(w http.ResponseWriter, r *http.Request) {
	renderer.Partial(w, "sidebar_repos", map[string]any{
		"Repos": mockTrackedRepos,
	})
}

func (h *DevPreviewHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	renderer.Page(w, "dashboard", map[string]any{
		"User":       user,
		"Repos":      mockTrackedRepos,
		"ActivePage": "dashboard",
	})
}

// Workflow handlers

func (h *DevPreviewHandler) ListWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	renderer.Partial(w, "workflow_runs", map[string]any{
		"Runs":  mockWorkflowRuns,
		"Owner": owner,
		"Name":  name,
	})
}

func (h *DevPreviewHandler) GetRunJobs(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	runID := r.PathValue("runID")
	renderer.Partial(w, "run_jobs", map[string]any{
		"Jobs":       mockWorkflowJobs,
		"Owner":      owner,
		"Name":       name,
		"RunID":      runID,
		"InProgress": false,
	})
}

func (h *DevPreviewHandler) GetJobLog(w http.ResponseWriter, r *http.Request) {
	renderer.Partial(w, "job_log", map[string]any{
		"Log":       mockJobLog,
		"Truncated": false,
	})
}

// Environment handlers

func (h *DevPreviewHandler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	renderer.Partial(w, "env_list", map[string]any{
		"Environments": mockEnvironments,
		"Owner":        owner,
		"Name":         name,
		"RepoID":       int64(101),
	})
}

func (h *DevPreviewHandler) NewEnvironmentPage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	repo := &gh.Repo{ID: 101, Owner: owner, Name: name, FullName: owner + "/" + name, HTMLURL: "https://github.com/" + owner + "/" + name}
	renderer.Page(w, "env_new", map[string]any{
		"User":         user,
		"Repo":         repo,
		"Environments": mockEnvironments,
		"ActivePage":   "repos",
	})
}

func (h *DevPreviewHandler) EnvDetail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	repo := &gh.Repo{ID: 101, Owner: owner, Name: name, FullName: owner + "/" + name, HTMLURL: "https://github.com/" + owner + "/" + name}
	renderer.Page(w, "env_detail", map[string]any{
		"User":       user,
		"Repo":       repo,
		"EnvName":    envName,
		"RepoID":     int64(101),
		"ActivePage": "repos",
	})
}

func (h *DevPreviewHandler) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := "new-environment"
	if err := r.ParseForm(); err == nil && r.FormValue("name") != "" {
		envName = r.FormValue("name")
	}
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Environment created", "type": "success"}}`)
	w.Header().Set("HX-Redirect", "/repos/"+owner+"/"+name+"/environments/"+envName)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DevPreviewHandler) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Environment deleted", "type": "info"}}`)
	w.Header().Set("HX-Redirect", "/repos/"+owner+"/"+name)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DevPreviewHandler) ExportEnvConfig(w http.ResponseWriter, r *http.Request) {
	type exportVar struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	result := struct {
		Variables []exportVar `json:"variables"`
		Secrets   []string    `json:"secrets"`
	}{}
	for _, v := range mockEnvVariables {
		result.Variables = append(result.Variables, exportVar{Name: v.Name, Value: v.Value})
	}
	for _, s := range mockEnvSecrets {
		result.Secrets = append(result.Secrets, s.Name)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Variable handlers

func (h *DevPreviewHandler) ListEnvVariables(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	renderer.Partial(w, "env_variables", map[string]any{
		"Variables": mockEnvVariables,
		"Owner":     owner,
		"Name":      name,
		"EnvName":   envName,
		"RepoID":    int64(101),
	})
}

func (h *DevPreviewHandler) CreateEnvVariable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Variable created", "type": "success"}, "refreshEnvVars": true}`)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DevPreviewHandler) UpdateEnvVariable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Variable updated", "type": "success"}, "refreshEnvVars": true}`)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DevPreviewHandler) DeleteEnvVariable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Variable deleted", "type": "info"}, "refreshEnvVars": true}`)
	w.WriteHeader(http.StatusNoContent)
}

// Secret handlers

func (h *DevPreviewHandler) ListEnvSecrets(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	renderer.Partial(w, "env_secrets", map[string]any{
		"Secrets": mockEnvSecrets,
		"Owner":   owner,
		"Name":    name,
		"EnvName": envName,
		"RepoID":  int64(101),
	})
}

func (h *DevPreviewHandler) CreateEnvSecret(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Secret created", "type": "success"}, "refreshEnvSecrets": true}`)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DevPreviewHandler) DeleteEnvSecret(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Secret deleted", "type": "info"}, "refreshEnvSecrets": true}`)
	w.WriteHeader(http.StatusNoContent)
}

// Deployment handlers

func (h *DevPreviewHandler) ListEnvDeployments(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	renderer.Partial(w, "env_deployments", map[string]any{
		"Deployments": mockDeployments,
		"Owner":       owner,
		"Name":        name,
		"EnvName":     envName,
	})
}

// Dispatch handlers

func (h *DevPreviewHandler) DispatchPage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	repo := &gh.Repo{ID: 101, Owner: owner, Name: name, FullName: owner + "/" + name, HTMLURL: "https://github.com/" + owner + "/" + name}
	renderer.Page(w, "dispatch", map[string]any{
		"User":       user,
		"Repo":       repo,
		"EnvName":    envName,
		"Workflows":  mockDispatchWorkflows,
		"ActivePage": "repos",
	})
}

func (h *DevPreviewHandler) DispatchWorkflow(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Workflow dispatched successfully", "type": "success"}}`)
	w.Header().Set("HX-Redirect", "/repos/"+owner+"/"+name)
	w.WriteHeader(http.StatusNoContent)
}

func (h *DevPreviewHandler) ListDispatchWorkflows(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	renderer.Partial(w, "dispatch_workflows", map[string]any{
		"Workflows": mockDispatchWorkflows,
		"Owner":     owner,
		"Name":      name,
		"EnvName":   envName,
	})
}

func (h *DevPreviewHandler) ListRepoRefs(w http.ResponseWriter, r *http.Request) {
	renderer.Partial(w, "ref_selector", map[string]any{
		"Branches": mockRepoRefs.Branches,
		"Tags":     mockRepoRefs.Tags,
	})
}
