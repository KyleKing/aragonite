package github_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge/github"
)

// stub routes every gh call through fn, which sees the gh arguments.
func stub(t *testing.T, fn func(args []string) ([]byte, error)) context.Context {
	t.Helper()

	return github.WithRunner(t.Context(),
		func(_ context.Context, _ string, _ []string, args ...string) ([]byte, error) { return fn(args) })
}

// timingRuns builds a workflow_runs page of n runs, each on its own commit.
func timingRuns(startID, n int) []byte {
	runs := make([]string, n)
	for i := range n {
		id := startID + i
		runs[i] = fmt.Sprintf(
			`{"id":%d,"head_sha":"sha%d","name":"CI","status":"completed","conclusion":"success",`+
				`"path":".github/workflows/ci.yml@refs/heads/main","run_attempt":2,"head_branch":"main",`+
				`"actor":{"login":"octocat"},"pull_requests":[{"number":7}],`+
				`"head_commit":{"timestamp":"2025-12-31T23:59:00Z"},`+
				`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:05:00Z"}`, id, id,
		)
	}

	return []byte(`{"workflow_runs":[` + strings.Join(runs, ",") + `]}`)
}

// pagedRuns answers run-list queries from pages keyed by "created range|page".
func pagedRuns(t *testing.T, pages map[string][]byte) context.Context {
	t.Helper()

	ctx := stub(t, func(args []string) ([]byte, error) {
		query, err := url.Parse(args[len(args)-1])
		if err != nil {
			return nil, fmt.Errorf("parsing %q: %w", args[len(args)-1], err)
		}

		key := query.Query().Get("created") + "|" + query.Query().Get("page")

		if body, ok := pages[key]; ok {
			return body, nil
		}

		return nil, fmt.Errorf("%w: unexpected query %s", errUnexpected, key)
	})

	return ctx
}

var errUnexpected = errors.New("unexpected")

// A rollup groups runs by commit, so head_sha, actor, attempt, and pull requests
// have to survive the trip, the "@ref" suffix leaves the workflow path, and a
// run on a day boundary is not counted twice. A full page asks for the next.
func TestListRunTimings_PagesEachDayAndKeepsWhatARollupGroupsBy(t *testing.T) {
	t.Parallel()

	day1 := "2026-01-01T00:00:00Z..2026-01-02T00:00:00Z"
	day2 := "2026-01-02T00:00:00Z..2026-01-03T00:00:00Z"

	ctx := pagedRuns(t, map[string][]byte{
		day1 + "|1": timingRuns(1, 100), day1 + "|2": timingRuns(101, 3), day2 + "|1": timingRuns(103, 2),
	})

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	filter := github.RunTimingFilter{Branch: "main"}

	got, err := github.ListRunTimings(ctx, "/repo", "o/r", filter, from, from.AddDate(0, 0, 2))
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Runs) != 104 {
		t.Fatalf("got %d runs, want 104 (run 103 is on both days)", len(got.Runs))
	}

	run := got.Runs[0]
	if run.HeadSHA != "sha1" || run.Actor != "octocat" || run.RunAttempt != 2 ||
		run.Path != ".github/workflows/ci.yml" || len(run.PRs) != 1 || run.PRs[0] != 7 {
		t.Errorf("run = %+v, want sha, actor, attempt, a trimmed path, and pull request 7", run)
	}
}

// A day that returns the API's 1000-result cap is halved until each half fits.
func TestListRunTimings_SplitsARangeThatHitsTheResultCap(t *testing.T) {
	t.Parallel()

	const (
		full = "2026-01-01T00:00:00Z..2026-01-02T00:00:00Z"
		low  = "2026-01-01T00:00:00Z..2026-01-01T12:00:00Z"
		high = "2026-01-01T12:00:00Z..2026-01-02T00:00:00Z"
	)

	pages := map[string][]byte{}
	add := func(created string, firstID, total int) {
		for page := 1; page <= total/100+1; page++ {
			pages[fmt.Sprintf("%s|%d", created, page)] = timingRuns(firstID+(page-1)*100, min(100, total-(page-1)*100))
		}
	}

	add(full, 1, 1000)
	add(low, 1, 400)
	add(high, 1001, 300)

	ctx := pagedRuns(t, pages)
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	got, err := github.ListRunTimings(ctx, "", "o/r", github.RunTimingFilter{}, from, from.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Runs) != 700 || len(got.Capped) != 0 {
		t.Fatalf("got %d runs and %d capped ranges, want 700 and none", len(got.Runs), len(got.Capped))
	}
}

func TestRunJobTimings_KeepsRunnerLabelsAndStepTimes(t *testing.T) {
	t.Parallel()

	ctx := stub(t, func(args []string) ([]byte, error) {
		if !strings.Contains(args[len(args)-1], "runs/9/jobs") {
			return nil, fmt.Errorf("%w: %v", errUnexpected, args)
		}

		return []byte(`{"jobs":[{"id":1,"name":"test","conclusion":"success","labels":["ubicloud"],` +
			`"created_at":"2026-01-01T00:00:00Z","started_at":"2026-01-01T00:00:20Z",` +
			`"completed_at":"2026-01-01T00:02:00Z",` +
			`"steps":[{"name":"Run tests","number":3,"conclusion":"success",` +
			`"started_at":"2026-01-01T00:00:30Z","completed_at":"2026-01-01T00:01:30Z"}]}]}`), nil
	})

	jobs, err := github.RunJobTimings(ctx, "", "o/r", 9)
	if err != nil {
		t.Fatal(err)
	}

	if len(jobs) != 1 {
		t.Fatalf("jobs = %+v, want one", jobs)
	}

	job, step := jobs[0], jobs[0].Steps[0]
	if job.RunID != 9 || job.Labels[0] != "ubicloud" || job.StartedAt.Sub(job.CreatedAt) != 20*time.Second ||
		step.CompletedAt.Sub(step.StartedAt) != time.Minute {
		t.Errorf("job = %+v, want the run ID, runner label, queue time, and step time", job)
	}
}
