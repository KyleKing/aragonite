package region_test

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/region"
)

const width = 60

func plainStyles() region.Styles {
	return region.Styles{Rule: lipgloss.NewStyle(), Label: lipgloss.NewStyle()}
}

func block() region.Region {
	return region.Region{
		Title:   "acme/alpha · #11",
		Head:    []region.Fact{{Label: "Reviewers", Value: "erin"}, {Label: "Checks", Value: "3 ok"}},
		Section: "description",
		Body:    []string{"one", "two", "three"},
		Caption: "Bump the deps",
	}
}

const caption = "───────────── Bump the deps ──"

// Render occupies exactly the height it was given whatever the region holds,
// so data landing underneath the cursor cannot move the rows above it. A
// trimmed region keeps its closing caption, and a long body loses its middle.
func TestRender_FillsExactlyItsHeight(t *testing.T) {
	t.Parallel()

	long := block()
	long.Body = []string{"first", "b", "c", "d", "e", "f", "last"}
	opening := []string{
		"── acme/alpha · #11 ──────────",
		"Reviewers erin",
		"Checks    3 ok",
		"── description ───────────────",
	}

	tests := []struct {
		name   string
		want   []string
		region region.Region
		height int
	}{
		{
			name:   "exact",
			region: block(),
			height: 8,
			want:   slices.Concat(opening, []string{"one", "two", "three", caption}),
		},
		{
			name:   "slack collects above the caption",
			region: block(),
			height: 10,
			want:   slices.Concat(opening, []string{"one", "two", "three", "", "", caption}),
		},
		{name: "tight keeps the caption", region: block(), height: 3, want: []string{opening[0], opening[1], caption}},
		{name: "one row is the caption", region: block(), height: 1, want: []string{caption}},
		{name: "no rows", region: block(), height: 0, want: nil},
		{
			name:   "a long body keeps its ends",
			region: long,
			height: 8,
			want:   slices.Concat(opening, []string{"first", "  ⋯ 5 more lines ⋯", "last", caption}),
		},
		{
			name:   "even elision favors the tail",
			region: long,
			height: 7,
			want:   slices.Concat(opening, []string{"  ⋯ 6 more lines ⋯", "last", caption}),
		},
		{
			name:   "a body with one row is the count",
			region: long,
			height: 6,
			want:   slices.Concat(opening, []string{"  ⋯ 7 more lines ⋯", caption}),
		},
		{
			name:   "an empty region is its rule",
			region: region.Region{},
			height: 2,
			want:   []string{"", "──────────────────────────────"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.region.Render(plainStyles(), 30, tc.height); !slices.Equal(got, tc.want) {
				t.Errorf(
					"Render(height %d) =\n%s\nwant\n%s",
					tc.height,
					strings.Join(got, "\n"),
					strings.Join(tc.want, "\n"),
				)
			}
		})
	}
}

// Facts pair up only where the caller asked for it and the width can carry
// it, and the last fact on a line runs to the edge instead of the column cap.
func TestLines_PacksTheHeadToTheColumnsAllowed(t *testing.T) {
	t.Parallel()

	b := block()
	b.Body, b.Section = nil, ""
	b.Head = append(b.Head, region.Fact{Label: "Branch", Value: strings.Repeat("branch-", 7)})

	tests := []struct {
		name    string
		head    []region.Fact
		want    []string
		columns int
		width   int
	}{
		{
			name: "one column by default", columns: 0, width: 40,
			want: []string{"Reviewers erin", "Checks    3 ok", "Branch    branch-branch-branch-branch-b…"},
		},
		{
			name: "paired when asked and wide", columns: 2, width: 120,
			want: []string{
				"Reviewers erin" + strings.Repeat(" ", 41) + "Checks    3 ok",
				"Branch    branch-branch-branch-branch-branch-branch-branch-",
			},
		},
		{
			name: "a column is capped at its width", columns: 2, width: 70,
			want: []string{
				"Reviewers erin" + strings.Repeat(" ", 22) + "Checks    3 ok",
				"Branch    branch-branch-branch-br…",
			},
		},
		{
			name:    "narrow falls back to one",
			columns: 2,
			width:   40,
			want:    []string{"Reviewers erin", "Checks    3 ok", "Branch    branch-branch-branch-branch-b…"},
		},
		{
			name:  "unlabeled facts take no label column",
			head:  []region.Fact{{Value: "x"}, {Value: "y"}},
			width: 40,
			want:  []string{" x", " y"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := region.Region{Head: b.Head, HeadColumns: tc.columns}
			if tc.head != nil {
				r.Head = tc.head
			}

			got := r.Lines(plainStyles(), tc.width, 20)
			if want := append(slices.Clone(tc.want), strings.Repeat("─", tc.width)); !slices.Equal(got, want) {
				t.Errorf("Lines =\n%q\nwant\n%q", got, want)
			}
		})
	}
}

// TestLinesTreatsHeightAsACeiling covers the sized-to-content path: a region
// that fits draws only what it holds, and one that does not is cut to the
// ceiling the same way Render cuts it.
func TestLinesTreatsHeightAsACeiling(t *testing.T) {
	t.Parallel()

	const natural = 8

	for _, ceiling := range []int{30, natural, 6, 3} {
		got := block().Lines(plainStyles(), width, ceiling)
		if want := min(ceiling, natural); len(got) != want {
			t.Errorf("ceiling %d: drew %d lines, want %d", ceiling, len(got), want)
		}

		if ceiling < natural {
			fixed := block().Render(plainStyles(), width, ceiling)
			if !slices.Equal(got, fixed) {
				t.Errorf("ceiling %d: Lines and Render differ:\n%q\n%q", ceiling, got, fixed)
			}
		}
	}
}
