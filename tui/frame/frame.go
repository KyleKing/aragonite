// Package frame draws the shell every screen shares: a header naming the screen
// and its tabs, a body, an optional panel about the row under the cursor, and a
// footer pinned to the last row with the keys on the left and a status message
// at the right.
//
// Pinning is the point. A footer stacked under a short body floats up the
// screen, and a toast written over the last row covers the keys. Here the body
// is padded to the footer, and a status takes room from the hints instead of
// drawing over them.
//
// The same geometry answers a click: BodyRow and TabAt say what a mouse event
// landed on, so a screen never re-derives where its own chrome is.
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

// Tab is one segment of the strip.
type Tab struct {
	Label string
	// Count is drawn as a chip after the label. Empty draws no chip.
	Count  string
	Active bool
	// Alert draws an inactive tab's Count in the error color: something there
	// is waiting on the reader.
	Alert bool
	// Stale draws Count dim, for a number read off a snapshot that a refresh
	// has not confirmed yet.
	Stale bool
}

// Frame is everything one screen puts around its body.
type Frame struct {
	Title  string
	Status string
	// Note is drawn at the right end of the Strip's rule: what narrows the body,
	// such as a filter query or the section scrolled past.
	Note  string
	Facts []Fact
	Tabs  []Tab
	Body  []string
	Hints []keyhint.Hint
	// Panel is drawn between the body and the footer on a raised surface the
	// footer shares, so the body is the same height whatever the panel says.
	Panel []string
	// Strip puts the tabs after the title, the facts at the right, and a
	// hairline row under the header that underlines the active tab, and draws
	// the hints bare. It retires the tabs-at-the-right header and bracketed
	// hints once gh-repo-dashboard sets it.
	Strip bool
}

const (
	// The header and the rule under it.
	stripTop = 2
	gap      = "  "
	tabGap   = "   "
	bareGap  = "   "
	// A status takes at most one share of this many of the footer's width.
	statusShare = 2
)

// Top is how many rows the header takes.
func Top(f Frame) int {
	if f.Strip {
		return stripTop
	}

	return 1
}

// Rows is how many body rows a frame height rows tall has.
func Rows(f Frame, height int) int {
	if height <= 1 {
		return 0
	}

	return max(height-Top(f)-len(f.Panel)-1, 0)
}

// BodyRow is the body line screen row y falls on, and false for a row of the
// header, the panel, or the footer.
func BodyRow(f Frame, height, y int) (int, bool) {
	row := y - Top(f)

	return row, row >= 0 && row < Rows(f, height)
}

// TabAt is the index of the tab drawn at cell (x, y), and false where no tab
// is. It reads the same layout Render draws, at the same width.
func TabAt(s skin.Skin, f Frame, width, x, y int) (int, bool) {
	if y != 0 {
		return 0, false
	}

	for i, sp := range layout(s, f, width).spans {
		if x >= sp.from && x < sp.to {
			return i, true
		}
	}

	return 0, false
}

// Render draws f into exactly height lines of at most width cells. Body lines
// are clipped to width and the body fills the rows between header and panel.
// A Status takes at most half the footer, and the hints are fitted to the rest.
func Render(s skin.Skin, f Frame, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	if height == 1 {
		return footer(s, f, width)
	}

	head := layout(s, f, width)
	lines := make([]string, 0, height)
	lines = append(lines, head.line)

	if f.Strip {
		lines = append(lines, rule(s, f, head, width))
	}

	for i := range Rows(f, height) {
		line := ""
		if i < len(f.Body) {
			line = ansi.Truncate(f.Body[i], width, "")
		}

		lines = append(lines, line)
	}

	foot := footer(s, f, width)
	if len(f.Panel) > 0 {
		for _, p := range f.Panel {
			lines = append(lines, Lit(ansi.Truncate(p, width, "…"), width, s.Panel))
		}

		foot = Lit(foot, width, s.Panel)
	}

	return strings.Join(append(lines[:min(len(lines), height-1)], foot), "\n")
}

// Lit pads text to width and draws it on face's background, restarting the
// background after every reset inside text so styled spans do not punch holes
// in it.
func Lit(text string, width int, face lipgloss.Style) string {
	on, _, _ := strings.Cut(face.Render("x"), "x")
	if on == "" {
		return text
	}

	if over := width - ansi.StringWidth(text); over > 0 {
		text += strings.Repeat(" ", over)
	}

	for _, reset := range []string{"\x1b[m", "\x1b[0m"} {
		text = strings.ReplaceAll(text, reset, reset+on)
	}

	return on + text + "\x1b[m"
}

