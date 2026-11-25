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

func TestSelectionIsATintInColorAndReverseVideoWithout(t *testing.T) {
	t.Parallel()

	ink := theme.Ink()

	tests := []struct {
		name    string
		mono    bool
		reverse bool
	}{
		{name: "color", mono: false, reverse: false},
		{name: "no color", mono: true, reverse: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := skin.BuildMono(ink, ink.Lavender, tc.mono)
			for name, face := range map[string]lipgloss.Style{"selection": s.Selection, "band": s.Band} {
				if got := face.GetReverse(); got != tc.reverse {
					t.Errorf("%s reverse is %v, want %v", name, got, tc.reverse)
				}

				_, none := face.GetBackground().(lipgloss.NoColor)
				if none != tc.reverse {
					t.Errorf("%s has no background is %v, want %v", name, none, tc.reverse)
				}
			}
		})
	}
}

// BenchmarkSelected is the face of one cell in the selected row, which a list
// asks for per cell per frame.
func BenchmarkSelected(b *testing.B) {
	s := skin.New(theme.Macchiato(), theme.Macchiato().Blue)
	st := s.Accent

	b.ReportAllocs()

	for b.Loop() {
		_ = s.Selected(st)
	}
}

// BenchmarkSelect is one selected row drawn.
func BenchmarkSelect(b *testing.B) {
	s := skin.New(theme.Macchiato(), theme.Macchiato().Blue)
	row := s.Accent.Render("○") + "  krpztump  " + s.Muted.Render("feat(cache): ttl")

	b.ReportAllocs()

	for b.Loop() {
		_ = s.Select(row, 80)
	}
}
