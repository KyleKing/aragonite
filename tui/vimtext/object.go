package vimtext

import "strings"

// object is a text object after i (inner) or a (around), ported from vim's
// textobject.c for an operator and a single-character visual selection.
func (f *Field) object(around bool, obj string, n int) (span, bool) {
	switch obj {
	case "w", "W":
		return f.currentWord(n, around, obj == "W")
	case "p":
		return f.currentPar(n, around)
	case `"`, "'", "`":
		return f.currentQuote(around, obj[0])
	case "b", "(", ")":
		return f.currentBlock(around, '(', ')', n)
	case "B", "{", "}":
		return f.currentBlock(around, '{', '}', n)
	case "[", "]":
		return f.currentBlock(around, '[', ']', n)
	case "<", ">":
		return f.currentBlock(around, '<', '>', n)
	}

	return span{}, false
}

// charSpan is the text from a to b as an operator reads it: b taken when the
// motion is inclusive, and vim's two rules for an exclusive end at the start
// of a line, which shorten it to the line before and make it linewise when it
// started in the indent.
func (f *Field) charSpan(a, b pos, k kind) span {
	from, to := order(a, b)

	if k == exclusive && to.col == 0 && to.line > from.line {
		if strings.TrimLeft(f.buf.lines[from.line][:from.col], " \t") == "" {
			return span{from: pos{from.line, 0}, to: pos{to.line - 1, 0}, lines: true, at: from}
		}

		to.line--
		to.col = len(f.buf.lines[to.line])

		if to.col > 0 {
			to.col = f.buf.lastChar(to.line)
			k = inclusive
		}
	}

	if k == inclusive {
		to.col = nextB(f.buf.lines[to.line], to.col)
	}

	return span{from: from, to: to, at: from}
}

// backInLine moves to the start of the run of p's class within its line.
func (f *Field) backInLine(p *pos, big bool) {
	l := f.buf.lines[p.line]
	c := f.buf.cls(*p, big)

	for p.col > 0 {
		q := pos{p.line, prevB(l, p.col)}
		if f.buf.cls(q, big) != c {
			break
		}

		*p = q
	}
}

// currentWord is iw, aw, iW, and aW: vim's current_word.
func (f *Field) currentWord(n int, include, big bool) (span, bool) {
	b := &f.buf
	p := f.cur

	f.backInLine(&p, big)
	start := p
	white := false

	if (b.cls(p, big) == clsBlank) == include {
		if !f.endWord(&p, 1, big, true, true) {
			return span{}, false
		}
	} else {
		f.fwdWord(&p, 1, big, true)
		f.oneLeft(&p)

		white = include
	}

	incl, ok := f.moreWords(&p, n-1, include, big)
	if !ok {
		return span{}, false
	}

	if white && (b.cls(p, big) != clsBlank || p.col == 0 && !incl) {
		start = f.whiteBefore(start, big)
	}

	k := exclusive
	if incl {
		k = inclusive
	}

	s := f.charSpan(start, p, k)
	s.at = start

	return s, true
}

// oneLeft steps back one character, onto the end of the line before from the
// start of a line.
func (f *Field) oneLeft(p *pos) {
	if p.col == 0 {
		f.buf.decl(p)

		return
	}

	p.col = prevB(f.buf.lines[p.line], p.col)
}

// moreWords extends a word object by n more words or runs of blanks, as a
// count asks, answering whether its end is taken.
func (f *Field) moreWords(p *pos, n int, include, big bool) (bool, bool) {
	b := &f.buf
	incl := true

	for ; n > 0; n-- {
		incl = true

		if b.incl(p) == -1 {
			return false, false
		}

		if include == (b.cls(*p, big) == clsBlank) {
			if !f.endWord(p, 1, big, true, true) {
				return false, false
			}

			continue
		}

		if !f.fwdWord(p, 1, big, true) && n > 1 {
			return false, false
		}

		if p.col == 0 {
			incl = false
		} else {
			p.col = prevB(b.lines[p.line], p.col)
		}
	}

	return incl, true
}

// whiteBefore takes the blanks before a word into aw when there are none after
// it, but never an indent.
func (f *Field) whiteBefore(start pos, big bool) pos {
	if start.col == 0 {
		return start
	}

	q := start
	q.col = prevB(f.buf.lines[q.line], q.col)
	f.backInLine(&q, big)

	if f.buf.cls(q, big) == clsBlank && q.col > 0 {
		return q
	}

	return start
}

// currentPar is ip and ap: vim's current_par.
func (f *Field) currentPar(n int, include bool) (span, bool) {
	b := &f.buf
	lw := func(i int) bool { return white(b.lines[i]) }
	start := f.parStart()
	whiteFront := lw(f.cur.line)

	end := start
	for end <= b.last() && lw(end) {
		end++
	}

	end--

	i := n
	if !include && whiteFront {
		i--
	}

	for ; i > 0; i-- {
		var ok bool
		if end, ok = f.parEnd(end, include, whiteFront && i == 1); !ok {
			return span{}, false
		}
	}

	if !whiteFront && !lw(end) && include {
		for start > 0 && lw(start-1) {
			start--
		}
	}

	return span{from: pos{start, 0}, to: pos{end, 0}, lines: true, at: pos{start, 0}}, true
}

