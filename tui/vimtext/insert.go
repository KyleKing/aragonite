package vimtext

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// insertState is the insert session under way.
type insertState struct {
	// cmd is what opened it (i, a, I, A, o, O, or c for a change) and count
	// how many times esc repeats what was typed.
	cmd   string
	typed []typedKey
	count int
	// ai marks a line holding nothing but the indent autoindent gave it,
	// which leaving the line takes back.
	ai bool
	// moved marks a session the cursor was moved in, after which the dot
	// command no longer repeats what is typed.
	moved bool
	// again marks the keys esc is typing again for a count, which are not
	// typed afresh.
	again bool
}

func (f *Field) startInsert(cmd string, n int) {
	ai := f.insert.ai && (cmd == "o" || cmd == "O" || cmd == "c")
	f.insert = insertState{cmd: cmd, count: n, ai: ai}

	if f.rec != nil {
		f.rec.insert = true
	}

	f.mode = Insert
}

// typing records a key of the session for esc's count and for the dot
// command to repeat.
func (f *Field) typing(tok, text string) {
	if f.insert.moved || f.insert.again {
		return
	}

	f.insert.typed = append(f.insert.typed, typedKey{tok: tok, text: text})

	if f.rec != nil && !f.replaying {
		f.rec.typed = append(f.rec.typed, typedKey{tok: tok, text: text})
	}
}

// insertKey handles a key in insert mode.
func (f *Field) insertKey(tok, text string) tea.Cmd {
	switch tok {
	case keyEnter:
		return f.submit()
	case keyEsc:
		if !f.vim {
			return f.cancel()
		}

		f.leaveInsert()

		return nil
	case keyCtrlJ, "alt+enter", "shift+enter":
		if f.oneLine {
			return nil
		}

		f.typing(keyCtrlJ, "")
		f.newline()

		return nil
	case keyCtrlN, keyCtrlP:
		f.completeStep(tok == keyCtrlN)

		return nil
	case "tab":
		text = "\t"
	}

	if f.editKey(tok) {
		f.typing(tok, "")

		return nil
	}

	if f.moveKey(tok) {
		return nil
	}

	if text != "" {
		f.typing("text", text)
		f.typeText(text)
	}

	return nil
}

// typeText puts text at the cursor.
func (f *Field) typeText(text string) {
	text = clean(text)
	if f.oneLine {
		text = strings.ReplaceAll(text, "\n", " ")
	}

	if text == "" {
		return
	}

	f.mutate()
	f.insert.ai = false
	f.cur = f.buf.insert(f.cur, text)
	f.setWant()
}

func (f *Field) newline() {
	f.mutate()

	l := f.buf.lines[f.cur.line]
	indent := indentOf(l)

	if f.insert.ai && l == indent {
		// A line left holding only the indent autoindent gave it is emptied, as
		// vim does.
		f.buf.lines[f.cur.line] = ""
		f.cur.col = 0
	}

	f.cur = f.buf.insert(f.cur, "\n"+indent)
	f.insert.ai = indent != ""
	f.setWant()
}

// editKey handles the keys that delete in insert mode.
func (f *Field) editKey(tok string) bool {
	b := &f.buf
	l := b.lines[f.cur.line]

	switch tok {
	case keyBackspace, keyCtrlH:
		if f.cur.col == 0 && f.cur.line == 0 {
			return true
		}

		from := f.cur
		b.dec(&from)
		f.cut(from, f.cur)
	case "delete":
		to := f.cur
		if b.inc(&to) == -1 {
			return true
		}

		f.cut(f.cur, to)
	case "ctrl+w":
		f.cut(f.wordBack(), f.cur)
	case "ctrl+u":
		from := pos{f.cur.line, 0}
		if ind := len(indentOf(l)); f.cur.col > ind {
			from.col = ind
		}

		if f.cur.col == 0 && f.cur.line > 0 {
			from = pos{f.cur.line - 1, len(b.lines[f.cur.line-1])}
		}

		f.cut(from, f.cur)
	default:
		return false
	}

	f.insert.ai = false

	return true
}

// cut deletes [from, to) without touching the register, as insert mode's
// deletions do not.
func (f *Field) cut(from, to pos) {
	if from == to {
		return
	}

	f.mutate()
	f.buf.remove(from, to)
	f.cur = from
	f.setWant()
}

// wordBack is where ctrl+w deletes back to: over blanks, then over one run of
// a class, and onto the line before from the start of a line.
func (f *Field) wordBack() pos {
	b := &f.buf
	p := f.cur

	if p.col == 0 {
		if p.line > 0 {
			b.dec(&p)
		}

		return p
	}

	for p.col > 0 {
		q := p
		q.col = prevB(b.lines[p.line], p.col)

		if b.cls(q, false) != clsBlank {
			break
		}

		p = q
	}

	if p.col == 0 {
		return p
	}

	q := p
	q.col = prevB(b.lines[p.line], p.col)
	c := b.cls(q, false)

	for p.col > 0 {
		q := p
		q.col = prevB(b.lines[p.line], p.col)

		if b.cls(q, false) != c {
			break
		}

		p = q
	}

	return p
}

