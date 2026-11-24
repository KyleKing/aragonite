package skin_test

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

// bold matches an SGR sequence carrying the bold parameter.
var bold = regexp.MustCompile(`\x1b\[(?:[0-9;]*;)?1(?:;[0-9;]*)?m`)

func hasColor(s string) bool {
	for _, seq := range regexp.MustCompile(`\x1b\[([0-9;]*)m`).FindAllStringSubmatch(s, -1) {
		for _, p := range strings.Split(seq[1], ";") {
			switch {
			case len(p) == 2 && (p[0] == '3' || p[0] == '4' || p[0] == '9') && p[1] <= '8':
				return true
			case len(p) == 3 && p[:2] == "10":
				return true
			}
		}
	}

	return false
}

// In 16 colors the selected row is reverse video, and a colored cell inside it
// (a status letter, a +3) reversed is a small block of its color in the bar.
// Every face inside the row drops its colors there; in truecolor and 256 colors
// the row is a tint and the cells keep theirs.
func TestSelected_SixteenColorsIsOneEvenBar(t *testing.T) {
	t.Parallel()

	p := theme.Macchiato()
	green := lipgloss.NewStyle().Foreground(p.Green).Bold(true)

	tests := []struct {
		name      string
		profile   colorprofile.Profile
		wantColor bool
	}{
		{name: "truecolor", profile: colorprofile.TrueColor, wantColor: true},
		{name: "256", profile: colorprofile.ANSI256, wantColor: true},
		{name: "16", profile: colorprofile.ANSI, wantColor: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := skin.For(p, p.Mauve, tc.profile)
			cell := s.Selected(green).Render("+3")
			on := skin.On(s.Band, green).Render("+3")
			row := s.Select(green.Render("M")+" file.go "+green.Render("+3"), 20)

			if got := hasColor(cell); got != tc.wantColor {
				t.Errorf("Selected cell %q carries a color: %v, want %v", cell, got, tc.wantColor)
			}

			if got := hasColor(on); got != tc.wantColor {
				t.Errorf("On(Band) cell %q carries a color: %v, want %v", on, got, tc.wantColor)
			}

			if !tc.wantColor && hasColor(row) {
				t.Errorf("Select row %q carries a color inside the reverse bar", row)
			}

			if !bold.MatchString(cell) {
				t.Errorf("Selected cell %q lost its weight", cell)
			}

			if w := ansi.StringWidth(row); w != 20 {
				t.Errorf("Select row is %d cells, want 20", w)
			}
		})
	}
}
