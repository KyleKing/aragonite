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

func TestTintKeepsTheAccentHueAndLiftsTowardMiddleGrey(t *testing.T) {
	t.Parallel()

	macchiato := theme.Macchiato()
	depth := theme.Depth{Lift: 0.06, Sat: 0.35}

	tests := []struct {
		accent color.Color
		name   string
	}{
		{name: "blue", accent: macchiato.Blue},
		{name: "red", accent: macchiato.Red},
		{name: "green", accent: macchiato.Green},
		{name: "mauve", accent: macchiato.Mauve},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tinted := theme.Tint(macchiato.Base, tc.accent, depth)

			accentHue, _, _ := colorOf(t, tc.accent).Hsl()
			tintHue, _, tintL := colorOf(t, tinted).Hsl()
			_, _, baseL := colorOf(t, macchiato.Base).Hsl()

			if gap := hueGap(tintHue, accentHue); gap > 1 {
				t.Errorf("hue moved %.2f degrees, want at most 1", gap)
			}

			lo, hi := min(baseL, 0.5), max(baseL, 0.5)
			if tintL < lo || tintL > hi {
				t.Errorf("lightness %.3f is outside [%.3f, %.3f]", tintL, lo, hi)
			}
		})
	}
}

func TestTintKeepsTwoAccentsApart(t *testing.T) {
	t.Parallel()

	macchiato := theme.Macchiato()
	depth := theme.Depth{Lift: 0.06, Sat: 0.35}

	blue := colorOf(t, theme.Tint(macchiato.Base, macchiato.Blue, depth)).Hex()
	red := colorOf(t, theme.Tint(macchiato.Base, macchiato.Red, depth)).Hex()

	if blue == red {
		t.Errorf("blue and red tinted onto one background both became %s", blue)
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
