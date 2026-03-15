package github

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
)

// WorkflowRunSummary is a lightweight run record for observability stats aggregation.
type WorkflowRunSummary struct {
	ID           int64
	WorkflowID   int64
	WorkflowName string
	Status       string
	Conclusion   string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	RunStartedAt time.Time
	HeadBranch   string
	HeadSHA      string
	HTMLURL      string
	DurationSecs int64
	Event        string
	Actor        string
	ActorAvatar  string
}

// WorkflowStat represents aggregated stats for a single workflow.
type WorkflowStat struct {
	WorkflowID      int64
	WorkflowName    string
	TotalRuns       int
	SuccessCount    int
	FailureCount    int
	SuccessRate     float64 // 0-100
	AvgDurationSecs int64
	LastRunAt       *time.Time
	LastConclusion  string
}

// RepoObservability is the top-level observability data for a repo.
type RepoObservability struct {
	TotalRuns       int
	SuccessRate     float64
	AvgDurationSecs int64
	TotalFailures   int
	WorkflowStats   []WorkflowStat
	RecentRuns      []WorkflowRunSummary
	BillableMinutes *RepoBillableMinutes
	Period          string
}

// RepoBillableMinutes represents Actions billing data for a repo.
type RepoBillableMinutes struct {
	Ubuntu  int
	MacOS   int
	Windows int
	Total   int
}

// WorkflowHistoryOptions configures filtering for GetWorkflowRunHistory.
type WorkflowHistoryOptions struct {
	WorkflowID int64
	Status     string // "", "success", "failure", "in_progress", "cancelled"
	Branch     string
	Page       int
	PerPage    int // default 25
}

// GetRepoObservability fetches workflow runs from the last 30 days and aggregates stats.
func GetRepoObservability(ctx context.Context, client *github.Client, owner, repo string) (*RepoObservability, error) {
	since := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	opts := &github.ListWorkflowRunsOptions{
		Created:     ">" + since,
		ListOptions: github.ListOptions{PerPage: 100},
	}

	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
	if err != nil {
		return nil, err
	}

	summaries := make([]WorkflowRunSummary, 0, len(runs.WorkflowRuns))
	for _, r := range runs.WorkflowRuns {
		summaries = append(summaries, mapWorkflowRun(r))
	}

	// Aggregate per-workflow stats
	statMap := make(map[int64]*WorkflowStat)
	var totalDuration int64
	var completedCount int

	for _, s := range summaries {
		ws, ok := statMap[s.WorkflowID]
		if !ok {
			ws = &WorkflowStat{
				WorkflowID:   s.WorkflowID,
				WorkflowName: s.WorkflowName,
			}
			statMap[s.WorkflowID] = ws
		}

		ws.TotalRuns++

		if ws.LastRunAt == nil || s.CreatedAt.After(*ws.LastRunAt) {
			t := s.CreatedAt
			ws.LastRunAt = &t
			ws.LastConclusion = s.Conclusion
		}

		switch s.Conclusion {
		case "success":
			ws.SuccessCount++
		case "failure":
			ws.FailureCount++
		}

		if s.Status == "completed" && s.DurationSecs > 0 {
			completedCount++
			totalDuration += s.DurationSecs
		}
	}

	// Compute per-workflow averages and success rates
	stats := make([]WorkflowStat, 0, len(statMap))
	for _, ws := range statMap {
		decided := ws.SuccessCount + ws.FailureCount
		if decided > 0 {
			ws.SuccessRate = float64(ws.SuccessCount) / float64(decided) * 100
		}

		// Compute avg duration for this workflow's completed runs
		var wfDuration int64
		var wfCompleted int
		for _, s := range summaries {
			if s.WorkflowID == ws.WorkflowID && s.Status == "completed" && s.DurationSecs > 0 {
				wfDuration += s.DurationSecs
				wfCompleted++
			}
		}
		if wfCompleted > 0 {
			ws.AvgDurationSecs = wfDuration / int64(wfCompleted)
		}

		stats = append(stats, *ws)
	}

	// Overall stats
	totalRuns := len(summaries)
	var totalSuccesses, totalFailures int
	for _, ws := range stats {
		totalSuccesses += ws.SuccessCount
		totalFailures += ws.FailureCount
	}

	var overallSuccessRate float64
	totalDecided := totalSuccesses + totalFailures
	if totalDecided > 0 {
		overallSuccessRate = float64(totalSuccesses) / float64(totalDecided) * 100
	}

	var avgDuration int64
	if completedCount > 0 {
		avgDuration = totalDuration / int64(completedCount)
	}

	// Recent runs (top 20)
	recentRuns := summaries
	if len(recentRuns) > 20 {
		recentRuns = recentRuns[:20]
	}

	// Collect distinct workflow IDs for billing
	workflowIDs := make([]int64, 0, len(statMap))
	for id := range statMap {
		workflowIDs = append(workflowIDs, id)
	}

	// Fetch billable minutes concurrently (best-effort)
	billable := GetRepoBillableMinutes(ctx, client, owner, repo, workflowIDs)

	return &RepoObservability{
		TotalRuns:       totalRuns,
		SuccessRate:     overallSuccessRate,
		AvgDurationSecs: avgDuration,
		TotalFailures:   totalFailures,
		WorkflowStats:   stats,
		RecentRuns:      recentRuns,
		BillableMinutes: billable,
		Period:          "Last 30 days",
	}, nil
}

