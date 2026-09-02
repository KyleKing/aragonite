// Package overlay centers a block of content over a full-screen frame: a
// modal, or a pane too small for what it holds and promoted out of the layout.
//
// lipgloss.Place centers each line of a multi-line string against that line's
// own width, so a block whose lines differ in length staggers instead of
// centering as one. Padding to a uniform width first is the whole reason this
// is a package rather than a call to Place.
package overlay

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/table"
)

// Styles are the faces an overlay draws with, passed in so the package never
// reaches for an application's palette.
type Styles struct {
	// Frame draws the border and padding the content sits inside, and is what
	// decides how much room the content has.
	Frame lipgloss.Style
	// Elision draws the marker standing in for lines too tall to show.
	Elision lipgloss.Style
}

// ElisionMarker replaces the lines an overlay taller than its frame cannot
// show, so content is never dropped without saying so.
const ElisionMarker = "…"

// Center draws content inside s.Frame, centered on a frame width by height
// cells. Content taller or wider than the room inside the frame is clipped,
// so an overlay that sizes itself passes through untouched and one that does
// not still fits.
func Center(content string, width, height int, s Styles) string {
	inner := ContentWidth(width, s)

	framed := s.Frame.Render(clip(content, inner, ContentHeight(height, s), s))

	return lipgloss.Place(
		width, height, lipgloss.Center, lipgloss.Center, framed, lipgloss.WithWhitespaceChars(" "),
	)
}

// ContentWidth is the room content has inside the frame, which is what a
// caller wraps prose to before handing it over.
func ContentWidth(width int, s Styles) int {
	return width - s.Frame.GetHorizontalFrameSize()
}

// ContentHeight is how many rows content has inside the frame.
func ContentHeight(height int, s Styles) int {
	return height - s.Frame.GetVerticalFrameSize()
}

// clip bounds content to the room inside the frame and pads every line to the
// width of the longest, so Place centers the block rather than each line.
func clip(content string, width, height int, s Styles) string {
	if width < 1 || height < 1 {
		return content
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = table.Truncate(line, width)
	}

	if len(lines) > height {
		lines = append(lines[:height-1:height-1], s.Elision.Render(ElisionMarker))
	}

	widest := 0
	for _, line := range lines {
		widest = max(widest, lipgloss.Width(line))
	}

	return lipgloss.NewStyle().Width(widest).Render(strings.Join(lines, "\n"))
}
