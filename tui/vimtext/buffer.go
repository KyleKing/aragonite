package vimtext

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// pos is a place in the buffer: a line and a byte offset into it that always
// sits on a grapheme cluster boundary. The offset len(line) is the end of the
// line, vim's NUL position, where only insert mode, visual mode, and a motion
// in flight may rest.
type pos struct {
	line, col int
}

func (p pos) less(q pos) bool {
	return p.line < q.line || p.line == q.line && p.col < q.col
}

func order(a, b pos) (pos, pos) {
	if b.less(a) {
		return b, a
	}

	return a, b
}

// ascii reports whether s holds no multi-byte rune, which makes every byte a
// grapheme cluster of its own.
func ascii(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}

// nextB is the boundary after the cluster starting at i, len(s) at the end.
func nextB(s string, i int) int {
	if i >= len(s) {
		return len(s)
	}

	if s[i] < utf8.RuneSelf && (i+1 == len(s) || s[i+1] < utf8.RuneSelf) {
		return i + 1
	}

	c, rest, _, _ := uniseg.FirstGraphemeClusterInString(s[i:], -1)
	if rest == "" {
		return len(s)
	}

	return i + len(c)
}

// prevB is the boundary before i, 0 at the start.
func prevB(s string, i int) int {
	if i <= 0 {
		return 0
	}

	if s[i-1] < utf8.RuneSelf && (i == 1 || s[i-2] < utf8.RuneSelf) {
		return i - 1
	}

	// A cluster can only be found walking forward, so walk the line to i. The
	// lines a field holds are prose, so this is a short walk.
	at, state := 0, -1
	for rest := s; at < i; {
		var c string

		c, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
		if at+len(c) >= i {
			return at
		}

		at += len(c)
	}

	return at
}

// clusterAt is the cluster starting at byte i.
func clusterAt(s string, i int) string {
	return s[i:nextB(s, i)]
}

// count is how many clusters s holds.
func count(s string) int {
	if ascii(s) {
		return len(s)
	}

	return uniseg.GraphemeClusterCount(s)
}

// advance moves n clusters forward from i, stopping at len(s).
func advance(s string, i, n int) int {
	for ; n > 0 && i < len(s); n-- {
		i = nextB(s, i)
	}

	return i
}

// Character classes, as vim's cls() answers them: blank, punctuation, word,
// and a class of their own for emoji and for each CJK script, so a run of
// ideographs is a word apart from the Latin around it. A script's class is
// the first code point of its block, as vim numbers them.
const (
	clsBlank    = 0
	clsPunct    = 1
	clsWord     = 2
	clsEmoji    = 3
	clsHiragana = 0x3040
	clsKatakana = 0x30a0
	clsHan      = 0x4e00
	clsHangul   = 0xac00
)

var scripts = []struct {
	table *unicode.RangeTable
	class int
}{
	{unicode.Han, clsHan},
	{unicode.Hiragana, clsHiragana},
	{unicode.Katakana, clsKatakana},
	{unicode.Hangul, clsHangul},
}

func classOf(c string, big bool) int {
	if c == "" {
		return clsBlank
	}

	r, _ := utf8.DecodeRuneInString(c)

	switch {
	case r == ' ' || r == '\t' || unicode.IsSpace(r):
		return clsBlank
	case big:
		return clsPunct
	case r < utf8.RuneSelf:
		return asciiClass(r)
	}

	for _, s := range scripts {
		if unicode.Is(s.table, r) {
			return s.class
		}
	}

	switch {
	case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r):
		return clsWord
	case unicode.Is(unicode.So, r) || r >= 0x1F000:
		return clsEmoji
	}

	return clsPunct
}

func asciiClass(r rune) int {
	if r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
		return clsWord
	}

	return clsPunct
}

// buffer is the text as lines, none holding a newline. It always has at least
// one line. Lines are immutable strings, so a snapshot for undo copies only
// the slice.
type buffer struct {
	lines []string
}

func newBuffer(text string) buffer {
	return buffer{lines: strings.Split(clean(text), "\n")}
}

// clean makes text what a field holds: a carriage return is a line break (a
// terminal pastes one for each, as tmux does), and other control characters
// than a tab or a line break are dropped.
func clean(text string) string {
	if !strings.ContainsFunc(text, func(r rune) bool {
		return r < ' ' && r != '\t' && r != '\n' || r == 0x7f
	}) {
		return text
	}

	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")

	return strings.Map(func(r rune) rune {
		if r < ' ' && r != '\t' && r != '\n' || r == 0x7f {
			return -1
		}

		return r
	}, text)
}

func (b *buffer) text() string { return strings.Join(b.lines, "\n") }

func (b *buffer) last() int { return len(b.lines) - 1 }

