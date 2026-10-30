package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// A filtered query returns at most maxFilteredRuns results, however many pages are requested.
	maxFilteredRuns = 1000
	dayLength       = 24 * time.Hour
	dayConcurrency  = 4
	// Below minSplitSpan a range that hits the cap is reported rather than halved.
	minSplitSpan = time.Minute
	halves       = 2
)

// RunTiming is one workflow run reduced to what a timing rollup needs. Unlike
// forge.WorkflowRun it carries the head SHA, the actor, the attempt, and the
// pull requests, which grouping runs by commit or author depends on.
type RunTiming struct {
	CommittedAt time.Time `json:"committed_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `json:"name"`
	Actor       string    `json:"actor,omitempty"`
	HeadBranch  string    `json:"head_branch,omitempty"`
	Path        string    `json:"path,omitempty"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	HeadSHA     string    `json:"head_sha"`
	PRs         []int     `json:"prs,omitempty"`
	ID          int64     `json:"id"`
	RunAttempt  int       `json:"run_attempt,omitempty"`
}

// RunTimingFilter narrows a run listing. An empty field matches everything.
type RunTimingFilter struct {
	Branch, Event, Actor string
}

// RunTimingSample is the runs created in a time range, plus the slices where the
// API's result cap may have hidden some.
type RunTimingSample struct {
	Runs   []RunTiming
	Capped []time.Time
}

// apiRunTiming is the Actions API's run shape, trimmed to what RunTiming reads.
type apiRunTiming struct {
	HeadCommit struct {
		Timestamp time.Time `json:"timestamp"`
	} `json:"head_commit"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Actor     struct {
		Login string `json:"login"`
	} `json:"actor"`
	Name         string `json:"name"`
	HeadBranch   string `json:"head_branch"`
	Path         string `json:"path"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	HeadSHA      string `json:"head_sha"`
	PullRequests []struct {
		Number int `json:"number"`
	} `json:"pull_requests"`
	ID         int64 `json:"id"`
	RunAttempt int   `json:"run_attempt"`
}

// ListRunTimings reads the runs matching filter created in [from, to), one day
// at a time, because a filtered query returns at most 1000 results. A range that
// hits the cap is halved down to a minute, and a range that still does is listed
// in Capped so the caller can say the sample is incomplete.
func ListRunTimings(
	ctx context.Context, repoPath, repo string, filter RunTimingFilter, from, to time.Time,
) (RunTimingSample, error) {
	var days []time.Time

	for day := from; day.Before(to); day = day.Add(dayLength) {
		days = append(days, day)
	}

	perDay := make([][]RunTiming, len(days))
	cappedPerDay := make([][]time.Time, len(days))

	var group errgroup.Group

	group.SetLimit(dayConcurrency)

	for i, day := range days {
		group.Go(func() error {
			end := day.Add(dayLength)
			if end.After(to) {
				end = to
			}

			runs, capped, err := fetchRunRange(ctx, repoPath, repo, filter, day, end)
			perDay[i], cappedPerDay[i] = runs, capped

			return err
		})
	}

	if err := group.Wait(); err != nil {
		return RunTimingSample{}, fmt.Errorf("listing runs by day: %w", err)
	}

	var sample RunTimingSample

	seen := make(map[int64]bool)

	for i, runs := range perDay {
		sample.Capped = append(sample.Capped, cappedPerDay[i]...)

		for j := range runs {
			if !seen[runs[j].ID] {
				seen[runs[j].ID] = true
				sample.Runs = append(sample.Runs, runs[j])
			}
		}
	}

	return sample, nil
}

func fetchRunRange(
	ctx context.Context, repoPath, repo string, filter RunTimingFilter, from, to time.Time,
) ([]RunTiming, []time.Time, error) {
	var runs []RunTiming

	for page := 1; ; page++ {
		batch, err := fetchRunPage(ctx, repoPath, repo, filter, from, to, page)
		if err != nil {
			return nil, nil, err
		}

		runs = append(runs, batch...)

		if len(batch) < maxRunsPerPage || len(runs) >= maxFilteredRuns {
			break
		}
	}

	if len(runs) < maxFilteredRuns {
		return runs, nil, nil
	}

	span := to.Sub(from)
	if span <= minSplitSpan {
		return runs, []time.Time{from}, nil
	}

	mid := from.Add(span / halves)

	first, firstCapped, err := fetchRunRange(ctx, repoPath, repo, filter, from, mid)
	if err != nil {
		return nil, nil, err
	}

	second, secondCapped, err := fetchRunRange(ctx, repoPath, repo, filter, mid, to)
	if err != nil {
		return nil, nil, err
	}

	return append(first, second...), append(firstCapped, secondCapped...), nil
}

