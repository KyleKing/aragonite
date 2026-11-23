package skin

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// ruleLead is the run of rule drawn before a section's name.
const ruleLead = "── "

// Rule is a section heading drawn as a quiet labeled rule, width cells wide:
// `── Files 10 ──────`. The name is in Title at body brightness rather than an
// accent, so headings stay out of the way of the rows that need attention, and
// the count, when there is one, is Muted. A name too long for the width is cut
// rather than wrapped, and the rule after it shrinks to nothing.
func (s Skin) Rule(name, count string, width int) string {
	head := s.Hairline.Render(ruleLead) + s.Title.Render(name)
	if count != "" {
		head += " " + s.Muted.Render(count)
	}

	used := lipgloss.Width(head)
	if used >= width {
		return lipgloss.NewStyle().MaxWidth(width).Render(head)
	}

	tail := width - used - 1
	if tail < 1 {
		return head
	}

	return head + " " + s.Hairline.Render(strings.Repeat("─", tail))
}
