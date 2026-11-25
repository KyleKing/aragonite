package vimtext

import "strings"

// kind is how an operator treats the text a motion crosses.
type kind int

const (
	exclusive kind = iota
	inclusive
	linewise
)

// target is where a motion lands and how an operator over it reads.
type target struct {
	p    pos
	kind kind
	// vertical keeps the column j and k aim for rather than setting it.
	vertical bool
	// toEnd is $, which j and k then follow to the end of each line.
	toEnd bool
}

// findState is the last f, F, t, or T, which ; and , repeat.
type findState struct {
	cmd  string
	char string
}

// motionLen answers how many tokens the motion at the start of keys takes: 0
// for no motion, -1 for one still waiting on a key.
func motionLen(keys []string) int {
	if len(keys) == 0 {
		return -1
	}

	switch k := keys[0]; {
	case k == "g":
		if len(keys) < 2 { //nolint:mnd // g and its second key
			return -1
		}

		if _, ok := gMotions[keys[1]]; ok {
			return 2 //nolint:mnd // g and its second key
		}

		return 0
	case takesChar(k) && k != "r":
		if len(keys) < 2 { //nolint:mnd // the command and its character
			return -1
		}

		return 2 //nolint:mnd // the command and its character
	}

	if _, ok := motions[keys[0]]; ok {
		return 1
	}

	return 0
}

// motionArgs is what a motion runs with: its keys, the count (at least 1),
// whether one was typed, and the operator waiting on it ("" for none).
type motionArgs struct {
	op      string
	keys    []string
	n       int
	counted bool
}

type motionFunc func(f *Field, a motionArgs) (target, bool)

// motions are the motions by their first key.
var motions = map[string]motionFunc{
	"h": left, keyLeft: left, keyBackspace: left,
	"l": toRight, keyRight: toRight, " ": toRight,
	"j": down, keyDown: down, keyCtrlJ: down, keyCtrlN: down,
	"k": up, keyUp: up, keyCtrlP: up,
	"0":     func(f *Field, _ motionArgs) (target, bool) { return target{p: pos{f.cur.line, 0}}, true },
	keyHome: func(f *Field, _ motionArgs) (target, bool) { return target{p: pos{f.cur.line, 0}}, true },
	"^": func(f *Field, _ motionArgs) (target, bool) {
		return target{p: pos{f.cur.line, f.buf.firstNonBlank(f.cur.line)}}, true
	},
	"$": dollar, keyEnd: dollar,
	"G": func(f *Field, a motionArgs) (target, bool) {
		if a.counted {
			return f.toLine(a.n - 1), true
		}

		return f.toLine(f.buf.last()), true
	},
	"w": toWord, "W": toWord,
	"b": back, "B": back,
	"e": wordEnd, "E": wordEnd,
	"f": find, "F": find, "t": find, "T": find,
	";": refind, ",": refind,
	"%": percent,
	"{": toParagraph, "}": toParagraph,
}

// gMotions are the motions after g.
var gMotions = map[string]motionFunc{
	"g": func(f *Field, a motionArgs) (target, bool) {
		if a.counted {
			return f.toLine(a.n - 1), true
		}

		return f.toLine(0), true
	},
	"e": backEnd, "E": backEnd,
}

// motion runs the motion in a.keys from the cursor.
func (f *Field) motion(keys []string, n int, counted bool, op string) (target, bool) {
	a := motionArgs{keys: keys, n: n, counted: counted, op: op}

	if keys[0] == "g" {
		return gMotions[keys[1]](f, a)
	}

	return motions[keys[0]](f, a)
}

func left(f *Field, a motionArgs) (target, bool) {
	p := f.cur
	if p.col == 0 {
		return target{}, false
	}

	for n := a.n; n > 0 && p.col > 0; n-- {
		p.col = prevB(f.buf.lines[p.line], p.col)
	}

	return target{p: p}, true
}

func toRight(f *Field, a motionArgs) (target, bool) { return f.right(a.n, a.op != "") }

func toWord(f *Field, a motionArgs) (target, bool) { return f.word(a.n, a.keys[0] == "W", a.op) }

func wordEnd(f *Field, a motionArgs) (target, bool) { return f.end(a.n, a.keys[0] == "E", a.op != "") }

func toParagraph(f *Field, a motionArgs) (target, bool) { return f.paragraph(a.n, a.keys[0] == "}") }

func up(f *Field, a motionArgs) (target, bool) { return f.vertical(-a.n) }

func down(f *Field, a motionArgs) (target, bool) { return f.vertical(a.n) }

func dollar(f *Field, a motionArgs) (target, bool) {
	line := f.cur.line + a.n - 1
	if line > f.buf.last() {
		return target{}, false
	}

	return target{p: pos{line, f.buf.lastChar(line)}, kind: inclusive, toEnd: true}, true
}

func back(f *Field, a motionArgs) (target, bool) {
	p := f.cur
	ok := f.backWord(&p, a.n, a.keys[0] == "B", false)

	return target{p: p}, ok
}

func backEnd(f *Field, a motionArgs) (target, bool) {
	p := f.cur
	ok := f.backWordEnd(&p, a.n, a.keys[1] == "E")

	return target{p: p, kind: inclusive}, ok
}

func find(f *Field, a motionArgs) (target, bool) {
	f.find = findState{cmd: a.keys[0], char: a.keys[1]}

	return f.findChar(a.keys[0], a.keys[1], a.n, false)
}

func refind(f *Field, a motionArgs) (target, bool) {
	return f.repeatFind(a.keys[0] == ",", a.n)
}

