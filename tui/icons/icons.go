// Package icons names the glyphs these tools draw beside files, folders,
// threads, and states, in one of three sets: plain unicode; none at all; and
// Nerd Font icons.
//
// Which set is drawn is resolved by Resolve and Explain: the ARAGONITE_ICONS
// override, then the tool's own setting, then Auto. Auto draws Nerd icons in a
// terminal that bundles the Nerd symbols itself, so no patched font is needed
// (WezTerm, kitty 0.33 and later, Ghostty), and unicode everywhere else,
// because a terminal without the font draws a Nerd icon as tofu.
//
// Auto reads the environment only and never asks the terminal: Bubble Tea owns
// stdin, and a query sent before it starts loses the first key typed. Inside
// tmux, TERM_PROGRAM names tmux, so the WEZTERM_*, KITTY_*, and GHOSTTY_*
// variables the tmux server inherited name the outer terminal instead. A tmux
// session started in one terminal and attached from another is therefore
// read as the first, and ARAGONITE_ICONS or the setting is how a reader
// corrects it.
//
// Unicode is the fallback because it is the only set whose width every
// terminal agrees on. A Nerd Font icon is a private-use code point: one cell to
// every width function and to tmux, and drawn two cells wide by a
// proportional ("Propo") Nerd Font or a fallback font, which overlaps whatever
// follows it. So a Nerd icon is only ever drawn through Prefix, as the icon and
// a space in a fixed two-cell slot, where the overflow lands on the blank.
//
// Unicode draws no file glyph at all and keeps the geometric state marks, so a
// file list reads the same in every set and only loses its filetype color.
// Every mark in it is width-stable: no Emoji=Yes code point (✔ ⚠ ▶ ⬇), only
// geometric shapes and dingbats every monospace font draws in one cell.
package icons

import (
	"image/color"
	"os"
	"path"
	"strings"

	"github.com/kyleking/aragonite/tui/theme"
)

// Mode is which set is drawn.
type Mode int

// The setting spellings Mode.String writes, and the terminals NerdTerminal names.
const (
	nerdWord    = "nerd"
	unicodeWord = "unicode"
	noneWord    = "none"
	autoWord    = "auto"

	wezterm = "WezTerm"
	kitty   = "kitty"
	ghostty = "Ghostty"
)

const (
	// Nerd draws Nerd Font icons. It needs a patched font or a terminal that
	// bundles the symbols.
	Nerd Mode = iota
	// Unicode draws only glyphs every monospace font carries, and is what
	// Auto falls back to.
	Unicode
	// None draws no icon at all; a screen says the same thing in words.
	None
	// Auto is a setting rather than a set: Resolve turns it into Nerd in a
	// terminal NerdTerminal recognizes and Unicode elsewhere. It is the
	// default. A Set built with it directly draws Unicode, since New does not
	// read the environment.
	Auto
)

// String is the mode's setting spelling: "nerd", "unicode", "none", or "auto".
func (m Mode) String() string {
	switch m {
	case Nerd:
		return nerdWord
	case Unicode:
		return unicodeWord
	case None:
		return noneWord
	case Auto:
		return autoWord
	}

	return unicodeWord
}

// EnvOverride names the variable that picks a set regardless of the tool's
// own setting: "auto" (or empty), "unicode" (or "plain"), "none" (or "off"),
// or "nerd".
const EnvOverride = "ARAGONITE_ICONS"

// Parse reads a setting's spelling of a mode, including "auto" and the empty
// string, both Auto. Anything it does not recognize is Auto and reports false.
func Parse(s string) (Mode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case autoWord, "":
		return Auto, true
	case unicodeWord, "plain":
		return Unicode, true
	case noneWord, "off":
		return None, true
	case nerdWord:
		return Nerd, true
	}

	return Auto, false
}

// Resolve picks the set to draw: the environment override, then a tool's
// configured value, then Auto, which reads the terminal from the environment.
// It never returns Auto. Explain says the same and why.
func Resolve(configured string) Mode { return Explain(configured).Mode }

// Source is where a Resolution's mode came from.
type Source int

const (
	// FromAuto is the default: the mode followed from the terminal.
	FromAuto Source = iota
	// FromEnv is the ARAGONITE_ICONS override.
	FromEnv
	// FromSetting is the tool's own configured value.
	FromSetting
)

// String names the source as a reader would set it: "auto", the override's
// variable name, or "setting".
func (s Source) String() string {
	switch s {
	case FromEnv:
		return EnvOverride
	case FromSetting:
		return "setting"
	case FromAuto:
	}

	return autoWord
}

