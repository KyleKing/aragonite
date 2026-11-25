package vimtext

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// maxCount caps a typed count, so 99999999x cannot spin.
const maxCount = 10000

// parse states.
const (
	more = iota
	invalid
	complete
)

// command is one parsed normal-mode command.
type command struct {
	// op is d, c, or y when an operator waits on what follows.
	op string
	// object is a text object (i or a and its letter); motion a motion; and
	// verb a command of its own, each with its argument.
	object, motion, verb []string
	// keys is the command without its counts, which the dot command replays.
	keys []string
	// count is the product of the counts typed, 0 when none was.
	count int
	// doubled is dd, cc, or yy.
	doubled bool
}

func isDigit(s string) bool { return len(s) == 1 && s[0] >= '0' && s[0] <= '9' }

// takeCount reads a count at keys[i:], a leading 0 being the motion rather
// than a digit.
func takeCount(keys []string, i int) (int, int) {
	n := 0

	for i < len(keys) && isDigit(keys[i]) && (n > 0 || keys[i] != "0") {
		n = min(n*10+int(keys[i][0]-'0'), maxCount)
		i++
	}

	return n, i
}

func mul(a, b int) int {
	switch {
	case a == 0:
		return b
	case b == 0:
		return a
	}

	return min(a*b, maxCount)
}

// objects are the text objects after i or a.
const objects = "wWpb()B{}[]<>\"'`"

// verbs takes keys[i:] as a command that is not a motion: how many tokens it
// takes, -1 for one still waiting, 0 for none.
func verbLen(keys []string, visual bool) int {
	switch keys[0] {
	case "x", "X", "s", "S", "~", "J", "p", "P", "D", "C", "Y", "u", keyCtrlR, ".", ":",
		"i", "a", "I", "A", "o", "O", "v", "V":
		if visual && (keys[0] == "i" || keys[0] == "a") {
			return 0
		}

		return 1
	case "r":
		if len(keys) < 2 { //nolint:mnd // r and its character
			return -1
		}

		return 2 //nolint:mnd // r and its character
	case "d", "c", "y", "U", "R":
		if visual {
			return 1
		}
	}

	return 0
}

// parse reads the keys typed so far as a command.
func parse(keys []string, visual bool) (command, int) {
	var c command

	n, i := takeCount(keys, 0)
	c.count = n

	if i == len(keys) {
		return c, more
	}

	c.keys = nonCount(keys)

	if !visual && isOperator(keys[i]) {
		return c.withOperator(keys, i)
	}

	rest := keys[i:]

	if visual && (rest[0] == "i" || rest[0] == "a") {
		return c.withObject(rest)
	}

	switch v := verbLen(rest, visual); {
	case v < 0:
		return c, more
	case v > 0:
		c.verb = rest

		return c, done(len(rest) == v)
	}

	return c.withMotion(rest)
}

// withOperator reads what follows an operator at keys[i]: a second count, then
// the operator again, a text object, or a motion.
func (c command) withOperator(keys []string, i int) (command, int) {
	c.op = keys[i]

	n2, i := takeCount(keys, i+1)
	c.count = mul(c.count, n2)

	if i == len(keys) {
		return c, more
	}

	rest := keys[i:]

	switch rest[0] {
	case c.op:
		c.doubled = true

		return c, done(len(rest) == 1)
	case "i", "a":
		return c.withObject(rest)
	}

	return c.withMotion(rest)
}

// withObject reads i or a and the object's letter.
func (c command) withObject(rest []string) (command, int) {
	if len(rest) == 1 {
		return c, more
	}

	c.object = rest

	return c, done(len(rest) == 2 && strings.Contains(objects, rest[1]))
}

func (c command) withMotion(rest []string) (command, int) {
	switch m := motionLen(rest); {
	case m < 0:
		return c, more
	case m == 0 || m != len(rest):
		return c, invalid
	}

	c.motion = rest

	return c, complete
}

func done(ok bool) int {
	if ok {
		return complete
	}

	return invalid
}

