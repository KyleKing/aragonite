package vimtext

// step is how one iteration of a word motion ended: carry on to the next
// count, stop where it is (a success), or fail.
type step int

const (
	stepOn step = iota
	stepDone
	stepFail
)

// The word motions below are vim's search.c (fwd_word, end_word, and the two
// that step backward) one count at a time, so the operator rules that hang off them
// (dw stopping at the end of its line, cw acting as ce) come out the same.

// fwdWord is w and W. With eol, which an operator asks for, the last count
// stops at the end of its line.
func (f *Field) fwdWord(p *pos, n int, big, eol bool) bool {
	for ; n > 0; n-- {
		switch f.fwdWordOnce(p, big, eol && n == 1) {
		case stepFail:
			return false
		case stepDone:
			return true
		case stepOn:
		}
	}

	return true
}

func (f *Field) fwdWordOnce(p *pos, big, eol bool) step {
	b := &f.buf
	sclass := b.cls(*p, big)

	i := b.inc(p)
	if i == -1 {
		return stepFail
	}

	if i >= 1 && eol {
		return stepDone
	}

	// The rest of the word the cursor started in, then the blanks after it,
	// stopping on an empty line, which is a word of its own.
	if sclass != clsBlank && f.advanceWhile(p, eol, func(q pos) bool { return b.cls(q, big) == sclass }) {
		return stepDone
	}

	if f.advanceWhile(p, eol, func(q pos) bool { return b.cls(q, big) == clsBlank && !b.emptyAt(q) }) {
		return stepDone
	}

	return stepOn
}

// advanceWhile steps forward while ok holds, answering true on running off the
// buffer or, with eol, off the end of the line.
func (f *Field) advanceWhile(p *pos, eol bool, ok func(pos) bool) bool {
	for ok(*p) {
		if i := f.buf.inc(p); i == -1 || i >= 1 && eol {
			return true
		}
	}

	return false
}

// emptyAt reports p at the start of an empty line.
func (b *buffer) emptyAt(p pos) bool { return p.col == 0 && b.empty(p.line) }

// endWord is e and E. With stop, a cursor already at the end of a word stays
// there for the first count, and with empty an empty line ends a word.
func (f *Field) endWord(p *pos, n int, big, stop, empty bool) bool {
	for ; n > 0; n-- {
		if f.endWordOnce(p, big, stop, empty) == stepFail {
			return false
		}

		stop = false
	}

	return true
}

func (f *Field) endWordOnce(p *pos, big, stop, empty bool) step {
	b := &f.buf
	sclass := b.cls(*p, big)

	if b.inc(p) == -1 {
		return stepFail
	}

	switch {
	case b.cls(*p, big) == sclass && sclass != clsBlank:
		if f.skip(p, sclass, big, true) {
			return stepFail
		}
	case !stop || sclass == clsBlank:
		for b.cls(*p, big) == clsBlank {
			if empty && b.emptyAt(*p) {
				return stepOn
			}

			if b.inc(p) == -1 {
				return stepFail
			}
		}

		if f.skip(p, b.cls(*p, big), big, true) {
			return stepFail
		}
	}

	b.dec(p)

	return stepOn
}

// backWord is b and B.
func (f *Field) backWord(p *pos, n int, big, stop bool) bool {
	for ; n > 0; n-- {
		switch f.backWordOnce(p, big, stop) {
		case stepFail:
			return false
		case stepDone:
			return true
		case stepOn:
		}

		stop = false
	}

	return true
}

func (f *Field) backWordOnce(p *pos, big, stop bool) step {
	b := &f.buf
	sclass := b.cls(*p, big)

	if b.dec(p) == -1 {
		return stepFail
	}

	if stop && sclass != b.cls(*p, big) && sclass != clsBlank {
		b.inc(p)

		return stepOn
	}

	// Blanks before the word, stopping on an empty line, then back to the
	// word's start.
	for b.cls(*p, big) == clsBlank {
		if b.emptyAt(*p) {
			return stepOn
		}

		if b.dec(p) == -1 {
			return stepDone
		}
	}

	if f.skip(p, b.cls(*p, big), big, false) {
		return stepDone
	}

	b.inc(p)

	return stepOn
}

// backWordEnd is ge and gE.
func (f *Field) backWordEnd(p *pos, n int, big bool) bool {
	for ; n > 0; n-- {
		switch f.backWordEndOnce(p, big) {
		case stepFail:
			return false
		case stepDone:
			return true
		case stepOn:
		}
	}

	return true
}

func (f *Field) backWordEndOnce(p *pos, big bool) step {
	b := &f.buf
	sclass := b.cls(*p, big)

	if b.dec(p) == -1 {
		return stepFail
	}

	if sclass != clsBlank && f.skip(p, sclass, big, false) {
		return stepDone
	}

	for b.cls(*p, big) == clsBlank && !b.emptyAt(*p) {
		if b.dec(p) == -1 {
			return stepDone
		}
	}

	return stepOn
}

// skip moves over characters of class c, answering true on running off the
// buffer.
func (f *Field) skip(p *pos, c int, big, forward bool) bool {
	move := f.buf.dec
	if forward {
		move = f.buf.inc
	}

	for f.buf.cls(*p, big) == c {
		if move(p) == -1 {
			return true
		}
	}

	return false
}

// findChar is f, F, t, and T within the line, again for ; and , repeating one.
func (f *Field) findChar(cmd, char string, n int, again bool) (target, bool) {
	l := f.buf.lines[f.cur.line]
	forward := cmd == "f" || cmd == "t"
	till := cmd == "t" || cmd == "T"

	col := f.cur.col

	var ok bool

	// A repeated t that already stands before its character would find it
	// again, so ; does not count the first character it looks at (vim
	// without ; in cpoptions).
	skipFirst := again && till && n == 1

	for n > 0 {
		if col, ok = stepCol(l, col, forward); !ok {
			return target{}, false
		}

		if clusterAt(l, col) == char && !skipFirst {
			n--
		}

		skipFirst = false
	}

	k := exclusive
	if forward {
		k = inclusive
	}

	if till {
		col, _ = stepCol(l, col, !forward)
	}

	return target{p: pos{f.cur.line, col}, kind: k}, true
}

// stepCol is one cluster along the line, false off either end.
func stepCol(l string, col int, forward bool) (int, bool) {
	if !forward {
		return prevB(l, col), col > 0
	}

	next := nextB(l, col)

	return next, next < len(l)
}

// paragraph is { and }, vim's findpar: to the next empty line, or to the end of
// the buffer, where } lands on the last character and an operator takes it.
func (f *Field) paragraph(n int, forward bool) (target, bool) {
	b := &f.buf
	dir := -1

	if forward {
		dir = 1
	}

	curr := f.cur.line

	for ; n > 0; n-- {
		next, ok := f.nextPar(curr, dir)
		if !ok && n > 1 {
			return target{}, false
		}

		curr = next
	}

	if curr == b.last() && forward && b.lines[curr] != "" {
		return target{p: pos{curr, b.lastChar(curr)}, kind: inclusive}, true
	}

	return target{p: pos{curr, 0}}, true
}

// nextPar is the empty line after the text from curr in direction dir, or the
// buffer's end, answering false at the end.
func (f *Field) nextPar(curr, dir int) (int, bool) {
	b := &f.buf
	skipped := false

	for first := true; ; first = false {
		if b.lines[curr] != "" {
			skipped = true
		}

		if !first && skipped && b.lines[curr] == "" {
			return curr, true
		}

		if curr+dir < 0 || curr+dir > b.last() {
			return curr, false
		}

		curr += dir
	}
}
