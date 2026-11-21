package preview_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/preview"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

// tagged is a face that names itself around the text it draws, so a test reads
// which face drew what without decoding escapes.
func tagged(tag string) lipgloss.Style {
	return lipgloss.NewStyle().Transform(func(s string) string { return tag + "<" + s + ">" })
}

var styles = preview.Styles{
	Heading: tagged("H"), Rule: tagged("R"), Accent: tagged("A"),
	Muted: tagged("M"), Label: tagged("L"), Body: tagged("B"),
}

// bold is how the focused half's accent title is drawn.
func bold(s string) string { return "\x1b[1m" + s + "\x1b[m" }

func section(heading string, rows, relevance int) preview.Section {
	list := make([]string, rows)
	for i := range list {
		list[i] = "row " + heading + "/" + string(rune('a'+i))
	}

	return preview.Section{
		Heading: heading, Count: strconv.Itoa(rows), Rows: list, Selectable: true, Relevance: relevance,
	}
}

func sections(s ...preview.Section) []preview.Section { return s }

func checkLines(t *testing.T, what string, got, want []string) {
	t.Helper()

	if !slices.Equal(got, want) {
		t.Errorf("%s =\n  %q\nwant\n  %q", what, got, want)
	}
}

// Every section keeps its floor, the even split comes before relevance, the
// focused section is served next, and the rest follows relevance. No section
// is given more than it would show.
func TestHeights_SharesTheAvailableLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sections  []preview.Section
		want      []int
		focused   int
		available int
	}{
		{name: "no sections", sections: nil, focused: 0, available: 5, want: []int{}},
		{
			name:     "floors only when nothing is spare",
			sections: sections(section("a", 6, 1), section("b", 0, 1), section("c", 2, 1)),
			focused:  -1, available: 4, want: []int{2, 1, 2},
		},
		{
			name:     "a single spare line is shared",
			sections: sections(section("a", 6, 1), section("b", 0, 1), section("c", 2, 1)),
			focused:  -1, available: 6, want: []int{3, 1, 2},
		},
		{
			name:     "equal relevance shares evenly until a section is full",
			sections: sections(section("a", 6, 1), section("b", 0, 1), section("c", 2, 1)),
			focused:  -1, available: 8, want: []int{4, 1, 3},
		},
		{
			name:     "relevance decides past the floors",
			sections: sections(section("a", 6, 1), section("b", 0, 1), section("c", 10, 9)),
			focused:  -1, available: 8, want: []int{3, 1, 4},
		},
		{
			name:     "an urgent section outdraws a satisfied idle one",
			sections: sections(section("h", 100, preview.Urgent), section("q", 3, preview.Idle)),
			focused:  -1, available: 30, want: []int{26, 4},
		},
		{
			name:     "the even split comes before relevance",
			sections: sections(section("h", 20, preview.Urgent), section("q", 20, preview.Idle)),
			focused:  -1, available: 30, want: []int{15, 15},
		},
		{
			name:     "the focused section takes its full want",
			sections: sections(section("a", 10, 0), section("b", 10, 0)),
			focused:  1, available: 22, want: []int{11, 11},
		},
		{
			name:     "a satisfied focused section takes no more",
			sections: sections(section("a", 10, 0), section("b", 2, 0)),
			focused:  1, available: 22, want: []int{11, 3},
		},
		{
			name:     "the focused section is served before relevance",
			sections: sections(section("a", 10, 1), section("b", 2, 1), section("c", 10, 1)),
			focused:  2, available: 8, want: []int{2, 2, 4},
		},
		{
			name:     "the first section can hold the focus",
			sections: sections(section("a", 10, 0), section("b", 10, 0)),
			focused:  0, available: 13, want: []int{7, 6},
		},
		{
			name:     "a focus past the end serves nobody",
			sections: sections(section("a", 10, 0), section("b", 10, 0)),
			focused:  2, available: 13, want: []int{6, 6},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := preview.Heights(tc.sections, tc.focused, tc.available); !slices.Equal(got, tc.want) {
				t.Errorf("Heights(available %d) = %v, want %v", tc.available, got, tc.want)
			}
		})
	}
}

