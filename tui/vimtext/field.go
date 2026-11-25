package vimtext

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Mode is what the keys typed into a field do.
type Mode int

// The modes. A field opens in Insert, and with the vim layer off it never
// leaves it.
const (
	Insert Mode = iota
	Normal
	Visual
	VisualLine
)

// String is the mode's badge text.
func (m Mode) String() string {
	switch m {
	case Insert:
		return "INSERT"
	case Normal:
		return "NORMAL"
	case Visual:
		return "VISUAL"
	case VisualLine:
		return "V-LINE"
	}

	return "UNKNOWN"
}

// SubmitMsg is a field saved with ⏎ (or :w). Nothing has left the machine: the
// caller decides what saving means.
type SubmitMsg struct {
	ID   string
	Text string
}

// CancelMsg is a field backed out of with esc from normal mode (from insert
// mode with the vim layer off), or :q. Text is what it held, so the caller can
// keep the draft or drop it.
type CancelMsg struct {
	ID   string
	Text string
}

// Register is the unnamed register: what y, d, c, x, and s fill and p and P
// put. A caller that hands one to several fields with [WithRegister] lets a
// yank in one be put in another.
type Register struct {
	Text string
	// Linewise is a register filled by whole lines (yy, dd, V), which p puts
	// on the line below rather than after the cursor.
	Linewise bool
}

// Validator answers why text cannot be saved, or nil when it can. ⏎ on a
// refused field draws the reason in the footer and saves nothing.
type Validator func(text string) error

// Option configures a Field at construction.
type Option func(*Field)

// WithID names the field in the messages it sends, so a screen with more than
// one can tell them apart.
func WithID(id string) Option { return func(f *Field) { f.id = id } }

// WithVim turns the modal layer on or off. It is on by default. Off, the
// field is a plain editor with the same ⏎, ctrl+j, and esc contract, and esc
// cancels.
func WithVim(on bool) Option { return func(f *Field) { f.vim = on } }

// WithOneLine makes a field for one line, such as a bookmark name: ctrl+j and
// alt+⏎ do nothing, j, k, o, and O do nothing, a line break pasted or put
// arrives as a space, and a line wider than the field scrolls sideways.
func WithOneLine() Option { return func(f *Field) { f.oneLine = true } }

// WithHeight bounds the text's rows: the field grows with what it holds from
// minRows to maxRows and scrolls beyond that. The default is 1 to 12.
func WithHeight(minRows, maxRows int) Option {
	return func(f *Field) {
		f.minH = max(1, minRows)
		f.maxH = max(f.minH, maxRows)
	}
}

// WithPlaceholder is what an empty field shows, dimmed.
func WithPlaceholder(s string) Option { return func(f *Field) { f.placeholder = s } }

// WithPrompt is drawn before the first row, such as "r: ", and later rows
// line up under the text rather than under it.
func WithPrompt(s string) Option { return func(f *Field) { f.prompt = s } }

// WithTitle is the line [Field.Render] draws above the text, naming what is
// being written.
func WithTitle(s string) Option { return func(f *Field) { f.title = s } }

// WithValidator refuses a save with a reason, for a value that has rules such
// as a bookmark name.
func WithValidator(v Validator) Option { return func(f *Field) { f.validate = v } }

// WithCompleter lets ctrl+n and ctrl+p complete the word before the cursor in
// insert mode.
func WithCompleter(c Completer) Option { return func(f *Field) { f.complete = c } }

// WithRegister shares the unnamed register with other fields.
func WithRegister(r *Register) Option { return func(f *Field) { f.reg = r } }

// WithClipboard copies whatever fills the register to the system clipboard
// through OSC 52, which a terminal applies only where it allows it.
func WithClipboard() Option { return func(f *Field) { f.clipboard = true } }

// WithEditor lets ctrl+e hand the text to $VISUAL or $EDITOR through [Edit],
// in a temporary file named by pattern (as os.CreateTemp takes it, such as
// "jojo-*.md", so the editor picks the filetype), and take it back. A caller
// that runs the editor its own way leaves this off and handles ctrl+e before
// the field sees it.
func WithEditor(pattern string) Option { return func(f *Field) { f.editor = pattern } }

// WithStyles sets the faces the field draws with. [StylesFor] builds them from
// a skin.
func WithStyles(s Styles) Option { return func(f *Field) { f.setStyles(s) } }

// Field is a text field with a vim layer. It is held by pointer: Update edits
// it in place, so a model copied per message keeps one field rather than
// copies that drift apart.
//
// It opens in insert mode with the cursor at the end of what it holds. ⏎ saves
// in every mode (SubmitMsg), ctrl+j and alt+⏎ insert a line break, and esc
// backs out one level: insert to normal, visual or a pending operator to
// normal, and normal to a CancelMsg. The field never handles ctrl+c, so it
// reaches the caller's quit. A field that has sent a SubmitMsg or CancelMsg takes no
// more keys until [Field.Reset].
type Field struct {
	cmd      *cmdline
	reg      *Register
	rec      *change
	comp     *completion
	look     *look
	validate Validator
	complete Completer

	find findState

	id, prompt, placeholder, title, editor string

	status string

	buf  buffer
	pend []string
	lay  layoutCache

	undo   history
	dot    change
	insert insertState

	cur, anchor pos
	// top is the first row drawn, as a line and a row within it.
	top pos

	// want is the display column j and k aim for, wantEnd that it is the end
	// of whatever line they land on ($).
	want       int
	mode       Mode
	hscrl      int
	minH, maxH int

	wantEnd   bool
	failed    bool
	vim       bool
	oneLine   bool
	clipboard bool
	clip      bool
	done      bool
	replaying bool
}

