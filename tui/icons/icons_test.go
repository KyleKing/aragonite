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
		{"", icons.Auto, true},
		{"auto", icons.Auto, true},
		{" Auto ", icons.Auto, true},
		{"nerd", icons.Nerd, true},
		{"UNICODE", icons.Unicode, true},
		{"plain", icons.Unicode, true},
		{"off", icons.None, true},
		{"none", icons.None, true},
		{"emoji", icons.Auto, false},
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
	terminal(t, nil)
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

// terminalVars are every variable NerdTerminal reads. A test sets each of
// them, so the terminal running the suite cannot leak into it.
var terminalVars = []string{
	"TERM", "TERM_PROGRAM", "WEZTERM_EXECUTABLE", "WEZTERM_PANE",
	"KITTY_WINDOW_ID", "GHOSTTY_RESOURCES_DIR", icons.EnvOverride,
}

// terminal clears every variable detection reads and then sets env.
func terminal(t *testing.T, env map[string]string) {
	t.Helper()

	for _, k := range terminalVars {
		t.Setenv(k, env[k])
	}
}

type explainCase struct {
	env        map[string]string
	name       string
	configured string
	says       string
	want       icons.Resolution
}

//nolint:paralleltest // each case sets the process environment
func TestExplain(t *testing.T) {
	for _, tt := range append(terminalCases(), precedenceCases()...) {
		t.Run(tt.name, func(t *testing.T) {
			terminal(t, tt.env)

			got := icons.Explain(tt.configured)
			if got != tt.want {
				t.Errorf("Explain(%q) = %+v, want %+v", tt.configured, got, tt.want)
			}

			if r := icons.Resolve(tt.configured); r != tt.want.Mode {
				t.Errorf("Resolve(%q) = %v, want %v", tt.configured, r, tt.want.Mode)
			}

			if tt.says != "" && got.String() != tt.says {
				t.Errorf("String() = %q, want %q", got.String(), tt.says)
			}
		})
	}
}

// terminalCases cover each terminal's signals, tmux with and without an
// inherited one, and a terminal started from another.
func terminalCases() []explainCase {
	return []explainCase{
		{
			name: "an unknown terminal stays unicode",
			env:  map[string]string{"TERM": "xterm-256color", "TERM_PROGRAM": "Apple_Terminal"},
			want: icons.Resolution{Mode: icons.Unicode, Source: icons.FromAuto},
			says: "unicode (auto: no Nerd terminal detected)",
		},
		{
			name: "no terminal variables at all",
			want: icons.Resolution{Mode: icons.Unicode, Source: icons.FromAuto},
		},
		{
			name: "WezTerm by TERM_PROGRAM",
			env:  map[string]string{"TERM": "xterm-256color", "TERM_PROGRAM": "WezTerm"},
			want: icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "WezTerm"},
			says: "nerd (auto: WezTerm)",
		},
		{
			name: "WezTerm by its executable",
			env:  map[string]string{"WEZTERM_EXECUTABLE": "/usr/bin/wezterm-gui"},
			want: icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "WezTerm"},
		},
		{
			name: "WezTerm under tmux by the inherited pane",
			env:  map[string]string{"TERM": "tmux-256color", "TERM_PROGRAM": "tmux", "WEZTERM_PANE": "3"},
			want: icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "WezTerm"},
		},
		{
			name: "tmux with nothing inherited stays unicode",
			env:  map[string]string{"TERM": "tmux-256color", "TERM_PROGRAM": "tmux"},
			want: icons.Resolution{Mode: icons.Unicode, Source: icons.FromAuto},
		},
		{
			name: "another terminal started from WezTerm names itself",
			env:  map[string]string{"TERM": "xterm-256color", "TERM_PROGRAM": "vscode", "WEZTERM_PANE": "0"},
			want: icons.Resolution{Mode: icons.Unicode, Source: icons.FromAuto},
		},
		{
			name: "kitty by its window",
			env:  map[string]string{"KITTY_WINDOW_ID": "1", "TERM": "xterm-256color"},
			want: icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "kitty"},
		},
		{
			name: "kitty by TERM",
			env:  map[string]string{"TERM": "xterm-kitty"},
			want: icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "kitty"},
		},
		{
			name: "Ghostty by TERM_PROGRAM",
			env:  map[string]string{"TERM_PROGRAM": "ghostty", "TERM": "xterm-ghostty"},
			want: icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "Ghostty"},
		},
		{
			name: "Ghostty by TERM",
			env:  map[string]string{"TERM": "xterm-ghostty"},
			want: icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "Ghostty"},
		},
		{
			name: "Ghostty under tmux by its resources",
			env:  map[string]string{"TERM_PROGRAM": "tmux", "GHOSTTY_RESOURCES_DIR": "/usr/share/ghostty"},
			want: icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "Ghostty"},
		},
	}
}

