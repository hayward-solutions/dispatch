# Actions Observability

Dispatch provides per-repository observability dashboards for GitHub Actions, giving you visibility into workflow health, performance trends, and billing usage.

## Accessing Observability

From any tracked repository's detail page, click the **Observability** button in the header to open the dashboard.

## Dashboard Overview

The observability page shows data from the **last 30 days** of workflow runs, fetched on-demand from the GitHub API.

### Summary Cards

Four cards at the top provide a high-level overview:

- **Total Runs** — total workflow runs across all workflows
- **Success Rate** — percentage of runs that succeeded (excludes in-progress, cancelled, and skipped runs)
- **Avg Duration** — average run time for completed workflows
- **Failed Runs** — total number of failed runs

### Workflow Breakdown

A table showing per-workflow statistics:

| Column | Description |
|--------|-------------|
| Workflow | Workflow name |
| Runs | Total runs in the period |
| Success Rate | Color-coded: green (>=80%), amber (50-79%), red (<50%) |
| Avg Duration | Average duration of completed runs |
| Last Run | Relative time of the most recent run |
| Status | Conclusion of the most recent run |

### Actions Minutes

Displays billable minutes for the current billing cycle, broken down by runner OS:

- **Ubuntu** — Linux runner minutes
- **macOS** — macOS runner minutes
- **Windows** — Windows runner minutes
- **Total** — combined billable minutes

!!! note
    Billing data is only available for private repositories on paid GitHub plans (Team or Enterprise). Public repositories use free Actions minutes and will show "Billing data unavailable."

### Run History

A filterable, paginated table of individual workflow runs with:

- **Workflow filter** — narrow to a specific workflow
- **Status filter** — filter by success, failure, in_progress, or cancelled
- **Branch filter** — filter by branch name

Each row shows the workflow name, branch, status badge, duration, start time, and a link to the run on GitHub.

## GitHub OAuth Scopes

Observability uses the same OAuth token as the rest of Dispatch. The required scopes are:

- `repo` — read access to workflow runs
- `workflow` — read access to workflow definitions and usage data

No additional scopes are needed. Billing minute data requires that the authenticated user has access to the repository's Actions usage, which is typically available to repository admins and organization owners.

## API Usage

Each visit to the observability page makes the following GitHub API calls:

- **1 call** to list workflow runs (up to 100 runs, last 30 days)
- **1 call** per distinct workflow to fetch billing usage (typically 3-5 workflows)

Filtering and paginating the run history makes 1 additional API call per interaction. All data is fetched on-demand with no caching or database storage.
