package keyhint

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/kyleking/aragonite/tui/table"
)

// PanelStyles are the faces Panel draws with.
type PanelStyles struct {
	// Frame draws the border and padding around the rows.
	Frame lipgloss.Style
	// On draws the key and text of a hint marked On, in place of Key and Text.
	On lipgloss.Style
	// Count draws a hint's Count.
	Count lipgloss.Style
	Styles
}

// Panel draws the next keys of a prefix as a bordered block, keys right-aligned
// in one column, then what each does, then its Count right-aligned. It is
// shortened to width where descriptions can give the room, and nil where not
// even one cell is left inside the frame. Place it with overlay.BottomRight,
// which clips what still overruns.
func Panel(s PanelStyles, hints []Hint, width int) []string {
	room := width - s.Frame.GetHorizontalFrameSize()
	if room < 1 || len(hints) == 0 {
		return nil
	}

	keys, whats, counts := 0, 0, 0

	for _, h := range hints {
		keys = max(keys, lipgloss.Width(h.Key))
		whats = max(whats, lipgloss.Width(panelWhat(h)))
		counts = max(counts, lipgloss.Width(h.Count))
	}

	extra := func(n int) int {
		if n == 0 {
			return 0
		}

		return len(Gutter) + n
	}

	whats = max(1, min(whats, room-keys-len(Gutter)-extra(counts)))
	rows := make([]string, 0, len(hints))

	for _, h := range hints {
		rows = append(rows, panelRow(s, h, keys, whats, counts))
	}

	return strings.Split(s.Frame.Render(strings.Join(rows, "\n")), "\n")
}

func panelWhat(h Hint) string {
	if len(h.Kids) > 0 {
		return h.What + "…"
	}

	return h.What
}

func panelRow(s PanelStyles, h Hint, keys, whats, counts int) string {
	text := table.Pad(table.Truncate(panelWhat(h), whats), whats, table.AlignLeft)
	key := table.Pad(h.Key, keys, table.AlignRight)

	keyFace, textFace := s.Key, s.Text

	switch {
	case h.Head:
		keyFace, textFace = s.Head, s.Head
	case h.Off:
		keyFace, textFace = s.Off, s.Off
	case h.On:
		keyFace, textFace = s.On, s.On
	}

	row := keyFace.Render(key) + Gutter + textFace.Render(text)
	if counts > 0 {
		row += Gutter + s.Count.Render(table.Pad(h.Count, counts, table.AlignRight))
	}

	return row
}
