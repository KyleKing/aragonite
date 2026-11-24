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

// A bar draws each distinct face once and reuses it, so every cell must still be what its own face
// renders, including faces that differ only in an attribute and faces a key cannot stand for.
func TestCellsDrawWhatTheirFacesRender(t *testing.T) {
	t.Parallel()

	track := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	thumb := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	red := lipgloss.Color("#ff0000")

	marks := []overview.Mark{
		overview.Change(10, red),
		overview.Change(20, lipgloss.Color("2")),
		{Row: 30, Glyph: "x", Face: lipgloss.NewStyle().Foreground(red).Bold(true), Rank: 1},
		{Row: 40, Glyph: "x", Face: lipgloss.NewStyle().Foreground(red).Padding(0, 1), Rank: 1},
		{Row: 50, Glyph: "x", Face: lipgloss.NewStyle().Foreground(red).Underline(true), Rank: 1},
		overview.Found(60, red),
		overview.Change(70, red),
	}

	bar := overview.New(10, 100, 0, track, thumb).Place(marks...)

	want := make([]string, 0, 1+len(marks))
	want = append(want, thumb.Render(overview.Thumb))

	for _, m := range marks {
		want = append(want, m.Face.Render(m.Glyph))
	}

	for i := range 8 {
		if got := bar.Cell(i); got != want[i] {
			t.Errorf("cell %d = %q, want %q", i, got, want[i])
		}
	}

	if got, again := bar.Cell(9), bar.Cell(9); got != track.Render(overview.Track) || again != got {
		t.Errorf("an empty cell drew %q then %q, want the track %q", got, again, track.Render(overview.Track))
	}
}

// BenchmarkBarLines is one frame's track: a 44-row bar over a file whose every row changed.
func BenchmarkBarLines(b *testing.B) {
	marks := make([]overview.Mark, 0, 2000)
	for i := range 2000 {
		marks = append(marks, overview.Change(i, lipgloss.Color([]string{"#a6da95", "#ed8796"}[i%2])))
	}

	track := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	thumb := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	b.ReportAllocs()

	for b.Loop() {
		_ = overview.New(44, 2000, 500, track, thumb).Place(marks...).Lines()
	}
}
