package vimtext

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// visualSpan is the selection: anchor to cursor with both ends taken, whole
// lines in V, and the line break after a $ end.
func (f *Field) visualSpan() span {
	a, b := order(f.anchor, f.cur)
	if f.mode == VisualLine {
		return span{from: pos{a.line, 0}, to: pos{b.line, 0}, lines: true, at: a}
	}

	to := b
	l := f.buf.lines[to.line]

	switch {
	case f.wantEnd && b == f.cur || to.col >= len(l):
		if to.line < f.buf.last() {
			to = pos{to.line + 1, 0}
		} else {
			to.col = len(l)
		}
	default:
		to.col = nextB(l, to.col)
	}

	return span{from: a, to: to, at: a}
}

// removeSpan deletes a span without touching the register.
func (f *Field) removeSpan(s span) {
	if s.lines {
		f.buf.deleteLines(s.from.line, s.to.line+1)
		f.cur = pos{min(s.from.line, f.buf.last()), 0}

		return
	}

	f.buf.remove(s.from, s.to)
	f.cur = s.from
}

// visualCmd runs a command in visual mode.
func (f *Field) visualCmd(c command) tea.Cmd {
	defer func() {
		if f.mode != Insert {
			f.closeGroup()
			f.finish()
			f.clampNormal()
		}
	}()

	switch {
	case c.object != nil:
		f.visualObject(c)
	case c.motion != nil:
		f.move(c)
	case c.verb != nil:
		f.visualVerb(c)
	}

	return nil
}

func (f *Field) visualObject(c command) {
	s, ok := f.object(c.object[0] == "a", c.object[1], c.n())
	if !ok {
		return
	}

	if s.lines {
		f.mode = VisualLine
		f.anchor, f.cur = pos{s.from.line, 0}, pos{s.to.line, 0}

		return
	}

	end := s.to
	if end.col > 0 {
		end.col = prevB(f.buf.lines[end.line], end.col)
	} else if end.line > s.from.line {
		end = pos{end.line - 1, len(f.buf.lines[end.line-1])}
	}

	if f.anchor == f.cur {
		f.anchor = s.from
	}

	f.cur = end
	f.setWant()
}

// visualVerb runs an operator or command over the selection.
func (f *Field) visualVerb(c command) {
	v := c.verb[0]

	switch v {
	case "o", "O":
		f.anchor, f.cur = f.cur, f.anchor
		f.setWant()

		return
	case "v", "V":
		f.toggleVisual(v)

		return
	case "I", "A", ".", keyCtrlR, ":", "i", "a":
		return
	}

	s := f.visualSpan()

	// The capitals act on whole lines, whatever the selection's shape.
	switch v {
	case "X", "D", "C", "S", "R", "Y":
		s = wholeLines(s)
	}

	f.recordVisual(c.verb, s)
	f.mode = Normal

	switch v {
	case "d", "x", "X", "D":
		f.apply("d", s)
	case "c", "s", "C", "S", "R":
		f.apply("c", s)
	case "y", "Y":
		f.apply("y", s)
	default:
		f.transform(c.verb, s)
	}
}

// transform runs the visual commands that rewrite the selection in place.
func (f *Field) transform(verb []string, s span) {
	switch verb[0] {
	case "~", "u", "U":
		f.caseSpan(verb[0], s)
	case "J":
		f.joinLines(s.from.line, max(2, s.to.line-s.from.line+1)) //nolint:mnd // J joins at least two lines
	case "p", "P":
		f.replaceSelection(s)
	case "r":
		f.replaceSpan(s, verb[1])
	}
}

func (f *Field) toggleVisual(v string) {
	want := Visual
	if v == "V" {
		want = VisualLine
	}

	if f.mode == want {
		f.mode = Normal
	} else {
		f.mode = want
	}
}

// wholeLines widens a selection to its lines. A selection that ends by taking
// a line break does not take the line after it.
func wholeLines(s span) span {
	last := s.to.line
	if !s.lines && s.to.col == 0 && s.to.line > s.from.line {
		last--
	}

	return span{from: pos{s.from.line, 0}, to: pos{last, 0}, lines: true, at: s.at}
}

