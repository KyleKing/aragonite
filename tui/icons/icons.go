// Package icons names the glyphs these tools draw beside files, folders,
// threads, and states, in one of three sets: plain unicode, the default; none
// at all; and Nerd Font icons, which a reader opts into.
//
// Unicode is the default because it is the only set whose width every
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

const (
	// Nerd draws Nerd Font icons. It needs a patched font, and a reader asks
	// for it.
	Nerd Mode = iota
	// Unicode draws only glyphs every monospace font carries, and is the
	// default.
	Unicode
	// None draws no icon at all; a screen says the same thing in words.
	None
)

// EnvOverride names the variable that picks a set regardless of the tool's
// own setting: "unicode" (or "plain"), "none" (or "off"), or "nerd".
const EnvOverride = "ARAGONITE_ICONS"

// Parse reads a setting's spelling of a mode. Anything it does not recognize
// is Unicode, the default, and reports false.
func Parse(s string) (Mode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "unicode", "plain", "":
		return Unicode, true
	case "none", "off":
		return None, true
	case "nerd":
		return Nerd, true
	}

	return Unicode, false
}

// Resolve picks a mode from the environment override, then a tool's
// configured value, then the default.
func Resolve(configured string) Mode {
	if env := os.Getenv(EnvOverride); env != "" {
		if m, ok := Parse(env); ok {
			return m
		}
	}

	m, _ := Parse(configured)

	return m
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

// New builds a set.
func New(m Mode, p theme.Palette) Set { return Set{mode: m, p: p} }

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
