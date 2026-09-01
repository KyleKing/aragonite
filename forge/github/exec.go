// Package github wraps the gh CLI to fetch pull request and workflow run data.
package github

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes one gh invocation. A consumer supplies its own to route every
// call in this package through machinery of its own: a recording seam, or a
// guard that refuses a mutating subcommand.
type Runner func(ctx context.Context, dir string, env []string, args ...string) ([]byte, error)

type ghRunner = Runner

type ghRunnerKey struct{}

// WithRunner returns a context that makes every gh call in this package go
// through fn instead of executing a subprocess directly. The runner travels on
// the context rather than in package state, so callers and tests can hold
// different ones concurrently.
func WithRunner(ctx context.Context, fn Runner) context.Context {
	return context.WithValue(ctx, ghRunnerKey{}, fn)
}

func withGHRunner(ctx context.Context, fn ghRunner) context.Context {
	return WithRunner(ctx, fn)
}

func runGH(ctx context.Context, dir string, env []string, args ...string) ([]byte, error) {
	if fn, ok := ctx.Value(ghRunnerKey{}).(ghRunner); ok {
		return fn(ctx, dir, env, args...)
	}

	cmd := exec.CommandContext(ctx, "gh", args...) // #nosec G204 -- fixed gh binary, args from internal tables
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running gh: %w", ghExecError(err))
	}

	return out, nil
}

// ghExecError surfaces gh's stderr, since (*exec.ExitError).Error() only
// reports the exit status and cmd.Output leaves the message on Stderr rather
// than in err itself.
func ghExecError(err error) error {
	var exitErr *exec.ExitError

	stderr := ""
	if errors.As(err, &exitErr) {
		stderr = strings.TrimSpace(string(exitErr.Stderr))
	}

	if stderr == "" {
		return err
	}

	return fmt.Errorf("%w: %s", err, stderr)
}
