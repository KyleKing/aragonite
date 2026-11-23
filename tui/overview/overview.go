// Package overview draws the one-column track on a frame's right edge that
// says how much of the content is on screen, where in it the frame sits, and
// what kind of thing waits off screen at each height: a change, an open
// finding, a draft, an unread hunk.
//
// The track is shared by the tools so a mark means the same thing in each. The
// package carries no palette of its own: a Mark names its glyph, its face, and
// how much it matters, and the caller's skin decides what that looks like.
package overview

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// The track's own glyphs. Both the track and the thumb are drawn, so where the
// frame sits reads off the thumb's length as well as its position, and a
// terminal with no color still shows it.
const (
	Track = "│"
	Thumb = "┃"
)

// Mark is one row worth knowing the position of.
type Mark struct {
	Face  lipgloss.Style
	Glyph string
	Row   int
	Rank  int
}

// Bar is the track for one frame: what each of its cells draws, top to bottom.
// A nil Bar means the content fits and no track is drawn.
type Bar struct {
	track lipgloss.Style
	thumb lipgloss.Style
	cells []cell
	total int
}

type cell struct {
	face  *lipgloss.Style
	glyph string
	rank  int
	thumb bool
}

// New lays out the track for a frame height rows tall over total rows of
// content scrolled offset rows down. Content that fits gets no track: a
// full-height thumb says the same as no bar at all and costs a column.
func New(height, total, offset int, track, thumb lipgloss.Style) *Bar {
	if height < 1 || total <= height {
		return nil
	}

	size := max(1, height*height/total)
	// The thumb reaches the bottom exactly when the last row is on screen, so
	// its position is measured against how far the offset can travel.
	travel := max(1, total-height)
	at := min(max(0, (height-size)*offset/travel), height-size)

	b := &Bar{cells: make([]cell, height), total: total, track: track, thumb: thumb}
	for i := range b.cells {
		b.cells[i].thumb = i >= at && i < at+size
	}

	return b
}

// Place puts marks onto the track by where their rows sit in the content. The
// thumb wins a collision, since a row inside the frame is one the reader can
// already see, and between two marks the higher Rank wins.
func (b *Bar) Place(marks ...Mark) *Bar {
	if b == nil {
		return nil
	}

	for k := range marks {
		m := &marks[k]
		i := m.Row * len(b.cells) / b.total
		if i < 0 || i >= len(b.cells) {
			continue
		}

		c := &b.cells[i]
		if c.thumb || (c.face != nil && c.rank >= m.Rank) {
			continue
		}

		face := m.Face
		c.glyph, c.face, c.rank = m.Glyph, &face, m.Rank
	}

	return b
}

// Height is the number of cells, which is the frame height, or 0 for no bar.
func (b *Bar) Height() int {
	if b == nil {
		return 0
	}

	return len(b.cells)
}

// Width is the columns the bar takes from each row: 1, or 0 when there is none.
func (b *Bar) Width() int {
	if b == nil {
		return 0
	}

	return 1
}

// Cell renders the track at row i of the frame.
func (b *Bar) Cell(i int) string {
	if b == nil || i < 0 || i >= len(b.cells) {
		return ""
	}

	c := b.cells[i]

	switch {
	case c.thumb:
		return b.thumb.Render(Thumb)
	case c.face != nil:
		return c.face.Render(c.glyph)
	default:
		return b.track.Render(Track)
	}
}

// Lines renders every cell, top to bottom.
func (b *Bar) Lines() []string {
	out := make([]string, b.Height())
	for i := range out {
		out[i] = b.Cell(i)
	}

	return out
}

// The glyphs the stock marks draw, chosen so each reads without color: a
// change is a bar like the sign column beside the line, a thing to act on is
// filled, and a thing already handled is hollow.
const (
	GlyphChange = "▍"
	GlyphAct    = "◆"
	GlyphDraft  = "●"
	GlyphDone   = "○"
	GlyphFound  = "•"
)

// Stock ranks, highest first in what a reader needs to find: something to act
// on outranks a draft, which outranks a change, which outranks a handled row.
const (
	RankDone = iota + 1
	RankChange
	RankDraft
	RankAct
)

// Change marks a changed line in the color of its kind: the added or the
// removed face, so the track says which way a hunk leans.
func Change(row int, c color.Color) Mark {
	return Mark{Row: row, Glyph: GlyphChange, Face: lipgloss.NewStyle().Foreground(c), Rank: RankChange}
}

// Act marks a row waiting on the reader: an open finding, a thread to answer.
func Act(row int, c color.Color) Mark {
	return Mark{Row: row, Glyph: GlyphAct, Face: lipgloss.NewStyle().Foreground(c), Rank: RankAct}
}

// Draft marks a row carrying something the reader wrote and has not sent.
func Draft(row int, c color.Color) Mark {
	return Mark{Row: row, Glyph: GlyphDraft, Face: lipgloss.NewStyle().Foreground(c), Rank: RankDraft}
}

// Done marks a row already handled: a read hunk, a resolved thread.
func Done(row int, c color.Color) Mark {
	return Mark{Row: row, Glyph: GlyphDone, Face: lipgloss.NewStyle().Foreground(c), Rank: RankDone}
}

// Found marks a search match.
func Found(row int, c color.Color) Mark {
	return Mark{Row: row, Glyph: GlyphFound, Face: lipgloss.NewStyle().Foreground(c), Rank: RankChange}
}
