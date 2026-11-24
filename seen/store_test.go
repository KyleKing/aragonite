package seen_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kyleking/aragonite/diff"
	"github.com/kyleking/aragonite/seen"
)

// readState is the seam jojo declares (model.ReadState). A *Store has to satisfy it as it is.
type readState interface {
	Unread(changeID, hunkHash string) bool
}

var _ readState = (*seen.Store)(nil)

// refs is n hunks of one file with distinct ids, for tests that only need ids.
func refs(n int) []seen.Ref {
	out := make([]seen.Ref, n)
	for i := range out {
		out[i] = seen.Ref{Path: "a.go", Hunk: i + 1, ID: seen.ID(fmt.Sprintf("%064d", i))}
	}

	return out
}

// A mark for a hunk the diff no longer carries is dead weight, and a file that only grows is one
// nobody can read. A scope nobody has opened has no file, which is not an error.
func TestSave_KeepsOnlyLiveHunks(t *testing.T) {
	t.Parallel()

	live := seen.Hunks(diff.Parse([]byte(patch)))
	path := filepath.Join(t.TempDir(), "seen", "pr-2.toml")

	set, err := seen.Load(path)
	if err != nil {
		t.Fatalf("loading a set that was never written: %v", err)
	}

	if set.Count(live) != 0 {
		t.Error("an unwritten set came back non-empty")
	}

	set.Mark(true, live[0].ID, live[1].ID, "a hunk that has since gone")

	if err := seen.Save(path, set, live); err != nil {
		t.Fatal(err)
	}

	back, err := seen.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := back.Count(live); got != 2 {
		t.Errorf("%d hunks came back read, want 2", got)
	}

	if back.Has("a hunk that has since gone") || set.Has("a hunk that has since gone") {
		t.Error("a mark for a hunk the diff dropped was kept")
	}
}

// Two readers of one scope each save what they changed, not what they loaded, so neither loses
// the other's marks, and an unmark wins over the stale read it replaces.
func TestSave_MergesWithAnotherWriter(t *testing.T) {
	t.Parallel()

	live := refs(4)
	dir := t.TempDir()

	one, two := seen.Open(dir), seen.Open(dir)

	a, err := one.Set("pr-2")
	if err != nil {
		t.Fatal(err)
	}

	b, err := two.Set("pr-2")
	if err != nil {
		t.Fatal(err)
	}

	a.Mark(true, live[0].ID, live[1].ID)
	b.Mark(true, live[2].ID)

	if err := one.Save("pr-2", live); err != nil {
		t.Fatal(err)
	}

	if err := two.Save("pr-2", live); err != nil {
		t.Fatal(err)
	}

	if got := b.Count(live); got != 3 {
		t.Errorf("after the second save it holds %d marks, want 3", got)
	}

	a.Mark(false, live[0].ID)

	if err := one.Save("pr-2", live); err != nil {
		t.Fatal(err)
	}

	two.Reload("pr-2")

	for i, want := range []bool{true, false, false, true} {
		if got := two.Unread("pr-2", string(live[i].ID)); got != want {
			t.Errorf("hunk %d: unread = %v, want %v", i, got, want)
		}
	}
}

// Many writers saving at once each keep every mark: the lock serializes them and each merges.
func TestSave_ConcurrentWritersLoseNothing(t *testing.T) {
	t.Parallel()

	const writers, each = 8, 10

	live := refs(writers * each)
	dir := t.TempDir()

	var wg sync.WaitGroup

	errs := make(chan error, writers*each)

	for w := range writers {
		wg.Go(func() {
			st := seen.Open(dir)

			set, err := st.Set("change")
			if err != nil {
				errs <- err

				return
			}

			for i := range each {
				set.Mark(true, live[w*each+i].ID)
				errs <- st.Save("change", live)
			}
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	back, err := seen.Load(filepath.Join(dir, "change.toml"))
	if err != nil {
		t.Fatal(err)
	}

	if got := back.Count(live); got != len(live) {
		t.Errorf("%d of %d marks survived", got, len(live))
	}
}

// The same as two real processes: each re-runs this test binary as a writer.
func TestSave_TwoProcessesLoseNothing(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("spawns processes")
	}

	const each = 25

	dir := t.TempDir()

	var wg sync.WaitGroup

	outs := make([][]byte, 2)
	errs := make([]error, 2)

	for p := range 2 {
		wg.Go(func() {
			//nolint:gosec // this test binary
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperWriter$")
			cmd.Env = append(os.Environ(), "SEEN_HELPER_DIR="+dir,
				"SEEN_HELPER_FROM="+strconv.Itoa(p*each), "SEEN_HELPER_COUNT="+strconv.Itoa(each))
			outs[p], errs[p] = cmd.CombinedOutput()
		})
	}

	wg.Wait()

	for p := range 2 {
		if errs[p] != nil {
			t.Fatalf("writer %d: %v\n%s", p, errs[p], outs[p])
		}
	}

	back, err := seen.Load(filepath.Join(dir, "shared.toml"))
	if err != nil {
		t.Fatal(err)
	}

	if got := back.Count(refs(2 * each)); got != 2*each {
		t.Errorf("%d of %d marks survived two processes", got, 2*each)
	}
}

// TestHelperWriter is the writer process TestSave_TwoProcessesLoseNothing starts, and does nothing
// on its own.
func TestHelperWriter(t *testing.T) {
	t.Parallel()

	dir := os.Getenv("SEEN_HELPER_DIR")
	if dir == "" {
		t.Skip("run by TestSave_TwoProcessesLoseNothing")
	}

	from, err := strconv.Atoi(os.Getenv("SEEN_HELPER_FROM"))
	if err != nil {
		t.Fatal(err)
	}

	count, err := strconv.Atoi(os.Getenv("SEEN_HELPER_COUNT"))
	if err != nil {
		t.Fatal(err)
	}

	live := refs(2 * count)
	st := seen.Open(dir)

	set, err := st.Set("shared")
	if err != nil {
		t.Fatal(err)
	}

	for i := from; i < from+count; i++ {
		set.Mark(true, live[i].ID)

		if err := st.Save("shared", live); err != nil {
			t.Fatal(err)
		}
	}
}

// A lock left by a process that died holding it is broken once stale, rather than blocking every
// save after it.
func TestSave_BreaksAStaleLock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "pr-2.toml")
	lock := path + ".lock"

	if err := os.WriteFile(lock, []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}

	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}

	set := seen.New()
	set.Mark(true, refs(1)[0].ID)

	if err := seen.Save(path, set, refs(1)); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the lock is still there after the save: %v", err)
	}
}

