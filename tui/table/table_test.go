package table_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/table"
)

func cols() []table.Column {
	return []table.Column{
		{Key: "name", Title: "NAME", Min: 16, Weight: 3},
		{Key: "branch", Title: "BRANCH", Min: 10, Weight: 2},
		{Key: "status", Title: "STATUS", Min: 12},
		{Key: "pr", Title: "PR", Min: 8, Priority: 1},
		{Key: "modified", Title: "MODIFIED", Min: 12, Priority: 2},
		{Key: "peers", Title: "PEERS", Min: 5, Priority: 3},
		{Key: "template", Title: "TEMPLATE", Min: 10, Weight: 1, Priority: 5},
	}
}

func visibleKeys(l table.Layout) []string {
	keys := make([]string, 0, len(l.Columns))
	for _, c := range l.Columns {
		keys = append(keys, c.Key)
	}

	return keys
}

func TestFitKeepsEveryColumnWhenWidthAllows(t *testing.T) {
	t.Parallel()

	l := table.Fit(cols(), 200)
	if l.Hidden != 0 {
		t.Errorf("hid %d columns at width 200, want 0", l.Hidden)
	}

	if got := l.Total(); got != 200 {
		t.Errorf("total width = %d, want 200", got)
	}
}

func TestFitDistributesSurplusByWeight(t *testing.T) {
	t.Parallel()

	const width = 155

	l := table.Fit(cols(), width)
	base := map[string]int{"name": 16, "branch": 10, "template": 10}
	gained := map[string]int{}
	for key, min := range base {
		gained[key] = l.Width(key) - min
	}

	if gained["name"] <= gained["branch"] || gained["branch"] <= gained["template"] {
		t.Errorf("surplus not ordered by weight: name +%d, branch +%d, template +%d",
			gained["name"], gained["branch"], gained["template"])
	}

	fixed := map[string]int{"status": 12, "pr": 8, "modified": 12, "peers": 5}
	for key, want := range fixed {
		if got := l.Width(key); got != want {
			t.Errorf("unweighted column %s widened to %d, want %d", key, got, want)
		}
	}

	if got := l.Total(); got != width {
		t.Errorf("total width = %d, want %d", got, width)
	}
}

func TestFitRespectsMax(t *testing.T) {
	t.Parallel()

	capped := []table.Column{
		{Key: "name", Title: "NAME", Min: 10, Max: 14, Weight: 1},
		{Key: "notes", Title: "NOTES", Min: 10, Weight: 1},
	}

	l := table.Fit(capped, 100)
	if got := l.Width("name"); got != 14 {
		t.Errorf("capped column is %d wide, want 14", got)
	}

	if got := l.Total(); got != 100 {
		t.Errorf("total width = %d, want 100; surplus freed by the cap was dropped", got)
	}
}

func TestFitCollapsesByAscendingPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    []string
		width   int
		hidden  int
		overrun bool
	}{
		{name: "everything fits", width: 200, want: []string{
			"name", "branch", "status", "pr", "modified", "peers", "template",
		}},
		{name: "pr hides first", width: 84, want: []string{
			"name", "branch", "status", "modified", "peers", "template",
		}, hidden: 1},
		{name: "modified hides second", width: 70, want: []string{
			"name", "branch", "status", "peers", "template",
		}, hidden: 2},
		{name: "peers and template outlast the rest", width: 60, want: []string{
			"name", "branch", "status", "template",
		}, hidden: 3},
		{name: "never-hidden columns overrun rather than vanish", width: 20, want: []string{
			"name", "branch", "status",
		}, hidden: 4, overrun: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := table.Fit(cols(), tt.width)
			if got := strings.Join(visibleKeys(l), " "); got != strings.Join(tt.want, " ") {
				t.Errorf("visible columns = %q, want %q", got, strings.Join(tt.want, " "))
			}

			if l.Hidden != tt.hidden {
				t.Errorf("hidden = %d, want %d", l.Hidden, tt.hidden)
			}

			if !tt.overrun && l.Total() > tt.width {
				t.Errorf("total width %d exceeds %d", l.Total(), tt.width)
			}
		})
	}
}

func TestMarkerCountsHiddenColumns(t *testing.T) {
	t.Parallel()

	if got := table.Fit(cols(), 200).Marker(); got != "" {
		t.Errorf("marker = %q with nothing hidden, want empty", got)
	}

	l := table.Fit(cols(), 70)
	if got := l.Marker(); got != "…+2" {
		t.Errorf("marker = %q, want %q", got, "…+2")
	}

	if !strings.HasSuffix(table.Header(l), "…+2") {
		t.Errorf("header does not announce hidden columns: %q", table.Header(l))
	}
}

func TestPadMeasuresDisplayWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		want  string
		width int
		align table.Align
	}{
		{name: "left pads on the right", text: "ab", width: 5, want: "ab   "},
		{name: "right pads on the left", text: "ab", width: 5, align: table.AlignRight, want: "   ab"},
		{name: "exact fit is untouched", text: "abcde", width: 5, want: "abcde"},
		{name: "overflow gets an ellipsis", text: "abcdefgh", width: 5, want: "abcd…"},
		{name: "wide glyph counts two cells", text: "🚀🚀🚀", width: 5, want: "🚀🚀…"},
		{name: "wide glyph leaves an odd cell padded", text: "🚀ab", width: 6, want: "🚀ab  "},
		{name: "no room for an ellipsis", text: "abc", width: 1, want: "a"},
		{name: "zero width is empty", text: "abc", width: 0, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := table.Pad(tt.text, tt.width, tt.align)
			if got != tt.want {
				t.Errorf("Pad(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
			}

			if w := lipgloss.Width(got); w != tt.width && tt.width > 0 {
				t.Errorf("Pad(%q, %d) is %d cells wide", tt.text, tt.width, w)
			}
		})
	}
}

func TestTruncateNeverSplitsAWideGlyph(t *testing.T) {
	t.Parallel()

	// The rocket needs two cells and the ellipsis one, so only "a" survives.
	if got := table.Truncate("a🚀b", 3); got != "a…" {
		t.Errorf("Truncate(%q, 3) = %q, want %q", "a🚀b", got, "a…")
	}

	//nolint:gosmopolitan // CJK is the point: every rune here is two cells wide
	if got := lipgloss.Width(table.Truncate("日本語テキスト", 7)); got > 7 {
		t.Errorf("truncated CJK text is %d cells wide, want at most 7", got)
	}
}

func TestPadKeepsEmojiPresentationRowsAligned(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		width int
	}{
		{"emoji plus variation selector fits", "⬆️ bump mise-action", 12},
		{"cut lands on the emoji", "⬆️ bump", 3},
		{"cut lands inside the cluster", "⬆️ bump", 2},
		{"plain text", "bump mise-action", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			padded := table.Pad(tt.text, tt.width, table.AlignLeft)
			if got := lipgloss.Width(padded); got != tt.width {
				t.Errorf("Pad(%q, %d) measures %d cells, want %d: %q", tt.text, tt.width, got, tt.width, padded)
			}
		})
	}
}

func TestRowPaintsEveryCell(t *testing.T) {
	t.Parallel()

	l := table.Fit(cols(), 70)
	face := lipgloss.NewStyle().Background(lipgloss.Color("#336699"))
	row := table.Row(l, []string{"alpha", "main"}, face)

	if w := lipgloss.Width(row); w != l.Total() {
		t.Errorf("row is %d cells, want the layout's %d", w, l.Total())
	}

	// Each cell, gutter, and the marker slot is its own painted span, so none
	// of the blanks between cells is left in the terminal's background.
	want := 2*len(l.Columns) - 1
	if l.Marker() != "" {
		want++
	}

	if got := strings.Count(row, "\x1b[48;"); got != want {
		t.Errorf("painted %d spans, want %d: %q", got, want, row)
	}
}

// han is n two-cell Han pairs, four cells each.
func han(n int) string {
	//nolint:gosmopolitan // CJK is the point: every rune here is two cells wide
	return strings.Repeat("漢字", n)
}

func breakpointCols() []table.Column {
	return []table.Column{
		{Key: "title", Min: 20, Weight: 1},
		{Key: "ref", Min: 12, From: 100},
		{Key: "author", Min: 10, From: 112},
	}
}

func TestFitHidesColumnsBelowTheirBreakpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    string
		dropped string
		width   int
		hidden  int
	}{
		{name: "below both", width: 99, want: "title", dropped: "ref author", hidden: 2},
		{name: "first floor", width: 100, want: "title ref", dropped: "author", hidden: 1},
		{name: "just under second", width: 111, want: "title ref", dropped: "author", hidden: 1},
		{name: "second floor", width: 112, want: "title ref author", dropped: "", hidden: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := table.Fit(breakpointCols(), tt.width)
			if got := strings.Join(visibleKeys(l), " "); got != tt.want {
				t.Errorf("visible = %q, want %q", got, tt.want)
			}

			if got := strings.Join(l.Dropped, " "); got != tt.dropped {
				t.Errorf("dropped = %q, want %q", got, tt.dropped)
			}

			if l.Hidden != tt.hidden {
				t.Errorf("hidden = %d, want %d", l.Hidden, tt.hidden)
			}

			if l.Total() > tt.width {
				t.Errorf("total width %d exceeds %d", l.Total(), tt.width)
			}
		})
	}
}

