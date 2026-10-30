// Package box draws content inside a bordered rectangle of exactly the cells it was given.
// A pane that stops being a rectangle (a wrapped line, an extra row) loses its bottom border
// or shifts its neighbors, so Render truncates the content instead of letting it overflow.
package box

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// BorderSize is the cells a border takes on each axis, which separates the size a box is
// given from the size its content gets.
const BorderSize = 2

// Styles are the two faces a box draws with, passed in so the package never reaches for a
// palette. Each carries its own border, so a focused box can differ in glyphs as well as
// color and focus survives NO_COLOR.
type Styles struct {
	// Idle draws a box that does not hold focus.
	Idle lipgloss.Style
	// Focused draws the box that holds focus.
	Focused lipgloss.Style
}

// Render draws content in a box of exactly width by height cells, border included.
// Lines past the inner height are dropped and lines past the inner width are cut with an
// ellipsis, never wrapped.
func Render(s Styles, width, height int, focused bool, content string) string {
	style := s.Idle
	if focused {
		style = s.Focused
	}

	inner := max(width-BorderSize, 0)
	rows := max(height-BorderSize, 0)

	lines := strings.Split(content, "\n")
	if len(lines) > rows {
		lines = lines[:rows]
	}

	for i, line := range lines {
		lines[i] = ansi.Truncate(line, inner, "…")
	}

	return style.Width(width).Height(height).MaxHeight(height).Render(strings.Join(lines, "\n"))
}
