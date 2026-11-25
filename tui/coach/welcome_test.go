package coach_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/coach"
	"github.com/kyleking/aragonite/tui/framecheck"
	"github.com/kyleking/aragonite/tui/keys"
)

var update = flag.Bool("update", false, "rewrite the golden frames")

func welcomeKeymap(t *testing.T, overrides map[string]string) *keys.Keymap {
	t.Helper()

	km, err := keys.Merge(slices.Concat(keys.Global(), keys.DiffView()), nil, overrides)
	if err != nil {
		t.Fatal(err)
	}

	return km
}

func sampleWelcome(t *testing.T, overrides map[string]string) *coach.Welcome {
	t.Helper()

	km := welcomeKeymap(t, overrides)
	w := &coach.Welcome{
		Title: "Welcome to jj-diff",
		Lead:  "Read a change hunk by hunk, and move what you select into another change.",
		Starters: coach.StartersFrom(km, keys.ActionRead, keys.NextAction(keys.ObjHunk),
			keys.ActionOnlyUnread, keys.ActionSearch, "no-such-action"),
	}
	w.HelpKeysFrom(km)

	return w
}

// screen is a filler frame width by height, so a golden shows the card over a screen.
func screen(width, height int) []string {
	out := make([]string, height)
	for i := range out {
		out[i] = ansi.Truncate(strings.Repeat(fmt.Sprintf("row %02d · ", i), width), width, "")
		out[i] += strings.Repeat(" ", width-ansi.StringWidth(out[i]))
	}

	return out
}

func TestWelcome_Frames(t *testing.T) {
	t.Parallel()

	for _, size := range [][2]int{{80, 24}, {100, 30}, {60, 16}} {
		w, h := size[0], size[1]
		name := fmt.Sprintf("welcome-%dx%d", w, h)

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			wc := sampleWelcome(t, nil)
			wc.Open()

			frame := strings.Join(wc.Over(sk(), screen(w, h), w, h), "\n")
			framecheck.Require(t, name, frame, framecheck.Width(w))
			welcomeGolden(t, name, ansi.Strip(frame)+"\n")
		})
	}
}

func TestWelcome_SmallTerminalFallsBackToALine(t *testing.T) {
	t.Parallel()

	for _, size := range [][2]int{{59, 24}, {80, 15}, {40, 10}} {
		w, h := size[0], size[1]

		wc := sampleWelcome(t, nil)
		wc.Open()

		base := screen(w, h)
		if got := wc.Over(sk(), base, w, h); !slices.Equal(got, base) {
			t.Errorf("%dx%d drew the card where it does not fit", w, h)
		}

		line := wc.Line(sk(), w)
		if lw := ansi.StringWidth(line); lw > w || lw == 0 {
			t.Errorf("%dx%d: the fallback line is %d cells", w, h, lw)
		}

		plain := ansi.Strip(line)
		if !strings.Contains(plain, "welcome") || !strings.Contains(plain, "? keys") {
			t.Errorf("%dx%d: the line lost the welcome or help key: %q", w, h, plain)
		}
	}

	wc := sampleWelcome(t, nil)
	welcomeGolden(t, "welcome-line-50", ansi.Strip(func() string { wc.Open(); return wc.Line(sk(), 50) }())+"\n")
}

func TestWelcome_StartersFollowRebinds(t *testing.T) {
	t.Parallel()

	wc := sampleWelcome(t, map[string]string{keys.ActionRead: "m", keys.ActionHelp: "F1"})
	wc.Open()

	plain := ansi.Strip(strings.Join(wc.Block(sk(), 80), "\n"))

	for _, want := range []string{"m     mark the hunk read", "F1    every key", "]h"} {
		if !strings.Contains(plain, want) {
			t.Errorf("the card lacks %q:\n%s", want, plain)
		}
	}

	if strings.Contains(plain, "no-such-action") || strings.Contains(plain, "space") {
		t.Errorf("the card names an unbound action or the old key:\n%s", plain)
	}
}

