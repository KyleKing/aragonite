// Package skin is the set of faces every screen in these tools draws with, so a
// title looks like a title in each of them and a reader moving between two is
// not relearning the same screen.
//
// It is deliberately the exception to the rule that rendering packages here
// carry no vocabulary of their own. That rule keeps a component from reaching
// for an application's palette; a skin is what an application builds and hands
// to those components, and having one shape for it is what stops three tools
// inventing three names for the same grey.
//
// The accent is what tells the tools apart. It colors the few things a screen
// is steered by (the cursor, the rail a comment hangs off, the key in a hint)
// and nothing else, so two tools are legibly different without being two
// themes. Every role that carries a color is paired with a glyph or a weight
// in the screens that use it, so NO_COLOR loses emphasis rather than meaning.
package skin

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/theme"
)

// Skin names the faces a screen asks for.
type Skin struct {
	// Title is the frame's own name, and the heading of a group of rows.
	Title lipgloss.Style
	// Subtitle is the counts and the state beside a title, and the footer.
	Subtitle lipgloss.Style
	// Heading names a file, a repository, a section: the thing rows hang off.
	Heading lipgloss.Style
	// Body is prose meant to be read, at the same weight as the content around
	// it. Anything quieter than this is not being read.
	Body lipgloss.Style
	// Muted is what is there to be looked past: line numbers, a hunk header,
	// the evidence under a comment.
	Muted lipgloss.Style
	// Accent is what the tool is steered by, and what tells it from its
	// siblings.
	Accent lipgloss.Style
	// Cursor marks where the keyboard is pointing. It is a glyph in the margin
	// rather than a reversed row: inverting a whole line means reading the
	// content through it.
	Cursor lipgloss.Style
	// Selection is the background of the row under the cursor, a tint quiet enough to read through.
	Selection lipgloss.Style
	// Key is a keyboard hint's bracketed letter.
	Key lipgloss.Style
	// Tab is a tab strip's unfocused segment: the name and count on a surface
	// one step above the background.
	Tab lipgloss.Style
	// TabActive is the segment being read: a filled chip in the accent, the
	// same shape as the cursor's own.
	TabActive lipgloss.Style
	// Badge is a count's chip on a surface one step above the background.
	Badge lipgloss.Style
	// BadgeActive is Badge for the count beside the segment being read.
	BadgeActive lipgloss.Style
	// Hairline is a rule between groups, quieter than an Edge.
	Hairline lipgloss.Style
	// Band is the background behind the row under the cursor, for faces drawn
	// on top of it. Subtext0 and brighter keep 4.5:1 contrast on it, Overlay2 does not.
	Band lipgloss.Style
	// Card is the background of a block raised off the frame.
	Card lipgloss.Style
	// Edge is the border of a raised block.
	Edge lipgloss.Style

	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
}

const (
	cardShare     = 0.75
	edgeShare     = 0.35
	selectionLift = 0.06
	selectionSat  = 0.35
	bandShare     = 0.16
)

// New builds a skin from a palette and the accent that names this tool.
func New(p theme.Palette, accent color.Color) Skin {
	s := p.Semantic()
	base := lipgloss.NewStyle()

	return Skin{
		Title:     base.Foreground(s.Text).Bold(true),
		Subtitle:  base.Foreground(p.Subtext0),
		Heading:   base.Foreground(p.Blue).Bold(true),
		Body:      base.Foreground(s.Text),
		Muted:     base.Foreground(p.Overlay1),
		Accent:    base.Foreground(accent),
		Cursor:    base.Foreground(accent).Bold(true),
		Selection: base.Background(theme.Tint(p.Base, accent, theme.Depth{Lift: selectionLift, Sat: selectionSat})),
		Key:       base.Foreground(s.Primary),
		Tab:       base.Foreground(p.Overlay1).Background(p.Surface0).Padding(0, 1),
		TabActive: base.Foreground(p.Base).Background(accent).Bold(true).
			Padding(0, 1),
		Badge:       base.Foreground(s.Text).Background(p.Surface1).Bold(true).Padding(0, 1),
		BadgeActive: base.Foreground(p.Base).Background(accent).Bold(true).Padding(0, 1),
		Hairline:    base.Foreground(p.Surface2),
		Band:        base.Background(theme.Mix(p.Base, p.Lavender, bandShare)),
		Card:        base.Background(theme.Mix(p.Base, p.Surface0, cardShare)),
		Edge:        base.Foreground(theme.Mix(p.Surface2, p.Overlay0, edgeShare)),
		Success:     base.Foreground(s.Success),
		Warning:     base.Foreground(s.Warning),
		Error:       base.Foreground(s.Error).Bold(true),
	}
}

// Detected builds a skin for whichever flavor the terminal's background calls
// for, which is the call every one of these tools makes on startup.
func Detected(accent func(theme.Palette) color.Color) Skin {
	p := theme.Detect()

	return New(p, accent(p))
}

// CursorBar is the glyph a cursor is drawn with, shared so the column a row
// spends on it is the same width everywhere.
const CursorBar = "▌"
