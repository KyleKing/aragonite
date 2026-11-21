package vcs_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/vcs"
)

// Which command answers is the whole of this: jj dropped git_head(), so a
// checkout read that reaches for it fails outright on a current jj, and a
// colocated repository has a .git that answers the same question.
func TestHeadSHA_AsksTheRepositoryItHas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		dirs []string
	}{
		{name: "git", dirs: []string{".git"}, want: "git rev-parse HEAD"},
		{name: "colocated jj", dirs: []string{".git", ".jj"}, want: "git rev-parse HEAD"},
		{
			name: "jj alone", dirs: []string{".jj"},
			want: "jj -R %s log --no-graph -r first_parent(@) -T commit_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for _, d := range tt.dirs {
				if err := os.Mkdir(filepath.Join(dir, d), 0o750); err != nil {
					t.Fatal(err)
				}
			}

			var got []string

			ctx := vcs.WithCommandRunner(t.Context(),
				func(_ context.Context, _, name string, args ...string) (string, error) {
					got = append([]string{name}, args...)

					return "deadbeef\n", nil
				})

			sha, err := vcs.HeadSHA(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}

			if sha != "deadbeef" {
				t.Errorf("HeadSHA = %q, want deadbeef", sha)
			}

			want := tt.want
			if strings.Contains(want, "%s") {
				want = fmt.Sprintf(want, dir)
			}

			if strings.Join(got, " ") != want {
				t.Errorf("ran %v, want %q", got, want)
			}
		})
	}
}

// A pull fetches, checks the two ancestry directions, and only then merges
// fast-forward. A rebase or force-push of the upstream leaves neither side a
// superset of the other, which it refuses to merge through rather than
// reporting a plain git failure. Autostash is never passed, since it would
// carry uncommitted work past a conflict. A jj checkout only fetches.
func TestPullFastForward_MergesOnlyAFastForward(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err    error
		runner func(*[]string) func(context.Context, string, string, ...string) (string, error)
		name   string
		want   string
		jj     bool
	}{
		{
			name:   "upstream reachable from HEAD",
			runner: fastForwardable,
			want:   "git fetch | git merge-base --is-ancestor @{u} HEAD | git merge --ff-only @{u}",
		},
		{
			name:   "HEAD behind the upstream",
			runner: behind,
			want: "git fetch | git merge-base --is-ancestor @{u} HEAD | " +
				"git merge-base --is-ancestor HEAD @{u} | git merge --ff-only @{u}",
		},
		{
			name:   "diverged refuses to merge",
			runner: diverged,
			want: "git fetch | git merge-base --is-ancestor @{u} HEAD | " +
				"git merge-base --is-ancestor HEAD @{u}",
			err: vcs.ErrDiverged,
		},
		{name: "jj only fetches", runner: fastForwardable, jj: true, want: "jj -R %s git fetch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tt.jj {
				if err := os.Mkdir(filepath.Join(dir, ".jj"), 0o750); err != nil {
					t.Fatal(err)
				}
			}

			var got []string

			err := vcs.PullFastForward(vcs.WithCommandRunner(t.Context(), tt.runner(&got)), dir)
			if !errors.Is(err, tt.err) || (tt.err == nil && err != nil) {
				t.Fatalf("PullFastForward() = %v, want %v", err, tt.err)
			}

			want := tt.want
			if strings.Contains(want, "%s") {
				want = fmt.Sprintf(want, dir)
			}

			if strings.Join(got, " | ") != want {
				t.Errorf("ran %q, want %q", strings.Join(got, " | "), want)
			}
		})
	}
}

func TestResetHardToUpstream_RunsAgainstUpstream(t *testing.T) {
	t.Parallel()

	var got []string

	ctx := vcs.WithCommandRunner(t.Context(), fastForwardable(&got))
	if err := vcs.ResetHardToUpstream(ctx, t.TempDir()); err != nil {
		t.Fatal(err)
	}

	want := "git reset --hard @{u}"
	if strings.Join(got, " | ") != want {
		t.Errorf("ran %q, want %q", strings.Join(got, " | "), want)
	}
}

// fastForwardable answers merge-base as if the upstream were reachable from
// HEAD, which is the ordinary case: nothing local would be lost by merging.
func fastForwardable(got *[]string) func(context.Context, string, string, ...string) (string, error) {
	return func(_ context.Context, _, name string, args ...string) (string, error) {
		*got = append(*got, strings.Join(append([]string{name}, args...), " "))

		return "", nil
	}
}

// errNotAnAncestor stands in for git merge-base --is-ancestor's exit 1,
// which reports a clean "no" rather than a command failure.
var errNotAnAncestor = errors.New("not an ancestor")

// diverged answers merge-base as if neither side were an ancestor of the
// other, which is what a rebase or force-push of the upstream leaves behind.
func diverged(got *[]string) func(context.Context, string, string, ...string) (string, error) {
	return func(_ context.Context, _, name string, args ...string) (string, error) {
		cmd := strings.Join(append([]string{name}, args...), " ")
		*got = append(*got, cmd)

		if strings.HasPrefix(cmd, "git merge-base ") {
			return "", errNotAnAncestor
		}

		return "", nil
	}
}

// behind answers as if HEAD were an ancestor of the upstream and not the
// other way round: the upstream moved on and HEAD can fast-forward to it.
func behind(got *[]string) func(context.Context, string, string, ...string) (string, error) {
	return func(_ context.Context, _, name string, args ...string) (string, error) {
		cmd := strings.Join(append([]string{name}, args...), " ")
		*got = append(*got, cmd)

		if cmd == "git merge-base --is-ancestor @{u} HEAD" {
			return "", errNotAnAncestor
		}

		return "", nil
	}
}
