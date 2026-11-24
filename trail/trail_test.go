package trail_test

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/trail"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

func TestShell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, want string
		argv       []string
	}{
		{name: "bare words", argv: []string{"jj", "squash", "--from", "wkwy"}, want: "jj squash --from wkwy"},
		{name: "spaces", argv: []string{"jj", "describe", "-m", "a b"}, want: "jj describe -m 'a b'"},
		{name: "quote in arg", argv: []string{"echo", "it's"}, want: `echo 'it'\''s'`},
		{name: "revset", argv: []string{"jj", "log", "-r", "mine() & ~empty()"}, want: "jj log -r 'mine() & ~empty()'"},
		{name: "empty arg", argv: []string{"git", "commit", "-m", ""}, want: "git commit -m ''"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := trail.Shell(tc.argv); got != tc.want {
				t.Errorf("Shell = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTrail_RingKeepsNewestFirst(t *testing.T) {
	t.Parallel()

	tr := trail.New(3)
	for i := range 5 {
		tr.Add(trail.Call{Argv: []string{"jj", string(rune('a' + i))}, Write: i%2 == 0})
	}

	got := make([]string, 0, 3)
	for _, c := range tr.Calls(false) {
		got = append(got, c.Argv[1])
	}

	if strings.Join(got, "") != "edc" {
		t.Errorf("Calls = %v, want e d c", got)
	}

	if w := tr.Calls(true); len(w) != 2 || w[0].Argv[1] != "e" {
		t.Errorf("writes = %+v", w)
	}

	if c, ok := tr.Last(func(c *trail.Call) bool { return !c.Write }); !ok || c.Argv[1] != "d" {
		t.Errorf("Last read = %+v %v", c, ok)
	}
}

func TestTrail_NilIsInert(t *testing.T) {
	t.Parallel()

	var tr *trail.Trail

	tr.Add(trail.Call{})

	if tr.Calls(false) != nil {
		t.Error("nil trail answered calls")
	}

	if _, ok := tr.Last(nil); ok {
		t.Error("nil trail answered a last call")
	}

	c := tr.Start([]string{"true"}, "", false).Done(nil, nil)
	if c.Exit != 0 {
		t.Errorf("Done on nil = %+v", c)
	}
}

func TestTrail_RunRecordsSuccessAndFailure(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}

	tr := trail.New(0)
	ctx := context.Background()

	out, err := tr.Run(ctx, t.TempDir(), false, "sh", "-c", "echo hi")
	if err != nil || strings.TrimSpace(string(out)) != "hi" {
		t.Fatalf("Run = %q, %v", out, err)
	}

	_, err = tr.Run(ctx, "", true, "sh", "-c", "echo 'Error: no such revision' >&2; echo more >&2; exit 3")
	if err == nil || !strings.Contains(err.Error(), "Error: no such revision") {
		t.Errorf("Run error = %v", err)
	}

	calls := tr.Calls(false)
	if len(calls) != 2 {
		t.Fatalf("calls = %d", len(calls))
	}

	if c := calls[0]; c.Exit != 3 || !c.Failed() || c.Said() != "Error: no such revision" ||
		!strings.Contains(c.Stderr, "more") || !c.Write {
		t.Errorf("failed call = %+v", c)
	}

	if _, err := tr.Run(ctx, "", false); err == nil {
		t.Error("Run with no program succeeded")
	}
}

var errBoom = errors.New("boom")

func TestExitCode(t *testing.T) {
	t.Parallel()

	if trail.ExitCode(nil) != 0 || trail.ExitCode(errBoom) != -1 {
		t.Error("ExitCode of nil or a plain error")
	}
}

func TestTrail_ConcurrentAdds(t *testing.T) {
	t.Parallel()

	tr := trail.New(50)

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			for range 10 {
				tr.Add(trail.Call{Argv: []string{"jj"}})
			}
		})
	}

	wg.Wait()

	if n := len(tr.Calls(false)); n != 50 {
		t.Errorf("kept %d, want 50", n)
	}
}

func TestEchoAndLines(t *testing.T) {
	t.Parallel()

	c := trail.Call{
		Argv: []string{"jj", "squash", "--from", "wkwy", "--into", "rnzx"}, Write: true,
		Start: time.Date(2026, 11, 24, 9, 3, 0, 0, time.UTC), Took: 300 * time.Millisecond,
	}

	if got := trail.Echo("squashed into rnzx", &c, "u undo"); got !=
		"squashed into rnzx · jj squash --from wkwy --into rnzx · u undo" {
		t.Errorf("Echo = %q", got)
	}

	failed := c
	failed.Exit, failed.Stderr = 1, "\nError: rnzx is immutable\n"

	s := skin.New(theme.Macchiato(), theme.Macchiato().Mauve)
	lines := trail.Lines(s, []trail.Call{c, failed}, 70, 0)

	if len(lines) != 3 {
		t.Fatalf("Lines = %d rows, want 3 (the failure explains itself)", len(lines))
	}

	for i, l := range lines[:2] {
		if w := ansi.StringWidth(l); w != 70 {
			t.Errorf("row %d is %d wide", i, w)
		}
	}

	plain := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"✓ 09:03 jj squash", "300ms", "✗ 09:03", "exit 1 · Error: rnzx is immutable"} {
		if !strings.Contains(plain, want) {
			t.Errorf("Lines missing %q:\n%s", want, plain)
		}
	}
}

func BenchmarkAddCalls(b *testing.B) {
	tr := trail.New(trail.DefaultSize)
	c := trail.Call{Argv: []string{"jj", "log", "-r", "@"}}

	for b.Loop() {
		tr.Add(c)
		tr.Calls(true)
	}
}

func BenchmarkShell(b *testing.B) {
	argv := []string{"jj", "describe", "-m", "fix: the thing's edge", "-r", "mine() & ~empty()"}

	for b.Loop() {
		trail.Shell(argv)
	}
}
