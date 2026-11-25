package vimtext

// maxHistory bounds the undo history, so a long session does not hold every
// state it passed through.
const maxHistory = 1000

// state is the text and where the cursor was when a change began.
type state struct {
	lines []string
	at    pos
}

// history is u and ctrl+r. A change records the text as it was before it, once,
// when it first edits; everything until the group closes is one step, which is
// how an insert session undoes as a whole.
type history struct {
	undo, redo []state
	open       bool
}

// mutate is called before every edit: the first one in a group records the
// state to come back to.
func (f *Field) mutate() {
	if f.undo.open {
		return
	}

	f.undo.open = true
	f.undo.undo = append(f.undo.undo, state{lines: f.buf.clone(), at: f.cur})
	f.undo.redo = nil

	if len(f.undo.undo) > maxHistory {
		f.undo.undo = f.undo.undo[1:]
	}
}

// closeGroup ends the step a change is recorded under.
func (f *Field) closeGroup() { f.undo.open = false }

func (f *Field) undoStep(n int) {
	f.travel(&f.undo.undo, &f.undo.redo, n, "already at the oldest change")
}

func (f *Field) redoStep(n int) {
	f.travel(&f.undo.redo, &f.undo.undo, n, "already at the newest change")
}

// travel moves n states from one stack to the other: the text goes back to the
// state popped, and what it held is pushed for the way back, both with the
// cursor where the change began.
func (f *Field) travel(from, to *[]state, n int, edge string) {
	for range n {
		if len(*from) == 0 {
			f.say(edge, false)

			break
		}

		st := (*from)[len(*from)-1]
		*from = (*from)[:len(*from)-1]
		*to = append(*to, state{lines: f.buf.lines, at: st.at})
		f.buf.lines = append([]string(nil), st.lines...)
		f.cur = st.at
	}

	f.afterHistory()
}

func (f *Field) afterHistory() {
	f.cur.line = min(f.cur.line, f.buf.last())
	f.clampNormal()
	f.setWant()
}

// change is what the dot command repeats: the command's keys with its count
// apart, so a new count can replace it, and the keys typed in the insert
// session it opened.
type change struct {
	vis    *visRepeat
	keys   []string
	typed  []typedKey
	count  int
	insert bool
}

// typedKey is one key of an insert session: its token, and the text it typed
// or the text a paste carried.
type typedKey struct {
	tok, text string
}

// record starts recording a change, unless the dot command is replaying one.
func (f *Field) record(count int, keys []string) {
	if f.replaying {
		return
	}

	f.rec = &change{count: count, keys: append([]string(nil), keys...)}
}

// finish makes the change being recorded the one the dot command repeats.
func (f *Field) finish() {
	if f.rec != nil {
		f.dot = *f.rec
		f.rec = nil
	}
}

// repeat is the dot command, with count replacing the change's own where one
// is given.
func (f *Field) repeat(count int) {
	if f.dot.keys == nil && f.dot.vis == nil {
		return
	}

	if count > 0 {
		f.dot.count = count
	}

	d := f.dot
	f.replaying = true

	defer func() { f.replaying = false }()

	if d.vis != nil {
		f.repeatVisual(d.vis)
		f.retype(d)

		return
	}

	if d.count > 0 {
		for _, r := range itoa(d.count) {
			f.normalKey(string(r))
		}
	}

	for _, k := range d.keys {
		f.normalKey(k)
	}

	f.retype(d)
}

// retype types again what a repeated change typed in insert mode.
func (f *Field) retype(d change) {
	if f.mode != Insert {
		f.closeGroup()
		f.clampNormal()

		return
	}

	for _, k := range d.typed {
		f.insertKey(k.tok, k.text)
	}

	f.insertKey(keyEsc, "")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var b [20]byte

	i := len(b)
	for ; n > 0; n /= 10 {
		i--
		b[i] = byte('0' + n%10)
	}

	return string(b[i:])
}
