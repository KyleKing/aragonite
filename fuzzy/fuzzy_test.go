package fuzzy_test

import (
	"slices"
	"testing"

	"github.com/kyleking/aragonite/fuzzy"
)

func TestScore_MatchesAndPositions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, text, query string
		pos               []int
		ok                bool
	}{
		{name: "empty query matches", text: "squash", query: "", ok: true},
		{name: "prefix", text: "squash", query: "sq", pos: []int{0, 1}, ok: true},
		{name: "case folds", text: "Squash", query: "sQ", pos: []int{0, 1}, ok: true},
		{name: "subsequence", text: "split the change", query: "stc", pos: []int{0, 6, 10}, ok: true},
		{name: "missing rune", text: "squash", query: "sx", ok: false},
		{name: "order matters", text: "squash", query: "hs", ok: false},
		{
			name: "spaces skipped", text: "jj rebase -b <rev>", query: "rebase -b", ok: true,
			pos: []int{3, 4, 5, 6, 7, 8, 10, 11},
		},
		{name: "later start wins", text: "status squash", query: "squ", pos: []int{7, 8, 9}, ok: true},
		{name: "unicode", text: "Ünïcode", query: "ünï", pos: []int{0, 1, 2}, ok: true},
		{name: "query longer than text", text: "s", query: "squash", ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, pos, ok := fuzzy.Score(tc.text, tc.query)
			if ok != tc.ok {
				t.Fatalf("Score(%q, %q) ok = %v, want %v", tc.text, tc.query, ok, tc.ok)
			}

			if !slices.Equal(pos, tc.pos) {
				t.Errorf("Score(%q, %q) pos = %v, want %v", tc.text, tc.query, pos, tc.pos)
			}
		})
	}
}

func TestScore_Ordering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, query, better, worse string
	}{
		{name: "contiguous beats spread", query: "sq", better: "squash", worse: "s-quiet"},
		{name: "word start beats middle", query: "new", better: "start a new change", worse: "renewal"},
		{name: "earlier beats later", query: "ab", better: "abandon", worse: "zzzzzzzzzzzz abandon"},
		{name: "case match beats fold", query: "S", better: "Split", worse: "split"},
		{name: "camel hump is a boundary", query: "nc", better: "nextConflict", worse: "nonce"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, _, okB := fuzzy.Score(tc.better, tc.query)
			w, _, okW := fuzzy.Score(tc.worse, tc.query)

			if !okB || !okW || b <= w {
				t.Errorf("Score(%q)=%d (%v) should beat Score(%q)=%d (%v) for %q",
					tc.better, b, okB, tc.worse, w, okW, tc.query)
			}
		})
	}
}

func TestRank(t *testing.T) {
	t.Parallel()

	items := []string{"rebase", "squash", "split", "abandon", "squash into"}
	id := func(s string) string { return s }

	tests := []struct {
		name, query string
		want        []string
	}{
		{name: "empty keeps input order", query: "", want: items},
		{name: "best first ties in order", query: "sq", want: []string{"squash", "squash into"}},
		{name: "drops misses", query: "sp", want: []string{"split"}},
		{name: "nothing", query: "zz", want: []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := []string{}
			for _, m := range fuzzy.Rank(tc.query, items, id) {
				got = append(got, m.Item)
			}

			if !slices.Equal(got, tc.want) {
				t.Errorf("Rank(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

func BenchmarkScore(b *testing.B) {
	for b.Loop() {
		fuzzy.Score("jj squash --from <rev> --into <dest> --keep-emptied", "sqkeep")
	}
}

func BenchmarkRank(b *testing.B) {
	items := make([]string, 0, 200)
	for i := range 200 {
		items = append(items, []string{"squash into", "split", "rebase onto", "abandon", "describe"}[i%5]+
			" the change at the cursor")
	}

	id := func(s string) string { return s }

	for b.Loop() {
		fuzzy.Rank("sqch", items, id)
	}
}
