package preview

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	ruleLead   = 2
	ruleSpaces = 2
	// Two ends of a list survive a middle elision.
	elisionEnds = 2
)

// LabelRule draws a labeled rule across width, for a group that is not a
// Section: the NEEDS line of a summary, a notes file's name.
func LabelRule(st Styles, label string, width int) string {
	tail := strings.Repeat("─", max(width-lipgloss.Width(label)-ruleLead-ruleSpaces, 0))

	return st.Rule.Render(strings.Repeat("─", ruleLead)+" ") +
		st.Heading.Render(label) + st.Rule.Render(" "+tail)
}

// Rule is the labeled rule that opens a section: its count beside the heading
// and its jump key at the right edge, where the eye already is. The accent
// runs the whole rule while the section holds the keyboard.
func (s *Section) Rule(st Styles, focused bool, width int) string {
	label := s.Heading
	if s.Selectable && s.Count != "" {
		label += " · " + s.Count
	}

	face, line := st.Heading, st.Rule
	if focused {
		face, line = st.Accent.Bold(true), st.Accent
	}

	key := st.Muted.Render(s.Key)
	tail := max(width-lipgloss.Width(label)-ruleLead-ruleSpaces-lipgloss.Width(s.Key)-1, 0)

	return line.Render(strings.Repeat("─", ruleLead)+" ") + face.Render(label) +
		line.Render(" "+strings.Repeat("─", tail)+" ") + key
}

// Window is the portion of a section's rows that fit lines, keeping the cursor
// in view where the section holds it. An Elide section drops the middle of an
// overlong list rather than its end, so its newest lines survive.
func (s *Section) Window(lines, cursor int, focused bool) []string {
	if lines <= 0 {
		return nil
	}

	if s.Elide && len(s.Rows) > lines {
		return elideMiddle(s.Rows, lines)
	}

	start, end := rangeFor(cursor, len(s.Rows), lines)
	if !focused {
		start, end = 0, min(lines, len(s.Rows))
	}

	return s.Rows[start:end]
}

// rangeFor is the half-open window of length rows that fits visible lines,
// keeping cursor near the middle.
func rangeFor(cursor, length, visible int) (int, int) {
	visible = max(visible, 1)

	start := max(cursor-visible/2, 0)

	end := start + visible
	if end > length {
		end = length
		start = max(end-visible, 0)
	}

	return start, end
}

// elideMiddle drops the middle of lines until it fits height, marking the cut,
// so the newest entries at a list's end survive alongside the oldest.
func elideMiddle(lines []string, height int) []string {
	if len(lines) <= height || height < 1 {
		return lines
	}

	kept := height - 1
	head := kept / elisionEnds
	tail := kept - head

	out := make([]string, 0, height)
	out = append(out, lines[:head]...)
	out = append(out, "  ⋯ "+strconv.Itoa(len(lines)-head-tail)+" more lines ⋯")

	return append(out, lines[len(lines)-tail:]...)
}

// Column draws the sections stacked: each its rule and the rows its share of
// height allows, with gap blank rows between sections so a tall column keeps
// its rhythm. The result is at most height lines; callers pad to their
// surface.
func Column(st Styles, sections []Section, focused, cursor, height, width, gap int) []string {
	if len(sections) == 0 || height <= 0 {
		return nil
	}

	heights := Heights(sections, focused, height-gap*max(len(sections)-1, 0))

	lines := make([]string, 0, height)
	for i := range sections {
		if i > 0 {
			for range gap {
				lines = append(lines, "")
			}
		}

		lines = append(lines, sections[i].Rule(st, i == focused, width))
		lines = append(lines, sections[i].Window(heights[i]-ruleHeight, cursor, i == focused)...)
	}

	return lines[:min(len(lines), height)]
}
