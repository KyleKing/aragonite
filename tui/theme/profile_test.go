package theme_test

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/theme"
)

func TestGround(t *testing.T) {
	t.Parallel()

	tint := lipgloss.Color("#22372c")
	ink, latte := theme.Ink().Base, theme.Latte().Base

	tests := []struct {
		want    color.Color
		base    color.Color
		name    string
		lift    theme.Lift
		profile colorprofile.Profile
	}{
		{name: "truecolor keeps the tint", base: ink, lift: theme.Wash, profile: colorprofile.TrueColor, want: tint},
		{
			name: "256 lifts a dark base toward the light", base: ink, lift: theme.Wash, profile: colorprofile.ANSI256,
			want: ansi.IndexedColor(237),
		},
		{
			name: "256 sinks under a dark base", base: ink, lift: theme.Sunk, profile: colorprofile.ANSI256,
			want: ansi.IndexedColor(234),
		},
		{
			name: "256 lifts a light base darker", base: latte, lift: theme.Wash, profile: colorprofile.ANSI256,
			want: ansi.IndexedColor(253),
		},
		{
			name: "256 past the light end is white", base: latte, lift: theme.Sunk, profile: colorprofile.ANSI256,
			want: ansi.IndexedColor(231),
		},
		{name: "16 draws none", base: ink, lift: theme.Wash, profile: colorprofile.ANSI, want: lipgloss.NoColor{}},
		{
			name: "no color draws none", base: ink, lift: theme.Wash, profile: colorprofile.ASCII,
			want: lipgloss.NoColor{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := theme.Ground(tint, tc.base, tc.lift, tc.profile); got != tc.want {
				t.Errorf("Ground() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDark(t *testing.T) {
	t.Parallel()

	if !theme.Dark(theme.Ink().Base) || !theme.Dark(theme.Macchiato().Base) || theme.Dark(theme.Latte().Base) {
		t.Error("Dark() misread a flavor's base")
	}
}
