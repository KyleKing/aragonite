// Package framecheck lints a rendered terminal frame for the alignment faults a
// golden test does not see: it compares bytes, so a frame that was wrong when it
// was recorded stays wrong forever and passes. The checks read the frame the way
// a terminal lays it out, one cell per column with a wide glyph taking two, using
// the same width function lipgloss measures with.
//
// Four rules, each a class of fault that has shipped:
//
//   - Width: a row wider than the frame wraps on a real terminal and pushes every
//     row under it down; a row short of the frame leaves the previous frame's
//     cells showing where a border was expected
//   - Corner: a box corner or tee whose arm has nothing to join, which is how a
//     box drawn one column off, or one missing its right edge, looks
//   - Edge: a box's side broken partway down by text, the trace of a row that
//     measured itself a cell narrower or wider than its neighbors
//   - Glyph: a glyph whose width terminals disagree on (glyph.Of): a text-default
//     emoji (⚠ ▶ ✖), a VS16 or ZWJ sequence, or a Nerd Font private-use icon,
//     any of which shifts or overlaps everything after it on the row without
//     any width function knowing. Content passes through glyph.Settle first
//
// A box that is covered by another (an overlay over a pane) is not broken: a side
// that runs into another box's border ends there.
//
// A revision graph (jj log) draws its lines with box glyphs and its revisions
// with node glyphs (○ ◆ @ × ~) that the lines run through: `├─╯` under `│ ○`
// branches into a node. Graph teaches Check that, so a log frame lints clean.
package framecheck

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"github.com/kyleking/aragonite/tui/glyph"
)

// Rule names which check a Fault broke.
type Rule string

// The rules Check applies.
const (
	RowWidth Rule = "width"
	Corner   Rule = "corner"
	Edge     Rule = "edge"
	Glyph    Rule = "glyph"
)

// Fault is one place a frame breaks a rule. Row and Col count from zero, in
// cells rather than bytes or runes.
type Fault struct {
	Rule   Rule
	Detail string
	Row    int
	Col    int
}

func (f Fault) String() string {
	return fmt.Sprintf("%d:%d %s: %s", f.Row+1, f.Col+1, f.Rule, f.Detail)
}

type options struct {
	allow    map[string]bool
	nodes    map[string]bool
	view     *viewport
	nerd     bool
	width    int
	maxWidth int
}

// viewport is the rows of a frame that scroll, first and last inclusive.
type viewport struct{ top, bottom int }

// Option adjusts what Check expects of a frame.
type Option func(*options)

// Width says every row is exactly w cells: the frame is a whole screen, so a
// short row leaves stale cells and a long one wraps.
func Width(w int) Option { return func(o *options) { o.width = w } }

// MaxWidth says no row is wider than w cells, for a block that is placed inside
// a screen and may be ragged on the right.
func MaxWidth(w int) Option { return func(o *options) { o.maxWidth = w } }

// Allow accepts glyphs the Glyph rule would otherwise report, for a frame that
// draws one deliberately and has checked it in the terminals it targets.
func Allow(glyphs ...string) Option {
	return func(o *options) {
		for _, g := range glyphs {
			o.allow[g] = true
		}
	}
}

// Viewport says rows top through bottom (counted from zero, inclusive) are a
// scrolled body between fixed rows, a header above and a footer below. A box
// the scroll has cut across is clipped there rather than broken, so a side
// running into the viewport's last row ends there. A box edge with none of its
// own rows inside the viewport is still a fault: a top edge alone on the last
// row, or a bottom edge alone on the first, draws a lid on nothing.
func Viewport(top, bottom int) Option {
	return func(o *options) { o.view = &viewport{top: top, bottom: bottom} }
}

// GraphNodes are the glyphs jj's default log templates draw a revision with: @ the
// working copy, ○ a mutable revision, ◆ an immutable one, × a conflicted one, ~
// elided history, and the ◉ ● ◌ ◇ older releases and hidden revisions use.
const GraphNodes = "@○◆×~◉●◌◇"

