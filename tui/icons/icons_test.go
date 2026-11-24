package icons_test

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/glyph"
	"github.com/kyleking/aragonite/tui/icons"
	"github.com/kyleking/aragonite/tui/theme"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want icons.Mode
		ok   bool
	}{
		{"", icons.Unicode, true},
		{"nerd", icons.Nerd, true},
		{"UNICODE", icons.Unicode, true},
		{"plain", icons.Unicode, true},
		{"off", icons.None, true},
		{"none", icons.None, true},
		{"emoji", icons.Unicode, false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, ok := icons.Parse(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Errorf("Parse(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestResolveLetsTheEnvironmentWin(t *testing.T) {
	t.Setenv(icons.EnvOverride, "unicode")

	if icons.Resolve("nerd") != icons.Unicode {
		t.Error("the environment should override the configured mode")
	}

	t.Setenv(icons.EnvOverride, "bogus")

	if icons.Resolve("unicode") != icons.Unicode {
		t.Error("an unreadable override should fall back to the configured mode")
	}
}

func TestFile(t *testing.T) {
	t.Parallel()

	nerd := icons.New(icons.Nerd, theme.Macchiato())

	tests := []struct{ name, want string }{
		{"internal/x/claim.go", ""},
		{"web/Card.TSX", ""},
		{"go.sum", ""},
		{"Dockerfile", ""},
		{"notes.unknownext", ""},
		{"noext", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := nerd.File(tt.name).Text; got != tt.want {
				t.Errorf("got %U, want %U", []rune(got), []rune(tt.want))
			}
		})
	}
}

func TestEveryGlyphIsOneCell(t *testing.T) {
	t.Parallel()

	for _, m := range []icons.Mode{icons.Nerd, icons.Unicode, icons.None} {
		s := icons.New(m, theme.Macchiato())
		for _, g := range []icons.Glyph{
			s.Folder(), s.Comment(), s.Branch(), s.PullRequest(), s.Commit(),
			s.Check(), s.Cross(), s.Alert(), s.Lock(), s.File("a.go"), s.File("x"),
		} {
			if w := lipgloss.Width(g.Text); w > 1 {
				t.Errorf("mode %v: %q is %d cells", m, g.Text, w)
			}
		}
	}
}

func TestUnicodeDrawsNoFileGlyph(t *testing.T) {
	t.Parallel()

	s := icons.New(icons.Unicode, theme.Macchiato())
	if g := s.File("a.go"); g.Prefix() != "" {
		t.Errorf("got %q", g.Prefix())
	}

	if s.Comment().Prefix() != "◆ " {
		t.Error("state marks stay geometric")
	}

	if s.Mode() != icons.Unicode {
		t.Error("mode")
	}
}

// The Nerd set is drawn as the icon and a space, so a font that draws the icon
// wider than its cell overlaps the blank rather than the name; None draws
// nothing; and Unicode draws only width-stable marks.
func TestPrefixIsAFixedSlot(t *testing.T) {
	t.Parallel()

	nerd := icons.New(icons.Nerd, theme.Macchiato())
	if p := nerd.File("a.go").Prefix(); lipgloss.Width(p) != 2 || p[len(p)-1] != ' ' {
		t.Errorf("nerd prefix %q is not an icon and a space", p)
	}

	none := icons.New(icons.None, theme.Macchiato())
	for _, g := range []icons.Glyph{none.Check(), none.Comment(), none.File("a.go")} {
		if g.Prefix() != "" {
			t.Errorf("none drew %q", g.Prefix())
		}
	}

	plain := icons.New(icons.Unicode, theme.Macchiato())
	for _, g := range []icons.Glyph{plain.Check(), plain.Cross(), plain.Alert(), plain.Comment()} {
		glyph.Each(g.Text, func(cluster string, _ int, c glyph.Class) bool {
			t.Errorf("unicode mark %q is %v", cluster, c)

			return true
		})
	}
}
