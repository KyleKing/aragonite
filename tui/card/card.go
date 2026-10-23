// Package card draws a block inline in a list or pane, with a rounded border named on top and captioned below.
// It is the inline sibling of overlay.Box, which centers a scrolling box over a frame.
package card

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	bodyChrome        = 4
	captionCells      = 2
	endCells          = 2
	minRun            = 1
	captionEdgeChrome = 2*endCells + captionCells
	noteEdgeChrome    = captionEdgeChrome + captionCells + minRun
)

const (
	sgrReset      = "\x1b[0m"
	sgrResetShort = "\x1b[m"
)

// Styles are the faces a card draws with, passed in so the package never reaches for a palette.
type Styles struct {
	// Edge colors the border characters.
	Edge lipgloss.Style
	// Fill is the background behind every cell of the card, border included.
	Fill lipgloss.Style
}

// Card is one block: a title and note on its top edge, body lines inside, and a foot on its bottom edge.
type Card struct {
	// Title sits at the left of the top edge, and gets room before Note does.
	Title string
	// Note sits at the right of the top edge, and may be empty.
	Note string
	// Foot sits at the left of the bottom edge, and may be empty.
	Foot string
	// Body is one styled line per row, cut to InnerWidth.
	Body []string
}

// InnerWidth is how many cells a body line has inside a card width cells wide.
func InnerWidth(width int) int {
	return max(0, width-bodyChrome)
}

// Render draws c as len(Body)+2 rows, each exactly width cells wide. A width under four
// cannot hold the border, so the rows are cut to width instead.
func (c Card) Render(s Styles, width int) []string {
	p := newPainter(s)
	inner := InnerWidth(width)

	rows := make([]string, 0, len(c.Body)+2)
	rows = append(rows, p.top(c.Title, c.Note, width))
	for _, line := range c.Body {
		rows = append(rows, p.body(truncate(line, inner), inner))
	}
	rows = append(rows, p.bottom(c.Foot, width))

	for i, row := range rows {
		rows[i] = fit(row, width, p.fill)
	}

	return rows
}

// newPainter reapplies the fill after each reset, so styled text keeps the card's background.
func newPainter(s Styles) painter {
	const marker = "x"

	prefix, _, _ := strings.Cut(s.Fill.Render(marker), marker)

	return painter{
		edge: s.Edge.Inherit(s.Fill),
		fill: s.Fill,
		reopen: strings.NewReplacer(
			sgrReset, sgrReset+prefix,
			sgrResetShort, sgrResetShort+prefix,
		),
	}
}

type painter struct {
	reopen *strings.Replacer
	edge   lipgloss.Style
	fill   lipgloss.Style
}

func (p painter) text(s string) string {
	return p.fill.Render(p.reopen.Replace(s))
}

func (p painter) top(title, note string, width int) string {
	titleRoom := width - captionEdgeChrome
	if note != "" {
		titleRoom = width - noteEdgeChrome
	}

	title = truncate(title, titleRoom)
	note = truncate(note, width-noteEdgeChrome-lipgloss.Width(title))

	head := p.edge.Render("╭─")
	if title != "" {
		head += p.text(" " + title + " ")
	}

	noteSeg := ""
	if note != "" {
		noteSeg = p.text(" " + note + " ")
	}

	run := max(0, width-lipgloss.Width(head)-lipgloss.Width(noteSeg)-endCells)

	return head + p.edge.Render(strings.Repeat("─", run)) + noteSeg + p.edge.Render("─╮")
}

func (p painter) body(line string, inner int) string {
	pad := strings.Repeat(" ", inner-lipgloss.Width(line))

	return p.edge.Render("│") + p.text(" "+line+pad+" ") + p.edge.Render("│")
}

func (p painter) bottom(foot string, width int) string {
	foot = truncate(foot, width-captionEdgeChrome)
	if foot == "" {
		return p.edge.Render("╰─" + strings.Repeat("─", max(0, width-2*endCells)) + "─╯")
	}

	head := p.edge.Render("╰─") + p.text(" "+foot+" ")
	run := max(0, width-lipgloss.Width(head)-endCells)

	return head + p.edge.Render(strings.Repeat("─", run)+"─╯")
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}

	return ansi.Truncate(s, n, "…")
}

func fit(row string, width int, fill lipgloss.Style) string {
	row = ansi.Truncate(row, width, "")

	return row + fill.Render(strings.Repeat(" ", max(0, width-lipgloss.Width(row))))
}
