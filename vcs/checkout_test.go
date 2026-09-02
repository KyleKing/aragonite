package vcs_test

import (
	"context"
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
