package vcs_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kyleking/aragonite/vcs"
)

// A dispatch target is any branch the remote holds, including one this
// checkout never created, so the listing comes from the remote-tracking refs
// rather than from refs/heads.
func TestRemoteBranchesAgainstRealGit(t *testing.T) {
	t.Parallel()

	origin := gitInit(t)
	gitRun(t, origin, "branch", "topic")
	gitRun(t, origin, "branch", "release/1.0")

	clone := filepath.Join(t.TempDir(), "clone")
	gitRun(t, filepath.Dir(clone), "clone", origin, clone)

	branches, err := vcs.RemoteBranches(t.Context(), clone)
	if err != nil {
		t.Fatalf("listing remote branches: %v", err)
	}

	want := []string{"main", "release/1.0", "topic"}
	if !slices.Equal(branches, want) {
		t.Errorf("remote branches are %v, want %v", branches, want)
	}

	name, ok := vcs.DefaultBranchName(t.Context(), clone)
	if !ok || name != "main" {
		t.Errorf("default branch is %q (%t), want main", name, ok)
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...) // #nosec G204 -- args are literals from this test
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