// moveKey handles the keys that move in insert mode. Moving ends the undo
// step and stops what the dot command repeats, as in vim.
func (f *Field) moveKey(tok string) bool {
	b := &f.buf
	p := f.cur

	switch tok {
	case keyLeft:
		if p.col > 0 {
			p.col = prevB(b.lines[p.line], p.col)
		}
	case keyRight:
		if p.col < len(b.lines[p.line]) {
			p.col = nextB(b.lines[p.line], p.col)
		}
	case keyHome:
		p.col = 0
	case keyEnd:
		p.col = len(b.lines[p.line])
	case "ctrl+left", "alt+left", "alt+b":
		f.backWord(&p, 1, false, false)
	case "ctrl+right", "alt+right", "alt+f":
		f.fwdWord(&p, 1, false, false)
	case keyUp, keyDown:
		if t, ok := f.vertical(map[string]int{keyUp: -1, keyDown: 1}[tok]); ok {
			f.moved(t.p)
		}

		return true
	default:
		return false
	}

	f.moved(p)
	f.setWant()

	return true
}

func (f *Field) moved(p pos) {
	if p != f.cur {
		f.closeGroup()
		f.insert.moved = true
		f.insert.ai = false
	}

	f.cur = p
}

// leaveInsert is esc from insert mode: what was typed repeated count times, the
// cursor back onto the character before it, and the session closed as one
// undo step.
func (f *Field) leaveInsert() {
	ins := f.insert
	if ins.count > 1 && !ins.moved {
		was := f.replaying
		f.replaying = true
		f.insert.again = true

		for range ins.count - 1 {
			if ins.cmd == "o" || ins.cmd == "O" {
				f.mutate()
				// vim opens every repeat below, O's included.
				f.openLine(true)
			}

			for _, k := range ins.typed {
				f.insertKey(k.tok, k.text)
			}
		}

		f.replaying = was
	}

	if l := f.buf.lines[f.cur.line]; f.insert.ai && white(l) {
		f.buf.lines[f.cur.line] = ""
		f.cur.col = 0
	}

	f.mode = Normal
	if f.cur.col > 0 {
		f.cur.col = prevB(f.buf.lines[f.cur.line], f.cur.col)
	}

	f.clampNormal()
	f.setWant()
	f.closeGroup()

	if !f.replaying {
		f.finish()
	}

	f.insert = insertState{}
}

// paste is a bracketed paste: typed at the cursor in insert mode, put before
// the cursor as one change in normal mode, and in place of the selection in
// visual mode.
func (f *Field) paste(text string) tea.Cmd {
	f.status, f.failed = "", false

	switch f.mode {
	case Insert:
		f.typing("paste", text)
		f.typeText(text)
	case Normal:
		f.closeGroup()
		f.typeText(text)
		f.closeGroup()
		f.clampNormal()
	case Visual, VisualLine:
		s := f.visualSpan()
		f.mutate()
		f.removeSpan(s)
		f.typeText(text)
		f.closeGroup()
		f.mode = Normal
		f.clampNormal()
	}

	return nil
}

// cmdline is the : line, which knows :w, :q, :wq, and :x.
type cmdline struct {
	text string
}

func (f *Field) cmdKey(tok string) tea.Cmd {
	switch tok {
	case keyEsc:
		f.cmd = nil
	case keyBackspace, keyCtrlH:
		if f.cmd.text == "" {
			f.cmd = nil

			return nil
		}

		f.cmd.text = f.cmd.text[:prevB(f.cmd.text, len(f.cmd.text))]
	case keyEnter:
		text := strings.TrimSpace(f.cmd.text)
		f.cmd = nil

		switch text {
		case "w", "wq", "x", "w!", "wq!", "x!":
			return f.submit()
		case "q", "q!":
			return f.cancel()
		case "":
			return nil
		}

		f.say("not a field command: "+text+" (:w saves, :q cancels)", true)
	default:
		if len([]rune(tok)) == 1 || tok == " " {
			f.cmd.text += tok
		}
	}

	return nil
}

func (f *Field) edited(msg EditedMsg) tea.Cmd {
	if msg.ID != f.id {
		return nil
	}

	if msg.Err != nil {
		f.say(msg.Err.Error()+"; the field keeps what it held", true)

		return nil
	}

	f.SetValue(strings.TrimSuffix(msg.Text, "\n"))

	return nil
}
