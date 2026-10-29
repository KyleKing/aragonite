package preview

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Field is one label-value line of a detail pane: the label quieter than the
// value it names.
func Field(st Styles, label, value string) string {
	return st.Label.Render(label+" ") + st.Body.Render(value)
}

// Pad fits lines to exactly height rows and pads each to width cells, because
// a shorter line leaves the previous frame's tail on screen.
func Pad(lines []string, height, width int) []string {
	out := make([]string, height)

	for i := range out {
		if i < len(lines) {
			out[i] = lines[i]
		}

		if gap := width - lipgloss.Width(out[i]); gap > 0 {
			out[i] += strings.Repeat(" ", gap)
		}
	}

	return out
}

// Wrap hard-wraps text to width and draws each line in face.
func Wrap(face lipgloss.Style, text string, width int) []string {
	wrapped := lipgloss.Wrap(strings.TrimSpace(text), max(width, 1), "")

	lines := strings.Split(wrapped, "\n")
	for i, line := range lines {
		lines[i] = face.Render(line)
	}

	return lines
}

// Marker reports how much of a windowed pane is on screen — "  shown/total" —
// so text running past the bottom is never mistaken for the whole of it.
// Empty when everything fits.
func Marker(offset, height, total int) string {
	if total <= height {
		return ""
	}

	return "  " + strconv.Itoa(min(offset+height, total)) + "/" + strconv.Itoa(total)
}

// Window is the detail's lines windowed to height rows starting at Scroll,
// clamped so a scroll past the end backs up to keep the pane full.
func (d Detail) Window(height int) []string {
	if height <= 0 {
		return nil
	}

	start := min(max(d.Scroll, 0), max(len(d.Lines)-height, 0))

	return d.Lines[start:min(start+height, len(d.Lines))]
}

// Head is the pane's heading line for the split layout: the item's name, and
// its scroll marker where the lines run past. The height argument is the
// pane's interior — the rows the windowed lines get, the marker's
// denominator.
func (d Detail) Head(st Styles, width, height int) string {
	face := st.Heading
	if d.Focused {
		face = st.Accent.Bold(true)
	}

	title := ansi.Truncate(d.Title+Marker(d.Scroll, height, len(d.Lines)), width, "…")

	return face.Render(title)
}

// Render is the whole pane for a detail drawn without sections beside it:
// the head line, then the windowed lines padded to fill, so a shorter pane
// still clears whatever the last frame drew there.
func (d Detail) Render(st Styles, width, height int) []string {
	return Pad(append([]string{d.Head(st, width, height)}, d.Window(height)...), height+1, width)
}

// Rule is the pane's heading for the stacked layout: its title and scroll
// marker ruled into the separator the two halves share, so the pane carries
// its name the way a section does. Height is the pane's interior, the
// marker's denominator.
func (d Detail) Rule(st Styles, width, height int) string {
	face, line := st.Heading, st.Rule
	if d.Focused {
		face, line = st.Accent.Bold(true), st.Accent
	}

	title := ansi.Truncate(d.Title+Marker(d.Scroll, height, len(d.Lines)), max(width-ruleLead-ruleSpaces, 1), "…")

	return line.Render(strings.Repeat("─", ruleLead)+" ") + face.Render(title) +
		line.Render(" "+strings.Repeat("─", max(width-lipgloss.Width(title)-ruleLead-ruleSpaces, 0)))
}
