package theme_test

import (
	"image/color"
	"math"
	"testing"

	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/kyleking/aragonite/tui/theme"
)

func colorOf(t *testing.T, c color.Color) colorful.Color {
	t.Helper()

	got, ok := colorful.MakeColor(c)
	if !ok {
		t.Fatalf("color %v has no RGB value", c)
	}

	return got
}

func hueGap(a, b float64) float64 {
	gap := math.Abs(a - b)

	return min(gap, 360-gap)
}

// A tint keeps the accent's hue at Sat of its saturation, and moves the base's
// lightness Lift of the way toward middle grey: up from a dark base, down from
// a light one. Two accents on one base therefore stay apart by their hues.
func TestTint_KeepsTheAccentHueAndLiftsTowardMiddleGrey(t *testing.T) {
	t.Parallel()

	macchiato, latte := theme.Macchiato(), theme.Latte()
	depth := theme.Depth{Lift: 0.25, Sat: 0.35}

	tests := []struct {
		base   color.Color
		accent color.Color
		name   string
	}{
		{name: "blue on dark", base: macchiato.Base, accent: macchiato.Blue},
		{name: "red on dark", base: macchiato.Base, accent: macchiato.Red},
		{name: "green on dark", base: macchiato.Base, accent: macchiato.Green},
		{name: "mauve on dark", base: macchiato.Base, accent: macchiato.Mauve},
		{name: "blue on light", base: latte.Base, accent: latte.Blue},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tinted := theme.Tint(tc.base, tc.accent, depth)

			accentHue, accentSat, _ := colorOf(t, tc.accent).Hsl()
			tintHue, tintSat, tintL := colorOf(t, tinted).Hsl()
			_, _, baseL := colorOf(t, tc.base).Hsl()

			if gap := hueGap(tintHue, accentHue); gap > 1 {
				t.Errorf("hue moved %.2f degrees, want at most 1", gap)
			}

			if want := accentSat * depth.Sat; math.Abs(tintSat-want) > 0.02 {
				t.Errorf("saturation %.3f, want %.3f", tintSat, want)
			}

			if want := baseL + (0.5-baseL)*depth.Lift; math.Abs(tintL-want) > 0.005 {
				t.Errorf("lightness %.3f, want %.3f (base %.3f lifted %.2f toward 0.5)", tintL, want, baseL, depth.Lift)
			}
		})
	}
}

func TestMixReturnsTheEndpoints(t *testing.T) {
	t.Parallel()

	macchiato := theme.Macchiato()

	tests := []struct {
		want color.Color
		name string
		t    float64
	}{
		{name: "zero is the first color", t: 0, want: macchiato.Base},
		{name: "one is the second color", t: 1, want: macchiato.Text},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := colorOf(t, theme.Mix(macchiato.Base, macchiato.Text, tc.t)).Hex()
			want := colorOf(t, tc.want).Hex()

			if got != want {
				t.Errorf("Mix(t=%v) = %s, want %s", tc.t, got, want)
			}
		})
	}
}

func TestContrastMatchesTheWCAGDefinition(t *testing.T) {
	t.Parallel()

	black, white := color.RGBA{A: 0xff}, color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

	tests := []struct {
		a, b color.Color
		name string
		want float64
	}{
		{name: "black on white", a: black, b: white, want: 21},
		{name: "white on black", a: white, b: black, want: 21},
		{name: "identical", a: white, b: white, want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := theme.Contrast(tc.a, tc.b); math.Abs(got-tc.want) > 0.01 {
				t.Errorf("Contrast = %.3f, want %.1f", got, tc.want)
			}
		})
	}
}