// percent is % alone, to the bracket's partner, and N% to the line N percent
// of the way down.
func percent(f *Field, a motionArgs) (target, bool) {
	if !a.counted {
		return f.match()
	}

	if a.n > 100 { //nolint:mnd // a percentage
		return target{}, false
	}

	return f.toLine((a.n*len(f.buf.lines)+99)/100 - 1), true //nolint:mnd // vim rounds the percentage up
}

// toLine is a linewise jump keeping the column, as nvim's nostartofline does.
func (f *Field) toLine(line int) target {
	line = max(0, min(line, f.buf.last()))

	return target{p: pos{line, f.colFor(line)}, kind: linewise, vertical: true}
}

func (f *Field) right(n int, op bool) (target, bool) {
	l := f.buf.lines[f.cur.line]
	p := f.cur

	if op {
		// An operator may take the last character, so l may step onto the end.
		if p.col >= len(l) {
			return target{}, false
		}

		return target{p: pos{p.line, advance(l, p.col, n)}}, true
	}

	limit := f.buf.lastChar(p.line)
	if f.mode == Insert {
		limit = len(l)
	}

	if p.col >= limit {
		return target{}, false
	}

	for ; n > 0 && p.col < limit; n-- {
		p.col = nextB(l, p.col)
	}

	return target{p: p}, true
}

func (f *Field) vertical(n int) (target, bool) {
	if f.oneLine {
		return target{}, false
	}

	line := f.cur.line + n
	if n > 0 && f.cur.line == f.buf.last() || n < 0 && f.cur.line == 0 {
		return target{}, false
	}

	line = max(0, min(line, f.buf.last()))

	return target{p: pos{line, f.colFor(line)}, kind: linewise, vertical: true}, true
}

// colFor is the column on line that j and k land on, from the column they aim
// for.
func (f *Field) colFor(line int) int {
	l := f.buf.lines[line]
	if f.wantEnd {
		if f.mode == Insert {
			return len(l)
		}

		return f.buf.lastChar(line)
	}

	col := atDisplay(l, f.want)
	if f.mode != Insert && col >= len(l) && l != "" {
		col = f.buf.lastChar(line)
	}

	return col
}

// word is w and W, vim's fwd_word with the operator rules nv_wordcmd adds: cw
// on a word changes to its end, and an operator never takes a w past the end
// of the line its last word is on.
func (f *Field) word(n int, big bool, op string) (target, bool) {
	b := &f.buf
	p := f.cur
	start := p

	if op == "c" {
		if c := b.cls(p, false); p.col < len(b.lines[p.line]) && c != clsBlank {
			ok := f.endWord(&p, n, big, true, false)
			f.adjust(&p, start, true)

			return target{p: p, kind: inclusive}, ok
		}
	}

	ok := f.fwdWord(&p, n, big, op != "")

	k := exclusive
	if f.adjust(&p, start, op != "") {
		k = inclusive
	}

	return target{p: p, kind: k}, ok || op != ""
}

func (f *Field) end(n int, big, op bool) (target, bool) {
	p := f.cur
	start := p
	ok := f.endWord(&p, n, big, false, false)
	f.adjust(&p, start, op)

	return target{p: p, kind: inclusive}, ok
}

// adjust is vim's adjust_cursor: a motion that moved forward onto the end of a
// line steps back onto its last character, which makes an operator over it
// inclusive. It answers whether it stepped.
func (f *Field) adjust(p *pos, start pos, op bool) bool {
	if !start.less(*p) || f.mode == Visual || f.mode == VisualLine {
		return false
	}

	if p.col > 0 && p.col >= len(f.buf.lines[p.line]) {
		p.col = f.buf.lastChar(p.line)

		return op
	}

	return false
}

func (f *Field) repeatFind(reverse bool, n int) (target, bool) {
	if f.find.cmd == "" {
		return target{}, false
	}

	cmd := f.find.cmd
	if reverse {
		cmd = map[string]string{"f": "F", "F": "f", "t": "T", "T": "t"}[cmd]
	}

	return f.findChar(cmd, f.find.char, n, true)
}

const (
	opens  = "([{"
	closes = ")]}"
)

// match is % without a count: the bracket under the cursor or the first one
// after it on the line, to its partner.
func (f *Field) match() (target, bool) {
	l := f.buf.lines[f.cur.line]

	at := -1

	for i := f.cur.col; i < len(l); i++ {
		if strings.IndexByte(opens+closes, l[i]) >= 0 {
			at = i

			break
		}
	}

	if at < 0 {
		return target{}, false
	}

	p, ok := f.partner(pos{f.cur.line, at})

	return target{p: p, kind: inclusive}, ok
}

// partner finds the bracket matching the one at p, counting nesting.
func (f *Field) partner(p pos) (pos, bool) {
	c := f.buf.lines[p.line][p.col]
	if i := strings.IndexByte(opens, c); i >= 0 {
		return f.scan(p, c, closes[i], true)
	}

	i := strings.IndexByte(closes, c)

	return f.scan(p, c, opens[i], false)
}

// scan walks from p (not counting it) for the unmatched closer, the depth
// going up on an opener and down on a closer.
func (f *Field) scan(p pos, opener, closer byte, forward bool) (pos, bool) {
	b := &f.buf
	depth := 0

	for {
		var r int
		if forward {
			r = b.inc(&p)
		} else {
			r = b.dec(&p)
		}

		if r == -1 {
			return pos{}, false
		}

		l := b.lines[p.line]
		if p.col >= len(l) {
			continue
		}

		switch l[p.col] {
		case opener:
			depth++
		case closer:
			if depth == 0 {
				return p, true
			}

			depth--
		}
	}
}
