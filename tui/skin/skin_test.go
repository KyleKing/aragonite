package skin_test

import (
	"image/color"
	"testing"

	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

func TestInkTextHoldsReadableContrast(t *testing.T) {
	t.Parallel()

	const floor = 4.5

	ink := theme.Ink()
	band := skin.New(ink, ink.Lavender).Band.GetBackground()

	tests := []struct {
		fg   color.Color
		bg   color.Color
		name string
	}{
		{name: "overlay2 on base", fg: ink.Overlay2, bg: ink.Base},
		{name: "subtext0 on base", fg: ink.Subtext0, bg: ink.Base},
		{name: "subtext0 on band", fg: ink.Subtext0, bg: band},
		{name: "text on band", fg: ink.Text, bg: band},
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