func TestSectionRule_DrawsHeadingCountAndKey(t *testing.T) {
	t.Parallel()

	prs := preview.Section{Heading: "PRS", Key: "p", Count: "3", Selectable: true}
	ci := preview.Section{Heading: "CI", Key: "c", Count: "3"}

	tests := []struct {
		name    string
		want    string
		section preview.Section
		width   int
		focused bool
	}{
		{name: "idle", section: prs, width: 20, want: "R<── >H<PRS · 3>R< ─────── >M<p>"},
		{
			name:    "focused",
			section: prs,
			focused: true,
			width:   20,
			want:    "A<── >" + bold("A<PRS · 3>") + "A< ─────── >M<p>",
		},
		{name: "unselectable drops its count", section: ci, width: 12, want: "R<── >H<CI>R< ──── >M<c>"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.section.Rule(styles, tc.focused, tc.width); got != tc.want {
				t.Errorf("Rule = %q, want %q", got, tc.want)
			}
		})
	}

	if got, want := preview.LabelRule(styles, "NEEDS", 14), "R<── >H<NEEDS>R< ─────>"; got != want {
		t.Errorf("LabelRule = %q, want %q", got, want)
	}
}

// A focused section keeps the cursor in view, an unfocused one starts at its
// first row, and an Elide section drops its middle rather than its end.
func TestSectionWindow_ShowsTheRowsThatFit(t *testing.T) {
	t.Parallel()

	rows := section("s", 10, 0)
	elide := preview.Section{Elide: true, Rows: []string{"1", "2", "3", "4", "5", "6"}}

	tests := []struct {
		name    string
		want    []string
		section preview.Section
		lines   int
		cursor  int
		focused bool
	}{
		{
			name:    "cursor in the middle",
			section: rows,
			lines:   3,
			cursor:  5,
			focused: true,
			want:    []string{"row s/e", "row s/f", "row s/g"},
		},
		{
			name:    "cursor near the end",
			section: rows,
			lines:   3,
			cursor:  8,
			focused: true,
			want:    []string{"row s/h", "row s/i", "row s/j"},
		},
		{
			name:    "cursor at the start",
			section: rows,
			lines:   3,
			cursor:  0,
			focused: true,
			want:    []string{"row s/a", "row s/b", "row s/c"},
		},
		{
			name:    "unfocused starts at the top",
			section: rows,
			lines:   3,
			cursor:  8,
			want:    []string{"row s/a", "row s/b", "row s/c"},
		},
		{name: "no lines", section: rows, lines: 0, cursor: 0, focused: true, want: nil},
		{name: "elide keeps both ends", section: elide, lines: 4, want: []string{"1", "  ⋯ 3 more lines ⋯", "5", "6"}},
		{name: "elide in one line is the marker alone", section: elide, lines: 1, want: []string{"  ⋯ 6 more lines ⋯"}},
		{name: "elide with room keeps all", section: elide, lines: 6, want: elide.Rows},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checkLines(t, "Window", tc.section.Window(tc.lines, tc.cursor, tc.focused), tc.want)
		})
	}
}

func TestColumn_StacksSectionsWithAGap(t *testing.T) {
	t.Parallel()

	three := sections(section("a", 3, 1), section("b", 1, 1), section("c", 3, 1))

	tests := []struct {
		name     string
		sections []preview.Section
		want     []string
		height   int
	}{
		{
			name: "the gap rows come out of the height", sections: three, height: 9,
			want: []string{
				"R<── >H<a · 3>R< ──── >M<>", "row a/a", "",
				"R<── >H<b · 1>R< ──── >M<>", "row b/a", "",
				"A<── >" + bold("A<c · 3>") + "A< ──── >M<>", "row c/b", "row c/c",
			},
		},
		{
			name: "cut to the height", sections: three, height: 4,
			want: []string{"R<── >H<a · 3>R< ──── >M<>", "row a/a", "", "R<── >H<b · 1>R< ──── >M<>"},
		},
		{name: "no height", sections: three, height: 0, want: nil},
		{name: "no sections", sections: nil, height: 9, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checkLines(t, "Column", preview.Column(styles, tc.sections, 2, 2, tc.height, 14, 1), tc.want)
		})
	}
}

