package coach_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/coach"
	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

var t0 = time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)

type state struct {
	conflict, described, empty bool
}

func rules() []coach.Rule[state] {
	return []coach.Rule[state]{
		{
			ID: "new", Action: "new", Tone: coach.Next, Cmd: "jj new",
			When: func(s state) bool { return s.described && !s.empty },
			Say:  func(state) string { return "@ is described" },
		},
		{
			ID: "describe", Action: "describe", Tone: coach.Next, Priority: 1, Cmd: "jj describe -r @",
			When: func(s state) bool { return !s.described && !s.empty },
			Say:  func(state) string { return "@ has changes and no description" },
			Why:  func(state) string { return "a message says what it is for" },
		},
		{
			ID: "resolve", Action: "resolve", Tone: coach.Do,
			When: func(s state) bool { return s.conflict },
			Say:  func(state) string { return "@ is conflicted" },
		},
	}
}

func keymap(t *testing.T) *keys.Keymap {
	t.Helper()

	km, err := keys.Merge(nil, []keys.Binding{
		{Action: "describe", Keys: []string{"d"}, What: "describe the change", Short: "describe"},
		{Action: "new", Keys: []string{"n"}, What: "new change", Short: "new"},
		{Action: "resolve", Keys: []string{"x"}, What: "resolve"},
		{Action: "conflict-next", Keys: []string{"] x"}, What: "next conflict"},
		{Action: "line-down", Keys: []string{"j"}, What: "line down"},
		{Action: "hunk-next", Keys: []string{"] h"}, What: "next hunk"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	return km
}

func TestNext_RanksToneThenPriorityAndHonorsTheGate(t *testing.T) {
	t.Parallel()

	c := coach.New(coach.Config[state]{Rules: rules(), Keymap: keymap(t), Start: t0})

	tests := []struct {
		gate keys.Gate
		name string
		want []string
		s    state
	}{
		{name: "nothing holds", s: state{empty: true}, want: nil},
		{name: "undescribed", s: state{}, want: []string{"describe"}},
		{name: "do before next", s: state{conflict: true}, want: []string{"resolve", "describe"}},
		{
			name: "gated action is left to its reason", s: state{conflict: true},
			gate: func(a string) keys.Availability {
				if a == "resolve" {
					return keys.Refuse("no conflict here")
				}

				return keys.Allow()
			},
			want: []string{"describe"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var got []string
			for _, sg := range c.All(tc.s, tc.gate) {
				got = append(got, sg.ID)
			}

			if !slices.Equal(got, tc.want) {
				t.Errorf("All = %v, want %v", got, tc.want)
			}
		})
	}

	sg, ok := c.Next(state{}, nil)
	if !ok || sg.Key != "d" || sg.What != "describe" || sg.Why == "" {
		t.Errorf("Next filled from the keymap = %+v", sg)
	}
}

func pool() []coach.Suggestion {
	return []coach.Suggestion{
		{ID: "tip.conflict-next", Action: "conflict-next", Say: "jumps to the next conflict"},
		{ID: "tip.hunk-next", Action: "hunk-next", Say: "jumps to the next hunk"},
	}
}

func TestTip_RateLimits(t *testing.T) {
	t.Parallel()

	l := memory(t)
	c := coach.New(coach.Config[state]{Ledger: l, Tips: pool(), Keymap: keymap(t), Start: t0})
	lim := coach.DefaultLimits()

	if _, ok := c.Tip(t0.Add(30*time.Second), false); ok {
		t.Error("a tip in the warmup")
	}

	at := t0.Add(lim.Warmup + time.Second)
	c.Press(at)

	if _, ok := c.Tip(at.Add(time.Second), false); ok {
		t.Error("a tip while not idle")
	}

	if _, ok := c.Tip(at.Add(lim.Idle), true); ok {
		t.Error("a tip while busy")
	}

	first, ok := c.Tip(at.Add(lim.Idle), false)
	if !ok || first.ID != "tip.conflict-next" || first.Key != "]x" || first.Tone != coach.Tip {
		t.Fatalf("first tip = %+v %v", first, ok)
	}

	if again, _ := c.Tip(at.Add(time.Hour), false); again.ID != first.ID {
		t.Error("a showing tip did not stay until a key")
	}

	c.Press(at.Add(lim.Idle + time.Second))

	if _, ok := c.Showing(); ok {
		t.Error("a key press did not clear the tip")
	}

	if _, ok := c.Tip(at.Add(lim.Every/2), false); ok {
		t.Error("a second tip inside Every")
	}

	second, ok := c.Tip(at.Add(lim.Every+lim.Idle+time.Second), false)
	if !ok || second.ID != "tip.hunk-next" {
		t.Errorf("second tip = %+v %v (the first is cooling down)", second, ok)
	}
}

func TestTip_PerSessionAndRetire(t *testing.T) {
	t.Parallel()

	tips := make([]coach.Suggestion, 0, 6)
	for i := range 6 {
		tips = append(tips, coach.Suggestion{ID: string(rune('a' + i)), Action: string(rune('a' + i)), Say: "x"})
	}

	lim := coach.DefaultLimits()
	c := coach.New(coach.Config[state]{Tips: tips, Start: t0})

	var shown []string

	at := t0.Add(lim.Warmup)
	for range 10 {
		at = at.Add(lim.Every + lim.Idle)
		if sg, ok := c.Tip(at, false); ok {
			shown = append(shown, sg.ID)
			c.Retire(sg.ID)
		}
	}

	if len(shown) != lim.PerSession {
		t.Errorf("session showed %v, want %d", shown, lim.PerSession)
	}
}

func TestTip_RetiredOnceUsedAndAfterThreeShowings(t *testing.T) {
	t.Parallel()

	l := memory(t)
	lim := coach.DefaultLimits()
	at := t0

	session := func() (coach.Suggestion, bool) {
		at = at.Add(lim.Cooldown + time.Hour)
		c := coach.New(coach.Config[state]{Ledger: l, Tips: pool()[:1], Start: at})

		return c.Tip(at.Add(lim.Warmup+lim.Idle), false)
	}

	for i := range lim.RetireAfter {
		if _, ok := session(); !ok {
			t.Fatalf("showing %d did not happen", i+1)
		}
	}

	if sg, ok := session(); ok {
		t.Errorf("tip shown a fourth time untaken: %+v", sg)
	}

	l2 := memory(t)
	c := coach.New(coach.Config[state]{Ledger: l2, Tips: pool()[:1], Start: t0})

	if _, ok := c.Tip(t0.Add(lim.Warmup+lim.Idle), false); !ok {
		t.Fatal("no tip")
	}

	c.Observe(coach.Event{At: t0.Add(2 * lim.Warmup), Action: "conflict-next"})

	c2 := coach.New(coach.Config[state]{Ledger: l2, Tips: pool()[:1], Start: t0.Add(lim.Cooldown * 2)})
	if sg, ok := c2.Tip(t0.Add(lim.Cooldown*2+lim.Warmup+lim.Idle), false); ok {
		t.Errorf("a taken tip came back: %+v", sg)
	}
}

type seenAll struct{}

func (seenAll) Count(string) int { return 1 }

func TestTip_SkipsWhatTheUsageLogSaysIsLearned(t *testing.T) {
	t.Parallel()

	c := coach.New(coach.Config[state]{Tips: pool(), Seen: seenAll{}, Start: t0})
	if sg, ok := c.Tip(t0.Add(time.Hour), false); ok {
		t.Errorf("tip for a used action: %+v", sg)
	}
}

func TestTip_OffInTheLedger(t *testing.T) {
	t.Parallel()

	l := memory(t)
	l.SetTipsOff(true)

	c := coach.New(coach.Config[state]{Ledger: l, Tips: pool(), Start: t0})
	if _, ok := c.Tip(t0.Add(time.Hour), false); ok {
		t.Error("tips off still tipped")
	}
}

func TestRule_KeyWithoutAKeymapAndMotionIsNotARun(t *testing.T) {
	t.Parallel()

	l := memory(t)
	c := coach.New(coach.Config[state]{Ledger: l, Start: t0, Rules: []coach.Rule[state]{{
		ID: "unread", Action: "] u", Key: "]u", What: "next unread", Tone: coach.Next,
		When: func(state) bool { return true }, Say: func(state) string { return "3 hunks unread" },
	}}})

	if sg, ok := c.Next(state{}, nil); !ok || sg.Key != "]u" || sg.What != "next unread" {
		t.Errorf("Next = %+v", sg)
	}

	c.Observe(coach.Event{At: t0, Action: "j", Motion: true})

	if l.Count("j") != 0 {
		t.Error("a motion counted as a run")
	}
}

func TestWake_ArmsOnlyWhenATipCouldCome(t *testing.T) {
	t.Parallel()

	lim := coach.DefaultLimits()
	c := coach.New(coach.Config[state]{Tips: pool(), Start: t0})

	if c.WakeAt(t0.Add(time.Second)) != nil {
		t.Error("Wake armed during the warmup")
	}

	at := t0.Add(lim.Warmup + time.Second)
	c.Press(at)

	if c.WakeAt(at) == nil {
		t.Fatal("Wake did not arm after the warmup")
	}

	if c.WakeAt(at.Add(time.Second)) != nil {
		t.Error("Wake armed twice")
	}

	if _, ok := c.Tip(at.Add(lim.Idle), false); !ok {
		t.Fatal("no tip when idle")
	}

	if c.WakeAt(at.Add(lim.Idle)) != nil {
		t.Error("Wake armed while a tip shows")
	}

	empty := coach.New(coach.Config[state]{Start: t0})
	if empty.WakeAt(at) != nil {
		t.Error("Wake armed with nothing to teach")
	}
}

func TestDetectors(t *testing.T) {
	t.Parallel()

	tip := coach.Suggestion{ID: "tip.hunk", Action: "hunk-next", Say: "jumps a hunk"}
	key := func(a string) (string, string, bool) { return "s", "squash", a == "squash" }

	tests := []struct {
		name   string
		det    coach.Detector
		events []coach.Event
		want   int
	}{
		{
			name: "repeat fires at n", det: coach.Repeat("line-down", 3, tip),
			events: evs("line-down", "line-down", "line-down"), want: 1,
		},
		{
			name: "repeat restarts on another action", det: coach.Repeat("line-down", 3, tip),
			events: evs("line-down", "line-down", "top", "line-down"), want: 0,
		},
		{name: "instead", det: coach.Instead([]string{"edit", "new"}, tip), events: evs("edit", "new"), want: 1},
		{
			name: "instead needs the order", det: coach.Instead([]string{"edit", "new"}, tip),
			events: evs("new", "edit", "edit", "new"), want: 1,
		},
		{
			name: "via palette", det: coach.ViaPalette(2, key),
			events: []coach.Event{
				{Action: "squash", Palette: true}, {Action: "squash"}, {Action: "squash", Palette: true},
			},
			want: 1,
		},
		{
			name: "via palette needs a key", det: coach.ViaPalette(1, key),
			events: []coach.Event{{Action: "arrange", Palette: true}}, want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := 0

			for _, ev := range tc.events {
				ev.At = t0
				if _, ok := tc.det.Observe(ev); ok {
					got++
				}
			}

			if got != tc.want {
				t.Errorf("fired %d, want %d", got, tc.want)
			}
		})
	}
}

func evs(actions ...string) []coach.Event {
	out := make([]coach.Event, len(actions))
	for i, a := range actions {
		out[i] = coach.Event{Action: a}
	}

	return out
}

func TestObserve_QueuesDetectorTipsFirst(t *testing.T) {
	t.Parallel()

	tip := coach.Suggestion{ID: "tip.hunk", Action: "hunk-next", Say: "jumps a hunk"}
	c := coach.New(coach.Config[state]{
		Tips: pool(), Detectors: []coach.Detector{coach.Repeat("line-down", 2, tip)}, Start: t0,
		Keymap: keymap(t),
	})

	c.Observe(coach.Event{At: t0, Action: "line-down"})
	c.Observe(coach.Event{At: t0, Action: "line-down"})

	sg, ok := c.Tip(t0.Add(time.Hour), false)
	if !ok || sg.ID != "tip.hunk" || sg.Key != "]h" {
		t.Errorf("Tip = %+v %v, want the detector's", sg, ok)
	}
}

func TestLedger_RoundTripsThroughDisk(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "app", "coach.json")

	l, err := coach.LedgerAt(path)
	if err != nil || !l.FirstRun() {
		t.Fatalf("fresh ledger: %v first=%v", err, l.FirstRun())
	}

	l.Ran("squash", t0)
	l.Ran("squash", t0.Add(time.Minute))
	l.Ran("split", t0.Add(time.Hour))
	l.Shown("tip.x", "resolve", t0)
	l.Retire("tip.y")

	if err := l.Save(); err != nil {
		t.Fatal(err)
	}

	back, err := coach.LedgerAt(path)
	if err != nil || back.FirstRun() {
		t.Fatalf("reread: %v first=%v", err, back.FirstRun())
	}

	if back.Count("squash") != 2 || !back.Last("split").Equal(t0.Add(time.Hour)) {
		t.Errorf("runs did not survive: squash=%d split=%v", back.Count("squash"), back.Last("split"))
	}

	if got := back.Recent(5); !slices.Equal(got, []string{"split", "squash"}) {
		t.Errorf("Recent = %v", got)
	}

	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if bad, err := coach.LedgerAt(path); err != nil || !bad.FirstRun() {
		t.Errorf("a corrupt ledger should start fresh: %v", err)
	}
}

func TestLedger_NilIsInert(t *testing.T) {
	t.Parallel()

	var l *coach.Ledger

	l.Ran("x", t0)
	l.Shown("a", "x", t0)

	if l.Count("x") != 0 || !l.FirstRun() || l.Save() != nil || l.Recent(3) != nil {
		t.Error("nil ledger did something")
	}
}

func TestStateDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")

	if dir, err := coach.StateDir("jojo"); err != nil || dir != "/state/jojo" {
		t.Errorf("StateDir = %q %v", dir, err)
	}
}

