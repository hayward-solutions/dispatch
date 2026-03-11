package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/google/go-github/v60/github"
	"github.com/hayward-solutions/dispatch.v2/internal/auth"
	gh "github.com/hayward-solutions/dispatch.v2/internal/github"
)

type EnvironmentsHandler struct{}

func NewEnvironmentsHandler() *EnvironmentsHandler {
	return &EnvironmentsHandler{}
}

func (h *EnvironmentsHandler) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	envName := r.FormValue("name")
	if envName == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if err := gh.CreateEnvironment(r.Context(), client, owner, name, envName); err != nil {
		slog.Error("create environment", "error", err)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Failed to create environment", "type": "error"}}`)
		http.Error(w, "failed to create environment", http.StatusInternalServerError)
		return
	}

	// Get repo ID for variable/secret creation
	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		slog.Error("get repo id after env create", "error", err)
	} else {
		// Create variables
		varNames := r.Form["var_names"]
		varValues := r.Form["var_values"]
		for i, vn := range varNames {
			if vn == "" {
				continue
			}
			vv := ""
			if i < len(varValues) {
				vv = varValues[i]
			}
			if err := gh.CreateEnvVariable(r.Context(), client, int(repoID), envName, vn, vv); err != nil {
				slog.Error("create env variable during env creation", "var", vn, "error", err)
			}
		}

		// Create secrets
		secretNames := r.Form["secret_names"]
		secretValues := r.Form["secret_values"]
		for i, sn := range secretNames {
			if sn == "" {
				continue
			}
			sv := ""
			if i < len(secretValues) {
				sv = secretValues[i]
			}
			if sv != "" {
				if err := gh.CreateOrUpdateEnvSecret(r.Context(), client, int(repoID), envName, sn, sv); err != nil {
					slog.Error("create env secret during env creation", "secret", sn, "error", err)
				}
			}
		}
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Environment created", "type": "success"}}`)
	w.Header().Set("HX-Redirect", "/repos/"+owner+"/"+name+"/environments/"+envName)
	w.WriteHeader(http.StatusNoContent)
}

