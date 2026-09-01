package vcs

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// RemoteBranchLister names the branches a remote holds. A caller dispatching
// work against a ref needs those rather than the local checkout's, since a
// branch nobody checked out here is still a valid target.
type RemoteBranchLister interface {
	RemoteBranches(ctx context.Context, repoPath string) ([]string, error)
}

// DefaultBranchResolver names a repository's default branch.
type DefaultBranchResolver interface {
	ResolveDefaultBranch(ctx context.Context, repoPath string) (string, bool)
}

// RemoteBranches names the branches the remote holds, sorted and deduplicated,
// with the "origin/" prefix and origin/HEAD dropped.
func RemoteBranches(ctx context.Context, repoPath string) ([]string, error) {
	lister, ok := GetOperations(repoPath).(RemoteBranchLister)
	if !ok {
		return nil, nil
	}

	names, err := lister.RemoteBranches(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("listing remote branches: %w", err)
	}

	return names, nil
}

// DefaultBranchName names the repository's default branch, reporting whether
// one was found. Unlike DefaultBranchHead it answers for a repository whose
// remote advertises no HEAD, by probing for a conventional name.
func DefaultBranchName(ctx context.Context, repoPath string) (string, bool) {
	resolver, ok := GetOperations(repoPath).(DefaultBranchResolver)
	if !ok {
		return "", false
	}

	return resolver.ResolveDefaultBranch(ctx, repoPath)
}

// RemoteBranches implements RemoteBranchLister by reading the remote-tracking
// refs, which name every branch the remote had at the last fetch.
func (g *GitOperations) RemoteBranches(ctx context.Context, repoPath string) ([]string, error) {
	out, err := g.runGit(ctx, repoPath, "branch", "-r", "--list")
	if err != nil {
		return nil, err
	}

	return sortedRefNames(strings.Split(out, "\n")), nil
}

// RemoteBranches implements RemoteBranchLister. A bookmark listing already
// covers every remote, so the remote names come out of the same read the
// branch list uses.
func (j *JJOperations) RemoteBranches(ctx context.Context, repoPath string) ([]string, error) {
	branches, err := j.GetBranchList(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(branches))
	for i := range branches {
		names = append(names, branches[i].Name)
	}

	return sortedRefNames(names), nil
}

// ResolveDefaultBranch implements DefaultBranchResolver for jj by looking for a
// conventional name among the bookmarks, since a jj repository advertises no
// HEAD of its own.
func (j *JJOperations) ResolveDefaultBranch(ctx context.Context, repoPath string) (string, bool) {
	branches, err := j.GetBranchList(ctx, repoPath)
	if err != nil {
		return "", false
	}

	for i := range branches {
		if IsDefaultBranchName(branches[i].Name) {
			return branches[i].Name, true
		}
	}

	return "", false
}

// sortedRefNames cleans a listing of ref names: the "origin/" prefix and the
// symbolic origin/HEAD entry go, and what is left is sorted and deduplicated.
func sortedRefNames(lines []string) []string {
	seen := make(map[string]bool, len(lines))
	names := make([]string, 0, len(lines))

	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" || strings.HasPrefix(name, "origin/HEAD") {
			continue
		}

		name = strings.TrimPrefix(name, "origin/")
		if seen[name] {
			continue
		}

		seen[name] = true
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
