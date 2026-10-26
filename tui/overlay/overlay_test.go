package overlay_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/overlay"
)

func testStyles() overlay.Styles {
	return overlay.Styles{
		Frame:   lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1, 3),
		Elision: lipgloss.NewStyle(),
	}
}

// lipgloss.Place centers each line against its own width, so a block of
// differing line lengths staggers. Every line of the overlay has to start in
// the same column, which is what padding to a uniform width buys.
func TestCenter_DrawsTheBlockAsOneRatherThanLineByLine(t *testing.T) {
	t.Parallel()

	styles := testStyles()
	content := "a short line\nand one that is considerably longer\nmid"

	rendered := ansi.Strip(overlay.Center(content, 80, 24, styles))

	lines := strings.Split(rendered, "\n")
	if len(lines) != 24 {
		t.Fatalf("the overlay is %d rows on a 24-row frame", len(lines))
	}

	var lefts []int

	for _, line := range lines {
		if trimmed := strings.TrimLeft(line, " "); trimmed != "" {
			lefts = append(lefts, len(line)-len(trimmed))
		}
	}

	if len(lefts) == 0 {
		t.Fatal("the overlay drew nothing")
	}

	for i, left := range lefts {
		if left != lefts[0] {
			t.Errorf("row %d starts at column %d, want %d: the block staggered", i, left, lefts[0])
		}
	}
}

// Content larger than the frame is clipped rather than pushing the overlay off
// screen, and the elision says so.
func TestCenter_ClipsToTheRoomInsideTheFrame(t *testing.T) {
	t.Parallel()

	styles := testStyles()

	if got := overlay.ContentWidth(80, styles); got != 72 {
		t.Errorf("content has %d cells inside a bordered, padded 80-cell frame, want 72", got)
	}

	if got := overlay.ContentHeight(24, styles); got != 20 {
		t.Errorf("content has %d rows inside the frame, want 20", got)
	}

	tall := strings.Repeat("row\n", 40) + strings.Repeat("w", 200)

	rendered := ansi.Strip(overlay.Center(tall, 80, 24, styles))
	if got := len(strings.Split(rendered, "\n")); got != 24 {
		t.Errorf("an overlarge overlay drew %d rows on a 24-row frame", got)
	}

	for _, line := range strings.Split(rendered, "\n") {
		if got := ansi.StringWidth(line); got > 80 {
			t.Fatalf("a row is %d cells wide on an 80-cell frame", got)
		}
	}

	if !strings.Contains(rendered, overlay.ElisionMarker) {
		t.Error("content was dropped without the elision saying so")
	}
}

func TestBottomRight_LandsFlushInTheCornerAndLeavesTheRestAlone(t *testing.T) {
	t.Parallel()

	panel := keyhint.PanelStyles{Frame: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)}
	hints := []keyhint.Hint{
		{Key: "a", What: "all repos", Count: "12", On: true},
		{Key: "d", What: "dirty only", Count: "3"},
		{Key: "x", What: "clear filters"},
	}

	tests := []struct {
		name          string
		width, height int
	}{
		{name: "wide", width: 80, height: 24},
		{name: "small", width: 40, height: 10},
		{name: "narrower than the panel", width: 14, height: 6},
		{name: "shorter than the panel", width: 30, height: 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			base := make([]string, tc.height)
			for i := range base {
				base[i] = strings.Repeat(string(rune('a'+i%26)), tc.width)
			}

			box := keyhint.Panel(panel, hints, tc.width)
			got := overlay.BottomRight(base, box, tc.width)

			if len(got) != tc.height {
				t.Fatalf("drew %d rows over a %d-row base", len(got), tc.height)
			}

			assertOutsideUntouched(t, base, got, box, tc.width)

			if last := ansi.Strip(got[tc.height-1]); !strings.HasSuffix(last, "╯") {
				t.Errorf("bottom row %q does not end in the panel's corner", last)
			}
		})
	}
}

func assertOutsideUntouched(t *testing.T, base, got, box []string, width int) {
	t.Helper()

	boxWidth := 0
	for _, l := range box {
		boxWidth = max(boxWidth, ansi.StringWidth(l))
	}

	boxWidth = min(boxWidth, width)
	top := len(base) - min(len(box), len(base))

	for i, line := range got {
		if w := ansi.StringWidth(line); w != width {
			t.Fatalf("row %d is %d cells wide, want %d", i, w, width)
		}

		kept := ansi.Strip(ansi.Cut(line, 0, width-boxWidth))
		if i < top {
			kept = ansi.Strip(line)
		}

		if want := base[i][:len(kept)]; kept != want {
			t.Errorf("row %d outside the panel is %q, want %q", i, kept, want)
		}
	}
}

func TestAtCoversWhatBlitDraws(t *testing.T) {
	t.Parallel()

	r := overlay.At([]string{"abc", "abcde"}, 4, 2)

	for _, c := range []struct {
		x, y int
		in   bool
	}{{4, 2, true}, {8, 3, true}, {9, 3, false}, {3, 2, false}, {4, 4, false}} {
		if got := r.Contains(c.x, c.y); got != c.in {
			t.Errorf("Contains(%d, %d) = %v, want %v", c.x, c.y, got, c.in)
		}
	}
}
