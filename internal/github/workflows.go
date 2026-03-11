package github

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
	"gopkg.in/yaml.v3"
)

type Workflow struct {
	ID     int64
	Name   string
	Path   string
	State  string
	Inputs []WorkflowInput
}

type WorkflowInput struct {
	Name        string
	Description string
	Required    bool
	Default     string
	Type        string
	Options     []string
}

type Environment struct {
	ID   int64
	Name string
}

// ListDispatchWorkflows returns workflows that have a workflow_dispatch trigger.
// Fetches workflow YAML files concurrently for faster loading.
func ListDispatchWorkflows(ctx context.Context, client *github.Client, owner, repo string) ([]Workflow, error) {
	opts := &github.ListOptions{PerPage: 100}
	ghWorkflows, _, err := client.Actions.ListWorkflows(ctx, owner, repo, opts)
	if err != nil {
		return nil, err
	}

	// Filter to active workflows first
	type activeWorkflow struct {
		id    int64
		name  string
		path  string
		state string
	}
	var active []activeWorkflow
	for _, w := range ghWorkflows.Workflows {
		if w.GetState() != "active" {
			continue
		}
		active = append(active, activeWorkflow{
			id:    w.GetID(),
			name:  w.GetName(),
			path:  w.GetPath(),
			state: w.GetState(),
		})
	}

	// Fetch all workflow YAML files concurrently
	type parseResult struct {
		index  int
		inputs []WorkflowInput
		ok     bool
	}

	ch := make(chan parseResult, len(active))
	for i, w := range active {
		go func(idx int, path string) {
			inputs, ok := parseWorkflowDispatch(ctx, client, owner, repo, path)
			ch <- parseResult{index: idx, inputs: inputs, ok: ok}
		}(i, w.path)
	}

	// Collect results
	parsed := make([]parseResult, len(active))
	for range active {
		r := <-ch
		parsed[r.index] = r
	}

	var results []Workflow
	for i, w := range active {
		if !parsed[i].ok {
			continue
		}
		results = append(results, Workflow{
			ID:     w.id,
			Name:   w.name,
			Path:   w.path,
			State:  w.state,
			Inputs: parsed[i].inputs,
		})
	}

	return results, nil
}

// parseWorkflowDispatch fetches a workflow YAML file and checks for workflow_dispatch trigger.
// Returns the parsed inputs and true if workflow_dispatch is present.
func parseWorkflowDispatch(ctx context.Context, client *github.Client, owner, repo, path string) ([]WorkflowInput, bool) {
	content, _, _, err := client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		slog.Debug("failed to fetch workflow content", "path", path, "error", err)
		return nil, false
	}

	if content == nil || content.Content == nil {
		return nil, false
	}

	decoded, err := base64.StdEncoding.DecodeString(*content.Content)
	if err != nil {
		slog.Debug("failed to decode workflow content", "path", path, "error", err)
		return nil, false
	}

	return extractDispatchInputs(decoded)
}

// workflowYAML is a minimal representation for parsing the `on` key.
type workflowYAML struct {
	On workflowOn `yaml:"on"`
}

// workflowOn handles `on:` being either a string, list, or map.
type workflowOn struct {
	Dispatch *workflowDispatch
}

type workflowDispatch struct {
	Inputs map[string]workflowInputYAML `yaml:"inputs"`
}

type workflowInputYAML struct {
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	Type        string   `yaml:"type"`
	Options     []string `yaml:"options"`
}

func (o *workflowOn) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// on: workflow_dispatch
		if value.Value == "workflow_dispatch" {
			o.Dispatch = &workflowDispatch{}
		}
		return nil

	case yaml.SequenceNode:
		// on: [push, workflow_dispatch]
		for _, item := range value.Content {
			if item.Value == "workflow_dispatch" {
				o.Dispatch = &workflowDispatch{}
				return nil
			}
		}
		return nil

	case yaml.MappingNode:
		// on: { workflow_dispatch: { inputs: ... } }
		raw := make(map[string]yaml.Node)
		if err := value.Decode(&raw); err != nil {
			return err
		}
		if node, ok := raw["workflow_dispatch"]; ok {
			d := &workflowDispatch{}
			if node.Kind == yaml.MappingNode {
				if err := node.Decode(d); err != nil {
					return err
				}
			}
			o.Dispatch = d
		}
		return nil
	}

	return nil
}