func TestStore_Path(t *testing.T) {
	t.Parallel()

	st := seen.Open("/state")

	tests := []struct {
		scope string
		want  string
	}{
		{"pr-12", "/state/pr-12.toml"},
		{"local", "/state/local.toml"},
		{"qpvuntsm", "/state/qpvuntsm.toml"},
		{"../up", "/state/..%2Fup.toml"},
		{"a b:c", "/state/a%20b%3Ac.toml"},
	}

	for _, tc := range tests {
		t.Run(tc.scope, func(t *testing.T) {
			t.Parallel()

			got, err := st.Path(tc.scope)
			if err != nil {
				t.Fatal(err)
			}

			if got != filepath.FromSlash(tc.want) {
				t.Errorf("Path(%q) = %q, want %q", tc.scope, got, tc.want)
			}
		})
	}

	for _, bad := range []string{"", ".", ".."} {
		if _, err := st.Path(bad); !errors.Is(err, seen.ErrScope) {
			t.Errorf("Path(%q) = %v, want ErrScope", bad, err)
		}
	}
}

// A nil store remembers nothing, and a scope whose file cannot be read shows every hunk as unread
// rather than hiding any.
func TestStore_UnreadFailsTowardUnread(t *testing.T) {
	t.Parallel()

	var none *seen.Store
	if !none.Unread("x", "y") {
		t.Error("a nil store answered read")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.toml"), []byte("not = [toml"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !seen.Open(dir).Unread("broken", "y") {
		t.Error("an unreadable scope answered read")
	}

	if err := seen.Open(dir).Save("never-asked", refs(1)); err != nil {
		t.Errorf("saving a scope never asked for: %v", err)
	}
}

func TestStateDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/xdg")

	got, err := seen.StateDir("jojo", "repo")
	if err != nil {
		t.Fatal(err)
	}

	if want := filepath.FromSlash("/xdg/jojo/repo"); got != want {
		t.Errorf("StateDir = %q, want %q", got, want)
	}

	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/r")

	got, err = seen.StateDir("jojo")
	if err != nil {
		t.Fatal(err)
	}

	if want := filepath.FromSlash("/home/r/.local/state/jojo"); got != want {
		t.Errorf("StateDir without XDG_STATE_HOME = %q, want %q", got, want)
	}
}

// A repository's key is stable, readable, and different for two clones that share a base name.
func TestRepoKey(t *testing.T) {
	t.Parallel()

	one := seen.RepoKey("/src/app")

	if one != seen.RepoKey("/src/app/") {
		t.Error("a trailing slash changed the key")
	}

	if !strings.HasPrefix(one, "app-") || len(one) != len("app-")+12 {
		t.Errorf("RepoKey = %q, want app- and 12 hex digits", one)
	}

	if one == seen.RepoKey("/work/app") {
		t.Error("two clones named app share a key")
	}
}

// Mark is a mark and a save in one call: a second store over the directory sees it at once.
func TestStore_MarkSavesAtOnce(t *testing.T) {
	t.Parallel()

	live := refs(3)
	dir := t.TempDir()

	if err := seen.Open(dir).Mark("qpvuntsm", true, live, live[1].ID, live[2].ID); err != nil {
		t.Fatal(err)
	}

	other := seen.Open(dir)

	set, err := other.Set("qpvuntsm")
	if err != nil {
		t.Fatal(err)
	}

	if got := set.Files(live); got[0].Read != 2 || got[0].Total != 3 {
		t.Errorf("Files = %+v, want 2 of 3 read", got)
	}

	if !other.Unread("qpvuntsm", string(live[0].ID)) || other.Unread("qpvuntsm", string(live[1].ID)) {
		t.Error("the other store disagrees about which hunks were marked")
	}
}
