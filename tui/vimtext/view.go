package vimtext

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"github.com/kyleking/aragonite/tui/glyph"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/pen"
	"github.com/kyleking/aragonite/tui/skin"
)

// Styles are the faces a field draws with.
type Styles struct {
	// Text is what the field holds, Placeholder what an empty one shows, and
	// Prompt what WithPrompt puts before it.
	Text, Placeholder, Prompt lipgloss.Style
	// Cursor is the cell under the cursor and Selection a visual selection.
	Cursor, Selection lipgloss.Style
	// Title is the line Render draws above the text.
	Title lipgloss.Style
	// Insert, Normal, and Visual are the mode's badge in the footer.
	Insert, Normal, Visual lipgloss.Style
	// Key and Hint draw the footer's keys and their words, Status a message
	// there and Error a refusal.
	Key, Hint, Status, Error lipgloss.Style
}

// DefaultStyles draws with attributes alone, for a caller without a skin.
func DefaultStyles() Styles {
	s := lipgloss.NewStyle()
	badge := s.Bold(true).Padding(0, 1)

	return Styles{
		Placeholder: s.Faint(true),
		Cursor:      s.Reverse(true),
		Selection:   s.Reverse(true),
		Title:       s.Bold(true),
		Insert:      badge.Reverse(true),
		Normal:      badge,
		Visual:      badge.Reverse(true).Italic(true),
		Key:         s.Bold(true),
		Hint:        s.Faint(true),
		Error:       s.Bold(true),
	}
}

// StylesFor draws a field in a skin's faces: the text as body copy, the
// selection as the cursor row's tint with the text's colors kept (reverse video
// at 16 colors and under NO_COLOR, as skin.Selected draws a selected row), the
// insert badge filled in the tool's accent, normal on a quiet chip, and visual
// in the accent on it.
func StylesFor(sk *skin.Skin) Styles {
	s := lipgloss.NewStyle()

	return Styles{
		Text:        sk.Body,
		Placeholder: sk.Muted,
		Prompt:      sk.Accent,
		Cursor:      s.Reverse(true),
		Selection:   sk.Selected(sk.Body),
		Title:       sk.Title,
		Insert:      sk.BadgeActive,
		Normal:      sk.Badge,
		Visual:      sk.Badge.Foreground(sk.Accent.GetForeground()),
		Key:         sk.Key,
		Hint:        sk.Muted,
		Status:      sk.Subtitle,
		Error:       sk.Error,
	}
}

// look is the faces a field draws with and the pens drawing them, kept apart
// from the field because the two are most of its size.
type look struct {
	styles Styles
	pens   pens
}

type pens struct {
	text, placeholder, prompt, cursor, selection, key, hint, status, fail pen.Pen
	// within is the cursor inside a selection, underlined so it still shows
	// where the selection is reverse video too.
	within pen.Pen
}

func (f *Field) setStyles(s Styles) {
	f.look = &look{styles: s}
	f.look.pens = pens{
		text: pen.Of(s.Text), placeholder: pen.Of(s.Placeholder), prompt: pen.Of(s.Prompt),
		cursor: pen.Of(s.Cursor), selection: pen.Of(s.Selection), key: pen.Of(s.Key),
		hint: pen.Of(s.Hint), status: pen.Of(s.Status), fail: pen.Of(s.Error),
		within: pen.Of(s.Cursor.Underline(true)),
	}
}

// tabStop is how far apart a tab's stops are drawn.
const tabStop = 4

// cell is one grapheme cluster as drawn: where it starts in the line, how many
// cells it takes, and what is drawn when that is not the cluster itself (a
// tab's spaces, a glyph settled for width).
type cell struct {
	disp   string
	off, w int
}

// layout is a line cut into cells and soft-wrapped into rows. A plain ASCII
// line keeps no cells: cell i is byte i, one cell wide. Cell n is the end of
// the line, one cell where an insert cursor or a selected line break draws.
type layout struct {
	src   string
	cells []cell
	rows  []int
	n     int
}

func (l *layout) plain() bool { return l.cells == nil }