func (h *EnvironmentsHandler) NewEnvironmentPage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	var (
		repo         *gh.Repo
		environments []gh.Environment
		wg           sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		var err error
		repo, err = gh.GetRepo(r.Context(), client, owner, name)
		if err != nil {
			slog.Error("get repo", "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		environments, err = gh.ListEnvironments(r.Context(), client, owner, name)
		if err != nil {
			slog.Error("list environments for new env page", "error", err)
		}
	}()
	wg.Wait()

	if repo == nil {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	renderer.Page(w, "env_new", map[string]any{
		"User":         user,
		"Repo":         repo,
		"Environments": environments,
		"ActivePage":   "repos",
	})
}

func (h *EnvironmentsHandler) ExportEnvConfig(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}

	var (
		vars    []gh.EnvVariable
		secrets []gh.EnvSecret
		wg      sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		var err error
		vars, err = gh.ListEnvVariables(r.Context(), client, int(repoID), envName)
		if err != nil {
			slog.Error("export env variables", "error", err)
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		secrets, err = gh.ListEnvSecrets(r.Context(), client, int(repoID), envName)
		if err != nil {
			slog.Error("export env secrets", "error", err)
		}
	}()
	wg.Wait()

	type exportVar struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	result := struct {
		Variables []exportVar `json:"variables"`
		Secrets   []string    `json:"secrets"`
	}{}

	for _, v := range vars {
		result.Variables = append(result.Variables, exportVar{Name: v.Name, Value: v.Value})
	}
	for _, s := range secrets {
		result.Secrets = append(result.Secrets, s.Name)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *EnvironmentsHandler) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	if err := gh.DeleteEnvironment(r.Context(), client, owner, name, envName); err != nil {
		slog.Error("delete environment", "error", err)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Failed to delete environment", "type": "error"}}`)
		http.Error(w, "failed to delete environment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Environment deleted", "type": "info"}}`)
	w.Header().Set("HX-Redirect", "/repos/"+owner+"/"+name)
	w.WriteHeader(http.StatusNoContent)
}

func (h *EnvironmentsHandler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	repo, err := gh.GetRepo(r.Context(), client, owner, name)
	if err != nil {
		slog.Error("get repo for environments", "error", err)
		renderer.Partial(w, "env_list", map[string]any{"Error": "Failed to load environments"})
		return
	}

	environments, err := gh.ListEnvironments(r.Context(), client, owner, name)
	if err != nil {
		slog.Error("list environments", "repo", owner+"/"+name, "error", err)
		renderer.Partial(w, "env_list", map[string]any{"Error": "Failed to load environments"})
		return
	}

	renderer.Partial(w, "env_list", map[string]any{
		"Environments": environments,
		"Owner":        owner,
		"Name":         name,
		"RepoID":       repo.ID,
	})
}

func (h *EnvironmentsHandler) EnvDetail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	repo, err := gh.GetRepo(r.Context(), client, owner, name)
	if err != nil {
		slog.Error("get repo", "error", err)
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	renderer.Page(w, "env_detail", map[string]any{
		"User":       user,
		"Repo":       repo,
		"EnvName":    envName,
		"RepoID":     repo.ID,
		"ActivePage": "repos",
	})
}

func (h *EnvironmentsHandler) ListEnvVariables(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		renderer.Partial(w, "env_variables", map[string]any{"Error": "Failed to load repository"})
		return
	}

	vars, err := gh.ListEnvVariables(r.Context(), client, int(repoID), envName)
	if err != nil {
		slog.Error("list env variables", "error", err)
		renderer.Partial(w, "env_variables", map[string]any{"Error": "Failed to load variables"})
		return
	}

	renderer.Partial(w, "env_variables", map[string]any{
		"Variables": vars,
		"Owner":     owner,
		"Name":      name,
		"EnvName":   envName,
		"RepoID":    repoID,
	})
}

func (h *EnvironmentsHandler) CreateEnvVariable(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	varName := r.FormValue("name")
	varValue := r.FormValue("value")
	if varName == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}

	if err := gh.CreateEnvVariable(r.Context(), client, int(repoID), envName, varName, varValue); err != nil {
		slog.Error("create env variable", "error", err)
		http.Error(w, "failed to create variable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Variable created", "type": "success"}, "refreshEnvVars": true}`)
	w.WriteHeader(http.StatusNoContent)
}

func (h *EnvironmentsHandler) UpdateEnvVariable(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	varName := r.PathValue("varName")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	varValue := r.FormValue("value")

	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}

	v := &github.ActionsVariable{Name: varName, Value: varValue}
	if err := gh.UpdateEnvVariable(r.Context(), client, int(repoID), envName, v); err != nil {
		slog.Error("update env variable", "error", err)
		http.Error(w, "failed to update variable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Variable updated", "type": "success"}, "refreshEnvVars": true}`)
	w.WriteHeader(http.StatusNoContent)
}

func (h *EnvironmentsHandler) DeleteEnvVariable(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	varName := r.PathValue("varName")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}

	if err := gh.DeleteEnvVariable(r.Context(), client, int(repoID), envName, varName); err != nil {
		slog.Error("delete env variable", "error", err)
		http.Error(w, "failed to delete variable", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Variable deleted", "type": "info"}, "refreshEnvVars": true}`)
	w.WriteHeader(http.StatusNoContent)
}

func (h *EnvironmentsHandler) ListEnvSecrets(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		renderer.Partial(w, "env_secrets", map[string]any{"Error": "Failed to load repository"})
		return
	}

	secrets, err := gh.ListEnvSecrets(r.Context(), client, int(repoID), envName)
	if err != nil {
		slog.Error("list env secrets", "error", err)
		renderer.Partial(w, "env_secrets", map[string]any{"Error": "Failed to load secrets"})
		return
	}

	renderer.Partial(w, "env_secrets", map[string]any{
		"Secrets": secrets,
		"Owner":   owner,
		"Name":    name,
		"EnvName": envName,
		"RepoID":  repoID,
	})
}

