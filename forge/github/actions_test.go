package github_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge/github"
)

// runsResponse builds an Actions run-list payload. Each entry is
// (id, workflow path, display name, minutes ago, status).
func runsResponse(entries ...[5]string) []byte {
	items := make([]string, 0, len(entries))

	for _, e := range entries {
		var minutes int
		if _, err := fmt.Sscanf(e[3], "%d", &minutes); err != nil {
			panic(err)
		}

		created := time.Now().Add(-time.Duration(minutes) * time.Minute).UTC().Format(time.RFC3339)
		items = append(items, fmt.Sprintf(
			`{"id":%s,"path":%q,"name":%q,"created_at":%q,"updated_at":%q,`+
				`"status":%q,"conclusion":"success","head_branch":"topic","event":"push"}`,
			e[0], e[1], e[2], created, created, e[4],
		))
	}

	return []byte(`{"workflow_runs":[` + strings.Join(items, ",") + `]}`)
}

func stubRuns(t *testing.T, payload []byte) (context.Context, *string) {
	t.Helper()

	var asked string

	ctx := github.WithRunner(t.Context(),
		func(_ context.Context, _ string, _ []string, a ...string) ([]byte, error) {
			asked = strings.Join(a, " ")

			return payload, nil
		})

	return ctx, &asked
}

func TestListRuns_AsksTheWorkflowEndpointAndTrimsThePath(t *testing.T) {
	t.Parallel()

	ctx, asked := stubRuns(t, runsResponse([5]string{"1", ".github/workflows/ci.yml", "CI", "5", "completed"}))

	runs, err := github.ListRuns(ctx, "/repo", "o/r", github.RunQuery{
		Workflow: "ci.yml", Branch: "topic", Status: "failure", Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(runs) != 1 || runs[0].Path != "ci.yml" {
		t.Fatalf("runs = %+v, want one run whose Path is the bare filename", runs)
	}

	if runs[0].HeadBranch != "topic" || runs[0].Event != "push" {
		t.Errorf("run lost its branch or event: %+v", runs[0])
	}

	// A named workflow must hit the per-workflow endpoint, since the runs
	// endpoint has no filter for it.
	for _, want := range []string{"actions/workflows/ci.yml/runs", "branch=topic", "status=failure", "per_page=5"} {
		if !strings.Contains(*asked, want) {
			t.Errorf("gh was asked %q, which is missing %q", *asked, want)
		}
	}
}

func TestLatestRunsOnBranch_KeepsOneCurrentStatePerWorkflowAndMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		want   []string
		within time.Duration
	}{
		{
			name:   "a re-run collapses to the newest, and a mode in the title stays separate",
			want:   []string{"3", "4", "5"},
			within: 0,
		},
		{
			name:   "a run older than the cutoff drops out unless it is still going",
			want:   []string{"3", "5"},
			within: time.Hour,
		},
	}

	// Runs 1 and 3 are the same workflow, so only 3 survives. Run 4 shares
	// deploy.yml with 5 but reports a different mode in its title. Run 5 is two
	// days old and still in progress.
	payload := runsResponse(
		[5]string{"1", ".github/workflows/ci.yml", "CI", "90", "completed"},
		[5]string{"3", ".github/workflows/ci.yml", "CI", "5", "completed"},
		[5]string{"4", ".github/workflows/deploy.yml", "Deploy (preview)", "120", "completed"},
		[5]string{"5", ".github/workflows/deploy.yml", "Deploy (up)", "2880", "in_progress"},
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, _ := stubRuns(t, payload)

			runs, err := github.LatestRunsOnBranch(ctx, "/repo", "o/r", "topic", tt.within)
			if err != nil {
				t.Fatal(err)
			}

			got := make([]string, 0, len(runs))
			for _, run := range runs {
				got = append(got, strconv.FormatInt(run.ID, 10))
			}

			if len(got) != len(tt.want) {
				t.Fatalf("kept runs %v, want %v", got, tt.want)
			}

			for _, want := range tt.want {
				if !strings.Contains(strings.Join(got, ","), want) {
					t.Errorf("kept runs %v, missing %s", got, want)
				}
			}
		})
	}
}