// precedenceCases cover the override, then the setting, then auto.
func precedenceCases() []explainCase {
	return []explainCase{
		{
			name:       "the setting beats auto",
			env:        map[string]string{"TERM_PROGRAM": "WezTerm"},
			configured: "plain",
			want:       icons.Resolution{Mode: icons.Unicode, Source: icons.FromSetting, Detail: "plain"},
			says:       "unicode (setting)",
		},
		{
			name:       "the setting can ask for nerd anywhere",
			env:        map[string]string{"TERM_PROGRAM": "Apple_Terminal"},
			configured: "nerd",
			want:       icons.Resolution{Mode: icons.Nerd, Source: icons.FromSetting, Detail: "nerd"},
		},
		{
			name:       "a setting of auto detects",
			env:        map[string]string{"TERM_PROGRAM": "WezTerm"},
			configured: "auto",
			want:       icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "WezTerm"},
		},
		{
			name:       "an unreadable setting detects",
			env:        map[string]string{"TERM_PROGRAM": "WezTerm"},
			configured: "emoji",
			want:       icons.Resolution{Mode: icons.Nerd, Source: icons.FromAuto, Detail: "WezTerm"},
		},
		{
			name:       "the override beats the setting",
			env:        map[string]string{"TERM_PROGRAM": "WezTerm", icons.EnvOverride: "off"},
			configured: "nerd",
			want:       icons.Resolution{Mode: icons.None, Source: icons.FromEnv, Detail: "off"},
			says:       "none (ARAGONITE_ICONS=off)",
		},
		{
			name:       "an override of auto defers to the setting",
			env:        map[string]string{"TERM_PROGRAM": "WezTerm", icons.EnvOverride: "auto"},
			configured: "plain",
			want:       icons.Resolution{Mode: icons.Unicode, Source: icons.FromSetting, Detail: "plain"},
		},
		{
			name: "an override fixes a misread tmux session",
			env:  map[string]string{"TERM_PROGRAM": "tmux", "WEZTERM_PANE": "3", icons.EnvOverride: "plain"},
			want: icons.Resolution{Mode: icons.Unicode, Source: icons.FromEnv, Detail: "plain"},
		},
	}
}

func TestNewDrawsAutoAsUnicode(t *testing.T) {
	t.Parallel()

	s := icons.New(icons.Auto, theme.Macchiato())
	if s.Mode() != icons.Unicode || s.Check().Text != "✓" || s.File("a.go").Text != "" {
		t.Errorf("an Auto set drew %v", s.Mode())
	}
}

func TestModeAndSourceSpellings(t *testing.T) {
	t.Parallel()

	for m, want := range map[icons.Mode]string{
		icons.Nerd: "nerd", icons.Unicode: "unicode", icons.None: "none", icons.Auto: "auto",
	} {
		if m.String() != want {
			t.Errorf("%d.String() = %q, want %q", m, m.String(), want)
		}

		if back, ok := icons.Parse(want); !ok || back != m {
			t.Errorf("Parse(%q) = %v, %v; want %v", want, back, ok, m)
		}
	}

	for s, want := range map[icons.Source]string{
		icons.FromAuto: "auto", icons.FromEnv: icons.EnvOverride, icons.FromSetting: "setting",
	} {
		if s.String() != want {
			t.Errorf("Source %d = %q, want %q", s, s.String(), want)
		}
	}
}