func TestDetail_WindowsAndMarksItsLines(t *testing.T) {
	t.Parallel()

	d := preview.Detail{Title: "#42", Lines: []string{"one", "two", "three", "four"}, Scroll: 1}
	focused := d
	focused.Focused = true

	windows := []struct {
		want   []string
		d      preview.Detail
		height int
	}{
		{d: d, height: 2, want: []string{"two", "three"}},
		{d: d, height: 1, want: []string{"two"}},
		{d: d, height: 0, want: nil},
		{d: preview.Detail{Lines: d.Lines, Scroll: 9}, height: 3, want: []string{"two", "three", "four"}},
		{d: preview.Detail{Lines: d.Lines, Scroll: -2}, height: 2, want: []string{"one", "two"}},
		{d: preview.Detail{Lines: d.Lines}, height: 9, want: d.Lines},
	}
	for _, w := range windows {
		checkLines(t, "Window("+strconv.Itoa(w.height)+")", w.d.Window(w.height), w.want)
	}

	for _, c := range []struct{ got, want string }{
		{d.Head(styles, 20, 2), "H<#42  3/4>"},
		{focused.Head(styles, 20, 2), bold("A<#42  3/4>")},
		{d.Rule(styles, 20, 2), "R<── >H<#42  3/4>R< ────────>"},
		{focused.Rule(styles, 20, 2), "A<── >" + bold("A<#42  3/4>") + "A< ────────>"},
		{preview.Marker(0, 4, 4), ""},
		{preview.Marker(2, 3, 9), "  5/9"},
		{preview.Marker(8, 3, 9), "  9/9"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}

	checkLines(t, "Render", focused.Render(styles, 12, 3),
		[]string{bold("A<#42  4/4>") + " ", "two         ", "three       ", "four        "})
}

func TestGrid_RendersSplitOrStacked(t *testing.T) {
	t.Parallel()

	split := preview.Grid{SidePercent: 40, SideMin: 10, DetailMin: 10, RulePad: 1}
	one := preview.Content{
		Sections: sections(section("a", 3, 1)),
		Detail:   preview.Detail{Title: "thing", Lines: []string{"body line"}},
		Focused:  -1,
	}
	oneFocused := one
	oneFocused.Detail.Focused = true

	stack := preview.Grid{SidePercent: 40, SideMin: 10, DetailMin: 40, StackMin: 3, SectionGap: 1}
	two := preview.Content{
		Sections: sections(section("a", 2, 1), section("b", 2, 1)),
		Detail:   preview.Detail{Title: "thing", Lines: []string{"body", "more"}},
		Focused:  -1,
	}
	tight := stack
	tight.StackMin = 1
	twoShort := two
	twoShort.Detail.Title = "t"

	tests := []struct {
		name          string
		want          []string
		content       preview.Content
		grid          preview.Grid
		width, height int
	}{
		{
			name: "split behind a padded rule", grid: split, content: one, width: 30, height: 4,
			want: []string{
				"R<── >H<a · 3>R< ── >M<> R<│> H<thing>       ",
				"row a/a      R<│> body line      ",
				"row a/b      R<│>                ",
				"row a/c      R<│>                ",
			},
		},
		{
			name: "a focused pane accents the rule", grid: split, content: oneFocused, width: 30, height: 4,
			want: []string{
				"R<── >H<a · 3>R< ── >M<> A<│> " + bold("A<thing>") + "       ",
				"row a/a      A<│> body line      ",
				"row a/b      A<│>                ",
				"row a/c      A<│>                ",
			},
		},
		{
			name: "stacked under a titled rule", grid: stack, content: two, width: 24, height: 10,
			want: []string{
				"R<── >H<a · 2>R< ────────────── >M<>",
				"row a/a                 ",
				"row a/b                 ",
				"                        ",
				"R<── >H<b · 2>R< ────────────── >M<>",
				"row b/a                 ",
				"row b/b                 ",
				"R<── >H<thing>R< ───────────────>",
				"body                    ",
				"more                    ",
			},
		},
		{
			name: "stacked keeps the pane its rows", grid: stack, content: two, width: 24, height: 6,
			want: []string{
				"R<── >H<a · 2>R< ────────────── >M<>",
				"row a/a                 ",
				"                        ",
				"R<── >H<thing>R< ───────────────>",
				"body                    ",
				"more                    ",
			},
		},
		{
			name: "a pane with no rows left is its rule", grid: tight, content: twoShort, width: 16, height: 4,
			want: []string{
				"R<── >H<a · 2>R< ────── >M<>",
				"row a/a         ",
				"                ",
				"R<── >H<t  0/2>R< ──────>",
			},
		},
		{name: "no width", grid: split, content: one, width: 0, height: 4, want: nil},
		{name: "no height", grid: split, content: one, width: 30, height: 0, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checkLines(t, "Render", tc.grid.Render(styles, false, tc.width, tc.height, tc.content), tc.want)
		})
	}
}

