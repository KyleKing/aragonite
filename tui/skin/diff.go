package skin

import (
	"image/color"

	"charm.land/lipgloss/v2"

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

// The two depths a changed line is drawn at. A 256-color terminal gets deeper
// values, since the cube carries almost no dark tints: held back there, added
// and removed both quantize into the grey ramp and the mark says nothing.
const (
	trueMarkLift, trueMarkSat = 0.28, 0.70
	trueWashLift, trueWashSat = 0.10, 0.42
	cubeMarkLift              = 0.34
	cubeWashLift              = 0.12
	cubeSat                   = 1.0
)

// Diff builds the diff faces for a palette. Millions says whether the
// terminal mixes its own colors (truecolor) rather than quantizing into the
// 256-color cube; a non-empty NO_COLOR draws signs as + and −.
func Diff(p theme.Palette, millions bool) DiffFaces {
	mark := theme.Depth{Lift: cubeMarkLift, Sat: cubeSat}
	wash := theme.Depth{Lift: cubeWashLift, Sat: cubeSat}

	if millions {
		mark = theme.Depth{Lift: trueMarkLift, Sat: trueMarkSat}
		wash = theme.Depth{Lift: trueWashLift, Sat: trueWashSat}
	}

	base := lipgloss.NewStyle()

	return DiffFaces{
		AddWash:    theme.Tint(p.Base, p.Green, wash),
		RemoveWash: theme.Tint(p.Base, p.Red, wash),
		AddMark:    theme.Tint(p.Base, p.Green, mark),
		RemoveMark: theme.Tint(p.Base, p.Red, mark),
		AddSign:    base.Foreground(p.Green),
		RemoveSign: base.Foreground(p.Red),
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
