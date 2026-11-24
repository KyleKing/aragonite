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
	"reflect"

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
	Face lipgloss.Style
	// stock is the color a stock constructor drew Face in, set only by them, so
	// a bar can draw every stock mark of one glyph and color from one render.
	stock color.Color
	Glyph string
	Row   int
	Rank  int
}

// Bar is the track for one frame: what each of its cells draws, top to bottom.
// A nil Bar means the content fits and no track is drawn.
type Bar struct {
	drawn map[drawKey]string
	track lipgloss.Style
	thumb lipgloss.Style
	cells []cell
	total int
}

type cell struct {
	face   lipgloss.Style
	stock  color.Color
	glyph  string
	rank   int
	thumb  bool
	marked bool
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
		if c.thumb || (c.marked && c.rank >= m.Rank) {
			continue
		}

		c.glyph, c.face, c.rank, c.marked, c.stock = m.Glyph, m.Face, m.Rank, true, m.stock
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

// Cell renders the track at row i of the frame. A bar draws the track, the
// thumb, and each stock mark's glyph and color once, since a frame's cells
// repeat a handful of them and a lipgloss render costs microseconds. A mark whose
// Face no longer has its stock color, or that was built by hand, renders afresh.
func (b *Bar) Cell(i int) string {
	if b == nil || i < 0 || i >= len(b.cells) {
		return ""
	}

	c := &b.cells[i]

	switch {
	case c.thumb:
		return b.draw(drawKey{glyph: Thumb, part: partThumb}, &b.thumb)
	case c.marked && keyable(c.stock) && c.face.GetForeground() == c.stock:
		return b.draw(drawKey{glyph: c.glyph, color: c.stock, part: partMark}, &c.face)
	case c.marked:
		return c.face.Render(c.glyph)
	default:
		return b.draw(drawKey{glyph: Track, part: partTrack}, &b.track)
	}
}

func (b *Bar) draw(key drawKey, face *lipgloss.Style) string {
	if out, seen := b.drawn[key]; seen {
		return out
	}

	if b.drawn == nil {
		b.drawn = map[drawKey]string{}
	}

	out := face.Render(key.glyph)
	b.drawn[key] = out

	return out
}

// drawKey names a drawn cell: the track and the thumb by their part, since the
// bar holds one face for each, and a stock mark by its glyph and color.
type drawKey struct {
	color color.Color
	glyph string
	part  uint8
}

// The parts of a bar a drawKey tells apart.
const (
	partTrack uint8 = iota
	partThumb
	partMark
)

// keyable says whether c can key a map, which a color type holding a slice cannot.
func keyable(c color.Color) bool {
	return c != nil && reflect.TypeOf(c).Comparable()
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
	return Mark{Row: row, Glyph: GlyphChange, Face: lipgloss.NewStyle().Foreground(c), Rank: RankChange, stock: c}
}

// Act marks a row waiting on the reader: an open finding, a thread to answer.
func Act(row int, c color.Color) Mark {
	return Mark{Row: row, Glyph: GlyphAct, Face: lipgloss.NewStyle().Foreground(c), Rank: RankAct, stock: c}
}

// Draft marks a row carrying something the reader wrote and has not sent.
func Draft(row int, c color.Color) Mark {
	return Mark{Row: row, Glyph: GlyphDraft, Face: lipgloss.NewStyle().Foreground(c), Rank: RankDraft, stock: c}
}

// Done marks a row already handled: a read hunk, a resolved thread.
func Done(row int, c color.Color) Mark {
	return Mark{Row: row, Glyph: GlyphDone, Face: lipgloss.NewStyle().Foreground(c), Rank: RankDone, stock: c}
}

// Found marks a search match.
func Found(row int, c color.Color) Mark {
	return Mark{Row: row, Glyph: GlyphFound, Face: lipgloss.NewStyle().Foreground(c), Rank: RankChange, stock: c}
}