func extractDispatchInputs(yamlContent []byte) ([]WorkflowInput, bool) {
	var wf workflowYAML
	if err := yaml.Unmarshal(yamlContent, &wf); err != nil {
		slog.Debug("failed to parse workflow YAML", "error", err)
		return nil, false
	}

	if wf.On.Dispatch == nil {
		return nil, false
	}

	var inputs []WorkflowInput
	for name, input := range wf.On.Dispatch.Inputs {
		typ := input.Type
		if typ == "" {
			typ = "string"
		}
		inputs = append(inputs, WorkflowInput{
			Name:        name,
			Description: input.Description,
			Required:    input.Required,
			Default:     input.Default,
			Type:        typ,
			Options:     input.Options,
		})
	}

	sort.Slice(inputs, func(i, j int) bool {
		return inputs[i].Name < inputs[j].Name
	})

	return inputs, true
}

// EnvironmentDeployment represents a deployment to a specific environment.
type EnvironmentDeployment struct {
	ID           int64
	SHA          string
	Ref          string
	Environment  string
	Description  string
	Creator      string
	CreatedAt    time.Time
	State        string // pending, success, failure, error, inactive, in_progress, queued
	LogURL       string // links back to the workflow run
	RunID        int64  // workflow run ID extracted from LogURL
	WorkflowName string
}

var runIDFromURLRegex = regexp.MustCompile(`/actions/runs/(\d+)`)

// ListEnvironmentDeployments returns recent deployments for a specific environment.
func ListEnvironmentDeployments(ctx context.Context, client *github.Client, owner, repo, environment string, limit int) ([]EnvironmentDeployment, error) {
	opts := &github.DeploymentsListOptions{
		Environment: environment,
		ListOptions: github.ListOptions{PerPage: limit},
	}
	deployments, _, err := client.Repositories.ListDeployments(ctx, owner, repo, opts)
	if err != nil {
		return nil, err
	}

	// Fetch latest status for each deployment concurrently
	type statusResult struct {
		index  int
		state  string
		logURL string
	}

	ch := make(chan statusResult, len(deployments))
	for i, d := range deployments {
		go func(idx int, deployID int64) {
			statuses, _, err := client.Repositories.ListDeploymentStatuses(ctx, owner, repo, deployID, &github.ListOptions{PerPage: 1})
			if err != nil || len(statuses) == 0 {
				ch <- statusResult{index: idx, state: "pending"}
				return
			}
			ch <- statusResult{
				index:  idx,
				state:  statuses[0].GetState(),
				logURL: statuses[0].GetLogURL(),
			}
		}(i, d.GetID())
	}

	statuses := make([]statusResult, len(deployments))
	for range deployments {
		r := <-ch
		statuses[r.index] = r
	}

	// Extract unique run IDs from LogURLs to fetch workflow names
	runIDSet := make(map[int64]bool)
	depRunIDs := make([]int64, len(deployments)) // run ID per deployment, 0 if unknown
	for i, s := range statuses {
		if m := runIDFromURLRegex.FindStringSubmatch(s.logURL); len(m) == 2 {
			if id, err := strconv.ParseInt(m[1], 10, 64); err == nil {
				depRunIDs[i] = id
				runIDSet[id] = true
			}
		}
	}

	// Fetch workflow run names concurrently
	type runNameResult struct {
		id   int64
		name string
	}
	runNameCh := make(chan runNameResult, len(runIDSet))
	for id := range runIDSet {
		go func(runID int64) {
			run, _, err := client.Actions.GetWorkflowRunByID(ctx, owner, repo, runID)
			if err != nil || run == nil {
				runNameCh <- runNameResult{id: runID}
				return
			}
			runNameCh <- runNameResult{id: runID, name: run.GetName()}
		}(id)
	}

	runNames := make(map[int64]string, len(runIDSet))
	for range runIDSet {
		r := <-runNameCh
		if r.name != "" {
			runNames[r.id] = r.name
		}
	}

	results := make([]EnvironmentDeployment, 0, len(deployments))
	for i, d := range deployments {
		dep := EnvironmentDeployment{
			ID:           d.GetID(),
			SHA:          d.GetSHA(),
			Ref:          d.GetRef(),
			Environment:  d.GetEnvironment(),
			Description:  d.GetDescription(),
			State:        statuses[i].state,
			LogURL:       statuses[i].logURL,
			RunID:        depRunIDs[i],
			WorkflowName: runNames[depRunIDs[i]],
		}
		if d.Creator != nil {
			dep.Creator = d.Creator.GetLogin()
		}
		if d.CreatedAt != nil {
			dep.CreatedAt = d.CreatedAt.Time
		}
		results = append(results, dep)
	}

	return results, nil
}

