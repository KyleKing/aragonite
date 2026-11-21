package display_test

import (
	"testing"
	"time"

	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/forge"
)

func TestPRInfoStatusDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected string
		pr       forge.PullRequest
	}{
		{
			name:     "draft pr",
			pr:       forge.PullRequest{IsDraft: true, State: "OPEN"},
			expected: "DRAFT",
		},
		{
			name:     "open pr",
			pr:       forge.PullRequest{State: "OPEN"},
			expected: "OPEN",
		},
		{
			name:     "merged pr",
			pr:       forge.PullRequest{State: "MERGED"},
			expected: "MERGED",
		},
		{
			name:     "closed pr",
			pr:       forge.PullRequest{State: "CLOSED"},
			expected: "CLOSED",
		},
		{
			name:     "unknown state passed through",
			pr:       forge.PullRequest{State: "UNKNOWN"},
			expected: "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := display.PRStatusDisplay(tt.pr)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPRInfoReviewStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected string
		pr       forge.PullRequest
	}{
		{
			name:     "approved via decision",
			pr:       forge.PullRequest{ReviewDecision: "APPROVED"},
			expected: "approved",
		},
		{
			name:     "changes requested",
			pr:       forge.PullRequest{ReviewDecision: "CHANGES_REQUESTED"},
			expected: "changes requested",
		},
		{
			name:     "review required",
			pr:       forge.PullRequest{ReviewDecision: "REVIEW_REQUIRED"},
			expected: "review required",
		},
		{
			name:     "approved via approvers list",
			pr:       forge.PullRequest{ApprovedBy: []string{"user1"}},
			expected: "approved",
		},
		{
			name:     "no review info",
			pr:       forge.PullRequest{},
			expected: display.EmDash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := display.PRReviewStatus(tt.pr)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestChecksStatusSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected string
		checks   forge.ChecksStatus
	}{
		{
			name:     "no checks",
			checks:   forge.ChecksStatus{Total: 0},
			expected: display.EmDash,
		},
		{
			name:     "all passing",
			checks:   forge.ChecksStatus{Total: 3, Passing: 3},
			expected: "passing",
		},
		{
			name:     "has failures",
			checks:   forge.ChecksStatus{Total: 3, Passing: 2, Failing: 1},
			expected: "failing",
		},
		{
			name:     "has pending",
			checks:   forge.ChecksStatus{Total: 3, Passing: 2, Pending: 1},
			expected: "pending",
		},
		{
			name:     "mixed (skipped)",
			checks:   forge.ChecksStatus{Total: 3, Passing: 2, Skipped: 1},
			expected: "mixed",
		},
		{
			name:     "failing takes priority over pending",
			checks:   forge.ChecksStatus{Total: 3, Failing: 1, Pending: 2},
			expected: "failing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := display.ChecksSummary(tt.checks)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestWorkflowRunStatusDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected string
		run      forge.WorkflowRun
	}{
		{
			name:     "completed shows conclusion",
			run:      forge.WorkflowRun{Status: "completed", Conclusion: "success"},
			expected: "success",
		},
		{
			name:     "in progress shows status",
			run:      forge.WorkflowRun{Status: "in_progress"},
			expected: "in_progress",
		},
		{
			name:     "queued shows status",
			run:      forge.WorkflowRun{Status: "queued"},
			expected: "queued",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := display.WorkflowRunStatusDisplay(tt.run)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestWorkflowSummaryStatusDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		expected string
		summary  forge.WorkflowSummary
	}{
		{
			name:     "no runs",
			summary:  forge.WorkflowSummary{Total: 0},
			expected: display.EmDash,
		},
		{
			name:     "all passing",
			summary:  forge.WorkflowSummary{Total: 2, Passing: 2},
			expected: "passing",
		},
		{
			name:     "has failures",
			summary:  forge.WorkflowSummary{Total: 2, Passing: 1, Failing: 1},
			expected: "failing",
		},
		{
			name:     "in progress",
			summary:  forge.WorkflowSummary{Total: 2, Passing: 1, InProgress: 1},
			expected: "running",
		},
		{
			name:     "mixed",
			summary:  forge.WorkflowSummary{Total: 3, Passing: 2},
			expected: "mixed",
		},
		{
			name:     "failing takes priority",
			summary:  forge.WorkflowSummary{Total: 3, Failing: 1, InProgress: 2},
			expected: "failing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := display.WorkflowSummaryStatusDisplay(tt.summary)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCheckDetailStatusDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		check    forge.CheckDetail
		expected string
	}{
		{"completed uses the conclusion", forge.CheckDetail{Status: "COMPLETED", Conclusion: "SUCCESS"}, "success"},
		{"in flight uses the status", forge.CheckDetail{Status: "IN_PROGRESS"}, "in progress"},
		{"queued", forge.CheckDetail{Status: "QUEUED"}, "queued"},
		{"unknown", forge.CheckDetail{}, "—"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := display.CheckStatusDisplay(tt.check); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestCheckDuration_RendersElapsedOrEmDash(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name  string
		want  string
		check forge.CheckDetail
	}{
		{
			name:  "completed",
			check: forge.CheckDetail{StartedAt: start, CompletedAt: start.Add(90 * time.Second)},
			want:  "1m30s",
		},
		{
			name:  "rounds to the second",
			check: forge.CheckDetail{StartedAt: start, CompletedAt: start.Add(1600 * time.Millisecond)},
			want:  "2s",
		},
		{name: "instant", check: forge.CheckDetail{StartedAt: start, CompletedAt: start}, want: "0s"},
		{name: "running", check: forge.CheckDetail{StartedAt: start}, want: display.EmDash},
		{name: "never started", check: forge.CheckDetail{CompletedAt: start}, want: display.EmDash},
		{
			name:  "clock skew",
			check: forge.CheckDetail{StartedAt: start, CompletedAt: start.Add(-time.Minute)},
			want:  display.EmDash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := display.CheckDuration(tt.check); got != tt.want {
				t.Errorf("CheckDuration = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPRActivityAndGlyph(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		activity string
		glyph    string
		pr       forge.PullRequest
	}{
		{name: "no activity", pr: forge.PullRequest{}, activity: display.EmDash, glyph: ""},
		{
			name:     "zero activity time",
			pr:       forge.PullRequest{Activity: &forge.PRActivity{Author: "a"}, ReviewDecision: "REVIEW_REQUIRED"},
			activity: display.EmDash,
			glyph:    "",
		},
		{
			name: "approved with recent comment",
			pr: forge.PullRequest{
				Activity:       &forge.PRActivity{At: time.Now().Add(-2 * time.Hour), Author: "kyle"},
				ReviewDecision: "APPROVED",
			},
			activity: "2 hours ago kyle",
			glyph:    "✓",
		},
		{
			name:     "changes requested",
			pr:       forge.PullRequest{ReviewDecision: "CHANGES_REQUESTED"},
			activity: display.EmDash,
			glyph:    "✗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := display.PRActivitySummary(&tt.pr); got != tt.activity {
				t.Errorf("PRActivitySummary = %q, want %q", got, tt.activity)
			}
			if got := display.PRReviewGlyph(&tt.pr); got != tt.glyph {
				t.Errorf("PRReviewGlyph = %q, want %q", got, tt.glyph)
			}
		})
	}
}

func TestDefaultBranchCIConclusion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		ci   forge.DefaultBranchCI
	}{
		{name: "no runs", ci: forge.DefaultBranchCI{}, want: "—"},
		{
			name: "every workflow green",
			ci: forge.DefaultBranchCI{Workflows: []forge.CIWorkflowRun{
				{Status: "completed", Conclusion: "success"},
				{Status: "completed", Conclusion: "success"},
			}},
			want: forge.StatusPassing,
		},
		{
			name: "one red outweighs the rest",
			ci: forge.DefaultBranchCI{Workflows: []forge.CIWorkflowRun{
				{Status: "completed", Conclusion: "success"},
				{Status: "completed", Conclusion: "failure"},
			}},
			want: forge.StatusFailing,
		},
		{
			name: "still running",
			ci: forge.DefaultBranchCI{Workflows: []forge.CIWorkflowRun{
				{Status: "completed", Conclusion: "success"},
				{Status: "in_progress"},
			}},
			want: "pending",
		},
		{
			name: "a red run wins even while another is running",
			ci: forge.DefaultBranchCI{Workflows: []forge.CIWorkflowRun{
				{Status: "in_progress"},
				{Status: "completed", Conclusion: "failure"},
			}},
			want: forge.StatusFailing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := display.CIConclusion(&tt.ci); got != tt.want {
				t.Errorf("Conclusion = %q, want %q", got, tt.want)
			}
		})
	}
}