// Resolution is the set Explain picked and why, for a help drawer to say.
type Resolution struct {
	// Detail is the terminal Auto recognized ("WezTerm", "kitty", "Ghostty"),
	// empty where it recognized none, or for FromEnv and FromSetting the
	// spelling that was read.
	Detail string
	// Mode is the set to draw: Nerd, Unicode, or None, never Auto.
	Mode Mode
	// Source is which input decided it.
	Source Source
}

// String says the resolution in a few words, for a `?` drawer:
// "nerd (auto: WezTerm)", "unicode (auto: no Nerd terminal detected)",
// "none (ARAGONITE_ICONS=off)", or "nerd (setting)".
func (r Resolution) String() string {
	switch r.Source {
	case FromEnv:
		return r.Mode.String() + " (" + EnvOverride + "=" + r.Detail + ")"
	case FromSetting:
		return r.Mode.String() + " (setting)"
	case FromAuto:
	}

	if r.Detail == "" {
		return r.Mode.String() + " (auto: no Nerd terminal detected)"
	}

	return r.Mode.String() + " (auto: " + r.Detail + ")"
}

// Explain resolves a mode as Resolve does and reports which input decided it.
// An override or setting that spells auto, or that Parse cannot read, defers
// to the next input, so a bad override never hides a good setting.
func Explain(configured string) Resolution {
	if env := os.Getenv(EnvOverride); env != "" {
		if m, ok := Parse(env); ok && m != Auto {
			return Resolution{Mode: m, Source: FromEnv, Detail: strings.TrimSpace(env)}
		}
	}

	if m, ok := Parse(configured); ok && m != Auto {
		return Resolution{Mode: m, Source: FromSetting, Detail: strings.TrimSpace(configured)}
	}

	if term := NerdTerminal(); term != "" {
		return Resolution{Mode: Nerd, Source: FromAuto, Detail: term}
	}

	return Resolution{Mode: Unicode, Source: FromAuto}
}

// NerdTerminal names the terminal the environment identifies when it is one
// that bundles the Nerd Font symbols ("WezTerm", "kitty", or "Ghostty"), and
// is empty otherwise. It reads, in order:
//
//   - TERM_PROGRAM: WezTerm or ghostty
//   - TERM: xterm-kitty or xterm-ghostty, which also survive ssh
//   - the variables each terminal exports (WEZTERM_EXECUTABLE, WEZTERM_PANE,
//     KITTY_WINDOW_ID, GHOSTTY_RESOURCES_DIR), but only where TERM_PROGRAM is
//     unset or names tmux or screen, since a terminal started from a WezTerm
//     shell (an editor's, say) inherits them and names itself in TERM_PROGRAM
func NerdTerminal() string {
	program := strings.ToLower(os.Getenv("TERM_PROGRAM"))

	switch program {
	case "wezterm":
		return wezterm
	case "ghostty":
		return ghostty
	case kitty:
		return kitty
	}

	switch os.Getenv("TERM") {
	case "xterm-kitty":
		return kitty
	case "xterm-ghostty":
		return ghostty
	}

	if program != "" && program != "tmux" && program != "screen" {
		return ""
	}

	switch {
	case os.Getenv("WEZTERM_PANE") != "" || os.Getenv("WEZTERM_EXECUTABLE") != "":
		return wezterm
	case os.Getenv("KITTY_WINDOW_ID") != "":
		return kitty
	case os.Getenv("GHOSTTY_RESOURCES_DIR") != "":
		return ghostty
	}

	return ""
}

// Glyph is one icon: the text drawn and the color it is drawn in. An empty
// Text means the set draws nothing there.
type Glyph struct {
	Color color.Color
	Text  string
}

// Prefix is the glyph followed by a space, or nothing when the set draws
// none, so a caller can put it in front of a name unconditionally. It is the
// only way a Nerd icon should be drawn: the space is the cell a wide-drawn
// icon overflows into, so nothing after it moves.
func (g Glyph) Prefix() string {
	if g.Text == "" {
		return ""
	}

	return g.Text + " "
}

// Set draws one mode's glyphs in one palette's colors.
type Set struct {
	p    theme.Palette
	mode Mode
}

// New builds a set. Auto draws as Unicode here, because New does not read the
// environment; pass a mode from Resolve instead.
func New(m Mode, p theme.Palette) Set {
	if m == Auto {
		m = Unicode
	}

	return Set{mode: m, p: p}
}

// Mode reports which set this is.
func (s Set) Mode() Mode { return s.mode }

func (s Set) pick(nerd, plain string, c color.Color) Glyph {
	switch s.mode {
	case Nerd:
		return Glyph{Text: nerd, Color: c}
	case None:
		return Glyph{}
	default:
		return Glyph{Text: plain, Color: c}
	}
}

// Folder marks a directory.
func (s Set) Folder() Glyph { return s.pick("", "", s.p.Blue) }

