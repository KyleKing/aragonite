package filter_test

import (
	"testing"

	"github.com/kyleking/aragonite/filter"
)

func TestAllKeepsOnlyWhatEveryPredicateKeeps(t *testing.T) {
	t.Parallel()

	even := func(n int) bool { return n%2 == 0 }
	over := func(n int) bool { return n > 2 }

	got := filter.Keep([]int{1, 2, 3, 4, 5, 6}, filter.All(even, over))
	want := []int{4, 6}

	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("All(even, over) kept %v, want %v", got, want)
	}
}

func TestAllWithNoPredicatesKeepsEverything(t *testing.T) {
	t.Parallel()

	got := filter.Keep([]int{1, 2, 3}, filter.All[int]())
	if len(got) != 3 {
		t.Errorf("All() with no predicates kept %v, want everything", got)
	}
}

func TestAnyKeepsWhatAnyPredicateKeeps(t *testing.T) {
	t.Parallel()

	one := func(n int) bool { return n == 1 }
	five := func(n int) bool { return n == 5 }

	got := filter.Keep([]int{1, 2, 3, 4, 5}, filter.Any(one, five))
	want := []int{1, 5}

	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Any(one, five) kept %v, want %v", got, want)
	}
}

func TestAnyWithNoPredicatesKeepsNothing(t *testing.T) {
	t.Parallel()

	got := filter.Keep([]int{1, 2, 3}, filter.Any[int]())
	if len(got) != 0 {
		t.Errorf("Any() with no predicates kept %v, want nothing", got)
	}
}

func TestNotInverts(t *testing.T) {
	t.Parallel()

	even := func(n int) bool { return n%2 == 0 }

	got := filter.Keep([]int{1, 2, 3, 4}, filter.Not(even))
	want := []int{1, 3}

	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Not(even) kept %v, want %v", got, want)
	}
}

func TestKeepPreservesOrderAndDropsNothingUnasked(t *testing.T) {
	t.Parallel()

	in := []int{5, 4, 3, 2, 1}

	got := filter.Keep(in, filter.All[int]())
	for i, v := range got {
		if v != in[i] {
			t.Fatalf("Keep reordered: got %v, want %v", got, in)
		}
	}
}

func TestMatchIsCaseInsensitiveUnlessThePatternCarriesAnUppercase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		pattern, text string
		want          bool
	}{
		{"pool", "connection Pool", true},
		{"Pool", "connection Pool", true},
		{"Pool", "connection pool", false},
		{"", "anything", true},
		{"missing", "connection Pool", false},
	}

	for _, tc := range cases {
		t.Run(tc.pattern+"/"+tc.text, func(t *testing.T) {
			t.Parallel()

			if got := filter.Match(tc.pattern, tc.text); got != tc.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tc.pattern, tc.text, got, tc.want)
			}
		})
	}
}

func TestTokensSplitsScopedWordsFromPlainText(t *testing.T) {
	t.Parallel()

	text, scoped := filter.Tokens("kind:thread pool", "kind")

	if text != "pool" {
		t.Errorf("text = %q, want %q", text, "pool")
	}

	if got := scoped["kind"]; len(got) != 1 || got[0] != "thread" {
		t.Errorf("scoped[kind] = %v, want [thread]", got)
	}
}

func TestTokensMatchesScopePrefixesCaseInsensitively(t *testing.T) {
	t.Parallel()

	_, scoped := filter.Tokens("Kind:thread", "kind")

	if got := scoped["kind"]; len(got) != 1 || got[0] != "thread" {
		t.Errorf("scoped[kind] = %v, want [thread] from an uppercase prefix", got)
	}
}

func TestTokensLeavesAnUnrecognizedPrefixAsPlainText(t *testing.T) {
	t.Parallel()

	text, scoped := filter.Tokens("p:123 pool", "kind")

	if text != "p:123 pool" {
		t.Errorf("text = %q, want the unrecognized prefix left untouched", text)
	}

	if len(scoped) != 0 {
		t.Errorf("scoped = %v, want nothing scoped", scoped)
	}
}

func TestTokensCollectsRepeatedScopeWords(t *testing.T) {
	t.Parallel()

	_, scoped := filter.Tokens("kind:thread kind:review", "kind")

	got := scoped["kind"]
	if len(got) != 2 || got[0] != "thread" || got[1] != "review" {
		t.Errorf("scoped[kind] = %v, want [thread review]", got)
	}
}

func TestTokensWithNoScopesReturnsQueryUnchanged(t *testing.T) {
	t.Parallel()

	text, scoped := filter.Tokens("just some words")

	if text != "just some words" {
		t.Errorf("text = %q, want the query untouched", text)
	}

	if len(scoped) != 0 {
		t.Errorf("scoped = %v, want empty", scoped)
	}
}