// WorkflowRun represents a single workflow run.
type WorkflowRun struct {
	ID           int64
	Name         string
	DisplayTitle string
	Status       string // queued, in_progress, completed
	Conclusion   string // success, failure, cancelled, skipped, etc.
	Event        string // workflow_dispatch, push, pull_request, etc.
	HeadBranch   string
	RunNumber    int
	HTMLURL      string
	CreatedAt    time.Time
	Actor        string
	ActorAvatar  string
}

// ListWorkflowRuns returns recent workflow runs for a repository.
func ListWorkflowRuns(ctx context.Context, client *github.Client, owner, repo string, limit int) ([]WorkflowRun, error) {
	opts := &github.ListWorkflowRunsOptions{
		ListOptions: github.ListOptions{PerPage: limit},
	}
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
	if err != nil {
		return nil, err
	}

	results := make([]WorkflowRun, 0, len(runs.WorkflowRuns))
	for _, r := range runs.WorkflowRuns {
		run := WorkflowRun{
			ID:           r.GetID(),
			Name:         r.GetName(),
			DisplayTitle: r.GetDisplayTitle(),
			Status:       r.GetStatus(),
			Conclusion:   r.GetConclusion(),
			Event:        r.GetEvent(),
			HeadBranch:   r.GetHeadBranch(),
			RunNumber:    r.GetRunNumber(),
			HTMLURL:      r.GetHTMLURL(),
		}
		if r.CreatedAt != nil {
			run.CreatedAt = r.CreatedAt.Time
		}
		if r.Actor != nil {
			run.Actor = r.Actor.GetLogin()
			run.ActorAvatar = r.Actor.GetAvatarURL()
		}
		results = append(results, run)
	}

	return results, nil
}

// ListEnvironments returns all environments for a repository.
func ListEnvironments(ctx context.Context, client *github.Client, owner, repo string) ([]Environment, error) {
	envResp, _, err := client.Repositories.ListEnvironments(ctx, owner, repo, nil)
	if err != nil {
		return nil, err
	}

	if envResp == nil {
		return nil, nil
	}

	results := make([]Environment, 0, len(envResp.Environments))
	for _, e := range envResp.Environments {
		results = append(results, Environment{
			ID:   e.GetID(),
			Name: e.GetName(),
		})
	}

	return results, nil
}

// RepoRefs holds branches and tags for a repository.
type RepoRefs struct {
	Branches []string
	Tags     []string
}

// ListRepoRefs returns branches and tags for a repository, fetched concurrently.
func ListRepoRefs(ctx context.Context, client *github.Client, owner, repo string) (*RepoRefs, error) {
	refs := &RepoRefs{}
	var branchErr, tagErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		branches, _, err := client.Repositories.ListBranches(ctx, owner, repo, &github.BranchListOptions{
			ListOptions: github.ListOptions{PerPage: 100},
		})
		if err != nil {
			branchErr = err
			return
		}
		for _, b := range branches {
			refs.Branches = append(refs.Branches, b.GetName())
		}
	}()
	go func() {
		defer wg.Done()
		tags, _, err := client.Repositories.ListTags(ctx, owner, repo, &github.ListOptions{PerPage: 100})
		if err != nil {
			tagErr = err
			return
		}
		for _, t := range tags {
			refs.Tags = append(refs.Tags, t.GetName())
		}
	}()
	wg.Wait()

	if branchErr != nil {
		return nil, branchErr
	}
	if tagErr != nil {
		return nil, tagErr
	}

	return refs, nil
}

