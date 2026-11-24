package keyusage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// Binding is one command a context offers: the keys that fire it as
// tea.KeyPressMsg.String() spells them, what it does, and, for a prefix key,
// the bindings its second key picks from.
type Binding struct {
	Keys []string
	What string
	Kids []Binding
}

// Keymap is every context's bindings. The same key may do different things in
// two contexts, and each is counted on its own.
type Keymap map[string][]Binding

// Stat is how often a binding fired and when it last did.
type Stat struct {
	Last  time.Time
	Count int
}

// Usage is a log read back against a Keymap. A path is a binding's first key,
// with a chord's keys joined by a space ("m x").
type Usage struct {
	hits    map[string]map[string]Stat
	misses  map[string]map[string]Stat
	actions map[string]map[string]Stat
	gated   map[string]map[string]Stat
	// Unmapped counts key lines whose context the Keymap does not name.
	Unmapped map[string]int
	// Skipped counts lines that did not parse.
	Skipped int
}

// Load reads every *.jsonl log in dir against km. A missing dir is an empty
// Usage.
func Load(dir string, km Keymap) (Usage, error) {
	u := Usage{
		hits:     map[string]map[string]Stat{},
		misses:   map[string]map[string]Stat{},
		actions:  map[string]map[string]Stat{},
		gated:    map[string]map[string]Stat{},
		Unmapped: map[string]int{},
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return u, fmt.Errorf("listing usage logs: %w", err)
	}

	for _, path := range files {
		if err := u.readFile(path, km); err != nil {
			return u, err
		}
	}

	return u, nil
}

// maxLine is far past any line Recorder writes, so only a corrupt file hits it.
const maxLine = 1 << 20

// pending is a prefix key waiting for the key that completes its chord.
type pending struct {
	b    *Binding
	ctx  string
	path string
}

// readFile tallies one host-month in order, since a chord is two consecutive
// lines of one session.
func (u *Usage) readFile(path string, km Keymap) error {
	//nolint:gosec // a log file found by Load's glob
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxLine)

	var wait *pending

	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			u.Skipped++

			continue
		}

		if e.E == CLI {
			u.tallyCLI(e, km)

			continue
		}

		if u.tallyAction(e) {
			continue
		}

		if e.E != "key" {
			continue
		}

		wait = u.tally(e, km, wait)
	}

	if err := sc.Err(); err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	return nil
}

func (u *Usage) tally(e Entry, km Keymap, wait *pending) *pending {
	if ignored(e.W) {
		return nil
	}

	if wait != nil && wait.ctx == e.S {
		if kid := find(wait.b.Kids, e.W); kid != nil {
			bump(u.hits, e.S, wait.path+" "+kid.Keys[0], e.T)
		} else {
			bump(u.misses, e.S, wait.path+" "+e.W, e.T)
		}

		return nil
	}

	bindings, ok := km[e.S]
	if !ok {
		u.Unmapped[e.S]++

		return nil
	}

	b := find(bindings, e.W)
	if b == nil {
		bump(u.misses, e.S, e.W, e.T)

		return nil
	}

	if len(b.Kids) > 0 {
		return &pending{ctx: e.S, path: b.Keys[0], b: b}
	}

	bump(u.hits, e.S, b.Keys[0], e.T)

	return nil
}

func find(bs []Binding, key string) *Binding {
	for i := range bs {
		if len(bs[i].Keys) > 0 && slices.Contains(bs[i].Keys, key) {
			return &bs[i]
		}
	}

	return nil
}

func bump(m map[string]map[string]Stat, ctx, path string, t time.Time) {
	if m[ctx] == nil {
		m[ctx] = map[string]Stat{}
	}

	s := m[ctx][path]
	s.Count++

	if t.After(s.Last) {
		s.Last = t
	}

	m[ctx][path] = s
}

// Stat is how often the binding at path fired in ctx.
func (u Usage) Stat(ctx, path string) Stat { return u.hits[ctx][path] }

// Level is how strongly a legend should brighten the binding at path in ctx,
// from 0 (pressed within a week) through 1 (7 days), 2 (14 days), to 3 (28 days
// or never).
func (u Usage) Level(ctx, path string, now time.Time) int {
	s := u.Stat(ctx, path)
	if s.Count == 0 {
		return len(levelDays)
	}

	idle := now.Sub(s.Last)
	level := 0

	for i, days := range levelDays {
		if idle >= time.Duration(days)*24*time.Hour {
			level = i + 1
		}
	}

	return level
}

// levelDays are the idle spans that raise a binding one brightening level.
var levelDays = [...]int{7, 14, 28} //nolint:gochecknoglobals // a constant table
