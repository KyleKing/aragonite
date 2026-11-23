package overview_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/overview"
)

func plain(b *overview.Bar) string {
	var sb strings.Builder
	for _, l := range b.Lines() {
		sb.WriteString(l)
	}

	return sb.String()
}

func TestNew(t *testing.T) {
	t.Parallel()

	plainFace := lipgloss.NewStyle()

	tests := []struct {
		name   string
		want   string
		height int
		total  int
		offset int
	}{
		{name: "content that fits draws nothing", height: 4, total: 4, offset: 0, want: ""},
		{name: "top", height: 4, total: 8, offset: 0, want: "┃┃││"},
		{name: "bottom reaches the last cell", height: 4, total: 8, offset: 4, want: "││┃┃"},
		{name: "thumb is never shorter than one cell", height: 4, total: 400, offset: 200, want: "│┃││"},
		{name: "an offset past the end stays on the track", height: 4, total: 8, offset: 99, want: "││┃┃"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := overview.New(tt.height, tt.total, tt.offset, plainFace, plainFace)
			if got := plain(b); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlace(t *testing.T) {
	t.Parallel()

	f := lipgloss.NewStyle()
	mark := func(row int, glyph string, rank int) overview.Mark {
		return overview.Mark{Row: row, Glyph: glyph, Face: f, Rank: rank}
	}

	tests := []struct {
		name  string
		want  string
		marks []overview.Mark
	}{
		{name: "a mark lands by its share of the content", marks: []overview.Mark{mark(10, "x", 1)}, want: "┃│x│"},
		{name: "the thumb wins a collision", marks: []overview.Mark{mark(0, "x", 9)}, want: "┃│││"},
		{name: "the higher rank wins", marks: []overview.Mark{mark(10, "a", 1), mark(11, "b", 2)}, want: "┃│b│"},
		{name: "a tie keeps the first", marks: []overview.Mark{mark(10, "a", 1), mark(11, "b", 1)}, want: "┃│a│"},
		{
			name:  "a row outside the content is dropped",
			marks: []overview.Mark{mark(-1, "x", 1), mark(99, "y", 1)},
			want:  "┃│││",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := overview.New(4, 20, 0, f, f).Place(tt.marks...)
			if got := plain(b); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNilBarIsSafe(t *testing.T) {
	t.Parallel()

	var b *overview.Bar

	b = b.Place(overview.Act(0, lipgloss.Color("1")))
	if b.Height() != 0 || b.Width() != 0 || b.Cell(0) != "" || len(b.Lines()) != 0 {
		t.Error("a nil bar should draw nothing")
	}
}

func TestStockRanks(t *testing.T) {
	t.Parallel()

	c := lipgloss.Color("1")
	if overview.Act(0, c).Rank <= overview.Draft(0, c).Rank ||
		overview.Draft(0, c).Rank <= overview.Change(0, c).Rank ||
		overview.Change(0, c).Rank <= overview.Done(0, c).Rank {
		t.Error("act > draft > change > done")
	}

	if overview.Found(0, c).Glyph != overview.GlyphFound {
		t.Error("found draws its own glyph")
	}
}