// Graph says the frame draws a revision graph, so a node glyph joins the graph
// lines above and below it as a │ would: `├─╯` under `│ ○` branches into the
// node rather than leaving an arm with nothing to join. The nodes are
// GraphNodes plus any extra a tool's template draws. It is an option rather
// than the default because outside a graph a node glyph is text (`@` in a
// sentence), and a box corner that runs into text is a fault.
func Graph(extra ...string) Option {
	return func(o *options) {
		if o.nodes == nil {
			o.nodes = map[string]bool{}
		}

		for _, r := range GraphNodes {
			o.nodes[string(r)] = true
		}

		for _, g := range extra {
			o.nodes[g] = true
		}
	}
}

// Nerd accepts private-use glyphs, for a frame drawn with the Nerd icon set a
// reader opted into. Each one must still be drawn through icons.Glyph.Prefix,
// with the blank cell its overflow lands on.
func Nerd() Option { return func(o *options) { o.nerd = true } }

// Check returns every fault in frame, which may carry ANSI escapes. A frame
// with none returns nil.
func Check(frame string, opts ...Option) []Fault {
	o := options{allow: map[string]bool{}}
	for _, opt := range opts {
		opt(&o)
	}

	g := newGrid(frame)
	g.nodes = o.nodes

	return slices.Concat(g.widths(o), g.glyphs(o), g.corners(o), g.edges(o))
}

// TB is the part of testing.TB that Require reports through.
type TB interface {
	Helper()
	Errorf(format string, args ...any)
}

// Require reports every fault in frame through t, naming the frame and printing
// it with a column ruler so the fault can be found by eye.
func Require(t TB, name, frame string, opts ...Option) {
	t.Helper()

	faults := Check(frame, opts...)
	if len(faults) == 0 {
		return
	}

	var b strings.Builder
	for _, f := range faults {
		b.WriteString("\n  " + f.String())
	}

	t.Errorf("%s: %d alignment fault(s):%s\n%s", name, len(faults), b.String(), Ruled(frame))
}

// Ruled returns frame stripped of escapes, with a column ruler above it and a
// row number before each row, which is what a fault's position is read against.
func Ruled(frame string) string {
	lines := strings.Split(ansi.Strip(frame), "\n")
	widest := 0

	for _, l := range lines {
		widest = max(widest, ansi.StringWidth(l))
	}

	const decade = 10

	var ruler strings.Builder
	for c := range widest {
		if c%decade == 0 {
			ruler.WriteString(strconv.Itoa(c / decade % decade))
		} else {
			ruler.WriteString(" ")
		}
	}

	var b strings.Builder
	b.WriteString("     " + ruler.String() + "\n")

	for i, l := range lines {
		fmt.Fprintf(&b, "%4d %s\n", i+1, l)
	}

	return b.String()
}

// cell is one column of a row. A wide glyph's second column is a cell with an
// empty text and wide set, so columns index cells directly.
type cell struct {
	text string
	wide bool
}

type grid struct {
	rows  [][]cell
	nodes map[string]bool
}

// arms is the directions the glyph s joins in this frame: a box glyph's own
// arms, and up and down for a graph node when the frame is a Graph.
func (g grid) arms(s string) (arm, bool) {
	if a, ok := joins[s]; ok {
		return a, true
	}

	if g.nodes[s] {
		return up | down, true
	}

	return 0, false
}

func newGrid(frame string) grid {
	lines := strings.Split(ansi.Strip(frame), "\n")
	g := grid{rows: make([][]cell, len(lines))}

	for i, l := range lines {
		state := -1
		rest := l

		for rest != "" {
			var cluster string
			var w int

			cluster, rest, w, state = uniseg.FirstGraphemeClusterInString(rest, state)
			switch w {
			case 0:
				// A zero-width cluster (a lone joiner) has no cell of its own.
				if n := len(g.rows[i]); n > 0 {
					g.rows[i][n-1].text += cluster
				}
			case 1:
				g.rows[i] = append(g.rows[i], cell{text: cluster})
			default:
				g.rows[i] = append(g.rows[i], cell{text: cluster})
				for range w - 1 {
					g.rows[i] = append(g.rows[i], cell{wide: true})
				}
			}
		}
	}

	return g
}

// at is the cell's text at (row, col), empty off the frame.
func (g grid) at(row, col int) string {
	if row < 0 || row >= len(g.rows) || col < 0 || col >= len(g.rows[row]) {
		return ""
	}

	return g.rows[row][col].text
}