func TestWelcome_OnlyHelpAndPaletteKeysActWhileDismissing(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		key  string
		pass bool
	}{
		{"j", false},
		{"a", false},
		{"S", false},
		{"enter", false},
		{"q", false},
		{"esc", false},
		{"?", true},
		{":", true},
	} {
		l, err := coach.LedgerAt("")
		if err != nil {
			t.Fatal(err)
		}

		wc := sampleWelcome(t, nil)
		if !wc.OpenIfDue(l) {
			t.Fatal("a fresh ledger did not want the welcome")
		}

		if pass := wc.Key(l, tc.key, t0); pass != tc.pass {
			t.Errorf("%s: pass %v, want %v", tc.key, pass, tc.pass)
		}

		if wc.Showing() || coach.Due(l) {
			t.Errorf("%s left the card up or due", tc.key)
		}

		if !wc.Key(l, "j", t0) {
			t.Errorf("%s: a key with the card down did not pass", tc.key)
		}
	}
}

func TestWelcome_ShowsOncePerReader(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "coach.json")

	l, err := coach.LedgerAt(path)
	if err != nil {
		t.Fatal(err)
	}

	wc := sampleWelcome(t, nil)
	if !wc.OpenIfDue(l) {
		t.Fatal("the first launch did not open the welcome")
	}

	wc.Dismiss(l, t0)

	if err := l.Save(); err != nil {
		t.Fatal(err)
	}

	again, err := coach.LedgerAt(path)
	if err != nil {
		t.Fatal(err)
	}

	if coach.Due(again) || (&coach.Welcome{}).OpenIfDue(again) {
		t.Error("the second launch wanted the welcome again")
	}

	if !again.Welcomed() {
		t.Error("the dismissal was not saved")
	}

	if coach.Due(nil) {
		t.Error("a nil ledger, which cannot remember a dismissal, wanted the welcome")
	}

	// `:welcome` opens it again whatever the ledger says.
	wc2 := &coach.Welcome{}
	wc2.Open()

	if !wc2.Showing() || coach.WelcomeEntry().Action != coach.WelcomeAction {
		t.Error(":welcome did not reopen the card")
	}
}

func TestWelcome_LedgerFromBeforeTheCardIsNotGreeted(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "coach.json")
	if err := os.WriteFile(path, []byte(`{"first":"2026-01-01T00:00:00Z","tips":{},"ran":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	l, err := coach.LedgerAt(path)
	if err != nil {
		t.Fatal(err)
	}

	if coach.Due(l) {
		t.Error("a reader who has run the tool before was greeted as new")
	}
}

func BenchmarkWelcomeOver(b *testing.B) {
	km, err := keys.Merge(slices.Concat(keys.Global(), keys.DiffView()), nil, nil)
	if err != nil {
		b.Fatal(err)
	}

	wc := &coach.Welcome{
		Title: "Welcome", Lead: "A sentence.", Starters: coach.StartersFrom(km, keys.ActionRead, keys.ActionSearch),
	}
	wc.Open()

	base := screen(100, 30)

	for b.Loop() {
		wc.Over(sk(), base, 100, 30)
	}
}

// welcomeGolden compares got with testdata/<name>.golden, rewriting it under -update.
func welcomeGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.MkdirAll("testdata", 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	if got != string(want) {
		t.Errorf("%s differs from its golden (go test ./tui/coach -update):\n%s", name, got)
	}
}

func TestWelcome_ReboundHelpKeyActsAndTheOldOneDoesNot(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		key  string
		pass bool
	}{{"F1", true}, {"?", false}, {":", true}} {
		l, err := coach.LedgerAt("")
		if err != nil {
			t.Fatal(err)
		}

		wc := sampleWelcome(t, map[string]string{keys.ActionHelp: "F1"})
		wc.Open()

		if pass := wc.Key(l, tc.key, t0); pass != tc.pass {
			t.Errorf("%s: pass %v, want %v", tc.key, pass, tc.pass)
		}
	}
}
