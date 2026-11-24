package glyph_test

import (
	"testing"

	"github.com/kyleking/aragonite/tui/framecheck"
	"github.com/kyleking/aragonite/tui/glyph"
)

func TestOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want glyph.Class
	}{
		{"a", glyph.Stable},
		{"▸", glyph.Stable},
		{"◆", glyph.Stable},
		{"✓", glyph.Stable},
		{"─", glyph.Stable},
		{"●", glyph.Stable},
		{"▌", glyph.Stable},
		{"⚠", glyph.TextEmoji},
		{"▶", glyph.TextEmoji},
		{"⬇", glyph.TextEmoji},
		{"✔", glyph.TextEmoji},
		{"⚠\U0000FE0E", glyph.Stable},
		{"❤\U0000FE0F", glyph.Presented},
		{"\U0001F469\U0000200D\U0001F4BB", glyph.Joined},
		{"", glyph.Private},
		{"\U0001F680", glyph.Wide},
		{"\U000065E5", glyph.Wide},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			if got := glyph.Of(tc.in); got != tc.want {
				t.Errorf("Of(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSettle(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{"plain ascii", "plain ascii"},
		{"⚠ careful", "▲ careful"},
		{"love ❤\U0000FE0F it", "love ♡ it"},
		{"\U0001F44D\U0001F3FD lgtm", "\U0001F44D lgtm"},
		{"\U0001F469\U0000200D\U0001F4BB dev", "\U0001F469 dev"},
		{"\U0001F1FA\U0001F1F8 flag", "US flag"},
		{" icon", "\U0000FFFD icon"},
		{"\U0001F680 ship", "\U0001F680 ship"},
		{"\U000065E5\U0000672C", "\U000065E5\U0000672C"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()

			got := glyph.Settle(tc.in)
			if got != tc.want {
				t.Errorf("Settle(%q) = %q, want %q", tc.in, got, tc.want)
			}

			glyph.Each(got, func(cluster string, _ int, c glyph.Class) bool {
				if c != glyph.Wide {
					t.Errorf("Settle(%q) left %q, %v", tc.in, cluster, c)
				}

				return true
			})
		})
	}
}

// TestChromeIsWidthStable holds every literal in this module to the policy the
// tools are held to: the icon table may carry Nerd glyphs, the glyph tables and
// framecheck's own fixtures name the unstable ones on purpose, and nothing else
// draws a glyph whose width a terminal can disagree on.
func TestChromeIsWidthStable(t *testing.T) {
	t.Parallel()

	framecheck.Source(t, "../..",
		framecheck.PrivateIn("tui/icons/icons.go"),
		framecheck.SkipFiles("tui/glyph/tables.go", "tui/glyph/glyph.go"),
	)
}

// An emoji newer than Unicode 9 is one cell to a terminal still on Unicode 9
// tables (WezTerm's default), so content draws a placeholder for it.
func TestRecentEmojiSettleToAPlaceholder(t *testing.T) {
	t.Parallel()

	for _, r := range []rune{0x1FA7A, 0x1F7E0, 0x1F972} {
		if got := glyph.Of(string(r)); got != glyph.Recent {
			t.Errorf("Of(%U) = %v, want Recent", r, got)
		}
	}

	if got, want := glyph.Settle(string(rune(0x1FA7A))+" health "+string(rune(0x1F7E0))+" Major"),
		string(rune(0x25CC))+" health "+string(rune(0x25CF))+" Major"; got != want {
		t.Errorf("Settle = %q, want %q", got, want)
	}

	if got := glyph.Of(string(rune(0x1F680))); got != glyph.Wide {
		t.Errorf("a Unicode 6 emoji is %v, want Wide", got)
	}
}