// parStart is the first line of the paragraph, or of the blank lines, the
// cursor is in.
func (f *Field) parStart() int {
	b := &f.buf
	start := f.cur.line
	whiteFront := white(b.lines[start])

	for start > 0 {
		above := white(b.lines[start-1])
		if whiteFront && !above || !whiteFront && (above || b.lines[start] == "") {
			break
		}

		start--
	}

	return start
}

// parEnd extends a paragraph object by one paragraph, with the blank lines
// after it unless this is the last count of an ap that started on blanks.
func (f *Field) parEnd(end int, include, last bool) (int, bool) {
	b := &f.buf
	lw := func(i int) bool { return white(b.lines[i]) }

	if end == b.last() {
		return end, false
	}

	doWhite := !include && lw(end+1)

	if include || !doWhite {
		end++
		for end < b.last() && !lw(end+1) && b.lines[end+1] != "" {
			end++
		}
	}

	if last && include {
		return end, true
	}

	if include || doWhite {
		for end < b.last() && lw(end+1) {
			end++
		}
	}

	return end, true
}

// findNextQuote is the first unescaped q at or after col, -1 for none.
func findNextQuote(l string, col int, q byte, escapes bool) int {
	for col < len(l) {
		switch c := l[col]; {
		case escapes && c == '\\':
			col++
			if col >= len(l) {
				return -1
			}
		case c == q:
			return col
		}

		col = nextB(l, col)
	}

	return -1
}

// findPrevQuote is the last unescaped q before col, or 0 when there is none,
// which the caller tells from a quote by looking.
func findPrevQuote(l string, col int, q byte) int {
	for col > 0 {
		col = prevB(l, col)

		n := 0
		for col-n > 0 && l[col-n-1] == '\\' {
			n++
		}

		if n&1 == 1 {
			col -= n
		} else if l[col] == q {
			break
		}
	}

	return col
}

// currentQuote is i", a", and the same for ' and `: vim's current_quote, on
// the cursor's line only.
func (f *Field) currentQuote(include bool, q byte) (span, bool) {
	l := f.buf.lines[f.cur.line]
	if l == "" {
		return span{}, false
	}

	start, end, ok := quotes(l, f.cur.col, q)
	if !ok {
		return span{}, false
	}

	isWhite := func(i int) bool { return i >= 0 && i < len(l) && (l[i] == ' ' || l[i] == '\t') }

	switch {
	case !include:
		start++
	case isWhite(end + 1):
		for isWhite(end + 1) {
			end++
		}

		end++
	default:
		for start > 0 && isWhite(start-1) {
			start--
		}

		end++
	}

	line := f.cur.line

	return span{from: pos{line, start}, to: pos{line, end}, at: pos{line, start}}, true
}

// quotes finds the pair of q around col: from the start of the line when col
// is on a quote, since whether it opens or closes depends on the ones before
// it, and otherwise the one before col (or the first after it) and the next.
func quotes(l string, col int, q byte) (int, int, bool) {
	if l[col] != q {
		start := findPrevQuote(l, col, q)
		if l[start] != q {
			if start = findNextQuote(l, start, q, false); start < 0 {
				return 0, 0, false
			}
		}

		end := findNextQuote(l, start+1, q, true)

		return start, end, end >= 0
	}

	for end := 0; ; end++ {
		start := findNextQuote(l, end, q, false)
		if start < 0 || start > col {
			return 0, 0, false
		}

		if end = findNextQuote(l, start+1, q, true); end < 0 {
			return 0, 0, false
		}

		if start <= col && col <= end {
			return start, end, true
		}
	}
}

// inIndent reports whether everything up to and including p is blank.
func (f *Field) inIndent(p pos) bool {
	return len(indentOf(f.buf.lines[p.line])) > p.col
}

// currentBlock is i(, a(, and the other brackets: vim's current_block.
func (f *Field) currentBlock(include bool, opener, closer byte, n int) (span, bool) {
	b := &f.buf
	p := f.cur

	if opener == '{' {
		for f.inIndent(p) {
			if b.inc(&p) != 0 {
				break
			}
		}
	}

	if l := b.lines[p.line]; p.col < len(l) && l[p.col] == opener {
		p.col++
	}

	var start pos

	for range n {
		q, ok := f.scan(p, closer, opener, false)
		if !ok {
			return span{}, false
		}

		start, p = q, q
	}

	cur, ok := f.scan(start, opener, closer, true)
	if !ok {
		return span{}, false
	}

	sol := false

	if !include {
		b.incl(&start)
		sol = cur.col == 0
		b.decl(&cur)

		for f.inIndent(cur) {
			sol = true

			if b.decl(&cur) != 0 {
				break
			}
		}
	}

	k := exclusive

	switch {
	case sol:
		b.incl(&cur)
	case !cur.less(start):
		k = inclusive
	default:
		cur = start
	}

	s := f.charSpan(start, cur, k)
	s.at = start

	return s, true
}