// cls is the class at p, with the end of a line blank as vim's NUL is.
func (b *buffer) cls(p pos, big bool) int {
	l := b.lines[p.line]
	if p.col >= len(l) {
		return clsBlank
	}

	return classOf(clusterAt(l, p.col), big)
}

func (b *buffer) empty(line int) bool { return b.lines[line] == "" }

// inc is vim's inc(): one cluster forward. It answers 2 on stepping onto the
// end of a line, 1 on moving to the next line, -1 at the end of the buffer,
// and 0 otherwise.
func (b *buffer) inc(p *pos) int {
	l := b.lines[p.line]
	if p.col < len(l) {
		p.col = nextB(l, p.col)
		if p.col < len(l) {
			return 0
		}

		return 2 //nolint:mnd // vim's code for stepping onto the end of a line
	}

	if p.line < b.last() {
		p.line, p.col = p.line+1, 0

		return 1
	}

	return -1
}

// dec is vim's dec(): one cluster back, onto the end of the line before at
// the start of a line (answering 1), -1 at the start of the buffer.
func (b *buffer) dec(p *pos) int {
	if p.col > 0 {
		p.col = prevB(b.lines[p.line], min(p.col, len(b.lines[p.line])))

		return 0
	}

	if p.line > 0 {
		p.line--
		p.col = len(b.lines[p.line])

		return 1
	}

	return -1
}

// incl is inc stepping over the end of a line it lands on.
func (b *buffer) incl(p *pos) int {
	r := b.inc(p)
	if r >= 1 && p.col > 0 {
		r = b.inc(p)
	}

	return r
}

// decl is dec stepping off the end of a line it lands on.
func (b *buffer) decl(p *pos) int {
	r := b.dec(p)
	if r == 1 && p.col > 0 {
		r = b.dec(p)
	}

	return r
}

// lastChar is the start of the line's last cluster, 0 on an empty line.
func (b *buffer) lastChar(line int) int {
	return prevB(b.lines[line], len(b.lines[line]))
}

// firstNonBlank is the offset of the line's first character that is not a
// space or a tab, or its last character when it has none.
func (b *buffer) firstNonBlank(line int) int {
	l := b.lines[line]

	i := 0
	for i < len(l) && (l[i] == ' ' || l[i] == '\t') {
		i++
	}

	if i == len(l) {
		return b.lastChar(line)
	}

	return i
}

func indentOf(l string) string {
	i := 0
	for i < len(l) && (l[i] == ' ' || l[i] == '\t') {
		i++
	}

	return l[:i]
}

// white reports a line holding nothing but blanks.
func white(l string) bool {
	return strings.TrimLeft(l, " \t") == ""
}

// slice is the text from a to b, a before b, with a newline between lines.
func (b *buffer) slice(from, to pos) string {
	if from.line == to.line {
		return b.lines[from.line][from.col:to.col]
	}

	var s strings.Builder

	s.WriteString(b.lines[from.line][from.col:])

	for i := from.line + 1; i < to.line; i++ {
		s.WriteByte('\n')
		s.WriteString(b.lines[i])
	}

	s.WriteByte('\n')
	s.WriteString(b.lines[to.line][:to.col])

	return s.String()
}

// remove deletes from a to b, a before b.
func (b *buffer) remove(from, to pos) {
	joined := b.lines[from.line][:from.col] + b.lines[to.line][to.col:]
	b.lines[from.line] = joined
	b.deleteLines(from.line+1, to.line+1)
}

// insert puts text at p and answers where it ends.
func (b *buffer) insert(p pos, text string) pos {
	if !strings.Contains(text, "\n") {
		l := b.lines[p.line]
		b.lines[p.line] = l[:p.col] + text + l[p.col:]

		return pos{p.line, p.col + len(text)}
	}

	parts := strings.Split(text, "\n")
	l := b.lines[p.line]
	tail := l[p.col:]
	b.lines[p.line] = l[:p.col] + parts[0]
	last := parts[len(parts)-1]
	parts[len(parts)-1] = last + tail
	b.insertLines(p.line+1, parts[1:]...)

	return pos{p.line + len(parts) - 1, len(last)}
}

func (b *buffer) insertLines(at int, ls ...string) {
	b.lines = append(b.lines[:at], append(append([]string(nil), ls...), b.lines[at:]...)...)
}

// deleteLines removes lines [from, to). A buffer emptied this way keeps one
// empty line.
func (b *buffer) deleteLines(from, to int) {
	if from >= to {
		return
	}

	b.lines = append(b.lines[:from], b.lines[to:]...)
	if len(b.lines) == 0 {
		b.lines = []string{""}
	}
}

// clone copies the line slice, which is all a snapshot needs since a line is
// never edited in place.
func (b *buffer) clone() []string {
	return append([]string(nil), b.lines...)
}
