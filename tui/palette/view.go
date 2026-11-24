package palette

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/overlay"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/table"
)

// Drawer sizing: a share of the frame between a floor and a ceiling, leaving the screen behind
// its left edge readable.
const (
	drawerMin    = 52
	drawerMax    = 76
	drawerShare  = 55
	percent      = 100
	drawerKeep   = 24
	borderCells  = 4
	keyColumnMax = 8
	rightColumn  = 9
	rightShare   = 3
	stripWhatMin = 22
	stripCmdMin  = 8
	stripMinRows = 3
	stripShare   = 2
	stripOf      = 5
	minListRows  = 3
	chrome       = 6
	halves       = 2
	cursorMark   = "▏"
	selectMark   = "▌ "
	noMark       = "  "
)

// Resolve is the layout the palette draws in at width.
func (m *Model) Resolve(width int) Layout {
	if m.layout != Auto {
		return m.layout
	}

	if width >= DrawerFrom {
		return Drawer
	}

	return Strip
}

// DrawerWidth is how wide the drawer is in a frame width cells wide.
func DrawerWidth(width int) int {
	return max(min(max(drawerMin, width*drawerShare/percent), drawerMax, width-drawerKeep), min(width, drawerMin))
}

// Rect is the cells the palette covers in a frame width by height cells, so a click outside it can
// close it. A closed palette covers nothing.
func (m *Model) Rect(width, height int) overlay.Rect {
	if !m.open || width < 1 || height < 1 {
		return overlay.Rect{}
	}

	block := m.View(width, height)

	if m.Resolve(width) == Drawer {
		return overlay.At(block, width-DrawerWidth(width), 0)
	}

	return overlay.At(block, 0, height-len(block))
}

// Over draws the palette onto base, a frame width cells wide whose every line is one row: the
// drawer down the right edge, or the strip over the bottom rows. A closed palette leaves base as
// it is.
func (m *Model) Over(base []string, width int) []string {
	if !m.open || len(base) == 0 {
		return base
	}

	block := m.View(width, len(base))

	if m.Resolve(width) == Drawer {
		return overlay.Blit(base, block, width-DrawerWidth(width), 0)
	}

	out := make([]string, len(base))
	copy(out, base)
	copy(out[len(out)-len(block):], block)

	return out
}

// View is the palette's block for a frame width by height cells: the drawer's box as tall as the
// frame, or the strip's rows, never taller than the frame.
func (m *Model) View(width, height int) []string {
	if !m.open || width < 1 || height < 1 {
		return nil
	}

	if m.Resolve(width) == Drawer {
		return m.drawer(DrawerWidth(width), height)
	}

	return m.strip(width, height)
}

func (m *Model) title() string {
	if m.mode == Browse {
		return "keys"
	}

	return "commands"
}

func (m *Model) hintStyles() keyhint.Styles {
	return keyhint.Styles{Key: m.skin.Key, Text: m.skin.Muted, Off: m.skin.Muted, Bare: true}
}

func (m *Model) footHints() []keyhint.Hint {
	if m.mode == Browse && !m.typing {
		return []keyhint.Hint{
			{Key: keyEnter, What: "run", Pin: true},
			{Key: "/", What: "filter", Priority: 1},
			{Key: "↑↓", What: "move"},
			{Key: keyEsc, What: "back", Pin: true},
		}
	}

	return []keyhint.Hint{
		{Key: keyEnter, What: "run", Pin: true},
		{Key: keyTab, What: "fill"},
		{Key: "ctrl+n/p", What: "move", Priority: 1},
		{Key: keyEsc, What: "close", Pin: true},
	}
}

// prompt is the query line: what has been typed, a prefix waiting, or how to start.
func (m *Model) prompt(width int) string {
	switch {
	case len(m.pending) > 0:
		return m.skin.Key.Render(strings.Join(m.pending, " ")) + m.skin.Muted.Render(" then…")
	case m.typing && m.mode == Browse:
		return m.skin.Accent.Render("/") + m.skin.Body.Render(table.TruncateLeft(m.query, width-len("/▏"))) +
			cursorMark
	case m.typing:
		return m.skin.Accent.Render(":") + " " + m.skin.Body.Render(table.TruncateLeft(m.query, width-len(": ▏"))) +
			cursorMark
	}

	return m.skin.Muted.Render(table.Truncate("a key explains itself · / filters", width))
}

// count is the tally a list's heading carries: how many it shows of how many there are.
func (m *Model) count() string {
	n := strconv.Itoa(len(m.hits)) + " of " + strconv.Itoa(len(m.entries))
	if m.mode == Run && strings.TrimSpace(m.query) == "" && len(m.pending) == 0 {
		n += " · here first"
	}

	return n
}

