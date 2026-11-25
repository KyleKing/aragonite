package vimtext_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/aragonite/tui/vimtext"
)

// only finds the one message of type T among msgs, failing otherwise.
func only[T any](t *testing.T, msgs []tea.Msg, out *T) {
	t.Helper()

	n := 0

	for _, m := range msgs {
		if v, ok := m.(T); ok {
			*out = v
			n++
		}
	}

	if n != 1 {
		t.Fatalf("want one %T among %#v", *out, msgs)
	}
}

func none[T any](t *testing.T, msgs []tea.Msg) {
	t.Helper()

	for _, m := range msgs {
		if _, ok := m.(T); ok {
			t.Fatalf("want no %T, got %#v", m, msgs)
		}
	}
}

var (
	errNoSpaces     = errors.New("a bookmark name has no spaces")
	errEditorFailed = errors.New("exit status 1")
)

// TestOpen_InsertAtEnd pins P15: a field opens in insert mode with the cursor
// after what it holds.
func TestOpen_InsertAtEnd(t *testing.T) {
	t.Parallel()

	f := vimtext.New("one\ntwo")
	line, col := f.Cursor()

	if f.Mode() != vimtext.Insert || line != 1 || col != 3 {
		t.Fatalf("opened in %s at %d:%d", f.Mode(), line, col)
	}

	press(t, f, "!")

	if f.Value() != "one\ntwo!" {
		t.Fatalf("typed into %q", f.Value())
	}
}

// TestEnter_SavesInEveryMode pins P15: ⏎ saves from insert, normal, and visual
// mode alike, and a saved field takes no more keys.
func TestEnter_SavesInEveryMode(t *testing.T) {
	t.Parallel()

	for _, seq := range []string{"<CR>", "<Esc><CR>", "<Esc>v<CR>", "<Esc>V<CR>", "<Esc>d<CR>"} {
		t.Run(seq, func(t *testing.T) {
			t.Parallel()

			f := vimtext.New("hi", vimtext.WithID("desc"))
			var msg vimtext.SubmitMsg
			only(t, press(t, f, seq), &msg)

			if msg.ID != "desc" || msg.Text != "hi" || !f.Done() {
				t.Fatalf("saved %+v, done %v", msg, f.Done())
			}

			press(t, f, "x<CR>")

			if f.Value() != "hi" {
				t.Fatalf("a saved field took keys: %q", f.Value())
			}
		})
	}
}

// TestEsc_BacksOutOneLevel pins P14 inside a field: esc leaves insert, then a
// pending operator or a selection, and only from bare normal mode cancels,
// carrying the text so the caller can keep the draft.
func TestEsc_BacksOutOneLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		seq    string
		cancel bool
		mode   vimtext.Mode
	}{
		{"<Esc>", false, vimtext.Normal},
		{"<Esc><Esc>", true, vimtext.Normal},
		{"<Esc>d<Esc>", false, vimtext.Normal},
		{"<Esc>2<Esc>", false, vimtext.Normal},
		{"<Esc>v<Esc>", false, vimtext.Normal},
		{"<Esc>v<Esc><Esc>", true, vimtext.Normal},
		{"<Esc>:<Esc>", false, vimtext.Normal},
	}

	for _, c := range cases {
		t.Run(c.seq, func(t *testing.T) {
			t.Parallel()

			f := vimtext.New("draft", vimtext.WithID("c"))
			msgs := press(t, f, c.seq)

			if !c.cancel {
				none[vimtext.CancelMsg](t, msgs)

				if f.Mode() != c.mode || f.Done() {
					t.Fatalf("in %s, done %v", f.Mode(), f.Done())
				}

				return
			}

			var msg vimtext.CancelMsg
			only(t, msgs, &msg)

			if msg.Text != "draft" || msg.ID != "c" {
				t.Fatalf("canceled with %+v", msg)
			}
		})
	}
}

// TestNewline_CtrlJAndAltEnter pins P15's line break, and that a one-line field
// has none.
func TestNewline_CtrlJAndAltEnter(t *testing.T) {
	t.Parallel()

	f := vimtext.New("ab")
	press(t, f, "<C-j>c<M-CR>d")

	if f.Value() != "ab\nc\nd" {
		t.Fatalf("got %q", f.Value())
	}

	one := vimtext.New("ab", vimtext.WithOneLine())
	press(t, one, "<C-j><M-CR>c<Esc>jkoO")

	if one.Value() != "abc" || one.Mode() != vimtext.Normal {
		t.Fatalf("one-line field holds %q in %s", one.Value(), one.Mode())
	}
}

