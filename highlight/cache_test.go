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

func TestCachePeekNeverLexesAndStartFillsInTheBackground(t *testing.T) {
	t.Parallel()

	d := diff.Parse([]byte(rawStringPatch))
	cache := highlight.NewCache(d)
	last := d.Files[0].Lines[len(d.Files[0].Lines)-1]

	if spans, ready := cache.Peek("q.go", last); ready || spans != nil {
		t.Fatalf("Peek before Start answered %v, %v, want nothing and not ready", spans, ready)
	}

	if _, ready := cache.Peek("q.go", last); ready {
		t.Fatal("Peek lexed the file itself")
	}

	<-cache.Start("q.go")

	spans, ready := cache.Peek("q.go", last)
	if !ready {
		t.Fatal("Peek after Start's channel closed was not ready")
	}

	if want := cache.Spans("q.go", last); len(spans) == 0 || spans[0] != want[0] {
		t.Errorf("Peek answered %v, want what Spans answers, %v", spans, want)
	}
}

func TestCacheStartTwiceLexesOnceAndSpansWaitsForIt(t *testing.T) {
	t.Parallel()

	d := bigDiff()
	cache := highlight.NewCache(d)

	first, second := cache.Start("big.go"), cache.Start("big.go")
	if first != second {
		t.Error("a second Start began another lexing rather than answering the first")
	}

	// Spans while Start is in flight waits for it rather than racing it.
	if got := cache.Spans("big.go", d.Files[0].Lines[0]); len(got) == 0 {
		t.Error("Spans during a background lex answered nothing")
	}

	select {
	case <-first:
	default:
		t.Error("Spans returned before the lexing it waited on finished")
	}
}

func TestCacheStartOnAnUnknownFileStillCloses(t *testing.T) {
	t.Parallel()

	cache := highlight.NewCache(diff.Parse([]byte(rawStringPatch)))
	<-cache.Start("missing.go")

	if spans, ready := cache.Peek("missing.go", diff.Line{Kind: diff.KindAdd, New: 1}); !ready || spans != nil {
		t.Errorf("an unknown file answered %v, %v, want no spans and ready", spans, ready)
	}
}

func TestCacheIsSafeForConcurrentReaders(t *testing.T) {
	t.Parallel()

	d := bigDiff()
	cache := highlight.NewCache(d)
	done := make(chan struct{})

	for range 4 {
		go func() {
			defer func() { done <- struct{}{} }()

			for _, l := range d.Files[0].Lines[:50] {
				cache.Peek("big.go", l)
				cache.Spans("big.go", l)
			}
		}()
	}

	for range 4 {
		<-done
	}
}

// BenchmarkCacheStartToReady is what a screen waits between asking for a file's colors in the
// background and having them.
func BenchmarkCacheStartToReady(b *testing.B) {
	d := bigDiff()

	for b.Loop() {
		<-highlight.NewCache(d).Start("big.go")
	}
}

// BenchmarkCachePeek is the per-line cost of drawing from a lexed file without blocking.
func BenchmarkCachePeek(b *testing.B) {
	d := bigDiff()
	lines := d.Files[0].Lines
	cache := highlight.NewCache(d)

	<-cache.Start("big.go")
	b.ResetTimer()

	for i := range b.N {
		cache.Peek("big.go", lines[i%len(lines)])
	}
}

// BenchmarkCacheLexFileBothSides opens a file whose old and new sides differ, which lexes the
// two sides at once.
func BenchmarkCacheLexFileBothSides(b *testing.B) {
	var patch strings.Builder

	fmt.Fprintf(&patch, "diff --git a/mixed.go b/mixed.go\n--- a/mixed.go\n+++ b/mixed.go\n@@ -1,1000 +1,1000 @@\n")

	for i := range 1000 {
		fmt.Fprintf(&patch, "-func f%d(a int) string { return \"old\" } // note\n", i)
		fmt.Fprintf(&patch, "+func f%d(a int) string { return \"new\" } // note\n", i)
	}

	d := diff.Parse([]byte(patch.String()))
	first := d.Files[0].Lines[0]

	for b.Loop() {
		highlight.NewCache(d).Spans("mixed.go", first)
	}
}
