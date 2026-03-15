package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/hayward-solutions/dispatch.v2/internal/auth"
	"github.com/hayward-solutions/dispatch.v2/internal/dispatch"
	"github.com/hayward-solutions/dispatch.v2/internal/engine"
	gh "github.com/hayward-solutions/dispatch.v2/internal/github"
)

// AdvancedHandler serves the advanced environment detail page with typed variable editing.
type AdvancedHandler struct{}

func NewAdvancedHandler() *AdvancedHandler {
	return &AdvancedHandler{}
}

// StepData holds the data needed to render a single flow step.
type StepData struct {
	Step      dispatch.Step
	Variables []VarWithValue
	StepIndex int
	LastStep  int
	Owner     string
	Name      string
	EnvName   string
}

// VarWithValue pairs a variable definition with its current value.
type VarWithValue struct {
	engine.Variable
	Value    any
	ValueJSON string
}

// AdvancedEnvDetail renders the advanced environment detail page.
func (h *AdvancedHandler) AdvancedEnvDetail(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	// Fetch repo, dispatch config, and variables concurrently
	var (
		repo       *gh.Repo
		cfg        *dispatch.Config
		variables  []engine.Variable
		repoErr    error
		configErr  error
		wg         sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		repo, repoErr = gh.GetRepo(r.Context(), client, owner, name)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		configBytes, err := gh.GetFileContent(r.Context(), client, owner, name, ".dispatch.yaml")
		if err != nil {
			configErr = err
			return
		}
		cfg, err = dispatch.Parse(configBytes)
		if err != nil {
			configErr = err
			return
		}
		varsBytes, err := gh.GetFileContent(r.Context(), client, owner, name, cfg.VariablesPath)
		if err != nil {
			configErr = err
			return
		}
		eng, err := engine.GetEngine(cfg.Mode)
		if err != nil {
			configErr = err
			return
		}
		variables, err = eng.ParseVariables(varsBytes)
		if err != nil {
			configErr = err
		}
	}()
	wg.Wait()

	if repoErr != nil {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}
	if configErr != nil {
		slog.Error("load advanced config", "error", configErr)
		http.Error(w, "failed to load dispatch config", http.StatusInternalServerError)
		return
	}

	renderer.Page(w, "advanced_env_detail", map[string]any{
		"User":       user,
		"Repo":       repo,
		"EnvName":    envName,
		"Config":     cfg,
		"Variables":  variables,
		"ActivePage": "repos",
	})
}

// GetStep renders a single flow step's form as an htmx partial.
func (h *AdvancedHandler) GetStep(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	stepIdxStr := r.PathValue("stepIdx")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	stepIdx, err := strconv.Atoi(stepIdxStr)
	if err != nil {
		http.Error(w, "invalid step index", http.StatusBadRequest)
		return
	}

	// Fetch config, variables, and current env var values
	configBytes, err := gh.GetFileContent(r.Context(), client, owner, name, ".dispatch.yaml")
	if err != nil {
		slog.Error("get dispatch config", "error", err)
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}
	cfg, err := dispatch.Parse(configBytes)
	if err != nil {
		http.Error(w, "invalid config", http.StatusInternalServerError)
		return
	}

	if stepIdx >= len(cfg.Flow) {
		http.Error(w, "step not found", http.StatusNotFound)
		return
	}

	// Parse variable definitions
	varsBytes, err := gh.GetFileContent(r.Context(), client, owner, name, cfg.VariablesPath)
	if err != nil {
		http.Error(w, "failed to load variables", http.StatusInternalServerError)
		return
	}
	eng, _ := engine.GetEngine(cfg.Mode)
	allVars, _ := eng.ParseVariables(varsBytes)

	// Get current env variable values
	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}
	envVars, err := gh.ListEnvVariables(r.Context(), client, int(repoID), envName)
	if err != nil {
		slog.Error("list env variables for step", "error", err)
	}
	_ = user // used via auth

	// Build value map from current env vars
	valueMap := make(map[string]string)
	for _, v := range envVars {
		valueMap[strings.ToUpper(v.Name)] = v.Value
	}

	step := cfg.Flow[stepIdx]
	ignoreSet := cfg.IgnoreSet()
	varMap := make(map[string]engine.Variable)
	for _, v := range allVars {
		varMap[v.Name] = v
	}

	var stepVars []VarWithValue
	for _, inputName := range step.Inputs {
		if ignoreSet[inputName] {
			continue
		}
		v, ok := varMap[inputName]
		if !ok {
			continue
		}

		vwv := VarWithValue{Variable: v}
		if rawVal, ok := valueMap[strings.ToUpper("TF_VAR_"+inputName)]; ok && rawVal != "" {
			// Try to parse as JSON for complex types
			if v.Type.Kind != engine.TypeString {
				parsed, err := engine.JSONToGoValue(rawVal)
				if err == nil {
					vwv.Value = parsed
					vwv.ValueJSON = rawVal
				} else {
					vwv.Value = rawVal
					vwv.ValueJSON = rawVal
				}
			} else {
				vwv.Value = rawVal
				vwv.ValueJSON = rawVal
			}
		}

		stepVars = append(stepVars, vwv)
	}

	renderer.Partial(w, "advanced_step", StepData{
		Step:      step,
		Variables: stepVars,
		StepIndex: stepIdx,
		LastStep:  len(cfg.Flow) - 1,
		Owner:     owner,
		Name:      name,
		EnvName:   envName,
	})
}

