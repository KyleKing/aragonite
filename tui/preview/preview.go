// Package preview draws the two-pane layout a screen uses to describe the row
// under its cursor: a column of labeled sections, and beside it — or beneath
// it where the width cannot hold both — a pane holding the item in full.
//
// The halves are drawn by pieces a caller composes: Section rows are rendered
// by the screen (they know what a row is), while Rule, Column, and the Detail
// pane own the chrome — the labeled rules, the heights each section earns,
// the vertical rule the halves split behind, and the scroll marker a windowed
// pane carries. The arithmetic lives here so a screen renders once, at the
// size it is given.
package preview

import (
	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/skin"
)

// Styles are the faces the panes draw with, handed in so the package never
// reaches for an application's palette.
type Styles struct {
	// Heading is a section's label and the detail pane's title.
	Heading lipgloss.Style
	// Rule is the hairline the labeled rules and the split's separator draw in.
	Rule lipgloss.Style
	// Accent is the rule and title of the half holding the keyboard.
	Accent lipgloss.Style
	// Muted is counts, jump keys, scroll markers, and empty notes.
	Muted lipgloss.Style
	// Label is a Field's label; Body its value and wrapped prose.
	Label lipgloss.Style
	Body  lipgloss.Style
}

// SkinStyles maps a skin onto the faces the panes ask for.
func SkinStyles(s skin.Skin) Styles {
	return Styles{
		Heading: s.Heading,
		Rule:    s.Hairline,
		Accent:  s.Accent,
		Muted:   s.Muted,
		Label:   s.Subtitle,
		Body:    s.Body,
	}
}

// Section is one labeled group in the column: its rule, the lines it would
// show given unlimited room, and how much of the available height it earns.
// Rows are drawn by the caller already styled and cut to width.
type Section struct {
	Heading string
	// Key is a jump key drawn at the rule's right edge, where the eye already
	// is. Empty draws none.
	Key  string
	Rows []string
	// Count is drawn beside the heading when Selectable is set.
	Count     int
	Relevance int
	// Selectable is false for a section that reports state rather than listing
	// items the cursor can land on; its count and key stay off the rule.
	Selectable bool
	// Elide cuts the middle of Rows too long for the height rather than their
	// end, for text whose newest lines are the last.
	Elide bool
}

// Relevance bounds. The scale is arbitrary; only the ratios matter, since
// height is shared in proportion to it: actionable state scores high, a clean
// or empty set the minimum, so a busy item arrives with its problems open.
const (
	Idle    = 1
	Present = 3
	Urgent  = 9
)

// Detail is what the pane says of the item under the cursor: its title and the
// lines describing it, scrolled to Scroll. Lines are drawn by the caller
// already styled, and may be longer than the pane.
type Detail struct {
	Title  string
	Lines  []string
	Scroll int
	// Focused says the pane holds the keyboard rather than the column beside
	// it: the title and the separating rule accent.
	Focused bool
}

const (
	ruleGlyph = "│"
	ruleWidth = 1
	percent   = 100
	// Both sides of the vertical rule take the padding.
	ruleSides = 2
)
