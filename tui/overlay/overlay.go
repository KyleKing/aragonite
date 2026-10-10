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
	"github.com/charmbracelet/x/ansi"

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

// Box draws lines inside a thin titled border, showing at most height of them
// starting at at. The box is as wide as the widest line rather than the
// window's, so scrolling does not change its shape. Lines hidden above or
// below the window are marked in the border, which is where a reader looks
// for the frame anyway.
func Box(s Styles, title string, lines []string, height, at int) []string {
	width := 0
	for _, l := range lines {
		width = max(width, lipgloss.Width(l))
	}
	width = max(width, lipgloss.Width(title))

	at = max(0, min(at, len(lines)-height))

	visible := lines
	if at < len(visible) {
		visible = visible[at:]
	}
	if len(visible) > height {
		visible = visible[:height]
	}

	mark := func(more bool) string {
		if more {
			return " · · ·"
		}

		return ""
	}

	top := "╭─ " + title + mark(at > 0) + " "
	bot := "╰" + mark(at+len(visible) < len(lines))
	out := []string{
		s.Frame.Render(top + strings.Repeat("─", max(0, width+3-lipgloss.Width(top))) + "╮"),
	}

	for _, l := range visible {
		out = append(out, s.Frame.Render("│ ")+l+strings.Repeat(" ", width-lipgloss.Width(l))+s.Frame.Render(" │"))
	}

	return append(out, s.Frame.Render(bot+strings.Repeat("─", max(0, width+3-lipgloss.Width(bot)))+"╯"))
}

// Blit draws box over base at cell (x, y), cutting the cells it covers out of
// each base line. What the box does not reach is left alone, so a floating
// window shows the screen around it.
func Blit(base, box []string, x, y int) []string {
	out := make([]string, len(base))
	copy(out, base)

	for i, l := range box {
		row := y + i
		if row < 0 || row >= len(out) {
			continue
		}

		left := ansi.Cut(out[row], 0, x)
		if d := x - ansi.StringWidth(left); d > 0 {
			left += strings.Repeat(" ", d)
		}

		out[row] = left + l + ansi.Cut(out[row], x+ansi.StringWidth(l), len(out[row]))
	}

	return out
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
