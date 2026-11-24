package palette

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/aragonite/tui/skin"
)

// Mode is what a key press does while the palette is open.
type Mode int

const (
	// Run is the `:` palette: typing searches, Enter runs.
	Run Mode = iota
	// Browse is the `?` legend: a key press explains that key, `/` filters, Enter runs.
	Browse
)

// Layout is where the palette draws.
type Layout int

const (
	// Auto is Drawer from DrawerFrom columns and Strip below.
	Auto Layout = iota
	// Drawer is a box down the right edge with an explainer pane under the list.
	Drawer
	// Strip is a band over the bottom rows, the query on the last.
	Strip
)

// DrawerFrom is the narrowest frame Auto draws a drawer in.
const DrawerFrom = 100

// RunMsg asks the tool to run an action, as if its key had been pressed.
type RunMsg struct {
	// Action is the entry's action, which for an entry built from a legend is its key sequence.
	Action string
	// Key is the entry's key label, which a status line can teach with: "ran squash · next time s".
	Key string
	// Flags are words starting with "-" typed after the search, "squash -u", for a verb that takes
	// flags to open with them set.
	Flags []string
	// From is the mode the run came from.
	From Mode
}

// RefusedMsg says Enter was pressed on an entry that is off here, so a tool can log the refusal.
// The palette shows the reason itself.
type RefusedMsg struct{ Action, Why string }

// ClosedMsg was sent when the palette closed without running anything.
//
// Deprecated: Update no longer sends it, since a message arriving after the close could reorder
// with a key the tool sends next. Ask IsOpen after Update instead.
type ClosedMsg struct{}

// Model is the palette's state. The zero value is closed; build one with New.
type Model struct {
	skin     skin.Skin
	now      func() time.Time
	entries  []Entry
	sections []Section
	hits     []Hit
	query    string
	status   string
	pending  []string
	cursor   int
	mode     Mode
	layout   Layout
	open     bool
	typing   bool
	noted    bool
}

// Option configures New.
type Option func(*Model)

// WithLayout fixes the layout rather than choosing by width.
func WithLayout(l Layout) Option { return func(m *Model) { m.layout = l } }

// WithClock replaces time.Now, which ages recent entries.
func WithClock(now func() time.Time) Option { return func(m *Model) { m.now = now } }

// New is a closed palette drawn with s.
func New(s skin.Skin, opts ...Option) *Model {
	m := &Model{skin: s, now: time.Now}
	for _, o := range opts {
		o(m)
	}

	return m
}

// Open shows entries in mode, with an empty query and the cursor on the first.
func (m *Model) Open(mode Mode, entries []Entry) {
	m.entries, m.mode, m.open = entries, mode, true
	m.query, m.status, m.pending, m.cursor, m.noted = "", "", nil, 0, false
	m.typing = mode == Run
	m.rank()
}

// Refresh replaces the entries while open, keeping the query and the cursor's entry where it can,
// for a screen whose state changed under the palette.
func (m *Model) Refresh(entries []Entry) {
	sel, ok := m.Selected()
	m.entries = entries
	m.rank()

	if ok {
		m.focus(sel.Action)
	}
}

// focus moves the cursor to the hit for action, leaving it where it is when none is listed.
func (m *Model) focus(action string) {
	for i := range m.hits {
		if m.hits[i].Action == action {
			m.cursor = i

			return
		}
	}
}

// Close hides the palette.
func (m *Model) Close() {
	m.open, m.typing, m.pending, m.noted = false, false, nil, false
}

// IsOpen reports whether the palette is showing.
func (m *Model) IsOpen() bool { return m.open }

// Mode is the mode it was opened in.
func (m *Model) Mode() Mode { return m.mode }

// Typing reports whether keys go into the query, which a tool's usage log must not record.
func (m *Model) Typing() bool { return m.open && m.typing }

// Query is what has been typed.
func (m *Model) Query() string { return m.query }

// Selected is the entry under the cursor, false when the list is empty.
func (m *Model) Selected() (Entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.hits) {
		return Entry{}, false
	}

	return m.hits[m.cursor].Entry, true
}

// Explained is the entry a key press in browse mode explained, false until one has.
func (m *Model) Explained() (Entry, bool) {
	if !m.noted {
		return Entry{}, false
	}

	return m.Selected()
}

// Hits is the list as drawn, in order.
func (m *Model) Hits() []Hit { return slices.Clone(m.hits) }

// Sections is the list as drawn, by section.
func (m *Model) Sections() []Section { return slices.Clone(m.sections) }

func (m *Model) rank() {
	switch {
	case len(m.pending) > 0:
		m.sections = []Section{{Name: strings.Join(m.pending, " ") + " then", Hits: hits(under(m.entries, m.pending))}}
	case m.mode == Browse && strings.TrimSpace(m.query) == "":
		m.sections = Groups(m.entries)
	default:
		m.sections = Rank(m.entries, m.query, m.now())
		if name, _ := splitFlags(m.query); name != m.query && len(m.sections[0].Hits) == 0 {
			m.sections = Rank(m.entries, name, m.now())
		}
	}

	m.hits = m.hits[:0:0]
	for i := range m.sections {
		m.hits = append(m.hits, m.sections[i].Hits...)
	}

	m.cursor = max(0, min(m.cursor, len(m.hits)-1))
}

func hits(entries []Entry) []Hit {
	out := make([]Hit, len(entries))
	for i := range entries {
		out[i] = Hit{Entry: entries[i]}
	}

	return out
}

// splitFlags parts a query into the words that search and the "-x" words after them.
func splitFlags(query string) (string, []string) {
	words := strings.Fields(query)

	for i, w := range words {
		if i > 0 && strings.HasPrefix(w, "-") {
			return strings.Join(words[:i], " "), words[i:]
		}
	}

	return query, nil
}

