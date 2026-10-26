// Package split decides where a screen's detail region sits next to its main
// block and joins the two: beside it behind a one-cell rule on a wide terminal,
// beneath it when the reader opened it on a medium one, and nowhere when there
// is no room.
//
// Both blocks are drawn by renderers that receive the size they will occupy, so
// a caller renders once, at the final width, instead of drawing wide and
// clipping. Render is total: every line it returns is exactly width cells and
// there are exactly height of them, so a shorter frame leaves nothing of the
// last one on screen.
package split

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Placement is where the detail region goes.
type Placement int

// Placements. The zero value is None, so an unset Placement draws no detail.
const (
	None Placement = iota
	Below
	Side
)

// Spec is the geometry of one screen's split.
type Spec struct {
	// SideMin is the width from which the detail sits beside main, always.
	SideMin int
	// MainMax caps main's width when side by side. Zero splits the width evenly.
	MainMax int
	// MinWidth and MinHeight are the smallest terminal that has room for the
	// detail below main.
	MinWidth  int
	MinHeight int
	// BelowPercent is the share of the height the detail takes when below.
	BelowPercent int
}

// Renderer draws a block into at most width cells by height lines.
type Renderer func(width, height int) []string

// Styles are the faces the join draws with.
type Styles struct {
	// Rule draws the cell between side-by-side blocks.
	Rule lipgloss.Style
}

const (
	ruleGlyph = "│"
	ruleWidth = 1
	percent   = 100
)

// Place picks the placement for a width by height area. Side holds at or above
// SideMin whether or not open is set. Below that, the detail appears only when
// open is set and the area is at least MinWidth by MinHeight.
func (s Spec) Place(width, height int, open bool) Placement {
	switch {
	case width <= 0 || height <= 0:
		return None
	case width >= s.SideMin:
		return Side
	case !open || width < s.MinWidth || height < s.MinHeight:
		return None
	}

	return Below
}

// MainWidth is the width main is drawn at under p.
func (s Spec) MainWidth(p Placement, width int) int {
	if p != Side {
		return width
	}

	if s.MainMax > 0 {
		return min(s.MainMax, width-ruleWidth)
	}

	return (width - ruleWidth) / 2 //nolint:mnd // an even split
}

// DetailHeight is how many lines the detail takes under p.
func (s Spec) DetailHeight(p Placement, height int) int {
	switch p {
	case Below:
		return height * s.BelowPercent / percent
	case Side:
		return height
	case None:
		return 0
	}

	return 0
}

// Render draws main and detail under p into exactly height lines of width
// cells. Under None the detail renderer is not called.
func (s Spec) Render(p Placement, st Styles, width, height int, main, detail Renderer) []string {
	if width <= 0 || height <= 0 {
		return nil
	}

	mainWidth := s.MainWidth(p, width)

	switch p {
	case Side:
		return s.beside(st, width, height, main, detail)
	case Below:
		detailHeight := s.DetailHeight(p, height)
		mainHeight := height - detailHeight

		return append(fit(main(mainWidth, mainHeight), mainWidth, mainHeight),
			fit(detail(width, detailHeight), width, detailHeight)...)
	case None:
		return fit(main(mainWidth, height), width, height)
	}

	return fit(main(mainWidth, height), width, height)
}

func (s Spec) beside(st Styles, width, height int, main, detail Renderer) []string {
	mainWidth := s.MainWidth(Side, width)
	detailWidth := width - mainWidth - ruleWidth

	left := fit(main(mainWidth, height), mainWidth, height)
	right := fit(detail(detailWidth, height), detailWidth, height)
	rule := st.Rule.Render(ruleGlyph)

	out := make([]string, height)
	for i := range out {
		out[i] = left[i] + rule + right[i]
	}

	return out
}

func fit(lines []string, width, height int) []string {
	out := make([]string, max(height, 0))

	for i := range out {
		if i < len(lines) {
			out[i] = ansi.Truncate(lines[i], width, "")
		}

		if gap := width - lipgloss.Width(out[i]); gap > 0 {
			out[i] += strings.Repeat(" ", gap)
		}
	}

	return out
}