// SaveStep saves the variables from a single flow step to GitHub env vars.
func (h *AdvancedHandler) SaveStep(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	owner := r.PathValue("owner")
	name := r.PathValue("name")
	envName := r.PathValue("env")
	stepIdxStr := r.PathValue("stepIdx")
	client := gh.NewClient(r.Context(), user.OAuthToken)

	stepIdx, err := strconv.Atoi(stepIdxStr)
	if err != nil {
		http.Error(w, "invalid step index", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	// Fetch config and variable defs to know the types
	configBytes, err := gh.GetFileContent(r.Context(), client, owner, name, ".dispatch.yaml")
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}
	cfg, _ := dispatch.Parse(configBytes)
	if stepIdx >= len(cfg.Flow) {
		http.Error(w, "step not found", http.StatusNotFound)
		return
	}

	varsBytes, err := gh.GetFileContent(r.Context(), client, owner, name, cfg.VariablesPath)
	if err != nil {
		http.Error(w, "failed to load variables", http.StatusInternalServerError)
		return
	}
	eng, _ := engine.GetEngine(cfg.Mode)
	allVars, _ := eng.ParseVariables(varsBytes)

	varMap := make(map[string]engine.Variable)
	for _, v := range allVars {
		varMap[v.Name] = v
	}

	repoID, err := getRepoID(r.Context(), client, owner, name)
	if err != nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return
	}

	_ = user // used via auth

	step := cfg.Flow[stepIdx]
	ignoreSet := cfg.IgnoreSet()

	for _, inputName := range step.Inputs {
		if ignoreSet[inputName] {
			continue
		}
		v, ok := varMap[inputName]
		if !ok {
			continue
		}

		var value string
		switch v.Type.Kind {
		case engine.TypeString:
			value = r.FormValue("var_" + inputName)
		case engine.TypeBool:
			value = r.FormValue("var_" + inputName)
			if value == "" {
				value = "false"
			}
		case engine.TypeNumber:
			value = r.FormValue("var_" + inputName)
		default:
			// Complex types: reassemble structured form fields into JSON
			var err error
			value, err = reassembleComplexValue(r, inputName, v.Type)
			if err != nil {
				slog.Error("reassemble complex value", "var", inputName, "error", err)
				value = ""
			}
		}

		if value == "" && v.HasDefault {
			continue
		}

		if err := gh.CreateOrUpdateEnvVariableByName(r.Context(), client, int(repoID), envName, "TF_VAR_"+inputName, value); err != nil {
			slog.Error("save env variable", "var", inputName, "error", err)
			w.Header().Set("HX-Trigger", fmt.Sprintf(`{"showToast": {"message": "Failed to save %s", "type": "error"}}`, inputName))
			http.Error(w, "failed to save variable", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Trigger", `{"showToast": {"message": "Step saved", "type": "success"}}`)
	w.WriteHeader(http.StatusNoContent)
}

// reassembleComplexValue reads structured form fields and returns a JSON string.
func reassembleComplexValue(r *http.Request, inputName string, varType engine.VarType) (string, error) {
	switch varType.Kind {
	case engine.TypeList:
		return reassembleList(r, inputName, varType.ElementType)
	case engine.TypeMap:
		if varType.ElementType != nil && varType.ElementType.Kind == engine.TypeObject {
			return reassembleMapObject(r, inputName, varType.ElementType.Attributes)
		}
		return reassembleMap(r, inputName, varType.ElementType)
	case engine.TypeObject:
		return reassembleObject(r, inputName, varType.Attributes)
	default:
		return r.FormValue("var_" + inputName), nil
	}
}

const maxFormItems = 1000

func reassembleList(r *http.Request, inputName string, elemType *engine.VarType) (string, error) {
	count, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("var_%s__count", inputName)))
	if count < 0 || count > maxFormItems {
		count = 0
	}
	items := make([]any, 0, count)
	for i := 0; i < count; i++ {
		val := r.FormValue(fmt.Sprintf("var_%s__list__%d", inputName, i))
		if val == "" {
			continue
		}
		items = append(items, coerceValue(val, elemType))
	}
	b, err := json.Marshal(items)
	return string(b), err
}

func reassembleMap(r *http.Request, inputName string, elemType *engine.VarType) (string, error) {
	count, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("var_%s__count", inputName)))
	if count < 0 || count > maxFormItems {
		count = 0
	}
	m := make(map[string]any)
	for i := 0; i < count; i++ {
		key := r.FormValue(fmt.Sprintf("var_%s__map__%d__key", inputName, i))
		val := r.FormValue(fmt.Sprintf("var_%s__map__%d__val", inputName, i))
		if key == "" {
			continue
		}
		m[key] = coerceValue(val, elemType)
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func reassembleMapObject(r *http.Request, inputName string, attrs []engine.ObjectAttribute) (string, error) {
	count, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("var_%s__count", inputName)))
	if count < 0 || count > maxFormItems {
		count = 0
	}
	m := make(map[string]any)
	for i := 0; i < count; i++ {
		key := r.FormValue(fmt.Sprintf("var_%s__map__%d__key", inputName, i))
		if key == "" {
			continue
		}
		obj := make(map[string]any)
		for _, attr := range attrs {
			fieldName := fmt.Sprintf("var_%s__map__%d__obj__%s", inputName, i, attr.Name)
			val := r.FormValue(fieldName)
			if val != "" {
				obj[attr.Name] = coerceValue(val, &attr.Type)
			} else if attr.Default != nil {
				obj[attr.Name] = attr.Default
			}
		}
		m[key] = obj
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func reassembleObject(r *http.Request, inputName string, attrs []engine.ObjectAttribute) (string, error) {
	obj := make(map[string]any)
	for _, attr := range attrs {
		fieldName := fmt.Sprintf("var_%s__obj__%s", inputName, attr.Name)
		val := r.FormValue(fieldName)
		if val != "" {
			obj[attr.Name] = coerceValue(val, &attr.Type)
		} else if attr.Default != nil {
			obj[attr.Name] = attr.Default
		}
	}
	b, err := json.Marshal(obj)
	return string(b), err
}

func coerceValue(raw string, t *engine.VarType) any {
	if t == nil {
		return raw
	}
	switch t.Kind {
	case engine.TypeBool:
		return raw == "true"
	case engine.TypeNumber:
		if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(raw, 64); err == nil {
			return f
		}
		return raw
	default:
		return raw
	}
}
