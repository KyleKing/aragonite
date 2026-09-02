package overlay_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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
