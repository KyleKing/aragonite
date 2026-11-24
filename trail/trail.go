// Package trail is a tool's command log: every external command it ran (jj, git, gh), whether it
// wrote, how long it took, how it exited, and what it said on stderr. A status line echoes the last
// write from it so the reader learns the command line behind a key, a `gm` list shows the whole
// session's log with each row copyable as a shell line, and a bug report gets a transcript rather
// than a description.
//
// It is session data held in memory, never written to disk, and it is bounded: a ring of the last
// N calls.
package trail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

// Call is one command run.
type Call struct {
	// Start is when it started.
	Start time.Time
	// Dir is the directory it ran in, empty for the tool's own.
	Dir string
	// Stderr is everything it wrote to stderr, kept whole because the line that explains a failure
	// is not always the first.
	Stderr string
	// Op is the operation a write created, when the tool knows it (a jj operation id), so a row
	// can be undone back to.
	Op string
	// Argv is the program and its arguments, as run.
	Argv []string
	// Took is how long it ran.
	Took time.Duration
	// Exit is its exit status: 0 for success, -1 when it never started or was killed.
	Exit int
	// Write marks a command that changed something, which a log shows by default where reads are
	// behind a toggle.
	Write bool
}

// Failed reports whether the call exited other than cleanly.
func (c *Call) Failed() bool { return c.Exit != 0 }

// Shell is the call as a line to paste into a shell.
func (c *Call) Shell() string { return Shell(c.Argv) }

// Said is the first line of stderr worth reading, the one a status line has room for.
func (c *Call) Said() string {
	for line := range strings.SplitSeq(c.Stderr, "\n") {
		if l := strings.TrimSpace(line); l != "" {
			return l
		}
	}

	return ""
}

// Shell quotes argv for a POSIX shell, leaving words that need no quoting bare, so the line reads
// as a person would type it and pastes as the program ran it.
func Shell(argv []string) string {
	out := make([]string, 0, len(argv))

	for _, a := range argv {
		out = append(out, quote(a))
	}

	return strings.Join(out, " ")
}

func quote(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\n\"'\\$`*?[]()<>|&;#~!{}") {
		return arg
	}

	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

// ExitCode is the exit status err carries: 0 for nil, the process's status for an exec.ExitError,
// and -1 for anything else (it never started, or was killed by a signal).
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() >= 0 {
		return exit.ExitCode()
	}

	return -1
}

// Trail is a bounded log of calls, safe for the commands a Bubble Tea program runs off its update
// loop. Every method on a nil Trail does nothing and answers empty.
type Trail struct {
	now  func() time.Time
	ring []Call
	next int
	full bool
	mu   sync.Mutex
}

// DefaultSize is how many calls New keeps when asked for none.
const DefaultSize = 200

// New is a trail keeping the last n calls, DefaultSize when n is not positive.
func New(n int) *Trail {
	if n <= 0 {
		n = DefaultSize
	}

	return &Trail{ring: make([]Call, n), now: time.Now}
}

// Add records a call.
func (t *Trail) Add(c Call) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.ring[t.next] = c
	t.next = (t.next + 1) % len(t.ring)
	t.full = t.full || t.next == 0
}

// Calls is every call kept, newest first, only the writes when writesOnly is set.
func (t *Trail) Calls(writesOnly bool) []Call {
	if t == nil {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	n := t.next
	if t.full {
		n = len(t.ring)
	}

	out := make([]Call, 0, n)

	for i := range n {
		c := t.ring[(t.next-1-i+len(t.ring))%len(t.ring)]
		if writesOnly && !c.Write {
			continue
		}

		out = append(out, c)
	}

	return out
}

// Last is the newest call matching keep, false when none does. A nil keep matches any.
func (t *Trail) Last(keep func(*Call) bool) (Call, bool) {
	for _, c := range t.Calls(false) {
		if keep == nil || keep(&c) {
			return c, true
		}
	}

	return Call{}, false
}

// Running is a call that has started and not finished.
type Running struct {
	trail *Trail
	call  Call
}

// Start begins timing a call to argv in dir, which Done records.
func (t *Trail) Start(argv []string, dir string, write bool) *Running {
	now := time.Now
	if t != nil {
		now = t.now
	}

	return &Running{trail: t, call: Call{Argv: slices.Clone(argv), Dir: dir, Write: write, Start: now()}}
}

// Done records the call as finished with err and what it wrote to stderr, and answers it.
func (r *Running) Done(err error, stderr []byte) Call {
	now := time.Now
	if r.trail != nil {
		now = r.trail.now
	}

	r.call.Took = now().Sub(r.call.Start)
	r.call.Exit = ExitCode(err)
	r.call.Stderr = string(stderr)

	if r.call.Stderr == "" && err != nil && r.call.Exit == -1 {
		r.call.Stderr = err.Error()
	}

	r.trail.Add(r.call)

	return r.call
}

// Run runs argv in dir and records it, answering its stdout. A failure's error carries the first
// line of stderr, and the trail keeps all of it.
func (t *Trail) Run(ctx context.Context, dir string, write bool, argv ...string) ([]byte, error) {
	if len(argv) == 0 {
		return nil, errNoProgram
	}

	//nolint:gosec // running the program a tool names is this function's purpose
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	r := t.Start(argv, dir, write)
	err := cmd.Run()
	c := r.Done(err, stderr.Bytes())

	if err != nil {
		if said := c.Said(); said != "" {
			return stdout.Bytes(), fmt.Errorf("%s: %s: %w", argv[0], said, err)
		}

		return stdout.Bytes(), fmt.Errorf("%s: %w", argv[0], err)
	}

	return stdout.Bytes(), nil
}

var errNoProgram = errors.New("no program to run")