func sk() skin.Skin { return skin.New(theme.Macchiato(), theme.Macchiato().Mauve) }

func TestRender(t *testing.T) {
	t.Parallel()

	next := coach.Suggestion{
		Tone: coach.Next, Key: "d", What: "describe", Say: "@ has changes but no description",
		Why: "a message says what it is for", Cmd: "jj describe -r @",
	}
	tip := coach.Suggestion{Tone: coach.Tip, Key: "]x", Say: "jumps to the next conflict"}

	if got := ansi.Strip(coach.Line(sk(), next, 80)); got != "▸ next  @ has changes but no description · d describe" {
		t.Errorf("Line = %q", got)
	}

	if got := ansi.Strip(coach.TipLine(sk(), tip, 80)); got != "◇ ]x jumps to the next conflict" {
		t.Errorf("TipLine = %q", got)
	}

	band := coach.BandLines(sk(), []coach.Suggestion{next}, &tip, 100)
	plain := ansi.Strip(strings.Join(band, "\n"))

	for _, want := range []string{
		"── next", "▸ d   describe", "a message says", "jj describe -r @", "◇ tip", "]x jumps",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("band missing %q:\n%s", want, plain)
		}
	}

	for i, l := range band {
		if w := ansi.StringWidth(l); w > 100 {
			t.Errorf("band row %d is %d wide", i, w)
		}
	}

	if coach.BandLines(sk(), nil, nil, 100) != nil {
		t.Error("an empty band took rows")
	}

	marked := coach.Mark([]keyhint.Hint{{Key: "d"}, {Key: "n"}}, next)
	if !marked[0].Suggest || marked[1].Suggest {
		t.Errorf("Mark = %+v", marked)
	}
}

