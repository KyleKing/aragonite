package seen_test

import (
	"path/filepath"
	"testing"

	"github.com/kyleking/aragonite/diff"
	"github.com/kyleking/aragonite/seen"
)

// A review over several changes reads each change's marks, and a hunk jojo marked under its
// change reads as read in second-look's own set without being written into that set's file.
func TestUnion_LaysEveryChangesMarksOverAReviewsOwn(t *testing.T) {
	t.Parallel()

	live := seen.Hunks(diff.Parse([]byte(patch)))
	if len(live) < 3 {
		t.Fatalf("the fixture has %d hunks, want 3 or more", len(live))
	}

	jj := seen.Open(t.TempDir())
	if err := jj.Mark("krpztump", true, live, live[0].ID); err != nil {
		t.Fatal(err)
	}

	if err := jj.Mark("mytstwqm", true, live, live[1].ID); err != nil {
		t.Fatal(err)
	}

	union, err := jj.Union("krpztump", "mytstwqm", "krpztump", "never-marked")
	if err != nil {
		t.Fatal(err)
	}

	if got := union.Count(live); got != 2 {
		t.Fatalf("the union read %d hunks, want 2", got)
	}

	own := filepath.Join(t.TempDir(), "local.toml")

	read, err := seen.Load(own)
	if err != nil {
		t.Fatal(err)
	}

	read.Mark(false, live[1].ID) // an unmark this process made before the union arrived
	read.Mark(true, live[2].ID)
	read.Include(union)

	if !read.Has(live[0].ID) || read.Has(live[1].ID) || !read.Has(live[2].ID) {
		t.Errorf("Include read %v %v %v, want true false true (a pending unmark wins)",
			read.Has(live[0].ID), read.Has(live[1].ID), read.Has(live[2].ID))
	}

	if err := seen.Save(own, read, live); err != nil {
		t.Fatal(err)
	}

	back, err := seen.Load(own)
	if err != nil {
		t.Fatal(err)
	}

	if back.Has(live[0].ID) || !back.Has(live[2].ID) {
		t.Errorf("the review's own file holds jojo's mark %v or lost its own %v", back.Has(live[0].ID), back.Has(live[2].ID))
	}

	union.Mark(true, live[2].ID)

	if s, _ := jj.Set("krpztump"); s.Has(live[2].ID) {
		t.Error("marking through the union reached a scope")
	}
}

// OpenJJ is the directory jojo and jj-diff keep a repository's marks in, from any workspace.
func TestOpenJJ_EveryWorkspaceOpensOneDirectory(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	base := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	}

	main := filepath.Join(base, "main")
	mkdir(t, filepath.Join(main, ".jj", "repo"))
	mkdir(t, filepath.Join(main, "src"))

	second := filepath.Join(base, "second")
	mkdir(t, filepath.Join(second, ".jj"))
	write(t, filepath.Join(second, ".jj", "repo"), "../../main/.jj/repo\n")

	want := filepath.Join(state, seen.JJApp, "seen", seen.RepoKey(filepath.Join(main, ".jj", "repo")))

	for _, dir := range []string{main, filepath.Join(main, "src"), second} {
		st, err := seen.OpenJJ(dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}

		if st.Dir() != want {
			t.Errorf("%s opened %s, want %s", dir, st.Dir(), want)
		}
	}

	if _, err := seen.OpenJJ(base); err == nil {
		t.Error("a directory in no workspace opened a store")
	}
}
