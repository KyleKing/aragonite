package vimtext

import (
	"strconv"
	"strings"
)

// Completer answers ctrl+n and ctrl+p in insert mode with candidates for the
// word before the cursor, given the cursor's line and the cursor's byte offset
// in it.
type Completer func(line string, col int) Completion

// Completion is a Completer's answer: the field replaces line[Start:col] with
// each of Items in turn, and after the last puts back what was typed, as vim's
// ctrl+n does.
type Completion struct {
	// Note is said when Items is empty, such as why nothing matched.
	Note  string
	Items []string
	Start int
}

// completion is a ctrl+n cycle under way.
type completion struct {
	stem  string
	items []string
	line  int
	start int
	idx   int
}

func (f *Field) completeStep(forward bool) {
	if f.complete == nil {
		return
	}

	if f.comp == nil {
		l := f.buf.lines[f.cur.line]
		c := f.complete(l, f.cur.col)

		if c.Start < 0 || c.Start > f.cur.col {
			return
		}

		if len(c.Items) == 0 {
			note := c.Note
			if note == "" {
				note = "nothing to complete here"
			}

			f.say(note, false)

			return
		}

		f.comp = &completion{line: f.cur.line, start: c.Start, stem: l[c.Start:f.cur.col], items: c.Items, idx: -1}
	}

	c := f.comp
	n := len(c.items)

	if forward {
		c.idx++
		if c.idx == n {
			c.idx = -1
		}
	} else {
		c.idx--
		if c.idx < -1 {
			c.idx = n - 1
		}
	}

	text := c.stem
	if c.idx >= 0 {
		text = c.items[c.idx]
	}

	f.mutate()
	f.buf.remove(pos{c.line, c.start}, f.cur)
	f.cur = f.buf.insert(pos{c.line, c.start}, strings.ReplaceAll(text, "\n", " "))
	f.setWant()

	if c.idx < 0 {
		f.say("back to what was typed · ctrl+n again for "+c.items[0], false)

		return
	}

	f.say(text+"  "+strconv.Itoa(c.idx+1)+" of "+strconv.Itoa(n), false)
}