// TestVimOff_PlainEditing is the field with the modal layer off: esc cancels
// from insert mode and every letter types.
func TestVimOff_PlainEditing(t *testing.T) {
	t.Parallel()

	f := vimtext.New("", vimtext.WithVim(false))
	msgs := press(t, f, "dd<C-j>ix<BS><Left><C-w>q<Esc>")

	var msg vimtext.CancelMsg
	only(t, msgs, &msg)

	if msg.Text != "ddqi" {
		t.Fatalf("canceled with %q", msg.Text)
	}

	if f.Badge() != "" {
		t.Fatalf("a plain field draws a badge: %q", f.Badge())
	}
}

// TestValidator_RefusesWithReason draws the refusal and saves nothing, and the
// next key clears it.
func TestValidator_RefusesWithReason(t *testing.T) {
	t.Parallel()

	f := vimtext.New("a b", vimtext.WithValidator(func(s string) error {
		if strings.Contains(s, " ") {
			return errNoSpaces
		}

		return nil
	}))

	none[vimtext.SubmitMsg](t, press(t, f, "<CR>"))

	if msg, failed := f.Status(); !failed || msg != "a bookmark name has no spaces" {
		t.Fatalf("status %q, failed %v", msg, failed)
	}

	if !strings.Contains(f.Footer(80), "a bookmark name has no spaces") {
		t.Fatalf("footer does not say why: %q", f.Footer(80))
	}

	press(t, f, "<Esc>hx")

	if msg, _ := f.Status(); msg != "" {
		t.Fatalf("status kept: %q", msg)
	}

	var msg vimtext.SubmitMsg
	only(t, press(t, f, "<CR>"), &msg)

	if msg.Text != "ab" {
		t.Fatalf("saved %q", msg.Text)
	}
}

// TestExLine is :w, :wq, :x, :q, and a command the field does not know.
func TestExLine(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		seq            string
		submit, cancel bool
	}{
		{"<Esc>:w<CR>", true, false},
		{"<Esc>:wq<CR>", true, false},
		{"<Esc>:x<CR>", true, false},
		{"<Esc>:q<CR>", false, true},
		{"<Esc>:q!<CR>", false, true},
		{"<Esc>:s/a/b<CR>", false, false},
		{"<Esc>:w<BS><BS><BS>x", false, false},
	} {
		t.Run(c.seq, func(t *testing.T) {
			t.Parallel()

			f := vimtext.New("text")
			msgs := press(t, f, c.seq)

			if c.submit {
				only(t, msgs, new(vimtext.SubmitMsg))
			} else {
				none[vimtext.SubmitMsg](t, msgs)
			}

			if c.cancel {
				only(t, msgs, new(vimtext.CancelMsg))
			} else {
				none[vimtext.CancelMsg](t, msgs)
			}
		})
	}
}

// TestCompletion cycles ctrl+n through the candidates and back to what was
// typed, ctrl+p the other way, as vim's ctrl+n does.
func TestCompletion(t *testing.T) {
	t.Parallel()

	logins := []string{"@alice", "@alex"}
	complete := func(line string, col int) vimtext.Completion {
		start := strings.LastIndexAny(line[:col], " ") + 1
		stem := line[start:col]

		var items []string

		for _, l := range logins {
			if strings.HasPrefix(l, stem) && stem != "" {
				items = append(items, l)
			}
		}

		return vimtext.Completion{Start: start, Items: items, Note: "nobody starts with " + stem}
	}

	steps := []struct{ keys, want, status string }{
		{"@al<C-n>", "cc @alice", "@alice  1 of 2"},
		{"<C-n>", "cc @alex", "@alex  2 of 2"},
		{"<C-n>", "cc @al", "back to what was typed · ctrl+n again for @alice"},
		{"<C-p>", "cc @alex", "@alex  2 of 2"},
		{" z<C-n>", "cc @alex z", "nobody starts with z"},
	}

	f := vimtext.New("cc ", vimtext.WithCompleter(complete))

	for _, s := range steps {
		press(t, f, s.keys)

		if msg, _ := f.Status(); f.Value() != s.want || msg != s.status {
			t.Fatalf("after %q: %q, status %q", s.keys, f.Value(), msg)
		}
	}

	offered := false
	for _, h := range f.Hints() {
		offered = offered || h.Key == "ctrl+n" && h.What == "complete"
	}

	if !offered {
		t.Fatalf("the hints do not offer completion: %+v", f.Hints())
	}
}

// TestPaste is a bracketed paste: typed in insert mode with each carriage
// return a line break (a terminal pastes one for each), one undo step in
// normal mode, the selection's replacement in visual mode, and a space for
// each line break in a one-line field.
func TestPaste(t *testing.T) {
	t.Parallel()

	f := vimtext.New("ab")
	f.Update(tea.PasteMsg{Content: "x\r\ny\rz\x07"})

	if f.Value() != "abx\ny\nz" {
		t.Fatalf("insert paste: %q", f.Value())
	}

	press(t, f, "<Esc>gg0")
	f.Update(tea.PasteMsg{Content: "12"})

	if f.Value() != "12abx\ny\nz" {
		t.Fatalf("normal paste: %q", f.Value())
	}

	press(t, f, "u")

	if f.Value() != "abx\ny\nz" {
		t.Fatalf("undoing a paste: %q", f.Value())
	}

	press(t, f, "vl")
	f.Update(tea.PasteMsg{Content: "Z"})

	if f.Value() != "Zx\ny\nz" || f.Mode() != vimtext.Normal {
		t.Fatalf("visual paste: %q in %s", f.Value(), f.Mode())
	}

	one := vimtext.New("", vimtext.WithOneLine())
	one.Update(tea.PasteMsg{Content: "a\nb"})

	if one.Value() != "a b" {
		t.Fatalf("one-line paste: %q", one.Value())
	}
}

