package display_test

import (
	"testing"
	"time"

	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/vcs"
)

func TestRelativeTimeJustNow(t *testing.T) {
	t.Parallel()
	result := display.RelativeTime(time.Now())
	if result != "just now" {
		t.Errorf("expected 'just now', got '%s'", result)
	}
}

func TestRelativeTimeMinutes(t *testing.T) {
	t.Parallel()
	result := display.RelativeTime(time.Now().Add(-5 * time.Minute))
	if result != "5 mins ago" {
		t.Errorf("expected '5 mins ago', got '%s'", result)
	}

	result = display.RelativeTime(time.Now().Add(-1 * time.Minute))
	if result != "1 min ago" {
		t.Errorf("expected '1 min ago', got '%s'", result)
	}
}

func TestRelativeTimeHours(t *testing.T) {
	t.Parallel()
	result := display.RelativeTime(time.Now().Add(-3 * time.Hour))
	if result != "3 hours ago" {
		t.Errorf("expected '3 hours ago', got '%s'", result)
	}

	result = display.RelativeTime(time.Now().Add(-1 * time.Hour))
	if result != "1 hour ago" {
		t.Errorf("expected '1 hour ago', got '%s'", result)
	}
}

func TestRelativeTimeDays(t *testing.T) {
	t.Parallel()
	result := display.RelativeTime(time.Now().Add(-2 * 24 * time.Hour))
	if result != "2 days ago" {
		t.Errorf("expected '2 days ago', got '%s'", result)
	}

	result = display.RelativeTime(time.Now().Add(-1 * 24 * time.Hour))
	if result != "1 day ago" {
		t.Errorf("expected '1 day ago', got '%s'", result)
	}
}

func TestRelativeTimeWeeks(t *testing.T) {
	t.Parallel()
	result := display.RelativeTime(time.Now().Add(-14 * 24 * time.Hour))
	if result != "2 weeks ago" {
		t.Errorf("expected '2 weeks ago', got '%s'", result)
	}

	result = display.RelativeTime(time.Now().Add(-7 * 24 * time.Hour))
	if result != "1 week ago" {
		t.Errorf("expected '1 week ago', got '%s'", result)
	}
}

func TestRelativeTimeMonths(t *testing.T) {
	t.Parallel()
	result := display.RelativeTime(time.Now().Add(-60 * 24 * time.Hour))
	if result != "2 months ago" {
		t.Errorf("expected '2 months ago', got '%s'", result)
	}
}

func TestRelativeTimeYears(t *testing.T) {
	t.Parallel()
	result := display.RelativeTime(time.Now().Add(-730 * 24 * time.Hour))
	if result != "2 years ago" {
		t.Errorf("expected '2 years ago', got '%s'", result)
	}
}

func TestRelativeTimeZero(t *testing.T) {
	t.Parallel()
	result := display.RelativeTime(time.Time{})
	if result != display.EmDash {
		t.Errorf("expected '—', got '%s'", result)
	}
}

func TestBranchInfoRelativeLastCommit(t *testing.T) {
	t.Parallel()
	b := vcs.BranchInfo{}
	if display.BranchRelativeLastCommit(b) != display.EmDash {
		t.Errorf("expected '—' for zero time, got '%s'", display.BranchRelativeLastCommit(b))
	}

	b.LastCommit = time.Now()
	if display.BranchRelativeLastCommit(b) == display.EmDash {
		t.Error("expected non-empty relative time")
	}
}

func TestCommitInfoRelativeDate(t *testing.T) {
	t.Parallel()
	c := vcs.CommitInfo{Date: time.Now()}
	if display.CommitRelativeDate(c) == display.EmDash {
		t.Error("expected non-empty relative date")
	}
}

func TestStashDetailRelativeDate(t *testing.T) {
	t.Parallel()
	s := vcs.StashDetail{Date: time.Now()}
	if display.StashRelativeDate(s) == display.EmDash {
		t.Error("expected non-empty relative date")
	}
}

func TestRepoStatusSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		summary  vcs.RepoSummary
	}{
		{name: "clean", summary: vcs.RepoSummary{}, expected: "✓"},
		{name: "staged only", summary: vcs.RepoSummary{Staged: 2}, expected: "+2"},
		{name: "unstaged only", summary: vcs.RepoSummary{Unstaged: 3}, expected: "~3"},
		{name: "untracked only", summary: vcs.RepoSummary{Untracked: 1}, expected: "?1"},
		{name: "ahead only", summary: vcs.RepoSummary{Ahead: 5}, expected: "↑5"},
		{name: "behind only", summary: vcs.RepoSummary{Behind: 3}, expected: "↓3"},
		{name: "mixed", summary: vcs.RepoSummary{Staged: 1, Unstaged: 2, Ahead: 3}, expected: "+1 ~2 ↑3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := display.RepoStatusSummary(tt.summary); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestRepoRelativeModified(t *testing.T) {
	t.Parallel()

	s := vcs.RepoSummary{}
	if display.RepoRelativeModified(s) != display.EmDash {
		t.Errorf("expected the em dash for zero time, got %q", display.RepoRelativeModified(s))
	}

	s.LastModified = time.Now()
	if display.RepoRelativeModified(s) == display.EmDash {
		t.Error("expected non-empty relative time")
	}
}
