package vcs_test

import (
	"context"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/vcs"
)

func TestHeadSHA_ReadsGitRevParse(t *testing.T) {
	t.Parallel()

	var got []string

	ctx := vcs.WithCommandRunner(t.Context(),
		func(_ context.Context, _, name string, args ...string) (string, error) {
			got = append([]string{name}, args...)

			return "deadbeef\n", nil
		})

	sha, err := vcs.HeadSHA(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if sha != "deadbeef" {
		t.Errorf("HeadSHA = %q, want deadbeef", sha)
	}
	if strings.Join(got, " ") != "git rev-parse HEAD" {
		t.Errorf("ran %v", got)
	}
}

// A pull that carries uncommitted work past a conflict is the failure this
// guards, so the flag list is asserted rather than only the exit status.
func TestPullFastForward_NeverAutostashes(t *testing.T) {
	t.Parallel()

	var got []string

	ctx := vcs.WithCommandRunner(t.Context(),
		func(_ context.Context, _, name string, args ...string) (string, error) {
			got = append([]string{name}, args...)

			return "", nil
		})

	if err := vcs.PullFastForward(ctx, t.TempDir()); err != nil {
		t.Fatal(err)
	}

	if strings.Join(got, " ") != "git pull --ff-only" {
		t.Errorf("ran %v", got)
	}
}