// TestRegister_SharedAndClipboard puts a yank from one field in another, and
// copies it to the clipboard only for a field that asks.
func TestRegister_SharedAndClipboard(t *testing.T) {
	t.Parallel()

	reg := &vimtext.Register{}
	a := vimtext.New("hello world", vimtext.WithRegister(reg), vimtext.WithClipboard())
	b := vimtext.New("", vimtext.WithRegister(reg))

	if msgs := press(t, a, "<Esc>0yiw"); len(msgs) != 1 {
		t.Fatalf("a clipboard field's yank answers %#v", msgs)
	}

	if reg.Text != "hello" || reg.Linewise {
		t.Fatalf("register %+v", reg)
	}

	if msgs := press(t, b, "<Esc>pyy"); len(msgs) != 0 {
		t.Fatalf("a field without the clipboard answers %#v", msgs)
	}

	if b.Value() != "hello" || !reg.Linewise {
		t.Fatalf("b holds %q, register %+v", b.Value(), reg)
	}
}

// TestCtrlC_ReachesTheCaller: the field never handles ctrl+c.
func TestCtrlC_ReachesTheCaller(t *testing.T) {
	t.Parallel()

	f := vimtext.New("x")
	if cmd := f.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd != nil || f.Value() != "x" || f.Done() {
		t.Fatal("the field took ctrl+c")
	}
}

// TestSetValue_KeepsHistory: u brings back what a SetValue replaced, and Reset
// starts over.
func TestSetValue_KeepsHistory(t *testing.T) {
	t.Parallel()

	f := vimtext.New("old")
	f.SetValue("new")
	press(t, f, "<Esc>u")

	if f.Value() != "old" {
		t.Fatalf("u after SetValue: %q", f.Value())
	}

	press(t, f, "<CR>")
	f.Reset("again")
	press(t, f, "<Esc>u")

	if f.Value() != "again" || f.Done() {
		t.Fatalf("after Reset: %q, done %v", f.Value(), f.Done())
	}
}

// TestEditor_RoundTrip: ctrl+e asks for $EDITOR only where WithEditor is
// given, the answer replaces the text, and a failure keeps the draft.
func TestEditor_RoundTrip(t *testing.T) {
	t.Parallel()

	if vimtext.New("x").Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}) != nil {
		t.Fatal("ctrl+e without WithEditor did something")
	}

	f := vimtext.New("draft", vimtext.WithID("d"), vimtext.WithEditor("test-*.md"))
	if f.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}) == nil {
		t.Fatal("ctrl+e asked for nothing")
	}

	f.Update(vimtext.EditedMsg{ID: "other", Text: "not mine"})
	f.Update(vimtext.EditedMsg{ID: "d", Text: "draft", Err: errEditorFailed})

	if msg, failed := f.Status(); f.Value() != "draft" || !failed || !strings.Contains(msg, "exit status 1") {
		t.Fatalf("after a failed edit: %q, %q", f.Value(), msg)
	}

	f.Update(vimtext.EditedMsg{ID: "d", Text: "from the editor\n"})

	if f.Value() != "from the editor" {
		t.Fatalf("after the edit: %q", f.Value())
	}
}

// TestHints_SayTheContract pins the footer's words in each mode (P15).
func TestHints_SayTheContract(t *testing.T) {
	t.Parallel()

	f := vimtext.New("x")
	if got := f.Footer(80); !strings.Contains(stripped(got), "INSERT  ⏎ save · ctrl+j new line · esc normal") {
		t.Fatalf("insert footer: %q", stripped(got))
	}

	press(t, f, "<Esc>")

	if got := f.Footer(80); !strings.Contains(stripped(got), "NORMAL  ⏎ save · i insert · esc cancel") {
		t.Fatalf("normal footer: %q", stripped(got))
	}

	press(t, f, "2d")

	if got := f.Footer(80); !strings.Contains(stripped(got), "NORMAL  2d ") {
		t.Fatalf("pending footer: %q", stripped(got))
	}

	plain := vimtext.New("x", vimtext.WithVim(false))
	if got := stripped(plain.Footer(80)); !strings.HasPrefix(got, "⏎ save · ctrl+j new line · esc cancel") {
		t.Fatalf("plain footer: %q", got)
	}
}
