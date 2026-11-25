package launch_test

import (
	"image/color"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/kyleking/aragonite/tui/launch"
)

// recorder is the program a launch builds: it notes how it was built and every key it is given.
type recorder struct {
	inits *int
	got   []string
	dark  bool
}

func (r recorder) Init() tea.Cmd {
	if r.inits != nil {
		*r.inits++
	}

	return nil
}

func (r recorder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		r.got = append(slices.Clip(r.got), key.String())
	}

	return r, nil
}

func (recorder) View() tea.View { return tea.NewView("") }

func key(r rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: r, Text: string(r)} }

func ctrlC() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl} }

func build(dark bool) tea.Model { return recorder{dark: dark} }

// TestLaunch_KeysTypedBeforeTheAnswerReachTheProgram pins what a launch is for: keys that arrive
// while the terminal is being asked are held, not lost, and reach the program in the order typed,
// built for the background the terminal answered.
func TestLaunch_KeysTypedBeforeTheAnswerReachTheProgram(t *testing.T) {
	t.Parallel()

	light := tea.BackgroundColorMsg{Color: color.RGBA{R: 0xef, G: 0xf1, B: 0xf5, A: 0xff}}
	dark := tea.BackgroundColorMsg{Color: color.RGBA{R: 0x24, G: 0x27, B: 0x3a, A: 0xff}}

	for _, tc := range []struct {
		answer   tea.Msg
		name     string
		wantDark bool
	}{
		{light, "a light background", false},
		{dark, "a dark background", true},
		{uv.PrimaryDeviceAttributesEvent{62, 22}, "device attributes with no background", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			inits := 0

			var m tea.Model = launch.New(func(dark bool) tea.Model { return recorder{dark: dark, inits: &inits} })
			for _, r := range "jjk" {
				m, _ = m.Update(key(r))
			}

			m, _ = m.Update(tc.answer)

			got, ok := m.(recorder)
			if !ok {
				t.Fatalf("the answer did not build the program, the model is %T", m)
			}

			if got.dark != tc.wantDark {
				t.Errorf("built dark=%t, want %t", got.dark, tc.wantDark)
			}

			if want := []string{"j", "j", "k"}; !slices.Equal(got.got, want) {
				t.Errorf("the program was given %q, want %q", got.got, want)
			}

			if inits != 1 {
				t.Errorf("the program's Init ran %d times, want once", inits)
			}
		})
	}
}

// TestLaunch_SilentTerminalStartsDarkAfterTheWait pins a terminal that answers nothing: the wait
// runs out and the program starts dark with what was typed.
func TestLaunch_SilentTerminalStartsDarkAfterTheWait(t *testing.T) {
	t.Parallel()

	m := launch.New(build).WithSettle(0)

	var next tea.Model = m

	next, _ = next.Update(key('j'))

	// The tick Init schedules; with no wait it fires at once.
	for _, msg := range drain(m.Init()) {
		next, _ = next.Update(msg)
	}

	got, ok := next.(recorder)
	if !ok {
		t.Fatalf("the wait running out did not build the program, the model is %T", next)
	}

	if !got.dark || !slices.Equal(got.got, []string{"j"}) {
		t.Errorf("built dark=%t given %q, want dark given [j]", got.dark, got.got)
	}
}

// TestLaunch_CtrlCDoesNotWaitOutASilentTerminal pins the way out while the terminal is asked:
// ctrl+c builds the program at once and reaches it after what was typed before it.
func TestLaunch_CtrlCDoesNotWaitOutASilentTerminal(t *testing.T) {
	t.Parallel()

	var m tea.Model = launch.New(build)

	m, _ = m.Update(key('j'))
	m, _ = m.Update(ctrlC())

	got, ok := m.(recorder)
	if !ok {
		t.Fatalf("ctrl+c was held for the answer, the model is %T", m)
	}

	if want := []string{"j", "ctrl+c"}; !slices.Equal(got.got, want) {
		t.Errorf("the program was given %q, want %q", got.got, want)
	}
}

