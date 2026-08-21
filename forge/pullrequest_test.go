package forge_test

import (
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge"
)

func TestPRInfoStatusDisplay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pr       forge.PullRequest
		expected string
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
			result := tt.pr.StatusDisplay()
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
		pr       forge.PullRequest
		expected string
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
			expected: forge.EmDash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.pr.ReviewStatus()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPRInfoMatchesUpstream(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pr       forge.PullRequest
		owner    string
		upstream string
		expected bool
	}{
		{
			name:     "matches regardless of local branch name",
			pr:       forge.PullRequest{HeadRef: "feature-x"},
			owner:    "kyleking",
			upstream: "origin/feature-x",
			expected: true,
		},
		{
			name:     "different head ref does not match",
			pr:       forge.PullRequest{HeadRef: "feature-x"},
			owner:    "kyleking",
			upstream: "origin/feature-y",
			expected: false,
		},
		{
			name:     "fork pr never matches on upstream alone",
			pr:       forge.PullRequest{HeadRef: "feature-x", HeadRepoOwner: "someone-else"},
			owner:    "kyleking",
			upstream: "origin/feature-x",
			expected: false,
		},
		{
			name:     "no upstream never matches",
			pr:       forge.PullRequest{HeadRef: "feature-x"},
			owner:    "kyleking",
			upstream: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.pr.MatchesUpstream(tt.owner, tt.upstream); got != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestChecksStatusSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		checks   forge.ChecksStatus
		expected string
	}{
		{
			name:     "no checks",
			checks:   forge.ChecksStatus{Total: 0},
			expected: forge.EmDash,
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
			result := tt.checks.Summary()
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
		run      forge.WorkflowRun
		expected string
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
			result := tt.run.StatusDisplay()
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
		summary  forge.WorkflowSummary
		expected string
	}{
		{
			name:     "no runs",
			summary:  forge.WorkflowSummary{Total: 0},
			expected: forge.EmDash,
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
			result := tt.summary.StatusDisplay()
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
			if got := tt.check.StatusDisplay(); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestCheckDetailDuration(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	completed := forge.CheckDetail{StartedAt: start, CompletedAt: start.Add(90 * time.Second)}
	if got := completed.Duration(); got != "1m30s" {
		t.Errorf("expected '1m30s', got %q", got)
	}

	running := forge.CheckDetail{StartedAt: start}
	if got := running.Duration(); got != "—" {
		t.Errorf("expected an em dash while running, got %q", got)
	}
}