// GetRepoBillableMinutes fetches billable minutes per workflow and sums them.
// Returns nil if unavailable (best-effort).
func GetRepoBillableMinutes(ctx context.Context, client *github.Client, owner, repo string, workflowIDs []int64) *RepoBillableMinutes {
	if len(workflowIDs) == 0 {
		return nil
	}

	type usageResult struct {
		ubuntu  int64
		macos   int64
		windows int64
	}

	results := make([]usageResult, len(workflowIDs))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var anySuccess bool

	for i, wfID := range workflowIDs {
		wg.Add(1)
		go func(idx int, id int64) {
			defer wg.Done()
			usage, _, err := client.Actions.GetWorkflowUsageByID(ctx, owner, repo, id)
			if err != nil {
				slog.Debug("get workflow usage", "workflow_id", id, "error", err)
				return
			}
			if usage.Billable == nil {
				return
			}
			var r usageResult
			for osName, bill := range *usage.Billable {
				if bill == nil {
					continue
				}
				ms := bill.GetTotalMS()
				switch osName {
				case "UBUNTU":
					r.ubuntu = ms
				case "MACOS":
					r.macos = ms
				case "WINDOWS":
					r.windows = ms
				}
			}
			mu.Lock()
			results[idx] = r
			anySuccess = true
			mu.Unlock()
		}(i, wfID)
	}
	wg.Wait()

	if !anySuccess {
		return nil
	}

	var total usageResult
	for _, r := range results {
		total.ubuntu += r.ubuntu
		total.macos += r.macos
		total.windows += r.windows
	}

	// If no billable time at all, treat as unavailable
	if total.ubuntu == 0 && total.macos == 0 && total.windows == 0 {
		return nil
	}

	// Convert milliseconds to minutes (round up so small values don't show as 0)
	msToMin := func(ms int64) int {
		if ms <= 0 {
			return 0
		}
		return int((ms + 59999) / 60000)
	}
	bill := &RepoBillableMinutes{
		Ubuntu:  msToMin(total.ubuntu),
		MacOS:   msToMin(total.macos),
		Windows: msToMin(total.windows),
	}
	bill.Total = bill.Ubuntu + bill.MacOS + bill.Windows
	return bill
}

// GetWorkflowRunHistory returns a paginated, filterable list of workflow runs.
func GetWorkflowRunHistory(ctx context.Context, client *github.Client, owner, repo string, opts WorkflowHistoryOptions) ([]WorkflowRunSummary, int, error) {
	if opts.PerPage <= 0 {
		opts.PerPage = 25
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}

	listOpts := &github.ListWorkflowRunsOptions{
		Branch: opts.Branch,
		Status: opts.Status,
		ListOptions: github.ListOptions{
			PerPage: opts.PerPage,
			Page:    opts.Page,
		},
	}

	var runs *github.WorkflowRuns
	var err error

	if opts.WorkflowID != 0 {
		runs, _, err = client.Actions.ListWorkflowRunsByID(ctx, owner, repo, opts.WorkflowID, listOpts)
	} else {
		runs, _, err = client.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, listOpts)
	}
	if err != nil {
		return nil, 0, err
	}

	summaries := make([]WorkflowRunSummary, 0, len(runs.WorkflowRuns))
	for _, r := range runs.WorkflowRuns {
		summaries = append(summaries, mapWorkflowRun(r))
	}

	return summaries, runs.GetTotalCount(), nil
}

func mapWorkflowRun(r *github.WorkflowRun) WorkflowRunSummary {
	s := WorkflowRunSummary{
		ID:           r.GetID(),
		WorkflowID:   r.GetWorkflowID(),
		WorkflowName: r.GetName(),
		Status:       r.GetStatus(),
		Conclusion:   r.GetConclusion(),
		HeadBranch:   r.GetHeadBranch(),
		HeadSHA:      r.GetHeadSHA(),
		HTMLURL:      r.GetHTMLURL(),
		Event:        r.GetEvent(),
	}
	if r.CreatedAt != nil {
		s.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt != nil {
		s.UpdatedAt = r.UpdatedAt.Time
	}
	if r.RunStartedAt != nil {
		s.RunStartedAt = r.RunStartedAt.Time
	}
	if r.Actor != nil {
		s.Actor = r.Actor.GetLogin()
		s.ActorAvatar = r.Actor.GetAvatarURL()
	}

	// Compute duration for completed runs
	if r.GetStatus() == "completed" && r.RunStartedAt != nil && r.UpdatedAt != nil {
		s.DurationSecs = int64(r.UpdatedAt.Time.Sub(r.RunStartedAt.Time).Seconds())
		if s.DurationSecs < 0 {
			s.DurationSecs = 0
		}
	}

	return s
}
