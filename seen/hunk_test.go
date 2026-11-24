package seen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/diff"
	"github.com/kyleking/aragonite/seen"
)

const patch = `diff --git a/internal/a.go b/internal/a.go
--- a/internal/a.go
+++ b/internal/a.go
@@ -1,3 +1,4 @@
 package a
+// added one
 func A() {}
@@ -20,3 +21,4 @@
 func B() {}
+// added two
 func C() {}
diff --git a/internal/b.go b/internal/b.go
--- a/internal/b.go
+++ b/internal/b.go
@@ -1,2 +1,3 @@
 package b
+// added three
`

// Identity is content, not position. A hunk that slides down the file because something above it
// grew is the same hunk and stays read; a hunk whose text changed is a different one and comes
// back unread. That is the whole of what makes read-state survive a force-push or a rebase. The
// same text in two files is two hunks: a change read in one place has not been read in the other.
func TestHunk_IdentityFollowsContentNotPosition(t *testing.T) {
	t.Parallel()

	want := seen.Hunk(diff.Parse([]byte(patch)), "internal/a.go", 1)

	tests := []struct {
		name  string
		patch string
		path  string
		hunk  int
		same  bool
	}{
		{"unchanged", patch, "internal/a.go", 1, true},
		{
			"line numbers shifted",
			strings.Replace(patch, "@@ -1,3 +1,4 @@", "@@ -41,3 +58,4 @@", 1),
			"internal/a.go", 1, true,
		},
		{"a line changed", strings.Replace(patch, "+// added one", "+// added ONE", 1), "internal/a.go", 1, false},
		{
			"a context line changed",
			strings.Replace(patch, " func A() {}", " func A(x int) {}", 1),
			"internal/a.go", 1, false,
		},
		{"another hunk of the file", patch, "internal/a.go", 2, false},
		{
			"the same text in another file",
			strings.ReplaceAll(patch, "added three", "added one"),
			"internal/b.go", 3, false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := seen.Hunk(diff.Parse([]byte(tc.patch)), tc.path, tc.hunk)
			if same := got == want; same != tc.same {
				t.Errorf("identity held = %v, want %v", same, tc.same)
			}
		})
	}
}

// Hunks rolls every id in one pass, and must agree with Hunk asking for each one alone; Parse is
// Hunks over text.
func TestHunks_AgreesWithHunkAndParse(t *testing.T) {
	t.Parallel()

	d := diff.Parse([]byte(patch))
	refs := seen.Hunks(d)

	if len(refs) != 3 {
		t.Fatalf("found %d hunks, want 3", len(refs))
	}

	parsed := seen.Parse([]byte(patch))

	for i, r := range refs {
		if one := seen.Hunk(d, r.Path, r.Hunk); one != r.ID {
			t.Errorf("hunk %d: Hunks said %s, Hunk said %s", i, r.ID, one)
		}

		if parsed[i] != r {
			t.Errorf("hunk %d: Parse said %+v, Hunks said %+v", i, parsed[i], r)
		}
	}

	if seen.Parse(nil) != nil {
		t.Error("empty text came back with hunks")
	}
}

// The fixtures were written by second-look v0.57.3's internal/seen over a patch of this
// repository's own history, every hunk marked in one file and every other hunk in the other. The
// ids must come out byte for byte, or every mark a reader already made reads as unread.
func TestHunks_MatchSecondLookV0573(t *testing.T) {
	t.Parallel()

	refs := seen.Parse(readFixture(t, "second-look-v0.57.3.patch.golden"))
	if len(refs) != 15 {
		t.Fatalf("found %d hunks, want 15", len(refs))
	}

	all, err := seen.Load(fixture("second-look-v0.57.3-all.toml.golden"))
	if err != nil {
		t.Fatal(err)
	}

	if got := all.Count(refs); got != len(refs) {
		t.Errorf("%d of %d ids match the ones second-look wrote", got, len(refs))
	}

	half, err := seen.Load(fixture("second-look-v0.57.3.toml.golden"))
	if err != nil {
		t.Fatal(err)
	}

	for i, r := range refs {
		if want := i%2 == 0; half.Has(r.ID) != want {
			t.Errorf("hunk %d (%s): read = %v, want %v", i, r.Path, !want, want)
		}
	}
}

// Writing the same marks must produce the file second-look wrote, byte for byte, so either
// version can read the other's.
func TestSave_WritesWhatSecondLookV0573Wrote(t *testing.T) {
	t.Parallel()

	refs := seen.Parse(readFixture(t, "second-look-v0.57.3.patch.golden"))
	set := seen.New()

	for i, r := range refs {
		if i%2 == 0 {
			set.Mark(true, r.ID)
		}
	}

	path := filepath.Join(t.TempDir(), "pr-2.toml")
	if err := seen.Save(path, set, refs); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // a test's temp dir
	if err != nil {
		t.Fatal(err)
	}

	if want := readFixture(t, "second-look-v0.57.3.toml.golden"); !bytes.Equal(got, want) {
		t.Errorf("wrote\n%s\nwant\n%s", got, want)
	}
}

func fixture(name string) string { return filepath.Join("testdata", name) }

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	raw, err := os.ReadFile(fixture(name))
	if err != nil {
		t.Fatal(err)
	}

	return raw
}
