package theme_test

import (
	"image/color"
	"io"
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

// TestProfileHonorsColortermInsideTmux pins the case that drew greys on a
// truecolor laptop: inside tmux, Detect ignores COLORTERM and asks `tmux info`,
// which a detached server answers without Tc or RGB. TTY_FORCE stands in for
// a terminal, since the test writes to none.
func TestProfileHonorsColortermInsideTmux(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  []string
		want colorprofile.Profile
	}{
		{
			name: "tmux with COLORTERM=truecolor",
			env:  []string{"TERM=tmux-256color", "COLORTERM=truecolor", "TTY_FORCE=1"},
			want: colorprofile.TrueColor,
		},
		{
			name: "screen with COLORTERM=24bit",
			env:  []string{"TERM=screen-256color", "COLORTERM=24bit", "TTY_FORCE=1"},
			want: colorprofile.TrueColor,
		},
		{name: "tmux alone", env: []string{"TERM=tmux-256color", "TTY_FORCE=1"}, want: colorprofile.ANSI256},
		{
			name: "tmux with no default-terminal and COLORTERM=truecolor",
			env:  []string{"TERM=screen", "TMUX=/tmp/tmux-1/default,1,0", "COLORTERM=truecolor", "TTY_FORCE=1"},
			want: colorprofile.TrueColor,
		},
		{
			name: "a 16-color TERM outside tmux keeps 16",
			env:  []string{"TERM=xterm-16color", "COLORTERM=", "TTY_FORCE=1"},
			want: colorprofile.ANSI,
		},
		{
			name: "NO_COLOR is not lifted",
			env:  []string{"TERM=xterm", "COLORTERM=truecolor", "NO_COLOR=1", "TTY_FORCE=1"},
			want: colorprofile.ASCII,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := theme.Profile(io.Discard, tc.env); got != tc.want {
				t.Errorf("Profile(%v) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

// Every accent of every flavor keeps a hue in 256 colors: none answers a grey,
// which colorprofile's own match does for Latte's red.
func TestHueNeverGreysAnAccent(t *testing.T) {
	t.Parallel()

	for name, p := range map[string]theme.Palette{
		"latte": theme.Latte(), "macchiato": theme.Macchiato(), "ink": theme.Ink(),
	} {
		for accent, c := range map[string]color.Color{
			"red": p.Red, "green": p.Green, "blue": p.Blue, "yellow": p.Yellow,
			"peach": p.Peach, "mauve": p.Mauve, "maroon": p.Maroon, "teal": p.Teal,
		} {
			got := colorprofile.ANSI256.Convert(theme.Hue(c, colorprofile.ANSI256))
			if r, g, b, _ := got.RGBA(); r == g && g == b {
				t.Errorf("%s %s is the grey %v in 256 colors", name, accent, got)
			}
		}
	}

	if got := theme.Hue(theme.Latte().Red, colorprofile.TrueColor); got != theme.Latte().Red {
		t.Errorf("truecolor changed the red to %v", got)
	}

	grey := theme.Latte().Overlay2
	if got := theme.Hue(grey, colorprofile.ANSI256); got != grey {
		t.Errorf("a grey of its own was moved to %v", got)
	}
}