// Update handles a key press, a paste, or a wheel notch while open, and ignores everything else.
// The command it answers carries a RunMsg or a RefusedMsg, and is nil otherwise; a tool that cares
// whether the palette closed asks IsOpen afterwards, so closing sends nothing that could reorder
// with a key the tool sends after it.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	if !m.open {
		return nil
	}

	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.cursor -= wheelRows
		case tea.MouseWheelDown:
			m.cursor += wheelRows
		default:
			return nil
		}

		m.cursor = max(0, min(m.cursor, len(m.hits)-1))
		m.noted = false
	case tea.PasteMsg:
		if m.typing {
			m.query += strings.ReplaceAll(msg.Content, "\n", " ")
			m.cursor, m.noted = 0, false
			m.rank()
		}
	case tea.KeyPressMsg:
		return m.key(msg)
	}

	return nil
}

// The keys the palette reads by name.
const (
	keyEnter    = "enter"
	keyEsc      = "esc"
	keyTab      = "tab"
	keyEnd      = "end"
	keyHalfUp   = "ctrl+u"
	keyHalfDown = "ctrl+d"
)

func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	m.status = ""

	switch {
	case k == "ctrl+c":
		return m.closing()
	case m.move(k):
		return nil
	case k == keyEnter:
		return m.run()
	case m.typing:
		return m.edit(msg)
	}

	return m.browse(msg)
}

func (m *Model) closing() tea.Cmd {
	m.Close()

	return nil
}

// How far a page key and a wheel notch move the cursor.
const (
	pageRows  = 8
	wheelRows = 3
)

// move handles the keys that move the cursor, which never type: the arrows and ctrl+n/p
// everywhere, the page keys, and home and end outside the query.
func (m *Model) move(k string) bool {
	switch k {
	case "up", "ctrl+p", "shift+tab":
		m.cursor--
	case "down", "ctrl+n":
		m.cursor++
	case "pgup":
		m.cursor -= pageRows
	case "pgdown":
		m.cursor += pageRows
	case keyHalfUp, keyHalfDown:
		// Half-page keys page the legend as they page the screen behind it, and clear a typed query
		// in the editing keys below.
		if m.typing {
			return false
		}

		if k == keyHalfUp {
			m.cursor -= pageRows
		} else {
			m.cursor += pageRows
		}
	case "home", keyEnd:
		if m.typing {
			return false
		}

		m.cursor = 0
		if k == keyEnd {
			m.cursor = len(m.hits) - 1
		}
	default:
		return false
	}

	m.cursor = max(0, min(m.cursor, len(m.hits)-1))
	m.noted = false

	return true
}

func (m *Model) run() tea.Cmd {
	sel, ok := m.Selected()
	if !ok {
		m.status = "nothing matches"

		return nil
	}

	if sel.Off {
		m.status = sel.Key + " is off here: " + sel.Why
		refused := RefusedMsg{Action: sel.Action, Why: sel.Why}

		return func() tea.Msg { return refused }
	}

	_, flags := splitFlags(m.query)
	run := RunMsg{Action: sel.Action, Key: sel.Key, Flags: flags, From: m.mode}
	m.Close()

	return func() tea.Msg { return run }
}

// edit handles a key while the query takes typing: in run mode always, in browse mode after `/`.
func (m *Model) edit(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case keyEsc:
		if m.mode != Browse {
			return m.closing()
		}

		m.typing, m.query = false, ""
	case "backspace":
		if m.query == "" && m.mode == Browse {
			m.typing = false

			return nil
		}

		_, size := utf8.DecodeLastRuneInString(m.query)
		m.query = m.query[:len(m.query)-size]
	case keyHalfUp:
		m.query = ""
	case "ctrl+w":
		m.query = strings.TrimRight(m.query, " ")
		m.query = m.query[:strings.LastIndex(m.query, " ")+1]
	case keyTab:
		if sel, ok := m.Selected(); ok {
			m.query = sel.Action
			m.rank()
			m.focus(sel.Action)
		}

		return nil
	default:
		if msg.Text == "" {
			return nil
		}

		m.query += msg.Text
	}

	m.cursor, m.noted = 0, false
	m.rank()

	return nil
}

// browse handles a key in the legend before `/`: it walks a prefix, explains the key that
// completes a sequence, and backs out a level on esc.
func (m *Model) browse(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()

	switch k {
	case keyEsc:
		return m.back()
	case "?":
		return m.closing()
	case "/":
		m.typing, m.pending, m.noted = true, nil, false
		m.rank()

		return nil
	}

	seq := append(slices.Clone(m.pending), k)
	m.pending, m.noted, m.cursor = nil, false, 0

	// A prefix walks before a leaf answers, as the chord itself would: a legend row "ctrl+d/u"
	// names u as well, and u opening a page must still open it.
	if len(under(m.entries, seq)) > 0 {
		m.pending = seq
		m.rank()

		return nil
	}

	if e, ok := Describe(m.entries, strings.Join(seq, " ")); ok {
		m.query = ""
		m.rank()
		m.focus(e.Action)
		m.noted = true

		return nil
	}

	m.status = "nothing on " + strings.Join(seq, " ") + " here"
	m.rank()

	return nil
}

func (m *Model) back() tea.Cmd {
	// An explained key is an answer on the page rather than a level of it, so esc after one closes
	// the legend as it would have before.
	switch {
	case len(m.pending) > 0:
		m.pending = m.pending[:len(m.pending)-1]
	case m.query != "":
		m.query = ""
	default:
		return m.closing()
	}

	m.cursor = 0
	m.rank()

	return nil
}