func (m *Model) drawer(width, height int) []string {
	inner := width - borderCells
	if inner < 1 {
		return nil
	}

	var detail []string
	if sel, ok := m.Selected(); ok {
		detail = Note(m.skin, sel, inner, m.now())
		if m.status != "" {
			detail = append(detail, m.skin.Warning.Render(table.Truncate(m.status, inner)))
		}
	} else if m.status != "" {
		detail = []string{m.skin.Warning.Render(table.Truncate(m.status, inner))}
	}

	room := height - chrome
	if len(detail) > room/halves {
		detail = detail[:max(0, room/halves)]
	}

	list := room - len(detail)
	if list < minListRows {
		detail, list = nil, room
	}

	rows := []string{m.prompt(inner)}
	rows = append(rows, m.list(inner, min(m.rows(), max(0, list)), true)...)
	rows = append(rows, "")
	rows = append(rows, detail...)

	for len(rows) < height-3 {
		rows = append(rows, "")
	}

	rows = append(rows, keyhint.Fit(m.hintStyles(), m.footHints(), inner))

	return box(m.skin, m.title()+" · "+m.count(), rows, inner)
}

func box(s skin.Skin, title string, rows []string, inner int) []string {
	head := "╭─ " + table.Truncate(title, inner-1) + " "
	out := make([]string, 0, len(rows)+halves)
	out = append(out, s.Edge.Render(head+strings.Repeat("─", max(0, inner+3-lipgloss.Width(head)))+"╮"))

	for _, r := range rows {
		r = ansi.Truncate(r, inner, "")
		pad := strings.Repeat(" ", max(0, inner-lipgloss.Width(r)))
		out = append(out, s.Edge.Render("│ ")+r+pad+s.Edge.Render(" │"))
	}

	return append(out, s.Edge.Render("╰"+strings.Repeat("─", inner+halves)+"╯"))
}

func (m *Model) strip(width, height int) []string {
	var note []string
	if sel, ok := m.Explained(); ok {
		note = Note(m.skin, sel, width-halves, m.now())
		for i := range note {
			note[i] = " " + note[i]
		}
	}

	list := min(m.rows(), max(stripMinRows, height*stripShare/stripOf))
	if 1+list+len(note)+1 > height {
		note = nil
		list = max(0, height-halves)
	}

	out := []string{m.skin.Rule(m.title(), m.count(), width)}
	out = append(out, m.list(width, list, false)...)
	out = append(out, note...)

	foot := keyhint.Fit(m.hintStyles(), m.footHints(), width/halves)
	left := m.prompt(width - lipgloss.Width(foot) - halves)

	if m.status != "" {
		left = m.skin.Warning.Render(m.status)
	}

	left = ansi.Truncate(left, width-lipgloss.Width(foot)-1, "")

	return append(out, left+strings.Repeat(" ", max(1, width-lipgloss.Width(left)-lipgloss.Width(foot)))+foot)
}

// line is one drawn row of the list: a section's rule, or a hit by its index in m.hits.
type line struct {
	rule string
	hit  int
}

func (m *Model) lines() []line {
	var out []line

	at := 0
	for _, s := range m.sections {
		out = append(out, line{rule: s.Name, hit: -1})
		for range s.Hits {
			out = append(out, line{hit: at})
			at++
		}
	}

	return out
}

// list draws rows lines of the list, scrolled so the cursor is on screen, padded to rows.
func (m *Model) list(width, rows int, drawer bool) []string {
	if rows < 1 {
		return nil
	}

	all := m.lines()
	sel := 0

	for i, l := range all {
		if l.hit == m.cursor {
			sel = i
		}
	}

	from, to := table.ScrollWindow(sel, len(all), rows)
	c := m.columns(width)
	out := make([]string, 0, rows)

	for _, l := range all[from:to] {
		if l.hit < 0 {
			out = append(out, m.skin.Rule(l.rule, "", width))

			continue
		}

		out = append(out, m.row(&m.hits[l.hit], l.hit == m.cursor, width, c, drawer))
	}

	if len(m.hits) == 0 {
		out = append(out, m.skin.Muted.Render(noMark+"nothing matches"))
	}

	return out
}

// Rows is how many lines the list takes drawn whole: a rule per section and a row per hit, or the
// one line saying nothing matches.
func (m *Model) rows() int {
	if len(m.hits) == 0 {
		return 1
	}

	return len(m.sections) + len(m.hits)
}

