package skin

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/kyleking/aragonite/tui/theme"
)

// DiffFaces are what a changed line is drawn with, shared so an added line in
// one tool is the same added line in the next.
//
// A changed line carries a pale wash across the whole row, the runs inside it
// that differ from the line it pairs with carry the deeper mark, and a sign
// in the gutter says which side it is on. The sign is what survives NO_COLOR,
// where no tint can show, so it turns into + and − there.
type DiffFaces struct {
	AddWash, RemoveWash color.Color
	AddMark, RemoveMark color.Color
	// AddSign and RemoveSign color the gutter sign and an added or removed
	// line's number.
	AddSign, RemoveSign lipgloss.Style

	mono bool
}

// The two depths a changed line is drawn at in truecolor, as shares of its
// side's color laid over the background: a wash quiet enough to read syntax
// colors through, and a mark about twice as deep for the runs that differ.
// Catppuccin's editor themes wash at 0.18 and GitHub at 0.15; deeper than this
// the wash competes with the syntax colors drawn on it, and a mark deeper than
// 0.25 drops Latte's text under 4.5:1 on the red.
const (
	washShare = 0.12
	markShare = 0.25
)

// Diff builds the diff faces for a palette. Millions says whether the
// terminal mixes its own colors (truecolor) rather than quantizing into the
// 256-color cube; a non-empty NO_COLOR draws signs as + and −. DiffFor takes
// the profile itself, and is what a tool that has detected one calls.
func Diff(p theme.Palette, millions bool) DiffFaces {
	if millions {
		return DiffFor(p, colorprofile.TrueColor)
	}

	return DiffFor(p, colorprofile.ANSI256)
}

// DiffFor builds the diff faces fitted to profile by theme.Ground. In
// truecolor a changed line is its side's color laid over the background. In 256 colors both
// sides wash in one grey a step above the frame and mark in a deeper one, since
// the cube's dark greens and reds (22, 52) are saturated blocks, and the side is
// said by the sign and the line number, which keep their color. In 16 colors
// there is no wash or mark at all.
func DiffFor(p theme.Palette, profile colorprofile.Profile) DiffFaces {
	ground := func(c color.Color, lift theme.Lift) color.Color {
		return theme.Ground(c, p.Base, lift, profile)
	}

	base := lipgloss.NewStyle()

	return DiffFaces{
		AddWash:    ground(theme.Blend(p.Base, p.Green, washShare), theme.Wash),
		RemoveWash: ground(theme.Blend(p.Base, p.Red, washShare), theme.Wash),
		AddMark:    ground(theme.Blend(p.Base, p.Green, markShare), theme.Emphasis),
		RemoveMark: ground(theme.Blend(p.Base, p.Red, markShare), theme.Emphasis),
		AddSign:    base.Foreground(theme.Hue(p.Green, profile)),
		RemoveSign: base.Foreground(theme.Hue(p.Red, profile)),
		mono:       noColor(),
	}
}

// SignBar is the sign bar's glyph: a thin bar beside the line number, like a gitsigns
// column, so the side reads off the gutter without a tint behind the code.
const SignBar = "▎"

// Sign is the gutter glyph for an added or removed line, unstyled: the bar
// where color can say which side, + or − where it cannot.
func (d DiffFaces) Sign(added bool) string {
	switch {
	case !d.mono:
		return SignBar
	case added:
		return "+"
	default:
		return "−"
	}
}
