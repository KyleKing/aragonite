package seen_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/aragonite/seen"
)

// Every workspace of one repository, and every directory inside one, must answer the same path, or
// jojo and jj-diff would keep marks for one repository in two directories.
func TestJJRepo_EveryWorkspaceAnswersTheMainStore(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	}

	main := filepath.Join(base, "main")
	store := filepath.Join(main, ".jj", "repo")
	mkdir(t, store)
	mkdir(t, filepath.Join(main, "src", "deep"))

	relative := filepath.Join(base, "second")
	mkdir(t, filepath.Join(relative, ".jj"))
	write(t, filepath.Join(relative, ".jj", "repo"), "../../main/.jj/repo\n")

	absolute := filepath.Join(base, "third")
	mkdir(t, filepath.Join(absolute, ".jj"))
	write(t, filepath.Join(absolute, ".jj", "repo"), store)

	for _, dir := range []string{main, filepath.Join(main, "src", "deep"), relative, absolute} {
		got, err := seen.JJRepo(dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}

		if got != store {
			t.Errorf("%s: answered %s, want %s", dir, got, store)
		}
	}
}

func TestJJRepo_Refuses(t *testing.T) {
	t.Parallel()

	empty := t.TempDir()
	mkdir(t, filepath.Join(empty, ".jj"))
	write(t, filepath.Join(empty, ".jj", "repo"), "  \n")

	if _, err := seen.JJRepo(empty); err == nil {
		t.Error("a workspace whose pointer names nothing answered a repository")
	}

	missing := t.TempDir()
	mkdir(t, filepath.Join(missing, ".jj"))

	if _, err := seen.JJRepo(missing); err == nil {
		t.Error("a workspace with no repo answered one")
	}

	if _, err := seen.JJRepo(string(filepath.Separator)); !errors.Is(err, seen.ErrNoJJ) {
		t.Errorf("outside any workspace: %v, want ErrNoJJ", err)
	}
}

func mkdir(t *testing.T, dir string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
