package vcs

import (
	"context"
	"time"
)

// MockOperations is a test double implementing Operations via injectable function fields.
type MockOperations struct {
	GetRepoSummaryFn        func(ctx context.Context, repoPath string) (RepoSummary, error)
	GetCurrentBranchFn      func(ctx context.Context, repoPath string) (string, error)
	GetUpstreamFn           func(ctx context.Context, repoPath, branch string) (string, error)
	GetAheadBehindFn        func(ctx context.Context, repoPath, branch, upstream string) (int, int, error)
	CompareBranchesFn       func(ctx context.Context, repoPath, branch, target string) (int, int, error)
	GetBranchListFn         func(ctx context.Context, repoPath string) ([]BranchInfo, error)
	GetStashListFn          func(ctx context.Context, repoPath string) ([]StashDetail, error)
	GetNewestModifiedFileFn func(ctx context.Context, repoPath string) (string, time.Time, error)
	GetWorktreeListFn       func(ctx context.Context, repoPath string) ([]WorktreeInfo, error)
	GetCommitLogFn          func(ctx context.Context, repoPath string, count int) ([]CommitInfo, error)
	GetLastModifiedFn       func(ctx context.Context, repoPath string) (int64, error)
	GetRemoteURLFn          func(ctx context.Context, repoPath string) (string, error)
	VCSTypeFn               func() Type
	FetchAllFn              func(ctx context.Context, repoPath string) (bool, string, error)
	PruneRemoteFn           func(ctx context.Context, repoPath string) (bool, string, error)
	PushBranchFn            func(ctx context.Context, repoPath, branch string, setUpstream bool) (bool, string, error)
	SwitchBranchFn          func(ctx context.Context, repoPath, branch string) (bool, string, error)
	CleanupMergedBranchesFn func(ctx context.Context, repoPath string, squashMerged []string) (bool, string, error)
	DeleteBranchFn          func(ctx context.Context, repoPath, branch string, force bool) (bool, string, error)
	ApplyStashFn            func(ctx context.Context, repoPath string, index int) (bool, string, error)
	DropStashFn             func(ctx context.Context, repoPath string, index int) (bool, string, error)
}

// DeleteBranch implements Operations.
func (m *MockOperations) DeleteBranch(
	ctx context.Context, repoPath, branch string, force bool,
) (bool, string, error) {
	if m.DeleteBranchFn != nil {
		return m.DeleteBranchFn(ctx, repoPath, branch, force)
	}

	return true, "Deleted " + branch, nil
}

// ApplyStash implements Operations.
func (m *MockOperations) ApplyStash(ctx context.Context, repoPath string, index int) (bool, string, error) {
	if m.ApplyStashFn != nil {
		return m.ApplyStashFn(ctx, repoPath, index)
	}

	return true, "Applied", nil
}

// DropStash implements Operations.
func (m *MockOperations) DropStash(ctx context.Context, repoPath string, index int) (bool, string, error) {
	if m.DropStashFn != nil {
		return m.DropStashFn(ctx, repoPath, index)
	}

	return true, "Dropped", nil
}

// GetRepoSummary implements Operations.
func (m *MockOperations) GetRepoSummary(ctx context.Context, repoPath string) (RepoSummary, error) {
	if m.GetRepoSummaryFn != nil {
		return m.GetRepoSummaryFn(ctx, repoPath)
	}

	return RepoSummary{Path: repoPath}, nil
}

// GetCurrentBranch implements Operations.
func (m *MockOperations) GetCurrentBranch(ctx context.Context, repoPath string) (string, error) {
	if m.GetCurrentBranchFn != nil {
		return m.GetCurrentBranchFn(ctx, repoPath)
	}

	return "main", nil
}

// GetUpstream implements Operations.
func (m *MockOperations) GetUpstream(ctx context.Context, repoPath, branch string) (string, error) {
	if m.GetUpstreamFn != nil {
		return m.GetUpstreamFn(ctx, repoPath, branch)
	}

	return "", nil
}

// GetAheadBehind implements Operations.
func (m *MockOperations) GetAheadBehind(ctx context.Context, repoPath, branch, upstream string) (int, int, error) {
	if m.GetAheadBehindFn != nil {
		return m.GetAheadBehindFn(ctx, repoPath, branch, upstream)
	}

	return 0, 0, nil
}

