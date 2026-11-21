package highlight_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/diff"
	"github.com/kyleking/aragonite/highlight"
)

const rawStringPatch = `diff --git a/q.go b/q.go
--- a/q.go
+++ b/q.go
@@ -1,4 +1,4 @@
 var q = ` + "`" + `
-select 1
+select 2
 ` + "`" + `
 var after = 5
`

func TestCacheLexesEachSideOfAFileWhole(t *testing.T) {
	t.Parallel()

	d := diff.Parse([]byte(rawStringPatch))
	cache := highlight.NewCache(d)

	for _, l := range d.Files[0].Lines {
		if l.Kind == diff.KindContext {
			continue
		}

		spans := cache.Spans("q.go", l)
		if len(spans) != 1 || spans[0].Class != highlight.String || spans[0].To != len(l.Text) {
			t.Errorf("%c line %q was lexed as %v, want one string span", l.Kind, l.Text, spans)
		}
	}

	last := d.Files[0].Lines[len(d.Files[0].Lines)-1]
	if got := cache.Spans("q.go", last); len(got) == 0 || got[0].Class != highlight.Keyword {
		t.Errorf("the line after the string was lexed as %v, want it to open with a keyword", got)
	}
}

func TestCacheAnswersNilForWhatItCannotLex(t *testing.T) {
	t.Parallel()

	cache := highlight.NewCache(diff.Parse([]byte(rawStringPatch)))

	if got := cache.Spans("missing.go", diff.Line{Kind: diff.KindAdd, New: 1}); got != nil {
		t.Errorf("a file the diff does not hold answered %v", got)
	}
}

func bigDiff() *diff.Diff {
	var patch strings.Builder

	fmt.Fprintf(&patch, "diff --git a/big.go b/big.go\n--- a/big.go\n+++ b/big.go\n@@ -1,2000 +1,2000 @@\n")

	for i := range 2000 {
		fmt.Fprintf(&patch, "+func f%d(a int) string { return \"v\" + fmt.Sprint(a, %d) } // note\n", i, i)
	}

	return diff.Parse([]byte(patch.String()))
}

func BenchmarkCacheLexFile(b *testing.B) {
	d := bigDiff()
	first := d.Files[0].Lines[0]

	for b.Loop() {
		highlight.NewCache(d).Spans("big.go", first)
	}
}

func BenchmarkCacheSpans(b *testing.B) {
	d := bigDiff()
	lines := d.Files[0].Lines
	cache := highlight.NewCache(d)

	cache.Spans("big.go", lines[0])
	b.ResetTimer()

	for i := range b.N {
		cache.Spans("big.go", lines[i%len(lines)])
	}
}

// A file is found by its new path, or by its old one where the change deleted
// it, and each is lexed in its own grammar.
func TestCache_FindsTheFileByEitherPath(t *testing.T) {
	t.Parallel()

	patch := "diff --git a/kept.go b/kept.go\n--- a/kept.go\n+++ b/kept.go\n@@ -1,1 +1,1 @@\n" +
		"-const a = 1\n+var b = \"x\"\n" +
		"diff --git a/gone.go b/gone.go\n--- a/gone.go\n+++ /dev/null\n@@ -1,1 +0,0 @@\n" +
		"-const c = 2\n" +
		"diff --git a/new.py b/new.py\n--- /dev/null\n+++ b/new.py\n@@ -0,0 +1,1 @@\n" +
		"+# hi\n"
	d := diff.Parse([]byte(patch))
	cache := highlight.NewCache(d)

	tests := []struct {
		path string
		want string
		line diff.Line
	}{
		{
			path: "kept.go",
			line: diff.Line{Text: "const a = 1", Kind: diff.KindRemove, Old: 1},
			want: "k[const] i[a] p[=] n[1]",
		},
		{
			path: "kept.go",
			line: diff.Line{Text: `var b = "x"`, Kind: diff.KindAdd, New: 1},
			want: `k[var] i[b] p[=] s["x"]`,
		},
		{
			path: "gone.go",
			line: diff.Line{Text: "const c = 2", Kind: diff.KindRemove, Old: 1},
			want: "k[const] i[c] p[=] n[2]",
		},
		{path: "new.py", line: diff.Line{Text: "# hi", Kind: diff.KindAdd, New: 1}, want: "c[# hi]"},
	}

	for _, tc := range tests {
		if got := classed(tc.line.Text, cache.Spans(tc.path, tc.line)); got != tc.want {
			t.Errorf("%s %q lexed as %s, want %s", tc.path, tc.line.Text, got, tc.want)
		}
	}
}
