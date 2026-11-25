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
//
// A Bubble Tea program must not call it before Program.Run. The query reads
// the terminal's input itself and discards everything that is not its reply,
// so a key typed as the command was entered is lost, and a terminal that
// answers nothing holds startup for seconds. Such a program checks [Pinned],
// and when the background is needed starts under tui/launch, which asks
// through the program's own input, and builds with [ForBackground]. Detect is
// for a tool that reads no keys, or that runs after the program has exited.
func Detect() Palette {
	return Select(os.Getenv(EnvOverride), func() bool {
		return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	})
}

// Pinned reports whether CATPPUCCIN_THEME names a flavor, in which case the
// terminal's background is never needed.
func Pinned() bool {
	pinned := true

	Select(os.Getenv(EnvOverride), func() bool { pinned = false; return true })

	return pinned
}

// ForBackground is the palette for a terminal whose background is dark or
// not, honoring the CATPPUCCIN_THEME override. It queries nothing: the caller
// learned the background already, as tui/launch does through the program's
// input.
func ForBackground(dark bool) Palette {
	return Select(os.Getenv(EnvOverride), func() bool { return dark })
}

// Select resolves an override string against a background probe, which is
// only called when the override does not name a flavor.
func Select(override string, dark func() bool) Palette {
	switch strings.ToLower(override) {
	case "ink":
		return Ink()
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
