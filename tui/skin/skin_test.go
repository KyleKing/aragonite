package skin_test

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

func TestInkTextHoldsReadableContrast(t *testing.T) {
	t.Parallel()

	const floor = 4.5

	ink := theme.Ink()
	s := skin.New(ink, ink.Lavender)
	band, panel := s.Band.GetBackground(), s.Panel.GetBackground()

	tests := []struct {
		fg   color.Color
		bg   color.Color
		name string
	}{
		{name: "overlay2 on base", fg: ink.Overlay2, bg: ink.Base},
		{name: "subtext0 on base", fg: ink.Subtext0, bg: ink.Base},
		{name: "subtext0 on band", fg: ink.Subtext0, bg: band},
		{name: "text on band", fg: ink.Text, bg: band},
		{name: "muted on panel", fg: s.Muted.GetForeground(), bg: panel},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := theme.Contrast(tc.fg, tc.bg); got < floor {
				t.Errorf("contrast is %.2f:1, want at least %.1f:1", got, floor)
			}
		})
	}
}

// NO_COLOR set to anything draws the selection and band in reverse video,
// where a tint cannot show; unset or empty, they are a tint.
func TestNew_SelectionIsATintInColorAndReverseVideoWithout(t *testing.T) {
	ink := theme.Ink()

	tests := []struct {
		name    string
		env     string
		reverse bool
	}{
		{name: "color", env: "", reverse: false},
		{name: "no color", env: "1", reverse: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tc.env)

			s := skin.New(ink, ink.Lavender)
			for name, face := range map[string]lipgloss.Style{"selection": s.Selection, "band": s.Band} {
				if got := face.GetReverse(); got != tc.reverse {
					t.Errorf("%s reverse is %v, want %v", name, got, tc.reverse)
				}

				_, none := face.GetBackground().(lipgloss.NoColor)
				if none != tc.reverse {
					t.Errorf("%s has no background is %v, want %v", name, none, tc.reverse)
				}
			}

			if got := s.Accent.GetForeground(); got != ink.Lavender {
				t.Errorf("the accent is %v, want the one New was given", got)
			}
		})
	}
}