type span struct{ from, to int }

// head is the header line and the cells each tab covers on it.
type head struct {
	line  string
	spans []span
}

func layout(s skin.Skin, f Frame, width int) head {
	if f.Strip {
		return stripHeader(s, f, width)
	}

	return tabsRight(s, f, width)
}

func stripHeader(s skin.Skin, f Frame, width int) head {
	var b strings.Builder

	b.WriteString(s.Title.Render(f.Title))
	col := lipgloss.Width(b.String())
	spans := make([]span, 0, len(f.Tabs))

	for _, t := range f.Tabs {
		face := s.Subtitle
		if t.Active {
			face = s.Title
		}

		part := face.Render(t.Label) + chip(s, t)
		b.WriteString(tabGap + part)
		col += len(tabGap)
		spans = append(spans, span{col, col + lipgloss.Width(part)})
		col += lipgloss.Width(part)
	}

	left := b.String()

	right := facts(s, f)
	if right == "" || width-lipgloss.Width(left)-lipgloss.Width(right) < 1 {
		return head{line: ansi.Truncate(left, width, "…"), spans: clipSpans(spans, width)}
	}

	return head{line: join(left, right, width), spans: spans}
}

func tabsRight(s skin.Skin, f Frame, width int) head {
	tabs := make([]string, 0, len(f.Tabs))
	for _, t := range f.Tabs {
		face := s.Tab
		if t.Active {
			face = s.TabActive
		}

		tabs = append(tabs, face.Render(t.Label)+chip(s, t))
	}

	right := strings.Join(tabs, " ")

	parts := []string{s.Title.Render(f.Title)}
	if text := facts(s, f); text != "" {
		parts = append(parts, text)
	}

	left := ansi.Truncate(strings.Join(parts, gap), max(width-lipgloss.Width(right)-len(gap), 0), "…")

	col := width - lipgloss.Width(right)
	spans := make([]span, 0, len(tabs))

	for _, t := range tabs {
		spans = append(spans, span{col, col + lipgloss.Width(t)})
		col += lipgloss.Width(t) + 1
	}

	return head{line: ansi.Truncate(join(left, right, width), width, ""), spans: clipSpans(spans, width)}
}

func clipSpans(spans []span, width int) []span {
	out := make([]span, len(spans))
	for i, sp := range spans {
		out[i] = span{max(sp.from, 0), min(sp.to, width)}
	}

	return out
}

func chip(s skin.Skin, t Tab) string {
	if t.Count == "" {
		return ""
	}

	face := s.Badge
	if t.Active {
		face = s.BadgeActive
	}

	switch {
	case t.Stale:
		face = face.Bold(false).Faint(true)
	case t.Alert && !t.Active:
		face = face.Foreground(s.Error.GetForeground())
	}

	return " " + face.Render(t.Count)
}

func facts(s skin.Skin, f Frame) string {
	parts := make([]string, 0, len(f.Facts))
	for _, fact := range f.Facts {
		parts = append(parts, tone(s, fact.Tone).Render(fact.Text))
	}

	return strings.Join(parts, gap)
}

// rule is the hairline under a Strip header, heavy under the active tab, with
// the Note at its right end.
func rule(s skin.Skin, f Frame, h head, width int) string {
	tail := ""
	room := width

	if f.Note != "" {
		tail = " " + s.Muted.Render(ansi.Truncate(f.Note, max(width/statusShare, 1), "…"))
		room = width - lipgloss.Width(tail)
	}

	under := span{room, room}
	for i, t := range f.Tabs {
		if t.Active && i < len(h.spans) {
			under = span{min(h.spans[i].from, room), min(h.spans[i].to, room)}
		}
	}

	return s.Hairline.Render(strings.Repeat("─", under.from)) +
		s.Accent.Render(strings.Repeat("━", under.to-under.from)) +
		s.Hairline.Render(strings.Repeat("─", room-under.to)) + tail
}

func footer(s skin.Skin, f Frame, width int) string {
	status := ansi.Truncate(s.Accent.Render(f.Status), width/statusShare, "…")

	room := width - lipgloss.Width(status)
	if status != "" {
		room -= len(gap)
	}

	faces := keyhint.Styles{Key: s.Key, Text: s.Subtitle, Off: s.Muted}
	if f.Strip {
		faces = keyhint.Styles{Key: s.Accent, Text: s.Muted, Off: s.Muted.Faint(true), Bare: true, Gap: bareGap}
	}

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
