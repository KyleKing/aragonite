package theme

import (
	"image/color"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// Depth sets how far a tint lifts a background toward middle grey and how much of
// the accent's saturation it keeps.
type Depth struct {
	// Lift is the share of the distance to middle grey that the lightness moves.
	Lift float64
	// Sat is the share of the accent's saturation that is kept.
	Sat float64
}

// Tint returns base's lightness carrying accent's hue at d.Sat of its saturation,
// lifted d.Lift toward middle grey. It keeps the hue rather than mixing the two colors,
// so two accents tinted onto one background stay apart.
func Tint(base, accent color.Color, d Depth) color.Color {
	const middleGrey = 0.5

	from, _ := colorful.MakeColor(base)
	to, _ := colorful.MakeColor(accent)

	hue, sat, _ := to.Hsl()
	_, _, dark := from.Hsl()

	return colorful.Hsl(hue, sat*d.Sat, dark+(middleGrey-dark)*d.Lift).Clamped()
}

// Mix returns the blend t of the way from a to b in CIE Lab, so 0 is a and 1 is b.
func Mix(a, b color.Color, t float64) color.Color {
	from, _ := colorful.MakeColor(a)
	to, _ := colorful.MakeColor(b)

	return from.BlendLab(to, t).Clamped()
}
