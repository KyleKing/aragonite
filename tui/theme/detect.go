package theme

import (
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

// EnvOverride names the variable that pins a flavor regardless of what the
// terminal reports.
const EnvOverride = "CATPPUCCIN_THEME"

// Detect picks a palette from the CATPPUCCIN_THEME override, falling back to
// the terminal's background color. Anything it cannot query reads as dark, so
// a piped or captured run is deterministic.
func Detect() Palette {
	return Select(os.Getenv(EnvOverride), func() bool {
		return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	})
}

// Select resolves an override string against a background probe, which is
// only called when the override does not name a flavor.
func Select(override string, dark func() bool) Palette {
	switch strings.ToLower(override) {
	case "latte", "light":
		return Latte()
	case "macchiato", "dark":
		return Macchiato()
	}

	if dark() {
		return Macchiato()
	}

	return Latte()
}