// visRepeat is a visual operator as the dot command repeats it: over as much text again
// from the cursor, the same number of lines and, for characters, as many on
// the last line (counted from the start of the selection on one line).
type visRepeat struct {
	verb  []string
	lines int
	cols  int
	whole bool
}

// recordVisual makes the dot command repeat a visual operator, but not a
// yank, which is not a change.
func (f *Field) recordVisual(verb []string, s span) {
	if f.replaying || verb[0] == "y" || verb[0] == "Y" {
		return
	}

	v := &visRepeat{verb: verb, lines: s.to.line - s.from.line + 1, whole: s.lines}

	switch {
	case s.lines:
	case s.from.line == s.to.line:
		v.cols = count(f.buf.lines[s.from.line][s.from.col:s.to.col])
	case s.to.col == 0:
		v.lines--
		v.cols = count(f.buf.lines[s.to.line-1]) + 1
	default:
		v.cols = count(f.buf.lines[s.to.line][:s.to.col])
	}

	f.rec = &change{vis: v}
}

// repeatVisual selects as much text as the recorded operator took, from the
// cursor, and runs it again.
func (f *Field) repeatVisual(v *visRepeat) {
	b := &f.buf
	f.anchor = f.cur
	f.wantEnd = false
	last := min(f.cur.line+v.lines-1, b.last())

	switch {
	case v.whole:
		f.mode = VisualLine
		f.cur.line = last
	case v.lines == 1:
		f.mode = Visual
		f.cur.col = advance(b.lines[f.cur.line], f.cur.col, v.cols-1)
	default:
		f.mode = Visual
		f.cur = pos{last, advance(b.lines[last], 0, v.cols-1)}
	}

	f.cur.col = min(f.cur.col, max(0, b.lastChar(f.cur.line)))
	f.visualVerb(command{verb: v.verb})
}

func (f *Field) caseSpan(v string, s span) {
	conv := swapCase

	switch v {
	case "u":
		conv = strings.ToLower
	case "U":
		conv = strings.ToUpper
	}

	f.mutate()

	if s.lines {
		for i := s.from.line; i <= s.to.line; i++ {
			f.buf.lines[i] = conv(f.buf.lines[i])
		}

		f.cur = pos{s.from.line, 0}

		return
	}

	text := conv(f.buf.slice(s.from, s.to))
	f.buf.remove(s.from, s.to)
	f.buf.insert(s.from, text)
	f.cur = s.from
}

func (f *Field) replaceSpan(s span, char string) {
	if r, size := utf8.DecodeRuneInString(char); size != len(char) || unicode.IsControl(r) {
		return
	}

	f.mutate()

	if s.lines {
		s = span{from: pos{s.from.line, 0}, to: pos{s.to.line, len(f.buf.lines[s.to.line])}}
	}

	parts := strings.Split(f.buf.slice(s.from, s.to), "\n")
	for i, p := range parts {
		parts[i] = strings.Repeat(char, count(p))
	}

	f.buf.remove(s.from, s.to)
	f.buf.insert(s.from, strings.Join(parts, "\n"))
	f.cur = s.from
}

// replaceSelection is p in visual mode: the selection swapped for the
// register, and the register then holding what was selected.
func (f *Field) replaceSelection(s span) {
	text, linewise := f.reg.Text, f.reg.Linewise
	if f.oneLine {
		text, linewise = strings.ReplaceAll(text, "\n", " "), false
	}

	var old string

	f.mutate()

	switch {
	case s.lines:
		old = strings.Join(f.buf.lines[s.from.line:s.to.line+1], "\n")
		repl := strings.Split(text, "\n")
		f.buf.lines = append(f.buf.lines[:s.from.line], append(repl, f.buf.lines[s.to.line+1:]...)...)
		f.cur = pos{s.from.line, f.buf.firstNonBlank(s.from.line)}
	case linewise:
		old = f.buf.slice(s.from, s.to)
		f.buf.remove(s.from, s.to)
		f.buf.insert(s.from, "\n"+text+"\n")
		f.cur = pos{s.from.line + 1, 0}
	default:
		old = f.buf.slice(s.from, s.to)
		f.buf.remove(s.from, s.to)
		end := f.buf.insert(s.from, text)
		f.cur = pos{end.line, prevB(f.buf.lines[end.line], end.col)}
	}

	f.yanked(old, s.lines)
	f.setWant()
}