// WorkflowJob represents a job within a workflow run.
type WorkflowJob struct {
	ID          int64
	RunID       int64
	Name        string
	Status      string // queued, in_progress, completed
	Conclusion  string // success, failure, cancelled, skipped
	StartedAt   time.Time
	CompletedAt time.Time
	Steps       []JobStep
}

// JobStep represents a single step within a workflow job.
type JobStep struct {
	Name        string
	Status      string
	Conclusion  string
	Number      int64
	StartedAt   time.Time
	CompletedAt time.Time
}

// ListWorkflowJobs returns the jobs for a workflow run.
func ListWorkflowJobs(ctx context.Context, client *github.Client, owner, repo string, runID int64) ([]WorkflowJob, error) {
	opts := &github.ListWorkflowJobsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	jobs, _, err := client.Actions.ListWorkflowJobs(ctx, owner, repo, runID, opts)
	if err != nil {
		return nil, err
	}

	results := make([]WorkflowJob, 0, len(jobs.Jobs))
	for _, j := range jobs.Jobs {
		job := WorkflowJob{
			ID:         j.GetID(),
			RunID:      j.GetRunID(),
			Name:       j.GetName(),
			Status:     j.GetStatus(),
			Conclusion: j.GetConclusion(),
		}
		if j.StartedAt != nil {
			job.StartedAt = j.StartedAt.Time
		}
		if j.CompletedAt != nil {
			job.CompletedAt = j.CompletedAt.Time
		}
		for _, s := range j.Steps {
			step := JobStep{
				Name:       s.GetName(),
				Status:     s.GetStatus(),
				Conclusion: s.GetConclusion(),
				Number:     s.GetNumber(),
			}
			if s.StartedAt != nil {
				step.StartedAt = s.StartedAt.Time
			}
			if s.CompletedAt != nil {
				step.CompletedAt = s.CompletedAt.Time
			}
			job.Steps = append(job.Steps, step)
		}
		results = append(results, job)
	}

	return results, nil
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// GetWorkflowJobLog fetches the log output for a workflow job.
// Returns the log text (truncated to maxLines from the end), whether it was truncated, and any error.
func GetWorkflowJobLog(ctx context.Context, client *github.Client, owner, repo string, jobID int64, maxLines int) (string, bool, error) {
	logURL, _, err := client.Actions.GetWorkflowJobLogs(ctx, owner, repo, jobID, 3)
	if err != nil {
		return "", false, fmt.Errorf("get job log URL: %w", err)
	}

	httpClient := client.Client()
	resp, err := httpClient.Get(logURL.String())
	if err != nil {
		return "", false, fmt.Errorf("download job log: %w", err)
	}
	defer resp.Body.Close()

	// Read lines into a ring buffer to keep the last maxLines
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // up to 1MB lines
	var lines []string
	totalLines := 0
	for scanner.Scan() {
		totalLines++
		line := ansiRegex.ReplaceAllString(scanner.Text(), "")
		if len(lines) < maxLines {
			lines = append(lines, line)
		} else {
			lines[totalLines%maxLines] = line
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return "", false, fmt.Errorf("read job log: %w", err)
	}

	truncated := totalLines > maxLines
	if truncated {
		// Reorder the ring buffer
		start := totalLines % maxLines
		ordered := make([]string, 0, maxLines)
		ordered = append(ordered, lines[start:]...)
		ordered = append(ordered, lines[:start]...)
		lines = ordered
	}

	var result string
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}

	return result, truncated, nil
}

// DispatchWorkflow triggers a workflow_dispatch event.
func DispatchWorkflow(ctx context.Context, client *github.Client, owner, repo string, workflowID int64, ref string, inputs map[string]interface{}) error {
	event := github.CreateWorkflowDispatchEventRequest{
		Ref:    ref,
		Inputs: inputs,
	}
	_, err := client.Actions.CreateWorkflowDispatchEventByID(ctx, owner, repo, workflowID, event)
	return err
}
