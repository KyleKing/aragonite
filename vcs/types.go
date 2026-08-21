// Package vcs reads and mutates a checkout through git or jj behind one
// interface.
//
// It holds data and predicates only. Anything that renders a glyph, a
// placeholder, or a human-readable duration belongs to the consumer's display
// layer, because two tools reading the same checkout render it differently.
package vcs

import (
	"path/filepath"
	"strings"
	"time"
)

// Type identifies the version control system managing a repo.
type Type int

// Type values.
const (
	TypeGit Type = iota
	TypeJJ
)

func (v Type) String() string {
	switch v {
	case TypeGit:
		return "git"
	case TypeJJ:
		return "jj"
	default:
		return "unknown"
	}
}

// RepoStatus is a checkout's position relative to its upstream, or the state of
// its working tree when it has none.
type RepoStatus int

// RepoStatus values.
const (
	RepoStatusClean RepoStatus = iota
	RepoStatusDirty
	RepoStatusAhead
	RepoStatusBehind
	RepoStatusDiverged
)

func (r RepoStatus) String() string {
	switch r {
	case RepoStatusClean:
		return "clean"
	case RepoStatusDirty:
		return "dirty"
	case RepoStatusAhead:
		return "ahead"
	case RepoStatusBehind:
		return "behind"
	case RepoStatusDiverged:
		return "diverged"
	default:
		return "unknown"
	}
}

// DefaultBranchNames are the conventional primary branch names, assumed
// wherever a repo's real default branch has not been resolved from its remote.
//
//nolint:gochecknoglobals // a constant list Go cannot spell as const
var DefaultBranchNames = []string{"main", "master", "trunk"}

// DetachedBranchLabel formats a detached HEAD's short commit as the branch
// label shown wherever a branch name would go.
func DetachedBranchLabel(shortHash string) string {
	return "(" + shortHash + ")"
}

// IsDetachedBranch reports whether label came from DetachedBranchLabel, so
// views can say "detached" rather than treating the commit as a branch name.
func IsDetachedBranch(label string) bool {
	return len(label) > 2 && strings.HasPrefix(label, "(") && strings.HasSuffix(label, ")")
}

// GitConfigOverride is a local git config value that differs from the same
// key's global value.
type GitConfigOverride struct {
	Key         string
	LocalValue  string
	GlobalValue string
}

// RepoSummary is what a checkout says about itself: its branch, its position
// against its upstream, and the state of its working tree.
type RepoSummary struct {
	Path         string
	VCSType      Type
	Branch       string
	Upstream     string
	Ahead        int
	Behind       int
	Staged       int
	Unstaged     int
	Untracked    int
	Conflicted   int
	StashCount   int
	LastModified time.Time

	RemoteProtocol string // "ssh", "https", or "" if unknown/no remote
	RemoteRepo     string // "owner/repo" derived from the remote URL, "" if unknown/no remote
	// RemoteID is the cache identity of the remote, "host/owner/repo"
	// lowercased, and "" when there is no resolvable remote. Values read off
	// the remote are cached under it so parallel checkouts share one fetch.
	RemoteID        string
	ConfigOverrides []GitConfigOverride
	// ParentPath is the repo this one borrows its refs from, set only for a
	// git worktree or jj workspace. It makes a linked checkout recognizable
	// from anywhere, not just from the parent that listed it.
	ParentPath string
	// NoCommits marks a repo whose branch has no commits yet, which has no
	// upstream for a different reason than a branch that was never pushed.
	NoCommits bool
}

// IsDetached reports whether the repo's HEAD points at a commit rather than a
// branch.
func (r RepoSummary) IsDetached() bool {
	return IsDetachedBranch(r.Branch)
}

// DirtyLabel names why IsDirty is true, since uncommitted files and unpushed
// commits need different work to resolve. Empty when the repo is neither.
func (r RepoSummary) DirtyLabel() string {
	switch {
	case r.UncommittedCount() > 0 && r.Ahead > 0:
		return "uncommitted, unpushed"
	case r.UncommittedCount() > 0:
		return "uncommitted"
	case r.Ahead > 0:
		return "unpushed"
	default:
		return ""
	}
}

// IsLinkedCheckout reports whether the repo is a git worktree or jj workspace
// of another checkout rather than a standalone clone.
func (r RepoSummary) IsLinkedCheckout() bool {
	return r.ParentPath != ""
}

// HasConfigOverrides reports whether the repo has any local git config value
// that differs from the same key's global value.
func (r RepoSummary) HasConfigOverrides() bool {
	return len(r.ConfigOverrides) > 0
}

// Name returns the checkout's directory name.
func (r RepoSummary) Name() string {
	return filepath.Base(r.Path)
}

// UncommittedCount returns the total number of staged, unstaged, untracked, and
// conflicted files.
func (r RepoSummary) UncommittedCount() int {
	return r.Staged + r.Unstaged + r.Untracked + r.Conflicted
}

// IsDirty reports whether the repo has uncommitted changes or unpushed commits.
func (r RepoSummary) IsDirty() bool {
	return r.UncommittedCount() > 0 || r.Ahead > 0
}

// Status returns the repo's overall RepoStatus.
func (r RepoSummary) Status() RepoStatus {
	switch {
	case r.Ahead > 0 && r.Behind > 0:
		return RepoStatusDiverged
	case r.Ahead > 0:
		return RepoStatusAhead
	case r.Behind > 0:
		return RepoStatusBehind
	case r.UncommittedCount() > 0:
		return RepoStatusDirty
	default:
		return RepoStatusClean
	}
}

// BranchInfo summarizes a single branch's tracking state.
type BranchInfo struct {
	Name       string
	Upstream   string
	Ahead      int
	Behind     int
	LastCommit time.Time
	IsCurrent  bool
	IsRemote   bool
	// Head is the branch tip's commit OID (git) or change target commit id
	// (jj), used to detect squash-merged branches whose tip matches a merged
	// pull request's head OID even though the branch itself was never merged.
	Head string
}

// CommitInfo summarizes a single commit.
type CommitInfo struct {
	Hash      string
	ShortHash string
	Subject   string
	Author    string
	Date      time.Time
}

// StashDetail summarizes a single stash entry.
type StashDetail struct {
	Index   int
	Message string
	Branch  string
	Date    time.Time
}

// WorktreeInfo summarizes a single git worktree.
type WorktreeInfo struct {
	Path     string
	Branch   string
	IsBare   bool
	IsLocked bool
}
