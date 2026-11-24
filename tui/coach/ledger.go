package coach

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Ledger is a tool's small local record of what the coach has shown and what the reader has run:
// each tip's showings, whether it was taken or retired, and each action's last run and count,
// which is the recent-commands list a palette ranks by. It lives at
// $XDG_STATE_HOME/<app>/coach.json, is always on, and holds no keystrokes, only tip ids and action
// names with times. It is separate from the opt-in keyusage log, so tips and recency work on a
// machine that never enabled that.
//
// Every method on a nil Ledger does nothing and answers as for a fresh one.
type Ledger struct {
	data  ledgerData
	path  string
	mu    sync.Mutex
	fresh bool
	dirty bool
}

type ledgerData struct {
	First   time.Time          `json:"first"`
	Tips    map[string]tipMark `json:"tips"`
	Ran     map[string]ranMark `json:"ran"`
	TipsOff bool               `json:"tips_off,omitempty"`
}

type tipMark struct {
	Last    time.Time `json:"last"`
	Action  string    `json:"action,omitempty"`
	Shown   int       `json:"shown"`
	Taken   bool      `json:"taken,omitempty"`
	Retired bool      `json:"retired,omitempty"`
}

type ranMark struct {
	Last  time.Time `json:"last"`
	Count int       `json:"count"`
}

// keepRan is how many actions the ledger remembers, dropping the longest unused past it.
const keepRan = 200

const (
	dirPerm  = 0o750
	filePerm = 0o600
)

// StateDir is where app keeps local state: $XDG_STATE_HOME/<app>, else ~/.local/state/<app>.
func StateDir(app string) (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, app), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding the state directory: %w", err)
	}

	return filepath.Join(home, ".local", "state", app), nil
}

// OpenLedger reads app's ledger from its state directory, or starts a fresh one when there is
// none, which is how FirstRun knows.
func OpenLedger(app string) (*Ledger, error) {
	dir, err := StateDir(app)
	if err != nil {
		return nil, err
	}

	return LedgerAt(filepath.Join(dir, "coach.json"))
}

// LedgerAt reads the ledger at path, fresh when the file does not exist, and in memory only when
// path is empty. A file that does not parse starts fresh rather than failing the tool, since it
// holds nothing that cannot be relearned.
func LedgerAt(path string) (*Ledger, error) {
	l := &Ledger{path: path, data: ledgerData{Tips: map[string]tipMark{}, Ran: map[string]ranMark{}}}

	if path == "" {
		l.fresh = true

		return l, nil
	}

	raw, err := os.ReadFile(path) //nolint:gosec // the tool's own state file
	switch {
	case errors.Is(err, fs.ErrNotExist):
		l.fresh = true

		return l, nil
	case err != nil:
		return nil, fmt.Errorf("reading the coach ledger: %w", err)
	}

	if json.Unmarshal(raw, &l.data) != nil {
		l.fresh = true
	}

	if l.data.Tips == nil {
		l.data.Tips = map[string]tipMark{}
	}

	if l.data.Ran == nil {
		l.data.Ran = map[string]ranMark{}
	}

	return l, nil
}

// FirstRun reports whether the ledger started fresh this session, which is when a tool shows its
// welcome card.
func (l *Ledger) FirstRun() bool {
	if l == nil {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.fresh
}

// Began records when the reader first ran the tool, once.
func (l *Ledger) Began(at time.Time) {
	l.update(func(d *ledgerData) {
		if d.First.IsZero() {
			d.First = at
		}
	})
}

// Shown records that tip id, which action would take, was shown at.
func (l *Ledger) Shown(id, action string, at time.Time) {
	l.update(func(d *ledgerData) {
		m := d.Tips[id]
		m.Shown++
		m.Last, m.Action = at, action
		d.Tips[id] = m
	})
}

// Ran records that action ran at, through any path, which is the recent-commands list.
func (l *Ledger) Ran(action string, at time.Time) {
	l.update(func(d *ledgerData) {
		r := d.Ran[action]
		r.Count++
		r.Last = at
		d.Ran[action] = r

		trim(d.Ran)
	})
}

// Took marks taken every shown tip that pointed at action, which retires it: the reader used what
// it taught.
func (l *Ledger) Took(action string) {
	l.update(func(d *ledgerData) {
		for id, m := range d.Tips {
			if m.Action == action && m.Shown > 0 {
				m.Taken = true
				d.Tips[id] = m
			}
		}
	})
}

func trim(ran map[string]ranMark) {
	if len(ran) <= keepRan {
		return
	}

	type aged struct {
		at     time.Time
		action string
	}

	all := make([]aged, 0, len(ran))
	for a, m := range ran {
		all = append(all, aged{action: a, at: m.Last})
	}

	slices.SortFunc(all, func(a, b aged) int { return a.at.Compare(b.at) })

	for _, a := range all[:len(all)-keepRan] {
		delete(ran, a.action)
	}
}

// Retire stops tip id from ever showing again.
func (l *Ledger) Retire(id string) {
	l.update(func(d *ledgerData) {
		m := d.Tips[id]
		m.Retired = true
		d.Tips[id] = m
	})
}

// SetTipsOff turns every tip off or back on, which `:tips off` sets.
func (l *Ledger) SetTipsOff(off bool) { l.update(func(d *ledgerData) { d.TipsOff = off }) }

// TipsOff reports whether tips are off.
func (l *Ledger) TipsOff() bool {
	if l == nil {
		return false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.data.TipsOff
}

// Last is when action last ran, zero when never: the palette's recency.
func (l *Ledger) Last(action string) time.Time {
	if l == nil {
		return time.Time{}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.data.Ran[action].Last
}

// Count is how many times action ran since the ledger began.
func (l *Ledger) Count(action string) int {
	if l == nil {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.data.Ran[action].Count
}

// Recent is up to n actions, most recently run first.
func (l *Ledger) Recent(n int) []string {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	out := make([]string, 0, len(l.data.Ran))
	for a := range l.data.Ran {
		out = append(out, a)
	}

	slices.SortFunc(out, func(a, b string) int {
		if c := l.data.Ran[b].Last.Compare(l.data.Ran[a].Last); c != 0 {
			return c
		}

		return cmp.Compare(a, b)
	})

	return out[:min(n, len(out))]
}

// tip is tip id's record.
func (l *Ledger) tip(id string) tipMark {
	if l == nil {
		return tipMark{}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.data.Tips[id]
}

func (l *Ledger) update(f func(*ledgerData)) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	f(&l.data)
	l.dirty = true
}

// Save writes the ledger if anything changed, through a temporary file renamed into place so a
// crash mid-write never leaves half a file. An in-memory ledger saves nothing.
func (l *Ledger) Save() error {
	if l == nil || l.path == "" {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.dirty {
		return nil
	}

	raw, err := json.MarshalIndent(l.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the coach ledger: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(l.path), dirPerm); err != nil {
		return fmt.Errorf("creating the state directory: %w", err)
	}

	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), filePerm); err != nil {
		return fmt.Errorf("writing the coach ledger: %w", err)
	}

	if err := os.Rename(tmp, l.path); err != nil {
		return fmt.Errorf("replacing the coach ledger: %w", err)
	}

	l.dirty = false

	return nil
}
