package vcs_test

import (
	"testing"

	"github.com/kyleking/aragonite/vcs"
)

func TestRepoSummaryName(t *testing.T) {
	t.Parallel()

	s := vcs.RepoSummary{Path: "/home/user/projects/my-repo"}
	if s.Name() != "my-repo" {
		t.Errorf("expected 'my-repo', got '%s'", s.Name())
	}
}

func TestRepoSummaryUncommittedCount(t *testing.T) {
	t.Parallel()

	s := vcs.RepoSummary{Staged: 2, Unstaged: 3, Untracked: 1, Conflicted: 0}
	if s.UncommittedCount() != 6 {
		t.Errorf("expected 6, got %d", s.UncommittedCount())
	}
}

func TestRepoSummaryIsDirty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		summary  vcs.RepoSummary
		expected bool
	}{
		{name: "clean repo", summary: vcs.RepoSummary{}, expected: false},
		{name: "has staged", summary: vcs.RepoSummary{Staged: 1}, expected: true},
		{name: "has unstaged", summary: vcs.RepoSummary{Unstaged: 1}, expected: true},
		{name: "has untracked", summary: vcs.RepoSummary{Untracked: 1}, expected: true},
		{name: "has ahead", summary: vcs.RepoSummary{Ahead: 1}, expected: true},
		{name: "only behind is not dirty", summary: vcs.RepoSummary{Behind: 1}, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.summary.IsDirty() != tt.expected {
				t.Errorf("expected IsDirty() = %v, got %v", tt.expected, tt.summary.IsDirty())
			}
		})
	}
}

func TestRepoSummaryStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		summary  vcs.RepoSummary
		expected vcs.RepoStatus
	}{
		{name: "clean", summary: vcs.RepoSummary{}, expected: vcs.RepoStatusClean},
		{name: "dirty", summary: vcs.RepoSummary{Unstaged: 1}, expected: vcs.RepoStatusDirty},
		{name: "ahead", summary: vcs.RepoSummary{Ahead: 1}, expected: vcs.RepoStatusAhead},
		{name: "behind", summary: vcs.RepoSummary{Behind: 1}, expected: vcs.RepoStatusBehind},
		{name: "diverged", summary: vcs.RepoSummary{Ahead: 1, Behind: 1}, expected: vcs.RepoStatusDiverged},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.summary.Status() != tt.expected {
				t.Errorf("expected Status() = %v, got %v", tt.expected, tt.summary.Status())
			}
		})
	}
}

func TestRepoSummaryDirtyLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		summary  vcs.RepoSummary
	}{
		{name: "clean", summary: vcs.RepoSummary{}, expected: ""},
		{name: "uncommitted", summary: vcs.RepoSummary{Unstaged: 1}, expected: "uncommitted"},
		{name: "unpushed", summary: vcs.RepoSummary{Ahead: 1}, expected: "unpushed"},
		{name: "both", summary: vcs.RepoSummary{Unstaged: 1, Ahead: 1}, expected: "uncommitted, unpushed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.summary.DirtyLabel(); got != tt.expected {
				t.Errorf("expected DirtyLabel() = %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestDetachedBranchLabelRoundTrips(t *testing.T) {
	t.Parallel()

	label := vcs.DetachedBranchLabel("abc1234")
	if !vcs.IsDetachedBranch(label) {
		t.Errorf("%q should read as detached", label)
	}
	if vcs.IsDetachedBranch("main") {
		t.Error("a branch name should not read as detached")
	}
}

func TestIsDefaultBranchName(t *testing.T) {
	t.Parallel()

	for _, name := range vcs.DefaultBranchNames {
		if !vcs.IsDefaultBranchName(name) {
			t.Errorf("%q should be a default branch name", name)
		}
	}
	if vcs.IsDefaultBranchName("feature/x") {
		t.Error("a feature branch should not be a default branch name")
	}
}
