package keyhint_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/keyhint"
)

func TestPageWrapsChipsBesideTheirGroup(t *testing.T) {
	t.Parallel()

	got := keyhint.Page(plain, []keyhint.Hint{
		{What: "moving", Head: true},
		{Key: "j / k", What: "a line"},
		{Key: "gg / G", What: "the ends"},
		{Key: "/", What: "search"},
		{What: "folds", Head: true},
		{Key: "z", What: "a note"},
	}, 80)

	want := []string{
		"  moving  [j / k] a line  [gg / G] the ends  [/] search",
		"   folds  [z] a note",
	}

	if len(got) != len(want) {
		t.Fatalf("Page() gave %d lines, want %d: %q", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// The heading column is as wide as the widest name, so every group's chips
// start on the same edge.
func TestPageAlignsGroupsOnOneColumn(t *testing.T) {
	t.Parallel()

	got := keyhint.Page(plain, []keyhint.Hint{
		{What: "a", Head: true},
		{Key: "j", What: "down"},
		{What: "a much longer name", Head: true},
		{Key: "k", What: "up"},
	}, 80)

	first := strings.Index(got[0], "[j]")
	second := strings.Index(got[1], "[k]")
	if first != second {
		t.Errorf("the chips start at columns %d and %d: %q", first, second, got)
	}
}

func TestPageWrapsAFullGroup(t *testing.T) {
	t.Parallel()

	got := keyhint.Page(plain, []keyhint.Hint{
		{What: "g", Head: true},
		{Key: "a", What: "one"},
		{Key: "b", What: "two"},
		{Key: "c", What: "three"},
		{Key: "d", What: "four"},
	}, 24)

	if len(got) != 2 {
		t.Fatalf("Page() gave %d lines, want 2: %q", len(got), got)
	}

	if first, next := strings.Index(got[0], "["), strings.Index(got[1], "["); first != next {
		t.Errorf("the wrapped line's chips sit at %d, not %d: %q", next, first, got)
	}
}

// A key that opens a page of its own says so, since otherwise it reads as a
// leaf.
func TestPageMarksAPrefixWithAnEllipsis(t *testing.T) {
	t.Parallel()

	got := keyhint.Page(plain, []keyhint.Hint{
		{Key: "z", What: "folds", Kids: []keyhint.Hint{{Key: "a", What: "all"}}},
	}, 40)

	if !strings.Contains(got[0], "folds…") {
		t.Errorf("the prefix is not marked: %q", got[0])
	}
}

func TestPageDrawsAnOffHintFlat(t *testing.T) {
	t.Parallel()

	styled := plain
	styled.Off = lipgloss.NewStyle().Faint(true)

	got := keyhint.Page(styled, []keyhint.Hint{{Key: "a", What: "add", Off: true}}, 40)

	if want := styled.Off.Render("[a] add"); got[0] != keyhint.Indent+want {
		t.Errorf("Off hint = %q, want %q", got[0], keyhint.Indent+want)
	}
}

// Prose carries no key and sits in the chip column so it reads as part of the
// group it is written under. A sentence too long for the column wraps to the
// next line rather than ending mid-word, since prose is what a page says that
// the keys cannot.
func TestPageWritesProseInTheColumn(t *testing.T) {
	t.Parallel()

	got := keyhint.Page(plain, []keyhint.Hint{
		{What: "moving", Head: true},
		{Key: "j", What: "down"},
		{What: "the same key takes it back"},
	}, 30)

	prose := strings.Index(got[1], "the same")
	chip := strings.Index(got[0], "[j]")
	if prose != chip {
		t.Errorf("the prose sits at column %d and the chips at %d: %q", prose, chip, got)
	}

	if len(got) != 3 || !strings.Contains(got[2], "it back") {
		t.Errorf("the prose did not read on to a second line: %q", got)
	}
}

// A page has to fit the frame it is drawn in, since a help screen that wraps
// is the one screen a reader cannot fall back on.
func TestPageFitsItsWidth(t *testing.T) {
	t.Parallel()

	hints := []keyhint.Hint{
		{What: "a group", Head: true},
		{Key: "z then a", What: strings.Repeat("long ", 20)},
		{Key: "b", What: "two"},
		{Key: "c", What: "three"},
		{What: strings.Repeat("prose ", 40)},
	}

	for _, width := range []int{24, 40, 60, 80, 120} {
		for _, line := range keyhint.Page(plain, hints, width) {
			if lipgloss.Width(line) > width {
				t.Errorf("at %d columns a line is %d wide: %q", width, lipgloss.Width(line), line)
			}
		}
	}
}

func TestAtWalksAPathIntoNestedPages(t *testing.T) {
	t.Parallel()

	folds := []keyhint.Hint{{Key: "a", What: "all"}, {Key: "i", What: "invert"}}
	root := []keyhint.Hint{
		{Key: "j", What: "down"},
		{Key: "z", What: "folds", Kids: folds},
		{Key: "g", What: "go", Kids: []keyhint.Hint{
			{Key: "f", What: "file", Kids: []keyhint.Hint{{Key: "n", What: "name"}}},
		}},
	}

	if got, ok := keyhint.At(root, nil); !ok || len(got) != len(root) {
		t.Fatalf("At(root) = %v, %v", got, ok)
	}

	if got, ok := keyhint.At(root, []string{"z"}); !ok || len(got) != len(folds) {
		t.Fatalf("At(z) = %v, %v", got, ok)
	}

	got, ok := keyhint.At(root, []string{"g", "f"})
	if !ok || len(got) != 1 || got[0].Key != "n" {
		t.Fatalf("At(g, f) = %v, %v", got, ok)
	}
}

// A leaf opens nothing, and a key the page does not have opens nothing either:
// both leave the path dead rather than answering some other page.
func TestAtDeadEndsOnALeafAndAMissingKey(t *testing.T) {
	t.Parallel()

	root := []keyhint.Hint{
		{Key: "j", What: "down"},
		{Key: "z", What: "folds", Kids: []keyhint.Hint{{Key: "a", What: "all"}}},
	}

	if _, ok := keyhint.At(root, []string{"j"}); ok {
		t.Error("At(j) reached a page under a leaf")
	}

	if _, ok := keyhint.At(root, []string{"x"}); ok {
		t.Error("At(x) reached a page under a key that is not there")
	}

	if _, ok := keyhint.At(root, []string{"z", "a", "a"}); ok {
		t.Error("At(z, a, a) reached a page under a leaf")
	}
}
