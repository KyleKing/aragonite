// Package keyusage logs the command keys a terminal tool's user presses and
// reads the log back against the tool's bindings, so a key nobody presses can
// be found and either dropped or made easier to find.
package keyusage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	dirPerm  = 0o750
	filePerm = 0o600
	monthFmt = "2006-01"
)

// Entry is one logged line. S is the context the key was pressed in, such as
// "review" or "review/help", E is "key" or an event name, and W is the key as
// tea.KeyPressMsg.String() spells it. A is the action an "act" or "gated" line
// names, which survives a rebind where a key does not; a reader that predates
// it skips those lines and still counts every key.
type Entry struct {
	T  time.Time `json:"t"`
	S  string    `json:"s"`
	E  string    `json:"e"`
	W  string    `json:"w,omitempty"`
	A  string    `json:"a,omitempty"`
	MS int64     `json:"ms,omitempty"`
}

// Recorder appends entries to <dir>/<host>-YYYY-MM.jsonl, one file per host and
// month so a synced folder never sees two writers on one file. Every method on
// a nil Recorder does nothing, which is what Open returns when recording is off.
type Recorder struct {
	now   func() time.Time
	f     *os.File
	dir   string
	host  string
	month string
	mu    sync.Mutex
}

// Option configures Open.
type Option func(*Recorder)

// WithDir records into dir regardless of the environment.
func WithDir(dir string) Option { return func(r *Recorder) { r.dir = dir } }

// WithClock replaces time.Now.
func WithClock(now func() time.Time) Option { return func(r *Recorder) { r.now = now } }

// WithHost replaces the hostname in file names.
func WithHost(host string) Option { return func(r *Recorder) { r.host = host } }

// Open returns the recorder for app, or nil when recording is off. It records
// into $<APP>_USAGE_DIR when set, else ~/Sync/.<app>/usage when ~/Sync exists,
// and is off otherwise so an unsynced machine never grows a folder that looks
// synced.
func Open(app string, opts ...Option) *Recorder {
	r := &Recorder{now: time.Now}
	for _, o := range opts {
		o(r)
	}

	if r.dir == "" {
		r.dir = Dir(app)
	}

	if r.dir == "" {
		return nil
	}

	if r.host == "" {
		r.host = hostname()
	}

	return r
}

// Dir is where app's log lives, or "" when recording is off.
func Dir(app string) string {
	if dir := os.Getenv(EnvVar(app)); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	synced := filepath.Join(home, "Sync")
	if info, err := os.Stat(synced); err != nil || !info.IsDir() {
		return ""
	}

	return filepath.Join(synced, "."+app, "usage")
}

// EnvVar names the variable that overrides app's log directory, such as
// SECOND_LOOK_USAGE_DIR.
func EnvVar(app string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(app)) + "_USAGE_DIR"
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}

	short, _, _ := strings.Cut(h, ".")

	return short
}

// Key records a command key pressed in ctx, except line movement. Callers must not pass keys typed
// into a text field: the log answers what was reached for, not what was written.
func (r *Recorder) Key(ctx, key string) {
	if ignored(key) {
		return
	}

	r.put(Entry{S: ctx, E: "key", W: key})
}

// ignored reports keys too constant to say anything about learning a tool:
// line-by-line movement is neither recorded nor reported.
func ignored(key string) bool {
	switch key {
	case "up", "down", "j", "k":
		return true
	}

	return false
}

// Event records something the session did in ctx, with an optional detail.
func (r *Recorder) Event(ctx, name, detail string) { r.put(Entry{S: ctx, E: name, W: detail}) }

// Since records an event carrying how long it took since start.
func (r *Recorder) Since(ctx, name string, start time.Time) {
	if r == nil {
		return
	}

	r.put(Entry{S: ctx, E: name, MS: r.now().Sub(start).Milliseconds()})
}

// Close releases the open file.
func (r *Recorder) Close() error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.f == nil {
		return nil
	}

	err := r.f.Close()
	r.f = nil

	if err != nil {
		return fmt.Errorf("closing the usage log: %w", err)
	}

	return nil
}

func (r *Recorder) put(e Entry) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	e.T = r.now()

	line, err := json.Marshal(e)
	if err != nil {
		return
	}

	f, err := r.file(e.T)
	if err != nil {
		return
	}

	//nolint:errcheck,gosec // a failed write is a lost line, never a broken screen
	f.Write(append(line, '\n'))
}

func (r *Recorder) file(t time.Time) (*os.File, error) {
	month := t.Format(monthFmt)
	if r.f != nil && r.month == month {
		return r.f, nil
	}

	if r.f != nil {
		//nolint:errcheck,gosec // the old month is finished; a failed close loses nothing written
		r.f.Close()
		r.f = nil
	}

	if err := os.MkdirAll(r.dir, dirPerm); err != nil {
		return nil, fmt.Errorf("creating the usage directory: %w", err)
	}

	path := filepath.Join(r.dir, r.host+"-"+month+".jsonl")

	//nolint:gosec // the configured directory plus a generated name
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, filePerm)
	if err != nil {
		return nil, errors.Join(errOpen, err)
	}

	r.f, r.month = f, month

	return f, nil
}

var errOpen = errors.New("opening the usage log")