func (h *EnvironmentsHandler) CreateEnvSecret(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	secretName := r.FormValue("name")
	secretValue := r.FormValue("value")
	if secretName == "" || secretValue == "" {
		http.Error(w, "name and value are required", http.StatusBadRequest)
		return
	}

	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}

	if err := gh.CreateOrUpdateEnvSecret(r.Context(), client, int(repoID), envName, secretName, secretValue); err != nil {
		slog.Error("create env secret", "error", err)
		http.Error(w, "failed to create secret", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Secret created", "type": "success"}, "refreshEnvSecrets": true}`)
	w.WriteHeader(http.StatusNoContent)
}

func (h *EnvironmentsHandler) DeleteEnvSecret(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	secretName := r.PathValue("secretName")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}

	if err := gh.DeleteEnvSecret(r.Context(), client, int(repoID), envName, secretName); err != nil {
		slog.Error("delete env secret", "error", err)
		http.Error(w, "failed to delete secret", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Secret deleted", "type": "info"}, "refreshEnvSecrets": true}`)
	w.WriteHeader(http.StatusNoContent)
}

func (h *EnvironmentsHandler) DispatchPage(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	// Fetch repo and workflows concurrently
	var (
		repo      *gh.Repo
		repoErr   error
		workflows []gh.Workflow
		wg        sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		repo, repoErr = gh.GetRepo(r.Context(), client, owner, name)
	}()
	go func() {
		defer wg.Done()
		var err error
		workflows, err = gh.ListDispatchWorkflows(r.Context(), client, owner, name)
		if err != nil {
			slog.Error("list dispatch workflows", "error", err)
		}
	}()
	wg.Wait()

	if repoErr != nil {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	renderer.Page(w, "dispatch", map[string]any{
		"User":       user,
		"Repo":       repo,
		"EnvName":    envName,
		"Workflows":  workflows,
		"ActivePage": "repos",
	})
}

func (h *EnvironmentsHandler) DispatchWorkflow(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	workflowIDStr := r.FormValue("workflow_id")
	ref := r.FormValue("ref")
	if workflowIDStr == "" || ref == "" {
		http.Error(w, "workflow_id and ref are required", http.StatusBadRequest)
		return
	}

	workflowID, err := strconv.ParseInt(workflowIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid workflow_id", http.StatusBadRequest)
		return
	}

	// Collect workflow inputs from form
	inputs := make(map[string]interface{})
	for key, values := range r.Form {
		if key == "workflow_id" || key == "ref" {
			continue
		}
		if len(values) > 0 && values[0] != "" {
			inputs[key] = values[0]
		}
	}

	if err := gh.DispatchWorkflow(r.Context(), client, owner, name, workflowID, ref, inputs); err != nil {
		slog.Error("dispatch workflow", "error", err)
		w.Header().Set("HX-Trigger", `{"showToast": {"message": "Failed to dispatch workflow", "type": "error"}}`)
		http.Error(w, "failed to dispatch", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Workflow dispatched successfully", "type": "success"}}`)
	w.Header().Set("HX-Redirect", "/repos/"+owner+"/"+name)
	w.WriteHeader(http.StatusNoContent)
}

func (h *EnvironmentsHandler) ListDispatchWorkflows(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	workflows, err := gh.ListDispatchWorkflows(r.Context(), client, owner, name)
	if err != nil {
		slog.Error("list dispatch workflows", "error", err)
		renderer.Partial(w, "dispatch_workflows", map[string]any{"Error": "Failed to load workflows"})
		return
	}

	renderer.Partial(w, "dispatch_workflows", map[string]any{
		"Workflows": workflows,
		"Owner":     owner,
		"Name":      name,
		"EnvName":   envName,
	})
}

func (h *EnvironmentsHandler) ListEnvDeployments(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	deployments, err := gh.ListEnvironmentDeployments(r.Context(), client, owner, name, envName, 20)
	if err != nil {
		slog.Error("list env deployments", "error", err)
		renderer.Partial(w, "env_deployments", map[string]any{"Error": "Failed to load deployments"})
		return
	}

	renderer.Partial(w, "env_deployments", map[string]any{
		"Deployments": deployments,
		"Owner":       owner,
		"Name":        name,
		"EnvName":     envName,
	})
}

func (h *EnvironmentsHandler) ListRepoRefs(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	refs, err := gh.ListRepoRefs(r.Context(), client, owner, name)
	if err != nil {
		slog.Error("list repo refs", "error", err)
		renderer.Partial(w, "ref_selector", map[string]any{"Error": "Failed to load refs"})
		return
	}

	renderer.Partial(w, "ref_selector", map[string]any{
		"Branches": refs.Branches,
		"Tags":     refs.Tags,
	})
}

// getRepoID is a helper to fetch the numeric repo ID needed for env API calls.
func getRepoID(ctx context.Context, client *github.Client, owner, name string) (int64, error) {
	repo, err := gh.GetRepo(ctx, client, owner, name)
	if err != nil {
		return 0, err
	}
	return repo.ID, nil
}
