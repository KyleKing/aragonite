package vimtext_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/aragonite/tui/vimtext"
)

// named are the keys a sequence spells in vim's angle-bracket notation.
var named = map[string]tea.KeyPressMsg{
	"Esc":     {Code: tea.KeyEscape},
	"CR":      {Code: tea.KeyEnter},
	"BS":      {Code: tea.KeyBackspace},
	"Del":     {Code: tea.KeyDelete},
	"Tab":     {Code: tea.KeyTab},
	"Space":   {Code: tea.KeySpace, Text: " "},
	"Left":    {Code: tea.KeyLeft},
	"Right":   {Code: tea.KeyRight},
	"Up":      {Code: tea.KeyUp},
	"Down":    {Code: tea.KeyDown},
	"Home":    {Code: tea.KeyHome},
	"End":     {Code: tea.KeyEnd},
	"M-CR":    {Code: tea.KeyEnter, Mod: tea.ModAlt},
	"C-Left":  {Code: tea.KeyLeft, Mod: tea.ModCtrl},
	"C-Right": {Code: tea.KeyRight, Mod: tea.ModCtrl},
}

// keys spells a key sequence in vim's notation: plain characters, and <Esc>,
// <CR>, <C-r>, and the other names above in angle brackets.
func keys(tb testing.TB, seq string) []tea.KeyPressMsg {
	tb.Helper()

	out := make([]tea.KeyPressMsg, 0, len(seq))

	for seq != "" {
		if k, rest, ok := angled(tb, seq); ok {
			out = append(out, k)
			seq = rest

			continue
		}

		r, size := utf8.DecodeRuneInString(seq)
		seq = seq[size:]

		if r == ' ' {
			out = append(out, named["Space"])

			continue
		}

		out = append(out, tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	return out
}

// angled reads a <Name> key at the start of seq.
func angled(tb testing.TB, seq string) (tea.KeyPressMsg, string, bool) {
	tb.Helper()

	end := strings.Index(seq, ">")
	if !strings.HasPrefix(seq, "<") || end < 2 {
		return tea.KeyPressMsg{}, seq, false
	}

	name := seq[1:end]

	if k, ok := named[name]; ok {
		return k, seq[end+1:], true
	}

	if c, ok := strings.CutPrefix(name, "C-"); ok && len(c) == 1 {
		return tea.KeyPressMsg{Code: rune(c[0]), Mod: tea.ModCtrl}, seq[end+1:], true
	}

	tb.Fatalf("unknown key <%s>", name)

	return tea.KeyPressMsg{}, seq, false
}

// press sends a key sequence to f, running every command it answers so the
// messages a key sends come back.
func press(tb testing.TB, f *vimtext.Field, seq string) []tea.Msg {
	tb.Helper()

	msgs := make([]tea.Msg, 0, len(seq))

	for _, k := range keys(tb, seq) {
		msgs = append(msgs, run(f.Update(k))...)
	}

	return msgs
}

// run runs a command and every command a batch holds, answering the messages.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	msg := cmd()

	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, run(c)...)
		}

		return out
	}

	if msg == nil {
		return nil
	}

	return []tea.Msg{msg}
}

// marked splits text holding a | at the cursor into the text and the cursor.
func marked(tb testing.TB, text string) (string, int, int) {
	tb.Helper()

	at := strings.Index(text, "|")
	if at < 0 {
		tb.Fatalf("no | marks the cursor in %q", text)
	}

	before := text[:at]
	line := strings.Count(before, "\n")
	col := len(before) - strings.LastIndex(before, "\n") - 1

	return text[:at] + text[at+1:], line, col
}

// mark puts a | in text at the cursor.
func mark(text string, line, col int) string {
	lines := strings.Split(text, "\n")
	if line < len(lines) && col <= len(lines[line]) {
		lines[line] = lines[line][:col] + "|" + lines[line][col:]
	}

	return strings.Join(lines, "\n")
}
