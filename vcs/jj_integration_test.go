package vcs_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kyleking/aragonite/vcs"
)

func jjInit(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj not installed")
	}

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()

		fullArgs := append([]string{
			"--config", "user.name=test",
			"--config", "user.email=test@example.com",
		}, args...)

		cmd := exec.CommandContext(t.Context(), "jj", fullArgs...) // #nosec G204 -- args are literals from this test
		cmd.Dir = dir

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("jj %v: %v\n%s", args, err, out)
		}
	}

	write := func(body string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(body), 0o600); err != nil {
			t.Fatalf("writing f.txt: %v", err)
		}
	}

	run("git", "init", "--colocate")
	write("one\ntwo\n")
	run("commit", "-m", "first")
	write("one\ntwo\nthree\nfour\n")
	run("commit", "-m", "second")

	return dir
}

func TestJJBlameAgainstRealJJ(t *testing.T) {
	t.Parallel()

	dir := jjInit(t)

	got, err := vcs.NewJJOperations().Blame(t.Context(), dir, "", "f.txt", []vcs.LineRange{{From: 1, To: 2}})
	if err != nil {
		t.Fatalf("Blame: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d blame lines, want 2: %+v", len(got), got)
	}

	for i, line := range got {
		if line.Author != "test" || line.Email != "test@example.com" || line.Line != i+1 {
			t.Errorf("line %d: unexpected %+v", i, line)
		}
	}

	// At the first commit the file held only its original two lines.
	older, err := vcs.NewJJOperations().Blame(t.Context(), dir, "@--", "f.txt", nil)
	if err != nil {
		t.Fatalf("Blame at @--: %v", err)
	}
	if len(older) != 2 {
		t.Fatalf("got %d blame lines at @--, want 2: %+v", len(older), older)
	}
}