// drain runs cmd and any batch it returns, collecting the messages that are not terminal writes.
func drain(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	switch msg := cmd().(type) {
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, drain(c)...)
		}

		return out
	case tea.RawMsg:
		return nil
	default:
		return []tea.Msg{msg}
	}
}

// TestLaunch_AsksForTheBackgroundThenTheDeviceAttributesInOneWrite pins the query's order, which is
// what lets a DA1 answer mean the terminal has no background to report.
func TestLaunch_AsksForTheBackgroundThenTheDeviceAttributesInOneWrite(t *testing.T) {
	t.Parallel()

	batch, ok := launch.New(build).WithSettle(0).Init()().(tea.BatchMsg)
	if !ok {
		t.Fatal("Init did not batch the query and the wait")
	}

	var raw []string

	for _, c := range batch {
		if msg, ok := c().(tea.RawMsg); ok {
			if s, ok := msg.Msg.(string); ok {
				raw = append(raw, s)
			}
		}
	}

	if want := []string{"\x1b]11;?\a\x1b[c"}; !slices.Equal(raw, want) {
		t.Errorf("Init wrote %q, want %q", raw, want)
	}
}

// TestLaunch_ViewIsBlankOnTheAltScreen pins the screen while the terminal is asked.
func TestLaunch_ViewIsBlankOnTheAltScreen(t *testing.T) {
	t.Parallel()

	view := launch.New(nil).View()
	if !view.AltScreen {
		t.Error("the launch drew on the main screen, which flashes before the program's alt screen")
	}
}

// TestKeys_HoldsUntilReleasedAndNeverCtrlC pins the hold-until-loaded helper: keys before the load
// come back in order on the first release, ctrl+c is never held, and nothing is held afterwards.
func TestKeys_HoldsUntilReleasedAndNeverCtrlC(t *testing.T) {
	t.Parallel()

	var k launch.Keys

	for _, r := range "jjk" {
		if !k.Hold(key(r)) {
			t.Fatalf("%q was not held before the load", r)
		}
	}

	if k.Hold(ctrlC()) {
		t.Error("ctrl+c was held")
	}

	if k.Ready() {
		t.Error("ready before the release")
	}

	held := k.Release()

	got := make([]string, 0, len(held))
	for _, msg := range held {
		got = append(got, msg.String())
	}

	if want := []string{"j", "j", "k"}; !slices.Equal(got, want) {
		t.Errorf("released %q, want %q", got, want)
	}

	if !k.Ready() || k.Hold(key('j')) {
		t.Error("a key was held after the release")
	}

	if again := k.Release(); len(again) != 0 {
		t.Errorf("a second release handed back %d keys", len(again))
	}
}

// TestKeys_CopiesDoNotShareHeldKeys pins Keys in a model held by value: a copy that holds a key
// never writes into the keys the original holds.
func TestKeys_CopiesDoNotShareHeldKeys(t *testing.T) {
	t.Parallel()

	var k launch.Keys

	k.Hold(key('a'))
	k.Hold(key('b'))

	other := k
	other.Hold(key('x'))
	k.Hold(key('c'))

	if got := k.Release(); len(got) != 3 || got[2].String() != "c" {
		t.Errorf("the original holds %v, want a b c", got)
	}

	if got := other.Release(); len(got) != 3 || got[2].String() != "x" {
		t.Errorf("the copy holds %v, want a b x", got)
	}
}

// TestReplay_HandsMessagesInOrder pins Replay on its own, which a model calls with the keys Keys
// released.
func TestReplay_HandsMessagesInOrder(t *testing.T) {
	t.Parallel()

	m, _ := launch.Replay(tea.Model(recorder{}), []tea.KeyPressMsg{key('a'), key('b')})

	got, ok := m.(recorder)
	if !ok || !slices.Equal(got.got, []string{"a", "b"}) {
		t.Errorf("replayed %q into %T, want [a b] into a recorder", got.got, m)
	}
}