// nonCount is keys with the digits that make up counts taken out.
func nonCount(keys []string) []string {
	out := make([]string, 0, len(keys))

	for i := 0; i < len(keys); {
		if isDigit(keys[i]) && keys[i] != "0" {
			_, i = takeCount(keys, i)

			continue
		}

		out = append(out, keys[i])
		i++

		// A character argument is never a count.
		if i < len(keys) && takesChar(out[len(out)-1]) {
			out = append(out, keys[i])
			i++
		}
	}

	return out
}

// normalKey handles a key in normal or visual mode.
func (f *Field) normalKey(tok string) tea.Cmd {
	switch tok {
	case keyEnter:
		f.pend = nil

		return f.submit()
	case keyEsc:
		if len(f.pend) > 0 {
			f.pend = nil

			return nil
		}

		if f.mode != Normal {
			f.mode = Normal
			f.clampNormal()

			return nil
		}

		return f.cancel()
	}

	f.pend = append(f.pend, tok)
	visual := f.mode == Visual || f.mode == VisualLine

	c, st := parse(f.pend, visual)

	switch st {
	case more:
		return nil
	case invalid:
		f.pend = nil

		return nil
	}

	f.pend = nil

	if visual {
		return f.visualCmd(c)
	}

	return f.exec(c)
}

func (c command) n() int { return max(1, c.count) }

// exec runs a parsed normal-mode command.
func (f *Field) exec(c command) tea.Cmd {
	defer func() {
		if f.mode != Insert {
			f.closeGroup()
			f.finish()
			f.clampNormal()
		}
	}()

	switch {
	case c.op != "":
		f.operate(c)
	case c.verb != nil:
		return f.verb(c)
	case c.motion != nil:
		f.move(c)
	}

	return nil
}

// move runs a motion without an operator.
func (f *Field) move(c command) {
	t, ok := f.motion(c.motion, c.n(), c.count > 0, "")
	if !ok {
		return
	}

	f.cur = t.p

	switch {
	case t.toEnd:
		f.wantEnd = true
	case !t.vertical:
		f.setWant()
	}
}

// span is the text an operator acts on: [from, to) a character at a time, or
// the lines from.line to to.line whole.
type span struct {
	from, to pos
	lines    bool
	// at is where a yank leaves the cursor: the start of what it took, at the
	// column the motion gave it.
	at pos
}

func (f *Field) operate(c command) {
	s, ok := f.opSpan(c)
	if !ok {
		return
	}

	f.record(c.count, c.keys)
	f.apply(c.op, s)
}

// opSpan is the text an operator's motion or object covers.
func (f *Field) opSpan(c command) (span, bool) {
	n := c.n()

	switch {
	case c.doubled:
		last := f.cur.line + n - 1
		if n > 1 && f.cur.line == f.buf.last() {
			return span{}, false
		}

		last = min(last, f.buf.last())

		return span{from: pos{f.cur.line, 0}, to: pos{last, 0}, lines: true, at: f.cur}, true
	case c.object != nil:
		return f.object(c.object[0] == "a", c.object[1], n)
	}

	t, ok := f.motion(c.motion, n, c.count > 0, c.op)
	if !ok {
		return span{}, false
	}

	if t.vertical && f.cur.line == t.p.line && c.motion[0] != "G" && c.motion[0] != "g" {
		return span{}, false
	}

	return f.motionSpan(t), true
}

// motionSpan turns a motion's landing into the text an operator covers.
func (f *Field) motionSpan(t target) span {
	if t.kind == linewise {
		from, to := order(f.cur, t.p)

		return span{from: pos{from.line, 0}, to: pos{to.line, 0}, lines: true, at: from}
	}

	return f.charSpan(f.cur, t.p, t.kind)
}

// apply runs an operator over a span.
func (f *Field) apply(op string, s span) {
	if s.lines {
		f.applyLines(op, s)

		return
	}

	text := f.buf.slice(s.from, s.to)

	switch op {
	case "y":
		f.yanked(text, false)
		f.cur = s.at
	case "d":
		if s.from == s.to {
			return
		}

		f.yanked(text, false)
		f.mutate()
		f.buf.remove(s.from, s.to)
		f.cur = s.from
	case "c":
		if text != "" {
			f.yanked(text, false)
		}

		f.mutate()
		f.buf.remove(s.from, s.to)
		f.cur = s.from
		f.startInsert("c", 1)
	}

	f.clampNormal()
	f.setWant()
}