func TestParseLayout(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]coach.Layout{"": coach.Inline, "inline": coach.Inline, "Band": coach.Band} {
		if got, err := coach.ParseLayout(in); err != nil || got != want {
			t.Errorf("ParseLayout(%q) = %v %v", in, got, err)
		}
	}

	if _, err := coach.ParseLayout("corner"); err == nil {
		t.Error("ParseLayout(corner) succeeded")
	}
}

func memory(tb testing.TB) *coach.Ledger {
	tb.Helper()

	l, err := coach.LedgerAt("")
	if err != nil {
		tb.Fatal(err)
	}

	return l
}

func BenchmarkTip(b *testing.B) {
	tips := make([]coach.Suggestion, 0, 100)
	for i := range 100 {
		tips = append(tips, coach.Suggestion{ID: string(rune(i + 'A')), Action: string(rune(i + 'A'))})
	}

	l := memory(b)
	c := coach.New(coach.Config[state]{Ledger: l, Tips: tips, Start: t0})

	for b.Loop() {
		c.Tip(t0.Add(30*time.Second), false)
	}
}

func BenchmarkAll(b *testing.B) {
	c := coach.New(coach.Config[state]{Rules: rules(), Start: t0})

	for b.Loop() {
		c.All(state{conflict: true}, nil)
	}
}
