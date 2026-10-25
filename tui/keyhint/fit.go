package keyhint

import (
	"slices"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Fit renders hints along one line of at most width cells, shedding whole hints
// rather than clipping one. A hint marked Off goes before any that is on, then
// the lowest Priority, and the later of two equals. The last hint standing is
// clipped to width, and a width of zero or less draws nothing.
func Fit(s Styles, hints []Hint, width int) string {
	if width <= 0 {
		return ""
	}

	kept := slices.Clone(hints)
	line := Line(s, kept)

	for len(kept) > 1 && lipgloss.Width(line) > width {
		kept = slices.Delete(kept, shed(kept), shed(kept)+1)
		line = Line(s, kept)
	}

	return ansi.Truncate(line, width, "")
}

func shed(hints []Hint) int {
	victim := 0

	for i, h := range hints {
		v := hints[victim]

		if (h.Off && !v.Off) || (h.Off == v.Off && h.Priority <= v.Priority) {
			victim = i
		}
	}

	return victim
}