func (f *Field) applyLines(op string, s span) {
	text := strings.Join(f.buf.lines[s.from.line:s.to.line+1], "\n")
	f.yanked(text, true)

	switch op {
	case "y":
		f.cur = pos{s.at.line, s.at.col}
	case "d":
		f.mutate()
		f.buf.deleteLines(s.from.line, s.to.line+1)
		// nostartofline: the column j and k aim for survives the lines going.
		f.cur.line = min(s.from.line, f.buf.last())
		f.cur.col = f.colFor(f.cur.line)
	case "c":
		f.mutate()

		indent := indentOf(f.buf.lines[s.from.line])
		f.buf.deleteLines(s.from.line+1, s.to.line+1)
		f.buf.lines[s.from.line] = indent
		f.cur = pos{s.from.line, len(indent)}
		f.startInsert("c", 1)
		f.insert.ai = indent != ""
		f.setWant()
	}
}

// verb runs a command that is not a motion or an operator.
func (f *Field) verb(c command) tea.Cmd {
	n := c.n()

	switch c.verb[0] {
	case "x", "X", "D", "C", "Y", "s", "S":
		f.short(c)
	case "r":
		f.replace(c, c.verb[1])
	case "~":
		f.record(c.count, c.keys)
		f.toggleCase(n)
	case "J":
		f.record(c.count, c.keys)
		f.join(c)
	case "p", "P":
		f.record(c.count, c.keys)
		f.put(c.verb[0] == "p", n)
	case "u":
		f.undoStep(n)
	case keyCtrlR:
		f.redoStep(n)
	case ".":
		f.repeat(c.count)
	case ":":
		f.cmd = &cmdline{}
	case "v", "V":
		f.anchor = f.cur
		f.mode = Visual

		if c.verb[0] == "V" {
			f.mode = VisualLine
		}
	case "i", "a", "I", "A", "o", "O":
		f.enter(c)
	}

	return nil
}

// short runs the one-key forms: x is dl, X dh, D d$, C c$, Y y$ (nvim's), s cl,
// and S cc.
func (f *Field) short(c command) {
	long := map[string]command{
		"x": {op: "d", motion: []string{"l"}},
		"X": {op: "d", motion: []string{"h"}},
		"D": {op: "d", motion: []string{"$"}},
		"C": {op: "c", motion: []string{"$"}},
		"Y": {op: "y", motion: []string{"$"}},
		"s": {op: "c", motion: []string{"l"}},
		"S": {op: "c", doubled: true},
	}[c.verb[0]]
	long.count = c.count
	long.keys = c.keys

	if c.verb[0] == "s" && f.buf.empty(f.cur.line) {
		f.record(c.count, c.keys)
		f.mutate()
		f.startInsert("c", 1)

		return
	}

	f.operate(long)
}

func (f *Field) replace(c command, char string) {
	if char == keyEsc || utf8.RuneCountInString(char) == 0 || len(char) > 1 && !utf8.ValidString(char) {
		return
	}

	if strings.HasPrefix(char, "ctrl+") || strings.HasPrefix(char, "alt+") || len(char) > 1 && count(char) > 1 {
		return
	}

	l := f.buf.lines[f.cur.line]
	n := c.n()

	end := f.cur.col
	for range n {
		if end >= len(l) {
			return
		}

		end = nextB(l, end)
	}

	f.record(c.count, c.keys)
	f.mutate()
	f.buf.lines[f.cur.line] = l[:f.cur.col] + strings.Repeat(char, n) + l[end:]
	f.cur.col += len(char) * (n - 1)
	f.setWant()
}

func swapCase(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case unicode.IsUpper(r):
			return unicode.ToLower(r)
		case unicode.IsLower(r):
			return unicode.ToUpper(r)
		}

		return r
	}, s)
}

