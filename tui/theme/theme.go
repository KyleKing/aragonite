// Package theme carries the Catppuccin palettes a terminal UI draws from, the
// terminal-background detection that picks between them, and a semantic view
// naming the eight roles most views actually reach for.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Palette is one Catppuccin flavor. Fields are grouped as the upstream spec
// groups them: monochromatic ramp first, then the accents.
type Palette struct {
	Base     color.Color
	Mantle   color.Color
	Crust    color.Color
	Surface0 color.Color
	Surface1 color.Color
	Surface2 color.Color
	Overlay0 color.Color
	Overlay1 color.Color
	Overlay2 color.Color
	Subtext0 color.Color
	Subtext1 color.Color
	Text     color.Color

	Rosewater color.Color
	Flamingo  color.Color
	Pink      color.Color
	Mauve     color.Color
	Red       color.Color
	Maroon    color.Color
	Peach     color.Color
	Yellow    color.Color
	Green     color.Color
	Teal      color.Color
	Sky       color.Color
	Sapphire  color.Color
	Blue      color.Color
	Lavender  color.Color
}

// Semantic names the roles a view asks for when it does not care which flavor
// is active: a title is Primary whether the terminal is light or dark.
type Semantic struct {
	Primary   color.Color
	Secondary color.Color
	Accent    color.Color
	Muted     color.Color
	Text      color.Color
	Success   color.Color
	Warning   color.Color
	Error     color.Color
}

// Semantic maps the palette onto the eight roles.
func (p Palette) Semantic() Semantic {
	return Semantic{
		Primary:   p.Mauve,
		Secondary: p.Surface2,
		Accent:    p.Teal,
		Muted:     p.Overlay2,
		Text:      p.Text,
		Success:   p.Green,
		Warning:   p.Yellow,
		Error:     p.Red,
	}
}

// Latte returns the Catppuccin Latte (light) palette.
func Latte() Palette {
	return Palette{
		Base:     lipgloss.Color("#eff1f5"),
		Mantle:   lipgloss.Color("#e6e9ef"),
		Crust:    lipgloss.Color("#dce0e8"),
		Surface0: lipgloss.Color("#ccd0da"),
		Surface1: lipgloss.Color("#bcc0cc"),
		Surface2: lipgloss.Color("#acb0be"),
		Overlay0: lipgloss.Color("#9ca0b0"),
		Overlay1: lipgloss.Color("#8c8fa1"),
		Overlay2: lipgloss.Color("#7c7f93"),
		Subtext0: lipgloss.Color("#6c6f85"),
		Subtext1: lipgloss.Color("#5c5f77"),
		Text:     lipgloss.Color("#4c4f69"),

		Rosewater: lipgloss.Color("#dc8a78"),
		Flamingo:  lipgloss.Color("#dd7878"),
		Pink:      lipgloss.Color("#ea76cb"),
		Mauve:     lipgloss.Color("#8839ef"),
		Red:       lipgloss.Color("#d20f39"),
		Maroon:    lipgloss.Color("#e64553"),
		Peach:     lipgloss.Color("#fe640b"),
		Yellow:    lipgloss.Color("#df8e1d"),
		Green:     lipgloss.Color("#40a02b"),
		Teal:      lipgloss.Color("#179299"),
		Sky:       lipgloss.Color("#04a5e5"),
		Sapphire:  lipgloss.Color("#209fb5"),
		Blue:      lipgloss.Color("#1e66f5"),
		Lavender:  lipgloss.Color("#7287fd"),
	}
}

// Macchiato returns the Catppuccin Macchiato (medium-dark) palette.
func Macchiato() Palette {
	return Palette{
		Base:     lipgloss.Color("#24273a"),
		Mantle:   lipgloss.Color("#1e2030"),
		Crust:    lipgloss.Color("#181926"),
		Surface0: lipgloss.Color("#363a4f"),
		Surface1: lipgloss.Color("#494d64"),
		Surface2: lipgloss.Color("#5b6078"),
		Overlay0: lipgloss.Color("#6e738d"),
		Overlay1: lipgloss.Color("#8087a2"),
		Overlay2: lipgloss.Color("#939ab7"),
		Subtext0: lipgloss.Color("#a5adcb"),
		Subtext1: lipgloss.Color("#b8c0e0"),
		Text:     lipgloss.Color("#cad3f5"),

		Rosewater: lipgloss.Color("#f4dbd6"),
		Flamingo:  lipgloss.Color("#f0c6c6"),
		Pink:      lipgloss.Color("#f5bde6"),
		Mauve:     lipgloss.Color("#c6a0f6"),
		Red:       lipgloss.Color("#ed8796"),
		Maroon:    lipgloss.Color("#ee99a0"),
		Peach:     lipgloss.Color("#f5a97f"),
		Yellow:    lipgloss.Color("#eed49f"),
		Green:     lipgloss.Color("#a6da95"),
		Teal:      lipgloss.Color("#8bd5ca"),
		Sky:       lipgloss.Color("#91d7e3"),
		Sapphire:  lipgloss.Color("#7dc4e4"),
		Blue:      lipgloss.Color("#8aadf4"),
		Lavender:  lipgloss.Color("#b7bdf8"),
	}
}