func TestGrid_DetailSizeMatchesTheLayout(t *testing.T) {
	t.Parallel()

	split := preview.Grid{SidePercent: 40, SideMin: 10, DetailMin: 10, RulePad: 1}
	stack := preview.Grid{SidePercent: 40, SideMin: 10, DetailMin: 40, StackMin: 1, SectionGap: 1}
	two := preview.Content{Sections: sections(section("a", 2, 1), section("b", 2, 1))}

	tests := []struct {
		name          string
		grid          preview.Grid
		width, height int
		w, h          int
		compact       bool
		stacked       bool
	}{
		{name: "split", grid: split, width: 30, height: 4, w: 15, h: 3},
		{name: "split at the narrowest", grid: split, width: 23, height: 4, w: 10, h: 3},
		{name: "one cell narrower stacks", grid: split, width: 22, height: 4, w: 22, h: 1, stacked: true},
		{name: "stacked", grid: stack, width: 16, height: 4, w: 16, h: 1, stacked: true},
		{
			name:    "compact stacks however wide",
			grid:    stack,
			compact: true,
			width:   100,
			height:  12,
			w:       100,
			h:       4,
			stacked: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.grid.Stacked(tc.compact, tc.width); got != tc.stacked {
				t.Errorf("Stacked = %v, want %v", got, tc.stacked)
			}

			if w, h := tc.grid.DetailSize(tc.compact, tc.width, tc.height, two); w != tc.w || h != tc.h {
				t.Errorf("DetailSize = %d, %d, want %d, %d", w, h, tc.w, tc.h)
			}
		})
	}
}

func TestFieldWrapAndPad(t *testing.T) {
	t.Parallel()

	if got := preview.Field(styles, "branch", "main"); got != "L<branch >B<main>" {
		t.Errorf("Field = %q", got)
	}

	checkLines(t, "Wrap", preview.Wrap(styles.Body, "  one two three four five six ", 10),
		[]string{"B<one two>", "B<three four>", "B<five six>"})
	checkLines(
		t,
		"Pad",
		preview.Pad([]string{"a", "abcdef", "x", "dropped"}, 3, 5),
		[]string{"a    ", "abcdef", "x    "},
	)
}

// The skin's faces land on the roles the panes ask for.
func TestSkinStyles_MapsTheSkin(t *testing.T) {
	t.Parallel()

	s := skin.New(theme.Ink(), theme.Ink().Sapphire)
	st := preview.SkinStyles(s)

	for name, faces := range map[string][2]lipgloss.Style{
		"heading": {st.Heading, s.Heading}, "rule": {st.Rule, s.Hairline}, "accent": {st.Accent, s.Accent},
		"muted": {st.Muted, s.Muted}, "label": {st.Label, s.Subtitle}, "body": {st.Body, s.Body},
	} {
		if got, want := faces[0].Render("x"), faces[1].Render("x"); got != want || ansi.Strip(got) != "x" {
			t.Errorf("%s draws %q, want %q", name, got, want)
		}
	}

	if strings.Count(st.Heading.Render("x")+st.Muted.Render("x"), "\x1b[") < 2 {
		t.Error("the skin's faces drew no color")
	}
}