func (f *Field) toggleCase(n int) {
	l := f.buf.lines[f.cur.line]
	if l == "" {
		return
	}

	end := advance(l, f.cur.col, n)

	f.mutate()
	f.buf.lines[f.cur.line] = l[:f.cur.col] + swapCase(l[f.cur.col:end]) + l[end:]
	f.cur.col = min(f.cur.col+len(swapCase(l[f.cur.col:end])), len(f.buf.lines[f.cur.line]))
	f.setWant()
}

// join is J: count lines (at least two) made one, each joined line's
// leading blanks replaced by a space unless the line before ends in one or
// the joined one starts with ).
func (f *Field) join(c command) {
	n := max(c.n(), 2) //nolint:mnd // J joins at least two lines
	if f.cur.line == f.buf.last() {
		return
	}

	n = min(n, f.buf.last()-f.cur.line+1)
	f.joinLines(f.cur.line, n)
}

func (f *Field) joinLines(first, n int) {
	f.mutate()

	b := &f.buf
	line := b.lines[first]
	col := 0

	for i := 1; i < n; i++ {
		next := strings.TrimLeft(b.lines[first+i], " \t")
		space := next != "" && line != "" && !strings.HasPrefix(next, ")") &&
			!strings.HasSuffix(line, " ") && !strings.HasSuffix(line, "\t")

		col = len(line)

		if space {
			line += " "
		}

		line += next
	}

	b.lines[first] = line
	b.deleteLines(first+1, first+n)
	f.cur = pos{first, col}
	f.setWant()
}

// put is p and P.
func (f *Field) put(after bool, n int) {
	text, linewise := f.reg.Text, f.reg.Linewise
	if text == "" && !linewise {
		return
	}

	if f.oneLine {
		text, linewise = strings.ReplaceAll(text, "\n", " "), false
	}

	f.mutate()

	if linewise {
		ls := strings.Split(text, "\n")

		all := make([]string, 0, len(ls)*n)
		for range n {
			all = append(all, ls...)
		}

		at := f.cur.line
		if after {
			at++
		}

		f.buf.insertLines(at, all...)
		f.cur = pos{at, f.buf.firstNonBlank(at)}
		f.setWant()

		return
	}

	text = strings.Repeat(text, n)
	at := f.cur

	if after && f.buf.lines[at.line] != "" {
		at.col = nextB(f.buf.lines[at.line], at.col)
	}

	end := f.buf.insert(at, text)
	if strings.Contains(text, "\n") {
		f.cur = at
	} else {
		f.cur = pos{end.line, prevB(f.buf.lines[end.line], end.col)}
	}

	f.setWant()
}

// enter is i, a, I, A, o, and O.
func (f *Field) enter(c command) {
	b := &f.buf
	l := b.lines[f.cur.line]
	cmd := c.verb[0]

	f.record(c.count, c.keys)

	switch cmd {
	case "a":
		if l != "" {
			f.cur.col = nextB(l, f.cur.col)
		}
	case "A":
		f.cur.col = len(l)
	case "I":
		f.cur.col = len(indentOf(l))
	case "o", "O":
		if f.oneLine {
			f.rec = nil

			return
		}

		f.mutate()
		f.openLine(cmd == "o")
	}

	f.startInsert(cmd, c.n())
}

// openLine puts an empty line below or above the cursor, holding the cursor
// line's indent.
func (f *Field) openLine(below bool) {
	indent := indentOf(f.buf.lines[f.cur.line])

	at := f.cur.line
	if below {
		at++
	}

	f.buf.insertLines(at, indent)
	f.cur = pos{at, len(indent)}
	f.insert.ai = indent != ""
}

// clampNormal keeps the cursor on a character, as normal mode requires.
func (f *Field) clampNormal() {
	f.cur.line = max(0, min(f.cur.line, f.buf.last()))
	if f.mode == Insert {
		f.cur.col = min(f.cur.col, len(f.buf.lines[f.cur.line]))

		return
	}

	if l := f.buf.lines[f.cur.line]; f.cur.col >= len(l) {
		f.cur.col = f.buf.lastChar(f.cur.line)
	}
}

// setWant makes the cursor's display column the one j and k aim for.
func (f *Field) setWant() {
	f.wantEnd = false
	f.want = displayCol(f.buf.lines[f.cur.line], f.cur.col)
}
