// Package frame draws the shell every screen shares: a header line naming the
// screen with its tabs at the right, a body, and a footer pinned to the last row
// with the keys on the left and a status message at the right.
//
// Pinning is the point. A footer stacked under a short body floats up the
// screen, and a toast written over the last row covers the keys. Here the body
// is padded to the footer, and a status takes room from the hints instead of
// drawing over them.
package frame

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/skin"
)

// Tone picks the face a Fact is drawn in.
type Tone int

// Tones, quietest first. The zero value is Muted, since most facts are counts.
const (
	Muted Tone = iota
	Accent
	Warning
	Error
)

// Fact is a short piece of state beside the title.
type Fact struct {
	Text string
	Tone Tone
}

// Tab is one segment of the strip at the right of the header.
type Tab struct {
	Label  string
	Active bool
}

// Frame is everything one screen puts around its body.
type Frame struct {
	Title  string
	Status string
	Facts  []Fact
	Tabs   []Tab
	Body   []string
	Hints  []keyhint.Hint
}

const (
	gap = "  "
	// Header and footer rows.
	chrome = 2
	// A status takes at most one share of this many of the footer's width.
	statusShare = 2
)

// Render draws f into exactly height lines of at most width cells. Body lines
// are clipped to width and the body fills the rows between header and footer.
// A Status takes at most half the footer, and the hints are fitted to the rest.
func Render(s skin.Skin, f Frame, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	if height == 1 {
		return footer(s, f, width)
	}

	lines := make([]string, 0, height)
	lines = append(lines, header(s, f, width))

	room := height - chrome
	for i := range room {
		line := ""
		if i < len(f.Body) {
			line = ansi.Truncate(f.Body[i], width, "")
		}

		lines = append(lines, line)
	}

	return strings.Join(append(lines, footer(s, f, width)), "\n")
}

func header(s skin.Skin, f Frame, width int) string {
	tabs := make([]string, 0, len(f.Tabs))
	for _, t := range f.Tabs {
		face := s.Tab
		if t.Active {
			face = s.TabActive
		}

		tabs = append(tabs, face.Render(t.Label))
	}

	right := strings.Join(tabs, " ")

	parts := make([]string, 0, 1+len(f.Facts))
	parts = append(parts, s.Title.Render(f.Title))

	for _, fact := range f.Facts {
		parts = append(parts, tone(s, fact.Tone).Render(fact.Text))
	}

	left := strings.Join(parts, gap)

	left = ansi.Truncate(left, max(width-lipgloss.Width(right)-len(gap), 0), "…")

	return ansi.Truncate(join(left, right, width), width, "")
}

func footer(s skin.Skin, f Frame, width int) string {
	status := ansi.Truncate(s.Accent.Render(f.Status), width/statusShare, "…")

	room := width - lipgloss.Width(status)
	if status != "" {
		room -= len(gap)
	}

	faces := keyhint.Styles{Key: s.Key, Text: s.Subtitle, Off: s.Muted}

	return join(keyhint.Fit(faces, f.Hints, room), status, width)
}

func join(left, right string, width int) string {
	return left + strings.Repeat(" ", max(width-lipgloss.Width(left)-lipgloss.Width(right), 0)) + right
}

func tone(s skin.Skin, t Tone) lipgloss.Style {
	switch t {
	case Accent:
		return s.Accent
	case Warning:
		return s.Warning
	case Error:
		return s.Error
	case Muted:
		return s.Muted
	}

	return s.Muted
}
