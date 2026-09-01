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
	// Key is a keyboard hint's bracketed letter.
	Key lipgloss.Style

	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
}

// New builds a skin from a palette and the accent that names this tool.
func New(p theme.Palette, accent color.Color) Skin {
	s := p.Semantic()
	base := lipgloss.NewStyle()

	return Skin{
		Title:    base.Foreground(s.Text).Bold(true),
		Subtitle: base.Foreground(p.Subtext0),
		Heading:  base.Foreground(p.Blue).Bold(true),
		Body:     base.Foreground(s.Text),
		Muted:    base.Foreground(p.Overlay1),
		Accent:   base.Foreground(accent),
		Cursor:   base.Foreground(accent).Bold(true),
		Key:      base.Foreground(s.Primary),
		Success:  base.Foreground(s.Success),
		Warning:  base.Foreground(s.Warning),
		Error:    base.Foreground(s.Error).Bold(true),
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
