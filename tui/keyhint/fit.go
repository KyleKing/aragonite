package keyhint

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Fit renders hints along one line of at most width cells, shedding whole hints
// rather than clipping one. A hint marked Off goes before any that is on, then
// the lowest Priority, and the later of two equals. A Pin hint is shed only
// after every other hint. The last hint standing is clipped to width, and a
// width of zero or less draws nothing.
//
// Each hint is drawn and measured once and shedding is arithmetic on those
// widths, so a footer costs one render per hint however many are shed.
func Fit(s Styles, hints []Hint, width int) string {
	if width <= 0 {
		return ""
	}

	kept := slices.Clone(hints)
	drawn := make([]string, len(kept))
	widths := make([]int, len(kept))
	gap := lipgloss.Width(s.gap())
	total := -gap

	for i, h := range kept {
		drawn[i] = One(s, h)
		if strings.Contains(drawn[i], "\n") {
			return fitSlow(s, hints, width)
		}

		widths[i] = lipgloss.Width(drawn[i])
		total += widths[i] + gap
	}

	for len(kept) > 1 && total > width {
		v := shed(kept)
		total -= widths[v] + gap
		kept = slices.Delete(kept, v, v+1)
		drawn = slices.Delete(drawn, v, v+1)
		widths = slices.Delete(widths, v, v+1)
	}

	return ansi.Truncate(strings.Join(drawn, s.gap()), width, "")
}

// fitSlow is Fit by measuring the whole line after each shed, for a hint whose
// text breaks the line, where widths stop adding up.
func fitSlow(s Styles, hints []Hint, width int) string {
	kept := slices.Clone(hints)
	line := Line(s, kept)

	for len(kept) > 1 && lipgloss.Width(line) > width {
		kept = slices.Delete(kept, shed(kept), shed(kept)+1)
		line = Line(s, kept)
	}

	return ansi.Truncate(line, width, "")
}

// Memo holds the last line a footer drew and the key it drew it for, so a
// frame redrawn with nothing changed (a cursor blink, a spinner tick, a key
// that moved nothing the footer reads) skips building and fitting its hints.
// K is whatever the footer's hints depend on: the screen, the width, the
// revision under the cursor, a generation the model bumps on a state change.
// A key that leaves out something the hints read draws a stale footer, so
// when in doubt put it in the key. The zero Memo is empty and ready.
type Memo[K comparable] struct {
	key  K
	line string
	ok   bool
}

// Line is the line draw returns for key, calling draw only when key differs
// from the last call's or the memo is empty.
func (m *Memo[K]) Line(key K, draw func() string) string {
	if m.ok && m.key == key {
		return m.line
	}

	m.key, m.line, m.ok = key, draw(), true

	return m.line
}

// Reset empties the memo, for a change its key does not carry, such as a new
// skin.
func (m *Memo[K]) Reset() { m.ok = false }

func shed(hints []Hint) int {
	victim := 0

	for i, h := range hints {
		v := hints[victim]

		if v.Pin != h.Pin {
			if v.Pin {
				victim = i
			}

			continue
		}

		if (h.Off && !v.Off) || (h.Off == v.Off && h.Priority <= v.Priority) {
			victim = i
		}
	}

	return victim
}

// Pinned renders only the hints marked Pin, which is the width a footer must
// leave them.
func Pinned(s Styles, hints []Hint) string {
	return Line(s, slices.DeleteFunc(slices.Clone(hints), func(h Hint) bool { return !h.Pin }))
}
