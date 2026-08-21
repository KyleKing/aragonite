package vcs

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// HeadSHA returns the commit the working copy is checked out on. A jj repo
// reports its colocated git HEAD, because that is the commit a code host
// compares against.
func HeadSHA(ctx context.Context, repoPath string) (string, error) {
	if DetectVCSType(repoPath) == TypeJJ {
		out, err := runCommand(ctx, "", "jj", "-R", repoPath,
			"log", "--no-graph", "-r", "git_head()", "-T", "commit_id")
		if err != nil {
			return "", fmt.Errorf("resolving the jj git head: %w", err)
		}

		return strings.TrimSpace(out), nil
	}

	sha, err := runCommand(ctx, repoPath, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolving HEAD: %w", err)
	}

	return sha, nil
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
