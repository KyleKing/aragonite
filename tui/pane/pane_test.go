package pane_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/aragonite/tui/pane"
)

// wait polls the emulated screen rather than sleeping, because the child
// draws when it draws.
func wait(t *testing.T, p *pane.Pane, want string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(p.String(), want) {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("the pane never drew %q; it holds:\n%s", want, p.String())
}

// gone waits out the child rather than asserting on a race with it.
func gone(t *testing.T, p *pane.Pane) pane.GoneMsg {
	t.Helper()

	out := make(chan pane.GoneMsg, 1)

	go func() {
		for {
			if msg, ok := p.Watch().(pane.GoneMsg); ok {
				out <- msg

				return
			}
		}
	}()

	select {
	case msg := <-out:
		return msg
	case <-time.After(10 * time.Second):
		t.Fatal("the child's exit was never reported")

		return pane.GoneMsg{}
	}
}

// The pane is a terminal, so the child should see the size it was given —
// nvim draws for whatever winsize answers, and a pane that lied about it
// draws nothing where it should.
func TestPaneGivesTheChildItsSize(t *testing.T) {
	t.Parallel()

	p, err := pane.New(t.Context(), []string{"sh", "-c", "stty size"}, 97, 11)
	if err != nil {
		t.Fatalf("starting the pane: %v", err)
	}

	wait(t, p, "11 97")

	if msg := gone(t, p); msg.Err != nil {
		t.Errorf("stty size exited with %v", msg.Err)
	}
}

// Keys pressed on the pane reach the child, which is the whole point of the
// thing: a key swallowed by the screen instead is the failure this guards.
func TestPaneSendsKeysToTheChild(t *testing.T) {
	t.Parallel()

	p, err := pane.New(t.Context(), []string{"cat"}, 60, 10)
	if err != nil {
		t.Fatalf("starting the pane: %v", err)
	}

	defer p.Kill()

	p.Send(tea.KeyPressMsg{Code: 'x', Text: "x"})

	wait(t, p, "x")
}

// A paste is text, not a burst of keypresses, and it is multi-byte more often
// than not: the emulator carries it to the child whole rather than dropping
// everything past the first byte.
func TestPaneSendsPasteToTheChild(t *testing.T) {
	t.Parallel()

	p, err := pane.New(t.Context(), []string{"cat"}, 60, 10)
	if err != nil {
		t.Fatalf("starting the pane: %v", err)
	}

	defer p.Kill()

	p.Paste("héllo → wörld")

	wait(t, p, "héllo → wörld")
}

// A child that exits ends the hand-off, with its exit code where the code was
// not clean.
func TestPaneReportsTheExit(t *testing.T) {
	t.Parallel()

	ok, err := pane.New(t.Context(), []string{"true"}, 40, 6)
	if err != nil {
		t.Fatalf("starting the pane: %v", err)
	}

	if msg := gone(t, ok); msg.Err != nil {
		t.Errorf("a clean exit reported %v", msg.Err)
	}

	bad, err := pane.New(t.Context(), []string{"false"}, 40, 6)
	if err != nil {
		t.Fatalf("starting the pane: %v", err)
	}

	if msg := gone(t, bad); msg.Err == nil {
		t.Error("a failed child read as a clean exit")
	}
}

// Killing a pane mid-run is how the screen leaves one it is done with: the
// pump has to see it end rather than block on a dead pty forever.
func TestPaneKillEndsTheWatch(t *testing.T) {
	t.Parallel()

	p, err := pane.New(t.Context(), []string{"cat"}, 40, 6)
	if err != nil {
		t.Fatalf("starting the pane: %v", err)
	}

	p.Kill()

	if msg := gone(t, p); msg.Err == nil {
		t.Error("a killed child reported a clean exit")
	}
}

// The transcript is what a shell session leaves behind: the pane as it was
// rendered, so a redraw the terminal resolved never reaches the note.
func TestPaneTranscriptKeepsTheSession(t *testing.T) {
	t.Parallel()

	p, err := pane.New(t.Context(), []string{"sh", "-c", "printf 'the-evidence\\r\\033[Kgone\\n'"}, 40, 6)
	if err != nil {
		t.Fatalf("starting the pane: %v", err)
	}

	if msg := gone(t, p); msg.Err != nil {
		t.Fatalf("the session exited with %v", msg.Err)
	}
	defer p.Kill()

	if got := p.Transcript(); !strings.Contains(got, "gone") || strings.Contains(got, "the-evidence") {
		t.Errorf("the transcript is not the rendered line: %q", got)
	}
}

// The cursor is the child's, so it keeps the shape the child asks for: nvim
// sets a bar in insert mode (DECSCUSR 5), and a pane that only ever draws a
// block lies about the mode the status line is showing. An underline
// (DECSCUSR 4) keeps the cell's text and marks it instead, so the cursor
// reads over the character it stands on.
func TestPaneDrawsTheCursorShapeTheChildAsks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		esc  string
		want string
	}{
		{"a bar", "5", "▏"},
		{"an underline", "4", "\x1b[4m"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p, err := pane.New(t.Context(),
				[]string{"sh", "-c", "printf '\\033[?25l\\033[?25h\\033[" + tc.esc + " qX'; sleep 2"}, 40, 6)
			if err != nil {
				t.Fatalf("starting the pane: %v", err)
			}

			wait(t, p, "X")
			defer p.Kill()

			var drawn bool
			for _, line := range p.Lines() {
				if strings.Contains(line, tc.want) {
					drawn = true
				}
			}

			if !drawn {
				t.Errorf("a %s-cursor ask drew nothing like it:\n%s", tc.name, strings.Join(p.Lines(), "\n"))
			}
		})
	}
}

// A resize reaches both sides of the pane: the emulator the screen draws, and
// the winsize the child would ask about.
func TestPaneResize(t *testing.T) {
	t.Parallel()

	p, err := pane.New(t.Context(), []string{"cat"}, 40, 6)
	if err != nil {
		t.Fatalf("starting the pane: %v", err)
	}

	defer p.Kill()

	p.Resize(120, 9)

	if got := p.Lines(); len(got) != 9 {
		t.Errorf("the pane draws %d rows after the resize", len(got))
	}
}