const (
	defaultMinHeight = 1
	defaultMaxHeight = 12
)

// New returns a field holding value, in insert mode with the cursor at the end.
func New(value string, opts ...Option) *Field {
	f := &Field{vim: true, minH: defaultMinHeight, maxH: defaultMaxHeight}
	f.setStyles(DefaultStyles())

	for _, opt := range opts {
		opt(f)
	}

	if f.reg == nil {
		f.reg = &Register{}
	}

	if f.oneLine {
		f.minH, f.maxH = 1, 1
	}

	f.Reset(value)

	return f
}

// Reset puts value in the field and opens it again: insert mode, the cursor
// at the end, no history, no message. It is how a screen reuses one field.
func (f *Field) Reset(value string) {
	if f.oneLine {
		value = strings.ReplaceAll(value, "\n", " ")
	}

	f.buf = newBuffer(value)
	f.mode = Insert
	f.done = false
	f.undo = history{}
	f.dot = change{}
	f.rec = nil
	f.pend = nil
	f.cmd = nil
	f.comp = nil
	f.insert = insertState{}
	f.status, f.failed = "", false
	f.top, f.hscrl = pos{}, 0
	f.toEnd()
}

// SetValue replaces the text, keeping the mode and the history so u brings
// back what was there, and puts the cursor at the end.
func (f *Field) SetValue(value string) {
	if f.oneLine {
		value = strings.ReplaceAll(value, "\n", " ")
	}

	f.closeGroup()
	f.mutate()
	f.buf = newBuffer(value)
	f.closeGroup()
	f.toEnd()
}

func (f *Field) toEnd() {
	f.cur.line = f.buf.last()
	f.cur.col = len(f.buf.lines[f.cur.line])

	if f.mode != Insert {
		f.clampNormal()
	}

	f.setWant()
}

// Value is the text the field holds.
func (f *Field) Value() string { return f.buf.text() }

// Mode is the mode the field is in.
func (f *Field) Mode() Mode { return f.mode }

// ID is the name WithID gave the field.
func (f *Field) ID() string { return f.id }

// Done reports a field that has sent a SubmitMsg or CancelMsg.
func (f *Field) Done() bool { return f.done }

// Cursor is the line and the byte offset into it where the cursor is, both
// from zero.
func (f *Field) Cursor() (int, int) { return f.cur.line, f.cur.col }

// Status is the message the footer is showing, such as a refusal or a
// completion's place in its list, and whether it reports a failure.
func (f *Field) Status() (string, bool) { return f.status, f.failed }

// Update handles one message: keys, a paste, and the answer to the field's own
// $EDITOR round trip. Anything else is ignored.
func (f *Field) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case EditedMsg:
		return f.edited(msg)
	case tea.PasteMsg:
		if f.done {
			return nil
		}

		return f.paste(msg.Content)
	case tea.KeyPressMsg:
		if f.done {
			return nil
		}

		return f.key(msg)
	}

	return nil
}

// token is a key as the engine reads it: the text it types where it types one,
// and its keystroke name otherwise.
func token(k tea.KeyPressMsg) string {
	s := k.String()
	if s == "space" {
		return " "
	}

	return s
}

func (f *Field) key(k tea.KeyPressMsg) tea.Cmd {
	tok := token(k)

	switch tok {
	case "ctrl+c":
		return nil
	case "ctrl+e":
		if f.editor != "" {
			return Edit(f.id, f.Value(), f.editor)
		}

		return nil
	}

	if f.cmd != nil {
		return f.cmdKey(tok)
	}

	f.status, f.failed = "", false

	if f.comp != nil && tok != keyCtrlN && tok != keyCtrlP {
		f.comp = nil
	}

	var cmd tea.Cmd
	if f.mode == Insert {
		cmd = f.insertKey(tok, insertText(k))
	} else {
		cmd = f.normalKey(tok)
	}

	return tea.Batch(cmd, f.flushClipboard())
}

// insertText is what a key types, or "" for a key that types nothing.
func insertText(k tea.KeyPressMsg) string {
	if k.Mod&^tea.ModShift != 0 {
		return ""
	}

	return k.Text
}

func (f *Field) submit() tea.Cmd {
	text := f.Value()

	if f.validate != nil {
		if err := f.validate(text); err != nil {
			f.status, f.failed = err.Error(), true

			return nil
		}
	}

	f.done = true
	id := f.id

	return func() tea.Msg { return SubmitMsg{ID: id, Text: text} }
}

func (f *Field) cancel() tea.Cmd {
	f.done = true
	id, text := f.id, f.Value()

	return func() tea.Msg { return CancelMsg{ID: id, Text: text} }
}

// say puts a message in the footer.
func (f *Field) say(s string, failed bool) { f.status, f.failed = s, failed }

// yanked fills the register.
func (f *Field) yanked(text string, linewise bool) {
	f.reg.Text, f.reg.Linewise = text, linewise
	f.clip = f.clipboard
}

func (f *Field) flushClipboard() tea.Cmd {
	if !f.clip {
		return nil
	}

	f.clip = false

	return tea.SetClipboard(f.reg.Text)
}