// CompareBranches implements Operations.
func (m *MockOperations) CompareBranches(ctx context.Context, repoPath, branch, target string) (int, int, error) {
	if m.CompareBranchesFn != nil {
		return m.CompareBranchesFn(ctx, repoPath, branch, target)
	}

	return 0, 0, nil
}

// GetBranchList implements Operations.
func (m *MockOperations) GetBranchList(ctx context.Context, repoPath string) ([]BranchInfo, error) {
	if m.GetBranchListFn != nil {
		return m.GetBranchListFn(ctx, repoPath)
	}

	return nil, nil
}

// GetStashList implements Operations.
func (m *MockOperations) GetStashList(ctx context.Context, repoPath string) ([]StashDetail, error) {
	if m.GetStashListFn != nil {
		return m.GetStashListFn(ctx, repoPath)
	}

	return nil, nil
}

// GetNewestModifiedFile implements Operations.
func (m *MockOperations) GetNewestModifiedFile(ctx context.Context, repoPath string) (string, time.Time, error) {
	if m.GetNewestModifiedFileFn != nil {
		return m.GetNewestModifiedFileFn(ctx, repoPath)
	}

	return "", time.Time{}, nil
}

// GetWorktreeList implements Operations.
func (m *MockOperations) GetWorktreeList(ctx context.Context, repoPath string) ([]WorktreeInfo, error) {
	if m.GetWorktreeListFn != nil {
		return m.GetWorktreeListFn(ctx, repoPath)
	}

	return nil, nil
}

// GetCommitLog implements Operations.
func (m *MockOperations) GetCommitLog(ctx context.Context, repoPath string, count int) ([]CommitInfo, error) {
	if m.GetCommitLogFn != nil {
		return m.GetCommitLogFn(ctx, repoPath, count)
	}

	return nil, nil
}

// GetLastModified implements Operations.
func (m *MockOperations) GetLastModified(ctx context.Context, repoPath string) (int64, error) {
	if m.GetLastModifiedFn != nil {
		return m.GetLastModifiedFn(ctx, repoPath)
	}

	return 0, nil
}

// GetRemoteURL implements Operations.
func (m *MockOperations) GetRemoteURL(ctx context.Context, repoPath string) (string, error) {
	if m.GetRemoteURLFn != nil {
		return m.GetRemoteURLFn(ctx, repoPath)
	}

	return "", nil
}

// VCSType implements Operations.
func (m *MockOperations) VCSType() Type {
	if m.VCSTypeFn != nil {
		return m.VCSTypeFn()
	}

	return TypeGit
}

// FetchAll implements Operations.
func (m *MockOperations) FetchAll(ctx context.Context, repoPath string) (bool, string, error) {
	if m.FetchAllFn != nil {
		return m.FetchAllFn(ctx, repoPath)
	}

	return true, "Fetched", nil
}

// PruneRemote implements Operations.
func (m *MockOperations) PruneRemote(ctx context.Context, repoPath string) (bool, string, error) {
	if m.PruneRemoteFn != nil {
		return m.PruneRemoteFn(ctx, repoPath)
	}

	return true, "Pruned", nil
}

// PushBranch implements Operations.
func (m *MockOperations) PushBranch(
	ctx context.Context, repoPath, branch string, setUpstream bool,
) (bool, string, error) {
	if m.PushBranchFn != nil {
		return m.PushBranchFn(ctx, repoPath, branch, setUpstream)
	}

	return true, "Pushed", nil
}

// SwitchBranch implements Operations.
func (m *MockOperations) SwitchBranch(ctx context.Context, repoPath, branch string) (bool, string, error) {
	if m.SwitchBranchFn != nil {
		return m.SwitchBranchFn(ctx, repoPath, branch)
	}

	return true, "Switched", nil
}

// CleanupMergedBranches implements Operations.
func (m *MockOperations) CleanupMergedBranches(
	ctx context.Context, repoPath string, squashMerged []string,
) (bool, string, error) {
	if m.CleanupMergedBranchesFn != nil {
		return m.CleanupMergedBranchesFn(ctx, repoPath, squashMerged)
	}

	return true, "No branches to cleanup", nil
}

var _ Operations = (*MockOperations)(nil)
