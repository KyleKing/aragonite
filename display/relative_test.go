package display_test

import (
	"testing"
	"time"

	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"
)

const day = 24 * time.Hour

func TestRelativeTime_NamesTheLargestWholeUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		ago  time.Duration
	}{
		{name: "now", ago: 0, want: "just now"},
		{name: "under a minute", ago: 59 * time.Second, want: "just now"},
		{name: "one minute", ago: time.Minute, want: "1 min ago"},
		{name: "minutes", ago: 5 * time.Minute, want: "5 mins ago"},
		{name: "last minute of the hour", ago: 59 * time.Minute, want: "59 mins ago"},
		{name: "one hour", ago: time.Hour, want: "1 hour ago"},
		{name: "hours", ago: 3 * time.Hour, want: "3 hours ago"},
		{name: "last hour of the day", ago: 23 * time.Hour, want: "23 hours ago"},
		{name: "one day", ago: day, want: "1 day ago"},
		{name: "days", ago: 2 * day, want: "2 days ago"},
		{name: "last day of the week", ago: 6 * day, want: "6 days ago"},
		{name: "one week", ago: 7 * day, want: "1 week ago"},
		{name: "weeks", ago: 14 * day, want: "2 weeks ago"},
		{name: "last day of the month", ago: 29 * day, want: "4 weeks ago"},
		{name: "one month", ago: 30 * day, want: "1 month ago"},
		{name: "months", ago: 60 * day, want: "2 months ago"},
		{name: "last day of the year", ago: 364 * day, want: "12 months ago"},
		{name: "one year", ago: 365 * day, want: "1 year ago"},
		{name: "years", ago: 730 * day, want: "2 years ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := display.RelativeTime(time.Now().Add(-tt.ago)); got != tt.want {
				t.Errorf("RelativeTime(now-%v) = %q, want %q", tt.ago, got, tt.want)
			}
		})
	}
}

// A table's time column is a handful of cells wide, so the compact form has to
// stay short at every scale and say something for a time the model lacks.
func TestRelativeTimeCompact_StaysShortAtEveryScale(t *testing.T) {
	t.Parallel()

	old := time.Now().AddDate(0, -2, 0)
	week := time.Now().Add(-7*day - time.Minute)
	tests := []struct {
		name string
		when time.Time
		want string
	}{
		{name: "zero", when: time.Time{}, want: display.EmDash},
		{name: "seconds", when: time.Now().Add(-30 * time.Second), want: "just now"},
		{name: "one minute", when: time.Now().Add(-time.Minute), want: "1m ago"},
		{name: "minutes", when: time.Now().Add(-21 * time.Minute), want: "21m ago"},
		{name: "one hour", when: time.Now().Add(-time.Hour), want: "1h ago"},
		{name: "hours", when: time.Now().Add(-5 * time.Hour), want: "5h ago"},
		{name: "one day", when: time.Now().Add(-day), want: "1d ago"},
		{name: "days", when: time.Now().Add(-3 * day), want: "3d ago"},
		{name: "a week falls back to the date", when: week, want: week.Format("Jan 2")},
		{name: "months", when: old, want: old.Format("Jan 2")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := display.RelativeTimeCompact(tt.when); got != tt.want {
				t.Errorf("RelativeTimeCompact(%v) = %q, want %q", tt.when, got, tt.want)
			}
		})
	}
}

// Each model's accessor reads its own time field and renders it the way
// RelativeTime does, with the em dash for a time the model lacks.
func TestModelRelativeTimes_ReadTheirOwnField(t *testing.T) {
	t.Parallel()

	threeHours := time.Now().Add(-3 * time.Hour)
	tests := []struct {
		got  func(time.Time) string
		name string
	}{
		{name: "branch", got: func(at time.Time) string {
			return display.BranchRelativeLastCommit(vcs.BranchInfo{LastCommit: at})
		}},
		{
			name: "commit",
			got:  func(at time.Time) string { return display.CommitRelativeDate(vcs.CommitInfo{Date: at}) },
		},
		{name: "stash", got: func(at time.Time) string { return display.StashRelativeDate(vcs.StashDetail{Date: at}) }},
		{name: "repo", got: func(at time.Time) string {
			return display.RepoRelativeModified(vcs.RepoSummary{LastModified: at})
		}},
		{name: "comment", got: func(at time.Time) string {
			return display.CommentRelativeCreated(forge.PRComment{CreatedAt: at})
		}},
		{name: "pr created", got: func(at time.Time) string {
			return display.PRDetailRelativeCreated(forge.PRDetail{CreatedAt: at})
		}},
		{name: "pr updated", got: func(at time.Time) string {
			return display.PRDetailRelativeUpdated(forge.PRDetail{UpdatedAt: at})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.got(threeHours); got != "3 hours ago" {
				t.Errorf("three hours ago reads %q, want %q", got, "3 hours ago")
			}
			if got := tt.got(time.Time{}); got != display.EmDash {
				t.Errorf("a zero time reads %q, want the em dash", got)
			}
		})
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
		{name: "conflicted only", summary: vcs.RepoSummary{Conflicted: 4}, expected: "!4"},
		{name: "ahead only", summary: vcs.RepoSummary{Ahead: 5}, expected: "↑5"},
		{name: "behind only", summary: vcs.RepoSummary{Behind: 3}, expected: "↓3"},
		{name: "mixed", summary: vcs.RepoSummary{Staged: 1, Unstaged: 2, Ahead: 3}, expected: "+1 ~2 ↑3"},
		{
			name: "every field in a fixed order",
			summary: vcs.RepoSummary{
				Staged: 1, Unstaged: 2, Untracked: 3, Conflicted: 4, Ahead: 5, Behind: 6,
			},
			expected: "+1 ~2 ?3 !4 ↑5 ↓6",
		},
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
