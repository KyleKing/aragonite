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

// PullFastForward advances the current branch to its upstream and changes
// nothing when it cannot.
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

	if _, err := runCommand(ctx, repoPath, "git", "pull", "--ff-only"); err != nil {
		return fmt.Errorf("git pull --ff-only: %w", withStderr(err))
	}

	return nil
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
