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
// is steered by (the selected row, the rail a comment hangs off, the key in a hint)
// and nothing else, so two tools are legibly different without being two
// themes. Every role that carries a color is paired with a glyph or a weight
// in the screens that use it, so NO_COLOR loses emphasis rather than meaning.
package skin

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

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
	// Cursor is the accent weight for text that names where the keyboard is
	// pointing. The row itself is marked by Selection.
	Cursor lipgloss.Style
	// Selection is the full-row face of the row under the cursor: a tint quiet
	// enough to read through, and reverse video under NO_COLOR, where a tint
	// cannot show.
	Selection lipgloss.Style
	// Key is a keyboard hint's bracketed letter.
	Key lipgloss.Style
	// Suggest is a key something is pointing the reader at: the next step a
	// coach names. It is the accent without weight, and the screens that use it
	// draw a ▸ beside it so NO_COLOR keeps the meaning.
	Suggest lipgloss.Style
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
	// on top of it, and reverse video under NO_COLOR. Subtext0 and brighter keep
	// 4.5:1 contrast on it, Overlay2 does not.
	Band lipgloss.Style
	// Card is the background of a block raised off the frame.
	Card lipgloss.Style
	// Panel is the background of the band pinned above a footer, which Muted
	// text keeps 4.5:1 contrast on.
	Panel lipgloss.Style
	// Edge is the border of a raised block.
	Edge lipgloss.Style
	// FileBand is the background of the row naming a file above its lines: a
	// step lifted off the frame, so files separate by depth rather than by
	// another rule. It is reverse video under NO_COLOR.
	FileBand lipgloss.Style

	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
}

const (
	cardShare     = 0.75
	edgeShare     = 0.7
	selectionLift = 0.06
	selectionSat  = 0.35
	bandShare     = 0.16
	badgeShare    = 0.5
	fileBandShare = 0.85
	noColorEnv    = "NO_COLOR"
)

// New builds a skin from a palette and the accent that names this tool, for a
// terminal that mixes its own colors. A non-empty NO_COLOR draws Selection and
// Band in reverse video.
func New(p theme.Palette, accent color.Color) Skin {
	return build(p, accent, colorprofile.TrueColor, noColor())
}

// For builds the skin New does, with every background fitted to profile by
// theme.Ground: blended tints in truecolor, steps of the grey ramp in 256 colors,
// and none in 16, where Selection and Band fall back to reverse video as they do
// under NO_COLOR. Profile is what colorprofile.Detect answers for the output the
// program draws to, which is the same answer Bubble Tea downsamples by.
func For(p theme.Palette, accent color.Color, profile colorprofile.Profile) Skin {
	return build(p, accent, profile, noColor())
}

func build(p theme.Palette, accent color.Color, profile colorprofile.Profile, mono bool) Skin {
	s := p.Semantic()
	base := lipgloss.NewStyle()
	ground := func(c color.Color, lift theme.Lift) color.Color {
		return theme.Ground(c, p.Base, lift, profile)
	}

	fileBand := base.Background(ground(theme.Mix(p.Base, p.Surface0, fileBandShare), theme.Wash))
	selection := base.Background(ground(
		theme.Tint(p.Base, accent, theme.Depth{Lift: selectionLift, Sat: selectionSat}), theme.Cursor,
	))
	band := base.Background(ground(theme.Mix(p.Base, p.Lavender, bandShare), theme.Cursor))

	if mono || profile <= colorprofile.ANSI && profile != colorprofile.Unknown {
		selection, band = base.Reverse(true), base.Reverse(true)
	}

	if mono {
		fileBand = base.Reverse(true)
	}

	return Skin{
		Title:     base.Foreground(s.Text).Bold(true),
		Subtitle:  base.Foreground(p.Subtext0),
		Heading:   base.Foreground(p.Blue).Bold(true),
		Body:      base.Foreground(s.Text),
		Muted:     base.Foreground(p.Overlay1),
		Accent:    base.Foreground(accent),
		Cursor:    base.Foreground(accent).Bold(true),
		Selection: selection,
		Key:       base.Foreground(s.Primary),
		Suggest:   base.Foreground(accent),
		Tab:       base.Foreground(p.Overlay1).Background(ground(p.Surface0, theme.Wash)).Padding(0, 1),
		TabActive: base.Foreground(p.Base).Background(accent).Bold(true).
			Padding(0, 1),
		Badge: base.Foreground(s.Text).Bold(true).Padding(0, 1).
			Background(ground(theme.Mix(p.Surface0, p.Surface1, badgeShare), theme.Emphasis)),
		BadgeActive: base.Foreground(p.Base).Background(accent).Bold(true).Padding(0, 1),
		Hairline:    base.Foreground(p.Surface2),
		Band:        band,
		Card:        base.Background(ground(theme.Mix(p.Base, p.Surface0, cardShare), theme.Surface)),
		Panel:       base.Background(ground(p.Mantle, theme.Sunk)),
		Edge:        base.Foreground(theme.Mix(p.Surface2, p.Overlay0, edgeShare)),
		FileBand:    fileBand,
		Success:     base.Foreground(s.Success),
		Warning:     base.Foreground(s.Warning),
		Error:       base.Foreground(s.Error).Bold(true),
	}
}

func noColor() bool { return os.Getenv(noColorEnv) != "" }

// Detected builds a skin for whichever flavor the terminal's background calls
// for, fitted to the color profile stdout supports, which is the call every one
// of these tools makes on startup.
func Detected(accent func(theme.Palette) color.Color) Skin {
	p := theme.Detect()

	return For(p, accent(p), colorprofile.Detect(os.Stdout, os.Environ()))
}