// columns are the key and right column widths, shared by every row so the columns line up, and
// whether any row has a command for the strip's command column to show. A list with nothing to
// say in the right column (no counts, no recent runs) draws none, so its descriptions get the room.
type columns struct {
	key, right int
	cmd        bool
}

func (m *Model) columns(width int) columns {
	c := columns{key: 1}

	for i := range m.hits {
		c.key = max(c.key, lipgloss.Width(m.hits[i].Key))
		c.right = max(c.right, lipgloss.Width(m.right(&m.hits[i])))
		c.cmd = c.cmd || m.hits[i].Cmd != ""
	}

	c.key = min(c.key, keyColumnMax)
	if c.right > 0 {
		c.right = min(max(c.right, rightColumn), width/rightShare)
	}

	return c
}

// row draws one hit: the mark, the key, the description with the matched letters lifted (and for an
// entry off here, the reason after it), and the right column, which is how recently or often it
// ran. The reason rides with the description so a long one never narrows every row's column.
// The strip adds the command column between, since it has the width and no explainer pane.
func (m *Model) row(h *Hit, selected bool, width int, c columns, drawer bool) string {
	face := func(st lipgloss.Style) lipgloss.Style {
		if selected {
			return st.Inherit(m.skin.Selection)
		}

		return st
	}

	mark := noMark
	if selected {
		mark = selectMark
	}

	keyFace, textFace := m.skin.Key, m.skin.Body
	if h.Off {
		keyFace, textFace = m.skin.Muted, m.skin.Muted
	}

	keyw, rightw := c.key, c.right

	room := width - lipgloss.Width(mark) - keyw - halves
	if rightw > 0 {
		room -= rightw + 1
	}

	cmdw := 0
	if whatw := max(stripWhatMin, room*stripShare/stripOf); !drawer && c.cmd && room-whatw-halves >= stripCmdMin {
		cmdw, room = room-whatw-halves, whatw
	}

	out := face(m.skin.Accent).Render(mark) +
		face(keyFace).Render(table.Pad(h.Key, keyw, table.AlignLeft)) + face(textFace).Render("  ") +
		what(h, max(1, room), face(textFace), face(m.skin.Accent.Bold(true)))

	if cmdw > 0 {
		out += face(m.skin.Muted).Render("  " + table.Pad(table.Truncate(h.Cmd, cmdw), cmdw, table.AlignLeft))
	}

	if rightw > 0 {
		out += face(m.skin.Muted).Render(" " + table.Pad(table.Truncate(m.right(h), rightw), rightw, table.AlignRight))
	}

	if pad := width - lipgloss.Width(out); pad > 0 {
		out += face(lipgloss.NewStyle()).Render(strings.Repeat(" ", pad))
	}

	return out
}

// what draws a description width cells wide, the runes the query matched in hi, and the alias
// the query found it by after it.
func what(h *Hit, width int, base, hi lipgloss.Style) string {
	text := h.What
	if h.Via != "" {
		text += " ← " + h.Via
	}

	if h.Off && h.Why != "" {
		text += " · " + h.Why
	}

	plain := []rune(table.Truncate(text, width))
	at := map[int]bool{}

	for _, p := range h.Pos {
		at[p] = true
	}

	var b strings.Builder

	// Runs of one face are drawn in one call, since a style costs far more per call than per rune.
	from := 0
	for i := 1; i <= len(plain); i++ {
		if i < len(plain) && lit(at, plain, i) == lit(at, plain, from) {
			continue
		}

		face := base
		if lit(at, plain, from) {
			face = hi
		}

		b.WriteString(face.Render(string(plain[from:i])))
		from = i
	}

	return b.String() + base.Render(strings.Repeat(" ", max(0, width-len(plain))))
}

// lit reports whether the rune at i is one the query matched, which a cut's ellipsis never is.
func lit(at map[int]bool, plain []rune, i int) bool {
	return at[i] && (i != len(plain)-1 || string(plain[i]) != table.Ellipsis)
}

func (m *Model) right(h *Hit) string {
	switch {
	case h.Off:
		return ""
	case !h.Recent.IsZero():
		return Ago(m.now().Sub(h.Recent))
	case h.Counted && h.Count == 0:
		return "never"
	case h.Counted:
		return strconv.Itoa(h.Count) + "×"
	}

	return ""
}

// Ago is a duration as a list column says it: "just now", "12m ago", "3h ago", "2d ago".
func Ago(d time.Duration) string {
	const day = 24 * time.Hour

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d/time.Minute)) + "m ago"
	case d < day:
		return strconv.Itoa(int(d/time.Hour)) + "h ago"
	}

	return strconv.Itoa(int(d/day)) + "d ago"
}