func fetchRunPage(
	ctx context.Context, repoPath, repo string, filter RunTimingFilter, from, to time.Time, page int,
) ([]RunTiming, error) {
	params := url.Values{}
	params.Set("per_page", strconv.Itoa(maxRunsPerPage))
	params.Set("page", strconv.Itoa(page))

	for key, value := range map[string]string{"branch": filter.Branch, "event": filter.Event, "actor": filter.Actor} {
		if value != "" {
			params.Set(key, value)
		}
	}

	params.Set("created", from.UTC().Format(time.RFC3339)+".."+to.UTC().Format(time.RFC3339))

	out, err := runGH(ctx, repoPath, nil, "api", "repos/"+repo+"/actions/runs?"+params.Encode())
	if err != nil {
		return nil, fmt.Errorf("listing runs for %s: %w", repo, err)
	}

	var payload struct {
		WorkflowRuns []apiRunTiming `json:"workflow_runs"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parsing the run list: %w", err)
	}

	timings := make([]RunTiming, 0, len(payload.WorkflowRuns))

	for i := range payload.WorkflowRuns {
		r := &payload.WorkflowRuns[i]

		prs := make([]int, 0, len(r.PullRequests))
		for _, pr := range r.PullRequests {
			prs = append(prs, pr.Number)
		}

		path, _, _ := strings.Cut(r.Path, "@")
		timings = append(timings, RunTiming{
			CommittedAt: r.HeadCommit.Timestamp, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			Name: r.Name, Actor: r.Actor.Login, HeadBranch: r.HeadBranch, Path: path,
			Status: r.Status, Conclusion: r.Conclusion, HeadSHA: r.HeadSHA, PRs: prs,
			ID: r.ID, RunAttempt: r.RunAttempt,
		})
	}

	return timings, nil
}

// StepTiming is one step of a job, with the timestamps a breakdown needs.
type StepTiming struct {
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Name        string    `json:"name"`
	Conclusion  string    `json:"conclusion"`
	Number      int       `json:"number"`
}

// JobTiming is one job of a run. The wait for a runner is StartedAt minus
// CreatedAt. Unlike forge.Job it carries created_at and the runner labels.
type JobTiming struct {
	CreatedAt   time.Time    `json:"created_at"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt time.Time    `json:"completed_at"`
	Name        string       `json:"name"`
	Conclusion  string       `json:"conclusion"`
	Labels      []string     `json:"labels"`
	Steps       []StepTiming `json:"steps"`
	ID          int64        `json:"id"`
	RunID       int64        `json:"run_id"`
}

// RunJobTimings reads every job of a run with step timestamps and runner labels.
func RunJobTimings(ctx context.Context, repoPath, repo string, runID int64) ([]JobTiming, error) {
	var jobs []JobTiming

	for page := 1; ; page++ {
		path := fmt.Sprintf("repos/%s/actions/runs/%d/jobs?per_page=%d&page=%d", repo, runID, maxRunsPerPage, page)

		out, err := runGH(ctx, repoPath, nil, "api", path)
		if err != nil {
			return nil, fmt.Errorf("listing jobs of run %d: %w", runID, err)
		}

		var payload struct {
			Jobs []JobTiming `json:"jobs"`
		}

		if err := json.Unmarshal(out, &payload); err != nil {
			return nil, fmt.Errorf("parsing the jobs of run %d: %w", runID, err)
		}

		for i := range payload.Jobs {
			payload.Jobs[i].RunID = runID
		}

		jobs = append(jobs, payload.Jobs...)

		if len(payload.Jobs) < maxRunsPerPage {
			return jobs, nil
		}
	}
}
