package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"
)

// RunQuery narrows a run listing. A zero value lists the most recent runs
// across every workflow.
type RunQuery struct {
	// Workflow is a workflow filename ("ci.yml"), not its display name. The
	// Actions API has no filter for the display name.
	Workflow string
	Branch   string
	Status   string
	Event    string
	Limit    int
}

// maxRunsPerPage is the largest per_page the Actions API accepts.
const maxRunsPerPage = 100

func (q RunQuery) path(repo string) string {
	params := url.Values{}

	limit := q.Limit
	if limit <= 0 || limit > maxRunsPerPage {
		limit = maxRunsPerPage
	}

	params.Set("per_page", strconv.Itoa(limit))

	for key, value := range map[string]string{
		"branch": q.Branch,
		"event":  q.Event,
		"status": q.Status,
	} {
		if value != "" {
			params.Set(key, value)
		}
	}

	base := "repos/" + repo + "/actions/runs"
	if q.Workflow != "" {
		base = "repos/" + repo + "/actions/workflows/" + url.PathEscape(q.Workflow) + "/runs"
	}

	return base + "?" + params.Encode()
}

// apiRun is the Actions API's run shape. It is spelled separately from
// forge.WorkflowRun because the API's field names are snake_case and its
// workflow reference is a path rather than a filename.
type apiRun struct {
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	HTMLURL      string    `json:"html_url"`
	HeadBranch   string    `json:"head_branch"`
	Event        string    `json:"event"`
	Path         string    `json:"path"`
	DisplayTitle string    `json:"display_title"`
	ID           int64     `json:"id"`
}

func (r apiRun) toForge() forge.WorkflowRun {
	return forge.WorkflowRun{
		ID:         r.ID,
		Name:       r.Name,
		Status:     r.Status,
		Conclusion: r.Conclusion,
		URL:        r.HTMLURL,
		HeadBranch: r.HeadBranch,
		Event:      r.Event,
		Path:       workflowFile(r.Path),
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

// workflowFile reduces the API's ".github/workflows/ci.yml" to "ci.yml", which
// is what RunQuery.Workflow and a workflow listing both speak in.
func workflowFile(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}

	return path
}

// ListRuns fetches recent workflow runs matching q, newest first.
//
// The repo argument is "owner/name". The repoPath argument is the checkout gh
// runs in, which is what supplies the host and the token.
func ListRuns(ctx context.Context, repoPath, repo string, q RunQuery) ([]forge.WorkflowRun, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "api", q.path(repo))
	if err != nil {
		return nil, fmt.Errorf("listing runs for %s: %w", repo, err)
	}

	var payload struct {
		WorkflowRuns []apiRun `json:"workflow_runs"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parsing the run list: %w", err)
	}

	runs := make([]forge.WorkflowRun, 0, len(payload.WorkflowRuns))
	for i := range payload.WorkflowRuns {
		runs = append(runs, payload.WorkflowRuns[i].toForge())
	}

	return runs, nil
}

// GetRun fetches one run by ID.
func GetRun(ctx context.Context, repoPath, repo string, runID int64) (*forge.WorkflowRun, error) {
	path := fmt.Sprintf("repos/%s/actions/runs/%d", repo, runID)

	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "api", path)
	if err != nil {
		return nil, fmt.Errorf("reading run %d: %w", runID, err)
	}

	var raw apiRun
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing run %d: %w", runID, err)
	}

	run := raw.toForge()

	return &run, nil
}

// RunJobs fetches a run's jobs with their steps and timings.
func RunJobs(ctx context.Context, repoPath, repo string, runID int64) ([]forge.Job, error) {
	path := fmt.Sprintf("repos/%s/actions/runs/%d/jobs?per_page=%d", repo, runID, maxRunsPerPage)

	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "api", path)
	if err != nil {
		return nil, fmt.Errorf("reading jobs for run %d: %w", runID, err)
	}

	var payload struct {
		Jobs []forge.Job `json:"jobs"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parsing jobs for run %d: %w", runID, err)
	}

	return payload.Jobs, nil
}

// LatestRunsOnBranch returns the newest run of each distinct workflow on a
// branch, newest first.
func LatestRunsOnBranch(
	ctx context.Context, repoPath, repo, branch string, within time.Duration,
) ([]forge.WorkflowRun, error) {
	runs, err := ListRuns(ctx, repoPath, repo, RunQuery{Branch: branch})
	if err != nil {
		return nil, err
	}

	return LatestPerWorkflow(runs, within), nil
}

// LatestPerWorkflow reduces a run listing to each workflow's current state,
// newest first, dropping anything that finished before within elapsed. A
// non-positive within keeps every age.
//
// A run listing is mostly history: one workflow re-run twenty times says
// nothing a reader needs beyond its current state. Runs are keyed by branch,
// workflow file, and display title together, because a workflow that reports a
// mode in its title (a Pulumi preview against a Pulumi deploy) has two current
// states rather than one.
func LatestPerWorkflow(runs []forge.WorkflowRun, within time.Duration) []forge.WorkflowRun {
	cutoff := time.Time{}
	if within > 0 {
		cutoff = time.Now().Add(-within)
	}

	latest := make(map[string]forge.WorkflowRun, len(runs))

	for i := range runs {
		run := &runs[i]

		// A run still going is current whatever its age says.
		if !run.IsActive() && run.CreatedAt.Before(cutoff) {
			continue
		}

		key := run.HeadBranch + "\x00" + run.Path + "\x00" + run.Name
		if seen, ok := latest[key]; ok && !seen.CreatedAt.Before(run.CreatedAt) {
			continue
		}

		latest[key] = *run
	}

	kept := make([]forge.WorkflowRun, 0, len(latest))
	for key := range latest {
		kept = append(kept, latest[key])
	}

	sort.Slice(kept, func(i, j int) bool { return kept[i].CreatedAt.After(kept[j].CreatedAt) })

	return kept
}