func (l *layout) off(i int) int {
	if i >= l.n {
		return len(l.src)
	}

	if l.plain() {
		return i
	}

	return l.cells[i].off
}

func (l *layout) width(i int) int {
	if i >= l.n || l.plain() {
		return 1
	}

	return l.cells[i].w
}

func (l *layout) space(i int) bool {
	return i < l.n && l.src[l.off(i)] == ' '
}

// text is what cell i draws.
func (l *layout) text(i int) string {
	if i >= l.n {
		return " "
	}

	if l.plain() {
		return l.src[i : i+1]
	}

	if c := l.cells[i]; c.disp != "" {
		return c.disp
	}

	return l.src[l.off(i):l.off(i+1)]
}

// cellOf is the cell starting at byte off.
func (l *layout) cellOf(off int) int {
	if l.plain() || off >= len(l.src) {
		return min(off, l.n)
	}

	lo, hi := 0, l.n
	for lo < hi {
		mid := (lo + hi) / 2 //nolint:mnd // a binary search halves
		if l.cells[mid].off < off {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	return lo
}

// rowOf is the row cell i is drawn on.
func (l *layout) rowOf(i int) int {
	r := 0
	for r+1 < len(l.rows) && l.rows[r+1] <= i {
		r++
	}

	return r
}

func (l *layout) rowEnd(r int) int {
	if r+1 < len(l.rows) {
		return l.rows[r+1]
	}

	return l.n + 1
}

func newLayout(src string, width int, wrap bool) *layout {
	l := &layout{src: src}

	if ascii(src) && !strings.Contains(src, "\t") {
		l.n = len(src)
	} else {
		l.cells = cells(src)
		l.n = len(l.cells)
	}

	l.rows = []int{0}
	if !wrap {
		return l
	}

	col, brk := 0, -1

	for i := 0; i <= l.n; i++ {
		w := l.width(i)

		if col+w > width && col > 0 {
			start := i
			if brk > l.rows[len(l.rows)-1] && brk < i {
				start = brk
			}

			l.rows = append(l.rows, start)

			col = 0
			for j := start; j < i; j++ {
				col += l.width(j)
			}
		}

		col += w

		if l.space(i) {
			brk = i + 1
		}
	}

	return l
}

// cells cuts a line into drawn clusters: a tab as spaces to the next stop, a
// glyph terminals measure differently settled (glyph.Settle) so the row lines
// up, and a cluster that draws no cell of its own on a dotted circle.
func cells(src string) []cell {
	out := make([]cell, 0, len(src))
	col, off, state := 0, 0, -1

	for rest := src; rest != ""; {
		var c string

		var w int

		c, rest, w, state = uniseg.FirstGraphemeClusterInString(rest, state)
		cl := cell{off: off, w: w}

		switch {
		case c == "\t":
			cl.w = tabStop - col%tabStop
			cl.disp = strings.Repeat(" ", cl.w)
		case !ascii(c):
			d := glyph.Settle(c)

			dw := uniseg.StringWidth(d)
			if dw == 0 {
				d, dw = "◌"+c, uniseg.StringWidth("◌"+c)
			}

			if d != c {
				cl.disp = d
			}

			cl.w = dw
		}

		out = append(out, cl)
		off += len(c)
		col += cl.w
	}

	return out
}

// layoutCache keeps each line's layout keyed by its text, so a keystroke lays
// out the one line it changed.
type layoutCache struct {
	m     map[string]*layout
	width int
	wrap  bool
}

func (f *Field) layout(line string, width int) *layout {
	c := &f.lay
	wrap := !f.oneLine

	if c.m == nil || c.width != width || c.wrap != wrap || len(c.m) > 4*len(f.buf.lines)+64 {
		c.m, c.width, c.wrap = make(map[string]*layout, len(f.buf.lines)), width, wrap
	}

	if l, ok := c.m[line]; ok {
		return l
	}

	l := newLayout(line, width, wrap)
	c.m[line] = l

	return l
}

// displayCol is the display column of byte col in line.
func displayCol(line string, col int) int {
	if ascii(line) && !strings.Contains(line, "\t") {
		return col
	}

	d := 0
	for _, c := range cells(line) {
		if c.off >= col {
			break
		}

		d += c.w
	}

	return d
}

// atDisplay is the byte offset of the cluster covering display column want,
// or the end of the line when it is shorter.
func atDisplay(line string, want int) int {
	if ascii(line) && !strings.Contains(line, "\t") {
		return min(want, len(line))
	}

	d := 0
	for _, c := range cells(line) {
		if d+c.w > want {
			return c.off
		}

		d += c.w
	}

	return len(line)
}

// textWidth is the width the text gets after the prompt.
func (f *Field) textWidth(width int) int {
	return max(1, width-ansi.StringWidth(f.prompt))
}

// rowCount is how many rows the text takes, counted no further than limit.
func (f *Field) rowCount(width, limit int) int {
	n := 0
	for _, l := range f.buf.lines {
		n += len(f.layout(l, width).rows)
		if n >= limit {
			return limit
		}
	}

	return n
}

// Height is how many rows View draws at width.
func (f *Field) Height(width int) int {
	if f.oneLine {
		return 1
	}

	return max(f.minH, f.rowCount(f.textWidth(width), f.maxH))
}

// rowRef is a row of the text: a line and a row within it.
type rowRef = pos

func (f *Field) rows(line, width int) int { return len(f.layout(f.buf.lines[line], width).rows) }

// scroll keeps the cursor's row inside the h rows drawn from f.top.
func (f *Field) scroll(width, h int) {
	l := f.layout(f.buf.lines[f.cur.line], width)
	c := rowRef{f.cur.line, l.rowOf(l.cellOf(f.cur.col))}

	f.top.line = min(f.top.line, f.buf.last())
	f.top.col = min(f.top.col, f.rows(f.top.line, width)-1)

	if c.less(f.top) {
		f.top = c

		return
	}

	// Rows from the top through the cursor's, counted no further than h.
	n := 0
	for line := f.top.line; line <= c.line && n <= h; line++ {
		first, last := 0, f.rows(line, width)-1
		if line == f.top.line {
			first = f.top.col
		}

		if line == c.line {
			last = c.col
		}

		n += last - first + 1
	}

	if n > h {
		f.top = f.back(c, h-1, width)
	}

	// Text that shrank leaves room under its end, which the rows above fill.
	below := 0
	for line := f.top.line; line <= f.buf.last() && below < h; line++ {
		below += f.rows(line, width)
		if line == f.top.line {
			below -= f.top.col
		}
	}

	if below < h {
		f.top = f.back(f.top, h-below, width)
	}
}

// back is the row n rows above r, stopping at the first.
func (f *Field) back(r rowRef, n, width int) rowRef {
	for ; n > 0; n-- {
		switch {
		case r.col > 0:
			r.col--
		case r.line > 0:
			r.line--
			r.col = f.rows(r.line, width) - 1
		default:
			return r
		}
	}

	return r
}

// selection is the visual selection's extent on line: the byte range it covers
// and whether it takes the line's end.
func (f *Field) selection(line int) (int, int, bool) {
	if f.mode != Visual && f.mode != VisualLine {
		return 0, 0, false
	}

	s := f.visualSpan()
	l := f.buf.lines[line]

	switch {
	case line < s.from.line || line > s.to.line:
		return 0, 0, false
	case s.lines:
		return 0, len(l), true
	}

	from, to, eol := 0, len(l), true
	if line == s.from.line {
		from = s.from.col
	}

	if line == s.to.line {
		to, eol = s.to.col, false
	}

	if s.from == s.to {
		return 0, 0, false
	}

	return from, to, eol
}

// View draws the text: as many rows as Height says at width, soft-wrapped,
// scrolled to keep the cursor in sight, every row exactly width cells.
func (f *Field) View(width int) string {
	tw := f.textWidth(width)
	pad := strings.Repeat(" ", max(0, width-tw))

	if f.placeholder != "" && len(f.buf.lines) == 1 && f.buf.lines[0] == "" {
		return f.placeholderView(tw, pad)
	}

	if f.oneLine {
		return f.oneLineView(tw, pad)
	}

	h := f.Height(width)
	f.scroll(tw, h)

	var b strings.Builder

	r := f.top
	for i := range h {
		if i > 0 {
			b.WriteByte('\n')
		}

		if r.line > f.buf.last() {
			b.WriteString(strings.Repeat(" ", width))

			continue
		}

		f.promptCell(&b, r == rowRef{}, pad)

		l := f.layout(f.buf.lines[r.line], tw)
		f.drawCells(&b, r.line, l, l.rows[r.col], l.rowEnd(r.col), tw)

		r.col++
		if r.col >= len(l.rows) {
			r = rowRef{r.line + 1, 0}
		}
	}

	return b.String()
}

func (f *Field) promptCell(b *strings.Builder, first bool, pad string) {
	switch {
	case pad == "":
	case first:
		f.look.pens.prompt.Write(b, f.prompt)
	default:
		b.WriteString(pad)
	}
}

// The faces a cell is drawn in, indexing the pens drawCells keeps.
const (
	faceText = iota
	faceSelected
	faceCursor
	faceWithin
)

// drawCells draws cells [from, to) of a line, padded to width.
func (f *Field) drawCells(b *strings.Builder, line int, l *layout, from, to, width int) {
	cursor := -1
	if line == f.cur.line {
		cursor = l.cellOf(f.cur.col)
	}

	selFrom, selTo, eol := f.selection(line)
	selected := func(i int) bool {
		if i >= l.n {
			return eol
		}

		o := l.off(i)

		return o >= selFrom && o < selTo
	}

	used := 0
	run := strings.Builder{}
	runPen := -1

	flush := func() {
		if run.Len() == 0 {
			return
		}

		p := f.look.pens
		[]pen.Pen{p.text, p.selection, p.cursor, p.within}[runPen].Write(b, run.String())
		run.Reset()
	}

	for i := from; i < to; i++ {
		face := faceText

		switch {
		case i == cursor && selected(i):
			face = faceWithin
		case i == cursor:
			face = faceCursor
		case selected(i):
			face = faceSelected
		case i >= l.n:
			continue
		}

		if used+l.width(i) > width {
			break
		}

		if face != runPen {
			flush()

			runPen = face
		}

		run.WriteString(l.text(i))
		used += l.width(i)
	}

	flush()
	b.WriteString(strings.Repeat(" ", max(0, width-used)))
}

func (f *Field) oneLineView(tw int, pad string) string {
	src := f.buf.lines[0]
	l := f.layout(src, tw)
	c := l.cellOf(f.cur.col)

	x := 0
	for i := range c {
		x += l.width(i)
	}

	if x < f.hscrl {
		f.hscrl = x
	}

	if x+l.width(c) > f.hscrl+tw {
		f.hscrl = x + l.width(c) - tw
	}

	first, skipped := 0, 0
	for first < l.n && skipped+l.width(first) <= f.hscrl {
		skipped += l.width(first)
		first++
	}

	var b strings.Builder

	f.promptCell(&b, true, pad)

	if lead := f.hscrl - skipped; lead > 0 && first < l.n {
		// A wide character cut by the left edge shows as blanks.
		b.WriteString(strings.Repeat(" ", min(lead, tw)))
		tw -= lead
		first++
	}

	f.drawCells(&b, 0, l, first, l.n+1, tw)

	return b.String()
}

func (f *Field) placeholderView(tw int, pad string) string {
	var b strings.Builder

	f.promptCell(&b, true, pad)

	ph := ansi.Truncate(glyph.Settle(f.placeholder), tw, "…")
	first := ""

	if ph != "" {
		first = clusterAt(ph, 0)
	}

	if first == "" || ansi.StringWidth(first) != 1 {
		f.look.pens.cursor.Write(&b, " ")
		ph = ansi.Truncate(ph, tw-1, "")
	} else {
		f.look.pens.cursor.Write(&b, first)
		ph = ph[len(first):]
	}

	f.look.pens.placeholder.Write(&b, ph)
	b.WriteString(strings.Repeat(" ", max(0, tw-1-ansi.StringWidth(ph))))

	width := tw + ansi.StringWidth(pad)
	for range f.Height(width) - 1 {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat(" ", width))
	}

	return b.String()
}

// How long a hint stays in a footer that runs out of room: save is pinned,
// and esc outlasts the rest because it is the way out.
const (
	keepEditor = iota + 2
	keepComplete
	keepChange
	keepVerb
	keepNewline
	keepSave
	keepEsc
)

// Hints are the keys the footer offers in the field's mode, for a caller that
// fits them into a footer of its own.
func (f *Field) Hints() []keyhint.Hint {
	save := keyhint.Hint{Key: "⏎", What: "save", Pin: true, Priority: keepSave}
	esc := keyhint.Hint{Key: keyEsc, What: "normal", Priority: keepEsc}

	var hs []keyhint.Hint

	switch f.mode {
	case Insert:
		hs = append(hs, save)
		if !f.oneLine {
			hs = append(hs, keyhint.Hint{Key: keyCtrlJ, What: "new line", Priority: keepNewline})
		}

		if !f.vim {
			esc.What = "cancel"
		}

		hs = append(hs, esc)

		if f.complete != nil {
			hs = append(hs, keyhint.Hint{Key: keyCtrlN, What: "complete", Priority: keepComplete})
		}
	case Normal:
		esc.What = "cancel"
		hs = append(hs, save, keyhint.Hint{Key: "i", What: "insert", Priority: keepNewline}, esc)
	case Visual, VisualLine:
		hs = append(hs, save,
			keyhint.Hint{Key: "d", What: "delete", Priority: keepVerb},
			keyhint.Hint{Key: "y", What: "yank", Priority: keepVerb},
			keyhint.Hint{Key: "c", What: "change", Priority: keepChange},
			esc)
	}

	if f.editor != "" {
		hs = append(hs, keyhint.Hint{Key: "ctrl+e", What: "$EDITOR", Priority: keepEditor})
	}

	return hs
}

// Badge is the mode's badge, or "" with the vim layer off.
func (f *Field) Badge() string {
	if !f.vim {
		return ""
	}

	st := f.look.styles.Insert

	switch f.mode {
	case Normal:
		st = f.look.styles.Normal
	case Visual, VisualLine:
		st = f.look.styles.Visual
	case Insert:
	}

	return st.Render(f.mode.String())
}

// Footer is one row, exactly width cells: the mode's badge, then the : line,
// a message, or the keys the mode offers.
func (f *Field) Footer(width int) string {
	var b strings.Builder

	if badge := f.Badge(); badge != "" {
		b.WriteString(badge)
		b.WriteByte(' ')
	}

	if len(f.pend) > 0 {
		f.look.pens.key.Write(&b, strings.Join(f.pend, ""))
		b.WriteByte(' ')
	}

	room := max(0, width-ansi.StringWidth(b.String()))

	switch {
	case f.cmd != nil:
		f.look.pens.key.Write(&b, ansi.Truncate(":"+f.cmd.text, max(0, room-1), ""))
		f.look.pens.cursor.Write(&b, " ")
	case f.status != "" && f.failed:
		f.look.pens.fail.Write(&b, ansi.Truncate(f.status, room, "…"))
	case f.status != "":
		f.look.pens.status.Write(&b, ansi.Truncate(f.status, room, "…"))
	default:
		st := keyhint.Styles{
			Key: f.look.styles.Key, Text: f.look.styles.Hint, Bare: true, Gap: f.look.pens.hint.Render(" · "),
		}
		b.WriteString(keyhint.Fit(st, f.Hints(), room))
	}

	out := ansi.Truncate(b.String(), width, "")

	return out + strings.Repeat(" ", max(0, width-ansi.StringWidth(out)))
}

// Render is the whole field: the title when there is one, the text, and the
// footer.
func (f *Field) Render(width int) string {
	parts := make([]string, 0, 3)
	if f.title != "" {
		t := ansi.Truncate(glyph.Settle(f.title), width, "…")
		parts = append(parts, f.look.styles.Title.Render(t)+strings.Repeat(" ", max(0, width-ansi.StringWidth(t))))
	}

	return strings.Join(append(parts, f.View(width), f.Footer(width)), "\n")
}
