package icons_test

import (
	"testing"

	"charm.land/lipgloss/v2"

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
		{"", icons.Nerd, true},
		{"nerd", icons.Nerd, true},
		{"UNICODE", icons.Unicode, true},
		{"off", icons.Unicode, true},
		{"none", icons.Unicode, true},
		{"emoji", icons.Nerd, false},
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

	for _, m := range []icons.Mode{icons.Nerd, icons.Unicode} {
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