func TestDroppedListsKeysInColumnOrderWhateverDroppedThem(t *testing.T) {
	t.Parallel()

	cols := []table.Column{
		{Key: "a", Min: 20, Weight: 1},
		{Key: "b", Min: 10, Priority: 2},
		{Key: "c", Min: 10, From: 100},
		{Key: "d", Min: 10, Priority: 1},
	}

	tests := []struct {
		want  string
		width int
	}{
		{width: 200, want: ""},
		{width: 60, want: "c"},
		{width: 40, want: "c d"},
		{width: 25, want: "b c d"},
	}

	for _, tt := range tests {
		l := table.Fit(cols, tt.width)
		if got := strings.Join(l.Dropped, " "); got != tt.want {
			t.Errorf("width %d: dropped = %q, want %q", tt.width, got, tt.want)
		}

		if l.Hidden != len(l.Dropped) {
			t.Errorf("width %d: hidden = %d, dropped %d keys", tt.width, l.Hidden, len(l.Dropped))
		}
	}
}

func TestRowCellsKeepsTheBandUnbrokenAcrossStyledCells(t *testing.T) {
	t.Parallel()

	const (
		bandSeq = "48;2;51;102;153"
		redSeq  = "48;2;255;0;0"
	)

	l := table.Fit(cols(), 70)
	band := lipgloss.NewStyle().Background(lipgloss.Color("#336699"))
	loud := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true)
	rogue := lipgloss.NewStyle().Background(lipgloss.Color("#ff0000"))

	tests := []struct {
		name  string
		cells []table.Cell
	}{
		{name: "styled and plain", cells: []table.Cell{{Text: "alpha", Style: loud}, {Text: "main"}}},
		{name: "cell background yields to the band", cells: []table.Cell{{Text: "alpha", Style: rogue}}},
		{name: "wide glyphs", cells: []table.Cell{{Text: "⬆️ " + han(6), Style: loud}, {Text: han(3), Style: loud}}},
		{name: "no cells", cells: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			row := table.RowCells(l, tt.cells, band)
			if w := lipgloss.Width(row); w != l.Total() {
				t.Errorf("row is %d cells, want the layout's %d", w, l.Total())
			}

			spans := 2*len(l.Columns) - 1
			if l.Marker() != "" {
				spans++
			}

			if got := strings.Count(row, bandSeq); got != spans {
				t.Errorf("band painted %d spans, want %d: %q", got, spans, row)
			}

			if strings.Contains(row, redSeq) {
				t.Errorf("a cell's own background broke the band: %q", row)
			}

			if len(tt.cells) > 0 && tt.cells[0].Style.GetBold() && !strings.Contains(row, "38;2;0;255;0") {
				t.Errorf("cell foreground lost: %q", row)
			}
		})
	}
}

func TestRuleFillsExactlyTheWidth(t *testing.T) {
	t.Parallel()

	plain := table.RuleStyles{}
	dashes := func(n int) string { return strings.Repeat(table.RuleGlyph, n) }

	tests := []struct {
		name   string
		label  string
		count  string
		want   string
		width  int
		narrow bool
	}{
		{name: "20", width: 20, label: "ready", count: "4", want: "ready " + dashes(12) + " 4"},
		{name: "40", width: 40, label: "waiting on others", count: "3", want: "waiting on others " + dashes(20) + " 3"},
		{
			name: "count wider than half", width: 20, label: "ready", count: "123456789012345678",
			want: "ready " + dashes(3) + " 123456789…",
		},
		{
			name: "label wider than room", width: 20, label: "a very long label indeed", count: "7",
			want: "a very long lab… " + dashes(1) + " 7",
		},
		{name: "no count", width: 12, label: "merged", want: "merged " + dashes(5)},
		{name: "no label", width: 12, count: "5", want: dashes(10) + " 5"},
		{name: "room for a count only", width: 2, label: "x", count: "5", want: " 5"},
		{name: "one cell", width: 1, label: "x", count: "5", want: dashes(1)},
		{name: "zero", width: 0, label: "x", count: "5", want: ""},
		{name: "wide label", width: 20, label: han(8), count: "2"},
		{name: "wide count", width: 20, label: "ready", count: "⬆️⬆️⬆️"},
		{name: "wide count cut", width: 8, label: "ready", count: han(3)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := table.Rule(plain, tt.label, tt.count, tt.width)
			if w := lipgloss.Width(got); w != tt.width {
				t.Errorf("Rule is %d cells, want %d: %q", w, tt.width, got)
			}

			if tt.want != "" && got != tt.want {
				t.Errorf("Rule = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMeasureReturnsTheWidestCellPerKey(t *testing.T) {
	t.Parallel()

	rows := [][]table.Cell{
		{{Text: "short"}, {Text: "⬆️"}, {Text: "x"}},
		{{Text: han(2)}, {Text: "ab"}},
		{{Text: "mid"}},
	}

	got := table.Measure(rows, []string{"a", "b", "c", "unfilled"})
	want := map[string]int{"a": 8, "b": 2, "c": 1, "unfilled": 0}

	for key, w := range want {
		if got[key] != w {
			t.Errorf("Measure[%q] = %d, want %d", key, got[key], w)
		}
	}
}
