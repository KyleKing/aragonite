package seen_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/diff"
	"github.com/kyleking/aragonite/seen"
)

// bigDiff is 20,944 lines across 136 files of 10 hunks each: the size of a lockfile
// regeneration plus a sweeping rename, which is the largest diff a reader opens and marks.
func bigDiff() []byte {
	var b strings.Builder

	for f := range 136 {
		path := fmt.Sprintf("internal/pkg%03d/file.go", f)
		fmt.Fprintf(&b, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n", path, path, path, path)

		for h := range 10 {
			fmt.Fprintf(&b, "@@ -%d,10 +%d,10 @@\n", h*40+1, h*40+1)

			for l := range 3 {
				fmt.Fprintf(&b, " \tcontext %d of hunk %d\n", l, h)
			}

			for l := range 4 {
				fmt.Fprintf(&b, "-\told line %d of a change worth reading\n", l)
				fmt.Fprintf(&b, "+\tnew line %d of a change worth reading\n", l)
			}

			for l := range 3 {
				fmt.Fprintf(&b, " \ttrailing context %d\n", l)
			}
		}
	}

	return []byte(b.String())
}

func BenchmarkHunks(b *testing.B) {
	d := diff.Parse(bigDiff())
	b.ReportAllocs()

	for b.Loop() {
		if len(seen.Hunks(d)) != 1360 {
			b.Fatal("wrong hunk count")
		}
	}
}

// BenchmarkParse is what a caller holding the patch text pays: the parse and the hashing.
func BenchmarkParse(b *testing.B) {
	patch := bigDiff()
	b.SetBytes(int64(len(patch)))
	b.ReportAllocs()

	for b.Loop() {
		seen.Parse(patch)
	}
}
