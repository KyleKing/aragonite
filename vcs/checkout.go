package vcs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HeadSHA returns the commit the working copy is checked out on, which is the
// commit a code host compares against.
//
// A colocated jj repository is read through its own .git rather than through a
// revset, because jj removed git_head() and a version that has dropped it
// answers "Function `git_head` doesn't exist", failing every read of the
// checkout. Git answers the same question with nothing left to rename, and it does not
// snapshot the working copy the way any jj command does. A jj repository with
// no .git has no commit a code host knows about, so the working copy's first
// parent stands in, which is what jj names as git_head()'s replacement.
func HeadSHA(ctx context.Context, repoPath string) (string, error) {
	if DetectVCSType(repoPath) == TypeJJ && !colocated(repoPath) {
		out, err := runCommand(ctx, "", "jj", "-R", repoPath,
			"log", "--no-graph", "-r", "first_parent(@)", "-T", "commit_id")
		if err != nil {
			return "", fmt.Errorf("resolving the jj working-copy parent: %w", err)
		}

		return out, nil
	}

	sha, err := runCommand(ctx, repoPath, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolving HEAD: %w", err)
	}

	return sha, nil
}

// colocated reports a jj repository that keeps a git repository beside it.
func colocated(repoPath string) bool {
	_, err := os.Stat(filepath.Join(repoPath, ".git"))

	return err == nil
}

// ErrDiverged reports that the working branch and its upstream share no
// fast-forward path in either direction, which is what a rebase or
// force-push of the upstream leaves behind. Neither side is a superset of
// the other, so no automatic pull is safe: only a caller that knows the
// local commits are disposable (its own prior checkout of the same branch,
// not independent work) can decide what happens next.
var ErrDiverged = errors.New("local branch has diverged from its upstream")

// PullFastForward advances the current branch to its upstream and changes
// nothing when it cannot outright fast-forward, reporting ErrDiverged rather
// than attempting a merge when the two histories have split.
//
// Never add --autostash. On git 2.x an --ff-only --autostash pull against a
// dirty file the pull also touches exits 0 while leaving UU conflict markers
// in the tree and the stash still on the stack, so the exit code says the
// working tree is fine when it is conflicted.
func PullFastForward(ctx context.Context, repoPath string) error {
	if DetectVCSType(repoPath) == TypeJJ {
		if _, err := runCommand(ctx, "", "jj", "-R", repoPath, "git", "fetch"); err != nil {
			return fmt.Errorf("jj git fetch: %w", withStderr(err))
		}

		return nil
	}

	if _, err := runCommand(ctx, repoPath, "git", "fetch"); err != nil {
		return fmt.Errorf("git fetch: %w", withStderr(err))
	}

	if hasDiverged(ctx, repoPath) {
		return ErrDiverged
	}

	if _, err := runCommand(ctx, repoPath, "git", "merge", "--ff-only", "@{u}"); err != nil {
		return fmt.Errorf("git merge --ff-only @{u}: %w", withStderr(err))
	}

	return nil
}

// ResetHardToUpstream discards the working branch's commits and points it at
// its upstream. The caller must have already established that discarding
// those commits loses nothing real (they came from second-look's own prior
// checkout, not from work done on top of it): this is not reversible in the
// working tree.
func ResetHardToUpstream(ctx context.Context, repoPath string) error {
	if _, err := runCommand(ctx, repoPath, "git", "reset", "--hard", "@{u}"); err != nil {
		return fmt.Errorf("git reset --hard @{u}: %w", withStderr(err))
	}

	return nil
}

// hasDiverged reports whether the working branch and its upstream share no
// fast-forward path either way. The caller has already fetched, so @{u}
// reflects the remote's current state.
func hasDiverged(ctx context.Context, repoPath string) bool {
	if isAncestor(ctx, repoPath, "@{u}", "HEAD") {
		return false // upstream is reachable from HEAD: already up to date, or HEAD is ahead
	}

	return !isAncestor(ctx, repoPath, "HEAD", "@{u}")
}

// isAncestor reports whether ancestor is reachable from descendant. Any
// failure of the underlying command, not just a confirmed "not an ancestor",
// answers false: a merge-base that cannot answer is not grounds to guess.
func isAncestor(ctx context.Context, repoPath, ancestor, descendant string) bool {
	_, err := runCommand(ctx, repoPath, "git", "merge-base", "--is-ancestor", ancestor, descendant)

	return err == nil
}

// withStderr surfaces the command's message, since (*exec.ExitError).Error()
// reports only the exit status and cmd.Output leaves the reason on Stderr.
func withStderr(err error) error {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}

	stderr := strings.TrimSpace(string(exitErr.Stderr))
	if stderr == "" {
		return err
	}

	return fmt.Errorf("%w: %s", err, stderr)
}