func (g grid) widths(o options) []Fault {
	var faults []Fault

	for i, r := range g.rows {
		w := len(r)
		switch {
		case o.width > 0 && w != o.width:
			faults = append(faults, Fault{
				Rule: RowWidth, Row: i, Col: min(w, o.width),
				Detail: fmt.Sprintf("row is %d cells, frame is %d", w, o.width),
			})
		case o.maxWidth > 0 && w > o.maxWidth:
			faults = append(faults, Fault{
				Rule: RowWidth, Row: i, Col: o.maxWidth,
				Detail: fmt.Sprintf("row is %d cells, more than %d", w, o.maxWidth),
			})
		}
	}

	return faults
}

func (g grid) glyphs(o options) []Fault {
	var faults []Fault

	for i, r := range g.rows {
		for j, c := range r {
			if c.wide || o.allow[c.text] {
				continue
			}

			switch class := glyph.Of(c.text); {
			case class == glyph.Stable, class == glyph.Wide, class == glyph.Private && o.nerd:
			default:
				faults = append(faults, Fault{
					Rule: Glyph, Row: i, Col: j,
					Detail: fmt.Sprintf("%q (U+%04X) is %s", c.text, first(c.text), class),
				})
			}
		}
	}

	return faults
}

// corners reports every corner or tee with an arm that joins nothing. An arm
// pointing off the frame is clipped rather than broken.
func (g grid) corners(o options) []Fault {
	var faults []Fault

	for i, r := range g.rows {
		for j, c := range r {
			arms, ok := joins[c.text]
			if !ok || arms.straight() {
				continue
			}

			loose := g.loose(i, j, arms)
			if loose == "" {
				continue
			}

			detail := fmt.Sprintf("%s has an arm with nothing to join: %s", c.text, loose)
			if v := o.view; v != nil && (i == v.bottom && arms&down != 0 || i == v.top && arms&up != 0) {
				detail = fmt.Sprintf("%s is a box edge on the viewport's edge with none of its box inside: %s",
					c.text, loose)
			}

			faults = append(faults, Fault{Rule: Corner, Row: i, Col: j, Detail: detail})
		}
	}

	return faults
}

func first(s string) rune {
	r, _ := utf8.DecodeRuneInString(s)

	return r
}

// loose names the arms of the glyph at (row, col) that join nothing, with what
// is there instead.
func (g grid) loose(row, col int, arms arm) string {
	var loose []string

	for _, d := range directions {
		if arms&d.arm == 0 {
			continue
		}

		ni, nj := row+d.dRow, col+d.dCol
		if ni < 0 || ni >= len(g.rows) || nj < 0 || d.dRow == 0 && nj >= len(g.rows[ni]) {
			continue
		}

		if a, _ := g.arms(g.at(ni, nj)); a&d.back == 0 {
			loose = append(loose, fmt.Sprintf("%s (%q)", d.name, g.at(ni, nj)))
		}
	}

	return strings.Join(loose, ", ")
}

// edges follows each box's sides down from its top corners and reports a side
// that text interrupts before it reaches a bottom corner. A side that meets
// another box's border is covered, and one that reaches the last row, or the
// last row of a Viewport, is clipped.
func (g grid) edges(o options) []Fault {
	var faults []Fault

	for i, r := range g.rows {
		for j, c := range r {
			if joins[c.text]&down == 0 || joins[c.text]&up != 0 {
				continue
			}

			last := len(g.rows) - 1
			if v := o.view; v != nil && i >= v.top && i <= v.bottom {
				last = v.bottom
			}

			if f, ok := g.trace(i, j, last); !ok {
				faults = append(faults, f)
			}
		}
	}

	return faults
}

func (g grid) trace(row, col, last int) (Fault, bool) {
	for i := row + 1; i <= last; i++ {
		here := g.at(i, col)
		arms, isBox := g.arms(here)

		switch {
		case isBox && arms&up != 0 && arms&down == 0:
			return Fault{}, true // the bottom corner
		case isBox && arms&up != 0:
			continue
		case isBox:
			return Fault{}, true // covered by another box's border
		}

		// The first row under the corner is the Corner rule's to report.
		if i == row+1 {
			return Fault{}, true
		}

		return Fault{
			Rule: Edge, Row: i, Col: col,
			Detail: fmt.Sprintf("side from %s at %d:%d stops at %q", g.at(row, col), row+1, col+1, here),
		}, false
	}

	return Fault{}, true
}