// Comment marks a thread or a comment count.
func (s Set) Comment() Glyph { return s.pick("", "◆", s.p.Mauve) }

// Branch marks a branch or bookmark.
func (s Set) Branch() Glyph { return s.pick("", "", s.p.Green) }

// PullRequest marks a pull request.
func (s Set) PullRequest() Glyph { return s.pick("", "", s.p.Lavender) }

// Commit marks a commit or a change.
func (s Set) Commit() Glyph { return s.pick("", "", s.p.Overlay2) }

// Check marks something passed or approved.
func (s Set) Check() Glyph { return s.pick("", "✓", s.p.Green) }

// Cross marks something failed or refused.
func (s Set) Cross() Glyph { return s.pick("", "✗", s.p.Red) }

// Alert marks something that needs a look.
func (s Set) Alert() Glyph { return s.pick("", "!", s.p.Peach) }

// Lock marks a lockfile or something protected.
func (s Set) Lock() Glyph { return s.pick("", "", s.p.Overlay1) }

// File marks a file by its name: a filetype icon in Nerd, nothing otherwise.
func (s Set) File(name string) Glyph {
	if s.mode != Nerd {
		return Glyph{}
	}

	icon, ok := byName[strings.ToLower(path.Base(name))]
	if !ok {
		icon, ok = byExt[strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))]
	}

	if !ok {
		icon = fi("\uf15b", muted)
	}

	return Glyph{Text: icon.glyph, Color: icon.color(s.p)}
}

// fi pairs a glyph with the palette role it is drawn in.
func fi(glyph string, hue func(theme.Palette) color.Color) fileIcon {
	return fileIcon{color: hue, glyph: glyph}
}

type fileIcon struct {
	color func(theme.Palette) color.Color
	glyph string
}

var (
	blue     = func(p theme.Palette) color.Color { return p.Blue }
	sapphire = func(p theme.Palette) color.Color { return p.Sapphire }
	sky      = func(p theme.Palette) color.Color { return p.Sky }
	yellow   = func(p theme.Palette) color.Color { return p.Yellow }
	peach    = func(p theme.Palette) color.Color { return p.Peach }
	red      = func(p theme.Palette) color.Color { return p.Red }
	green    = func(p theme.Palette) color.Color { return p.Green }
	mauve    = func(p theme.Palette) color.Color { return p.Mauve }
	teal     = func(p theme.Palette) color.Color { return p.Teal }
	muted    = func(p theme.Palette) color.Color { return p.Overlay1 }
)

var byExt = map[string]fileIcon{
	"go": fi("", sapphire), "mod": fi("", sapphire), "sum": fi("", muted),
	"ts": fi("", blue), "mts": fi("", blue), "cts": fi("", blue),
	"tsx": fi("", sky), "jsx": fi("", sky),
	"js": fi("", yellow), "mjs": fi("", yellow), "cjs": fi("", yellow),
	"py": fi("", yellow), "rs": fi("", peach), "rb": fi("", red),
	"java": fi("", red), "kt": fi("", mauve), "swift": fi("", peach),
	"c": fi("", blue), "h": fi("", mauve), "cpp": fi("", blue), "cc": fi("", blue),
	"hpp": fi("", mauve), "lua": fi("", blue), "php": fi("", mauve),
	"zig": fi("", peach), "nix": fi("", sky), "vue": fi("", green),
	"svelte": fi("", peach), "sql": fi("", yellow),
	"json": fi("", yellow), "jsonc": fi("", yellow),
	"yaml": fi("", red), "yml": fi("", red), "toml": fi("", peach),
	"ini": fi("", muted), "cfg": fi("", muted), "conf": fi("", muted),
	"pkl": fi("", teal), "md": fi("", blue), "mdx": fi("", blue),
	"html": fi("", peach), "css": fi("", blue), "scss": fi("", mauve),
	"sh": fi("", green), "bash": fi("", green), "zsh": fi("", green), "fish": fi("", green),
	"svg": fi("", peach), "png": fi("", mauve), "jpg": fi("", mauve),
	"gif": fi("", mauve), "webp": fi("", mauve),
	"lock": fi("", muted), "golden": fi("", yellow),
}

var byName = map[string]fileIcon{
	"dockerfile": fi("", blue), "makefile": fi("", muted),
	".gitignore": fi("", peach), ".gitattributes": fi("", peach),
	"package-lock.json": fi("", muted), "pnpm-lock.yaml": fi("", muted),
	"yarn.lock": fi("", muted), "cargo.lock": fi("", muted),
	"go.sum": fi("", muted), "uv.lock": fi("", muted), "poetry.lock": fi("", muted),
	"license": fi("", yellow), "readme.md": fi("", blue),
}
