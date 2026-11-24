package skin

import (
	"image/color"

	"github.com/charmbracelet/colorprofile"

	"github.com/kyleking/aragonite/tui/theme"
)

// BuildMono is New with NO_COLOR given rather than read, for a test running in parallel.
func BuildMono(p theme.Palette, accent color.Color, mono bool) Skin {
	return build(p, accent, colorprofile.TrueColor, mono)
}
