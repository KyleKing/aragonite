package vcs_test

import (
	"testing"

	"github.com/kyleking/aragonite/vcs"
)

// Every state a summary reports is derived from its counts: how many files
// are uncommitted, whether it is dirty and why, and where it stands against
// its upstream, with ahead-and-behind outranking a dirty tree.
func TestRepoSummary_DerivesItsStateFromItsCounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		label       string
		summary     vcs.RepoSummary
		status      vcs.RepoStatus
		uncommitted int
		dirty       bool
	}{
		{name: "clean", summary: vcs.RepoSummary{}, status: vcs.RepoStatusClean},
		{
			name:        "staged",
			summary:     vcs.RepoSummary{Staged: 2},
			uncommitted: 2,
			dirty:       true,
			label:       "uncommitted",
			status:      vcs.RepoStatusDirty,
		},
		{
			name:        "unstaged",
			summary:     vcs.RepoSummary{Unstaged: 1},
			uncommitted: 1,
			dirty:       true,
			label:       "uncommitted",
			status:      vcs.RepoStatusDirty,
		},
		{
			name:        "untracked",
			summary:     vcs.RepoSummary{Untracked: 1},
			uncommitted: 1,
			dirty:       true,
			label:       "uncommitted",
			status:      vcs.RepoStatusDirty,
		},
		{
			name:        "conflicted",
			summary:     vcs.RepoSummary{Conflicted: 1},
			uncommitted: 1,
			dirty:       true,
			label:       "uncommitted",
			status:      vcs.RepoStatusDirty,
		},
		{
			name: "every kind of file", summary: vcs.RepoSummary{Staged: 2, Unstaged: 3, Untracked: 1, Conflicted: 4},
			uncommitted: 10, dirty: true, label: "uncommitted", status: vcs.RepoStatusDirty,
		},
		{
			name:    "ahead",
			summary: vcs.RepoSummary{Ahead: 1},
			dirty:   true,
			label:   "unpushed",
			status:  vcs.RepoStatusAhead,
		},
		{name: "only behind is not dirty", summary: vcs.RepoSummary{Behind: 1}, status: vcs.RepoStatusBehind},
		{
			name:    "diverged",
			summary: vcs.RepoSummary{Ahead: 1, Behind: 1},
			dirty:   true,
			label:   "unpushed",
			status:  vcs.RepoStatusDiverged,
		},
		{
			name: "uncommitted and unpushed", summary: vcs.RepoSummary{Unstaged: 1, Ahead: 1},
			uncommitted: 1, dirty: true, label: "uncommitted, unpushed", status: vcs.RepoStatusAhead,
		},
		{
			name:        "behind outranks a dirty tree",
			summary:     vcs.RepoSummary{Unstaged: 1, Behind: 2},
			uncommitted: 1,
			dirty:       true,
			label:       "uncommitted",
			status:      vcs.RepoStatusBehind,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := tt.summary
			if got := s.UncommittedCount(); got != tt.uncommitted {
				t.Errorf("UncommittedCount() = %d, want %d", got, tt.uncommitted)
			}
			if got := s.IsDirty(); got != tt.dirty {
				t.Errorf("IsDirty() = %v, want %v", got, tt.dirty)
			}
			if got := s.DirtyLabel(); got != tt.label {
				t.Errorf("DirtyLabel() = %q, want %q", got, tt.label)
			}
			if got := s.Status(); got != tt.status {
				t.Errorf("Status() = %v, want %v", got, tt.status)
			}
		})
	}
}

func TestRepoSummary_NameLinkAndOverrides(t *testing.T) {
	t.Parallel()

	s := vcs.RepoSummary{Path: "/home/user/projects/my-repo", Branch: "main"}
	if s.Name() != "my-repo" || s.IsLinkedCheckout() || s.HasConfigOverrides() || s.IsDetached() {
		t.Errorf("a plain clone reads as name %q, linked %v, overrides %v, detached %v",
			s.Name(), s.IsLinkedCheckout(), s.HasConfigOverrides(), s.IsDetached())
	}

	s.ParentPath = "/home/user/projects/main-repo"
	s.ConfigOverrides = []vcs.GitConfigOverride{{Key: "user.email"}}
	s.Branch = vcs.DetachedBranchLabel("abc1234")

	if !s.IsLinkedCheckout() || !s.HasConfigOverrides() || !s.IsDetached() {
		t.Errorf("a linked, overridden, detached checkout reads as linked %v, overrides %v, detached %v",
			s.IsLinkedCheckout(), s.HasConfigOverrides(), s.IsDetached())
	}
}

func TestBranchNames_DetachedAndDefault(t *testing.T) {
	t.Parallel()

	if got := vcs.DetachedBranchLabel("abc1234"); got != "(abc1234)" {
		t.Errorf("DetachedBranchLabel = %q, want (abc1234)", got)
	}

	tests := []struct {
		label     string
		detached  bool
		isDefault bool
	}{
		{label: "(abc1234)", detached: true},
		{label: "(a)", detached: true},
		{label: "()"},
		{label: "("},
		{label: ""},
		{label: "(main"},
		{label: "main)"},
		{label: "main", isDefault: true},
		{label: "master", isDefault: true},
		{label: "trunk", isDefault: true},
		{label: "feature/x"},
	}

	for _, tt := range tests {
		if got := vcs.IsDetachedBranch(tt.label); got != tt.detached {
			t.Errorf("IsDetachedBranch(%q) = %v, want %v", tt.label, got, tt.detached)
		}
		if got := vcs.IsDefaultBranchName(tt.label); got != tt.isDefault {
			t.Errorf("IsDefaultBranchName(%q) = %v, want %v", tt.label, got, tt.isDefault)
		}
	}
}
