package preview_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/preview"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

var styles = preview.SkinStyles(skin.New(theme.Ink(), theme.Ink().Sapphire))

func plain(s string) string { return ansi.Strip(s) }

func section(heading string, rows int) preview.Section {
	list := make([]string, rows)
	for i := range list {
		list[i] = "row " + heading + " " + string(rune('a'+i))
	}

	return preview.Section{Heading: heading, Count: rows, Rows: list, Selectable: true}
}

func TestHeightsKeepEverySectionVisible(t *testing.T) {
	t.Parallel()

	sections := []preview.Section{section("a", 6), section("b", 0), section("c", 2)}
	heights := preview.Heights(sections, -1, 8)

	for i, h := range heights {
		if h < 1 {
			t.Fatalf("section %d dropped below its rule: %v", i, heights)
		}
	}

	if got := heights[0] + heights[1] + heights[2]; got > 8 {
		t.Fatalf("heights %v total %d over the 8 available", heights, got)
	}
}

func TestHeightsSplitsSurplusByRelevance(t *testing.T) {
	t.Parallel()

	hungry := section("hungry", 100)
	hungry.Relevance = preview.Urgent
	quiet := section("quiet", 3)
	quiet.Relevance = preview.Idle

	heights := preview.Heights([]preview.Section{hungry, quiet}, -1, 30)

	if heights[0] <= heights[1] {
		t.Fatalf("urgent section should outdraw an idle one: %v", heights)
	}
	if heights[1] != 4 {
		t.Fatalf("a satisfied section should keep only its want: %v", heights)
	}
}

func TestHeightsServesTheFocusedSection(t *testing.T) {
	t.Parallel()

	sections := []preview.Section{section("a", 10), section("b", 10)}
	heights := preview.Heights(sections, 1, 22)

	if heights[1] != 11 {
		t.Fatalf("focused section should take its full want: %v", heights)
	}
}

func TestSectionRuleDrawsHeadingCountAndKey(t *testing.T) {
	t.Parallel()

	s := preview.Section{Heading: "PULL REQUESTS", Key: "p", Count: 3, Selectable: true}
	rule := plain(s.Rule(styles, false, 40))

	if !strings.Contains(rule, "PULL REQUESTS · 3") {
		t.Fatalf("rule lost its label or count: %q", rule)
	}
	if !strings.HasSuffix(rule, " p") {
		t.Fatalf("rule lost its jump key: %q", rule)
	}

	blank := preview.Section{Heading: "CI"}
	rule = plain(blank.Rule(styles, false, 40))
	if strings.Contains(rule, "·") {
		t.Fatalf("an unselectable section still drew a count: %q", rule)
	}
}

func TestSectionWindowKeepsTheCursorInView(t *testing.T) {
	t.Parallel()

	s := section("s", 10)
	rows := s.Window(3, 8, true)
	if rows[0] != "row s h" || rows[2] != "row s j" {
		t.Fatalf("cursor window drifted: %v", rows)
	}

	rows = s.Window(3, 8, false)
	if rows[0] != "row s a" {
		t.Fatalf("an unfocused section should start at its first row: %v", rows)
	}
}

func TestElideMiddleKeepsBothEnds(t *testing.T) {
	t.Parallel()

	s := preview.Section{Elide: true, Rows: []string{"1", "2", "3", "4", "5", "6"}}
	rows := s.Window(4, 0, false)

	if rows[0] != "1" || rows[3] != "6" {
		t.Fatalf("elide lost an end: %v", rows)
	}
	if !strings.Contains(rows[1], "more lines") {
		t.Fatalf("elide did not mark the cut: %v", rows)
	}
}

func TestColumnSeparatesSections(t *testing.T) {
	t.Parallel()

	lines := preview.Column(styles, []preview.Section{section("a", 2), section("b", 2)}, -1, 0, 20, 40, 1)
	if len(lines) != 7 { // rule+2 rows, a gap, rule+2 rows
		t.Fatalf("column drew %d lines: %v", len(lines), lines)
	}
	if lines[3] != "" {
		t.Fatalf("gap row missing before the second section: %v", lines)
	}

	joined := plain(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "a · 2") || !strings.Contains(joined, "b · 2") {
		t.Fatalf("column lost a rule: %q", joined)
	}
}

func TestDetailWindowsAndMarks(t *testing.T) {
	t.Parallel()

	d := preview.Detail{
		Title:  "#42 fix the thing",
		Lines:  []string{"one", "two", "three", "four"},
		Scroll: 1,
	}

	if rows := d.Window(2); len(rows) != 2 || rows[0] != "two" {
		t.Fatalf("window wrong: %v", rows)
	}

	head := plain(d.Head(styles, 40, 2))
	if !strings.Contains(head, "#42 fix the thing") || !strings.Contains(head, "3/4") {
		t.Fatalf("title lost its name or marker: %q", head)
	}

	short := preview.Detail{Title: "x", Lines: []string{"only"}}
	if marker := preview.Marker(0, 4, len(short.Lines)); marker != "" {
		t.Fatalf("a pane that fits still marks: %q", marker)
	}
}

func TestGridSplitPadsTheRule(t *testing.T) {
	t.Parallel()

	grid := preview.Grid{SidePercent: 40, SideMin: 10, DetailMin: 10, RulePad: 1}
	lines := grid.Render(styles, false, 60, 6, preview.Content{
		Sections: []preview.Section{section("a", 3)},
		Detail:   preview.Detail{Title: "thing", Lines: []string{"body line"}},
	})

	if len(lines) != 6 {
		t.Fatalf("grid drew %d rows of 6", len(lines))
	}
	first := plain(lines[0])
	if !strings.Contains(first, " │ ") {
		t.Fatalf("the halves have no padded separation: %q", first)
	}

	detail := first[strings.Index(first, "│")+len("│"):]
	if !strings.HasPrefix(detail, " thing") {
		t.Fatalf("pane starts flush against the rule: %q", detail)
	}
}

func TestGridStacksWhenNarrow(t *testing.T) {
	t.Parallel()

	grid := preview.Grid{SidePercent: 40, SideMin: 10, DetailMin: 40, StackMin: 3, SectionGap: 1}
	lines := grid.Render(styles, false, 60, 20, preview.Content{
		Sections: []preview.Section{section("a", 2), section("b", 2)},
		Detail:   preview.Detail{Title: "thing", Lines: []string{"body"}},
	})

	if len(lines) != 20 {
		t.Fatalf("stacked grid drew %d rows of 20", len(lines))
	}

	var ruled bool
	for _, l := range lines {
		if strings.Contains(plain(l), "── thing ──") {
			ruled = true
		}
	}
	if !ruled {
		t.Fatalf("stacked detail lost its titled rule: %q", strings.Join(lines, "\n"))
	}
}

func TestFieldWrapAndPad(t *testing.T) {
	t.Parallel()

	if got := plain(preview.Field(styles, "branch", "main")); got != "branch main" {
		t.Fatalf("field wrong: %q", got)
	}

	wrapped := preview.Wrap(styles.Body, "one two three four five six", 10)
	if len(wrapped) < 2 {
		t.Fatalf("wrap did not wrap: %v", wrapped)
	}

	padded := preview.Pad([]string{"a"}, 3, 5)
	if len(padded) != 3 || padded[2] != "     " {
		t.Fatalf("pad wrong: %q", padded)
	}
}
