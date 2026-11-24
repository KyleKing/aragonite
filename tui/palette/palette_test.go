package palette_test

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"
	"github.com/kyleking/aragonite/tui/palette"
	"github.com/kyleking/aragonite/tui/skin"
	"github.com/kyleking/aragonite/tui/theme"
)

var now = time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

func sk() skin.Skin { return skin.New(theme.Macchiato(), theme.Macchiato().Mauve) }

func entries() []palette.Entry {
	return []palette.Entry{
		{
			Action: "split", Key: "S", What: "split", Group: "Change", Cmd: "jj split -i -r @",
			Prose: []string{"Splits the change at the cursor into two."}, Keys: []string{"S"},
			Related: []string{"]h hunk"}, Flags: "-p siblings", Count: 3, Counted: true,
		},
		{
			Action: "squash", Key: "s", What: "squash into", Group: "Change",
			Cmd: "jj squash --from @ --into <dest>", Also: []string{"amend", "fixup"}, Keys: []string{"s"},
		},
		{Action: "squash-u", Key: "s-u", What: "squash, keep message", Group: "Change", Recent: now.Add(-time.Hour)},
		{
			Action: "resolve", Key: "x", What: "resolve", Group: "Change", Off: true, Why: "@ has no conflict",
			Keys: []string{"x"},
		},
		{Action: "hunk-next", Key: "]h", What: "next hunk", Group: "Jump", Keys: []string{"] h"}},
		{Action: "hunk-prev", Key: "[h", What: "previous hunk", Group: "Jump", Keys: []string{"[ h"}},
	}
}

func actions(hits []palette.Hit) []string {
	out := make([]string, 0, len(hits))
	for i := range hits {
		out = append(out, hits[i].Action)
	}

	return out
}

func press(m *palette.Model, presses ...string) []tea.Msg {
	var msgs []tea.Msg

	for _, k := range presses {
		msg := tea.KeyPressMsg{Text: k}
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		case "ctrl+d":
			msg = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
		default:
			msg.Code, _ = utf8.DecodeRuneInString(k)
		}

		if cmd := m.Update(msg); cmd != nil {
			msgs = append(msgs, cmd())
		}
	}

	return msgs
}

func TestRank_EmptyQuerySections(t *testing.T) {
	t.Parallel()

	got := palette.Rank(entries(), "", now)

	names := make([]string, 0, len(got))
	for _, s := range got {
		names = append(names, s.Name)
	}

	want := []string{palette.SectionHere, palette.SectionRecent, palette.SectionOff}
	if !slices.Equal(names, want) {
		t.Fatalf("sections = %v, want %v", names, want)
	}

	if a := actions(got[1].Hits); !slices.Equal(a, []string{"squash-u"}) {
		t.Errorf("recent = %v", a)
	}

	if a := actions(got[2].Hits); !slices.Equal(a, []string{"resolve"}) {
		t.Errorf("off = %v", a)
	}
}

func TestRank_RecentKeepsNewestAndReturnsTheRest(t *testing.T) {
	t.Parallel()

	es := make([]palette.Entry, 0, palette.RecentLimit+2)
	for i := range palette.RecentLimit + 2 {
		es = append(es, palette.Entry{
			Action: string(rune('a' + i)), What: "x", Recent: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	got := palette.Rank(es, "", now)
	if a := actions(got[1].Hits); !slices.Equal(a, []string{"a", "b", "c", "d", "e"}) {
		t.Errorf("recent = %v", a)
	}

	if a := actions(got[0].Hits); !slices.Equal(a, []string{"f", "g"}) {
		t.Errorf("here = %v", a)
	}
}

func TestRank_Query(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, query, first, via string
	}{
		{name: "description", query: "split", first: "split"},
		{name: "git alias teaches the jj word", query: "amend", first: "squash", via: "amend"},
		{name: "exact key wins", query: "x", first: "resolve"},
		{name: "command shape", query: "--into", first: "squash"},
		{name: "here beats off", query: "re", first: "hunk-prev"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := palette.Rank(entries(), tc.query, now)
			if len(got) != 1 || len(got[0].Hits) == 0 {
				t.Fatalf("Rank(%q) = %+v", tc.query, got)
			}

			if h := got[0].Hits[0]; h.Action != tc.first || h.Via != tc.via {
				t.Errorf("Rank(%q)[0] = %s via %q, want %s via %q; all %v",
					tc.query, h.Action, h.Via, tc.first, tc.via, actions(got[0].Hits))
			}
		})
	}
}

func TestGroups_KeepsLegendOrder(t *testing.T) {
	t.Parallel()

	got := palette.Groups(entries())
	if len(got) != 2 || got[0].Name != "Change" || got[1].Name != "Jump" || len(got[0].Hits) != 4 {
		t.Errorf("Groups = %+v", got)
	}
}

func TestFromKeymap_GatesAndTeaches(t *testing.T) {
	t.Parallel()

	km, err := keys.Merge(nil, []keys.Binding{
		{Action: "squash", Keys: []string{"s"}, What: "squash into", Group: "Change"},
		{Action: "resolve", Keys: []string{"x"}, What: "resolve", Group: "Change"},
		{Action: "push", Keys: []string{"P"}, What: "push", Group: "Remote", Contexts: keys.Log},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	gate := func(a string) keys.Availability {
		if a == "resolve" {
			return keys.Refuse("@ has no conflict")
		}

		return keys.Allow()
	}

	got := palette.FromKeymap(km, keys.View, gate, map[string]palette.About{
		"squash": {Cmd: "jj squash", Also: []string{"amend"}},
	})

	if a := len(got); a != 2 {
		t.Fatalf("FromKeymap = %d entries, want 2 (push is Log only)", a)
	}

	if got[0].Cmd != "jj squash" || !slices.Equal(got[0].Also, []string{"amend"}) || got[0].Off {
		t.Errorf("squash = %+v", got[0])
	}

	if !got[1].Off || got[1].Why != "@ has no conflict" {
		t.Errorf("resolve = %+v", got[1])
	}
}

func TestFromLegend_FlattensChordsAndCarriesProse(t *testing.T) {
	t.Parallel()

	tree := []keyhint.Hint{
		{Head: true, What: "Move"},
		{Key: "j/k", What: "line"},
		{What: "The arrows scroll inside the legend."},
		{Key: "]", What: "go to", Kids: []keyhint.Hint{
			{Key: "h", What: "next hunk"},
			{Key: "u", What: "next unread", Off: true, Why: "nothing unread"},
		}},
		{Key: "m", What: "a state", Off: true, Why: "no comment here", Kids: []keyhint.Hint{
			{Key: "r", What: "ready"},
		}},
		{Head: true, What: "Write"},
		{Key: "e", What: "edit"},
	}

	got := palette.FromLegend(tree)
	if a := actions(hitsOf(got)); !slices.Equal(a, []string{"j", "] h", "] u", "m r", "e"}) {
		t.Fatalf("actions = %v", a)
	}

	if !slices.Equal(got[0].Keys, []string{"j", "k"}) || got[0].Group != "Move" ||
		!slices.Equal(got[0].Prose, []string{"The arrows scroll inside the legend."}) {
		t.Errorf("j/k = %+v", got[0])
	}

	if got[1].Key != "]h" || got[1].Group != "go to" {
		t.Errorf("]h = %+v", got[1])
	}

	if !got[2].Off || got[2].Why != "nothing unread" {
		t.Errorf("]u = %+v", got[2])
	}

	if !got[3].Off || got[3].Why != "no comment here" {
		t.Errorf("mr under an off chord = %+v", got[3])
	}

	if e, ok := palette.Describe(got, "k"); !ok || e.Action != "j" {
		t.Errorf("Describe(k) = %+v %v, want the j/k row", e, ok)
	}

	if _, ok := palette.Describe(got, "z"); ok {
		t.Error("Describe(z) found a row")
	}
}

func TestSequences(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		"j/k":       {"j", "k"},
		"ctrl+d/u":  {"ctrl+d", "ctrl+u"},
		"gg/G":      {"g g", "G"},
		"+/-":       {"+", "-"},
		"?/esc":     {"?", "esc"},
		"space":     {"space"},
		"r/t/x/R":   {"r", "t", "x", "R"},
		"/":         {"/"},
		"shift+tab": {"shift+tab"},
	}

	for label, want := range tests {
		if got := palette.Sequences(label); !slices.Equal(got, want) {
			t.Errorf("Sequences(%q) = %q, want %q", label, got, want)
		}
	}
}

func TestBrowse_PrefixWalksBeforeALeafNamingTheSameKey(t *testing.T) {
	t.Parallel()

	m := palette.New(sk())
	m.Open(palette.Browse, palette.FromLegend([]keyhint.Hint{
		{Key: "ctrl+d/u", What: "half a page"},
		{Key: "u", What: "one of them", Kids: []keyhint.Hint{{Key: "c", What: "a comment"}}},
	}))

	press(m, "u")

	if _, ok := m.Explained(); ok || len(m.Hits()) != 1 || m.Hits()[0].Action != "u c" {
		t.Errorf("u explained instead of walking: %+v", m.Hits())
	}
}

func hitsOf(es []palette.Entry) []palette.Hit {
	out := make([]palette.Hit, len(es))
	for i := range es {
		out[i] = palette.Hit{Entry: es[i]}
	}

	return out
}

type seen map[string]time.Time

func (s seen) Last(a string) time.Time { return s[a] }
func (s seen) Count(a string) int {
	if s[a].IsZero() {
		return 0
	}

	return 1
}

func TestWithSeen(t *testing.T) {
	t.Parallel()

	got := palette.WithSeen(entries(), seen{"split": now}, true)
	if !got[0].Recent.Equal(now) || got[0].Count != 3 || !got[1].Counted {
		t.Errorf("WithSeen = %+v / %+v", got[0], got[1])
	}

	if palette.WithSeen(entries(), nil, true)[1].Counted {
		t.Error("WithSeen(nil) marked counts known")
	}
}

func open(mode palette.Mode) *palette.Model {
	m := palette.New(sk(), palette.WithClock(func() time.Time { return now }))
	m.Open(mode, entries())

	return m
}

func TestRun_TypeAndEnterRunsWithFlags(t *testing.T) {
	t.Parallel()

	m := open(palette.Run)
	msgs := press(m, "s", "p", "l", " ", "-", "p", "enter")
	if m.IsOpen() || len(msgs) != 1 {
		t.Fatalf("open=%v msgs=%v", m.IsOpen(), msgs)
	}

	run, ok := msgs[0].(palette.RunMsg)
	if !ok || run.Action != "split" || !slices.Equal(run.Flags, []string{"-p"}) || run.From != palette.Run {
		t.Errorf("RunMsg = %+v", msgs[0])
	}
}

func TestRun_OffEntryRefusesAndStaysOpen(t *testing.T) {
	t.Parallel()

	m := open(palette.Run)
	msgs := press(m, "x", "enter")
	if !m.IsOpen() || len(msgs) != 1 {
		t.Fatalf("open=%v msgs=%v", m.IsOpen(), msgs)
	}

	if r, ok := msgs[0].(palette.RefusedMsg); !ok || r.Why != "@ has no conflict" {
		t.Errorf("msg = %+v", msgs[0])
	}

	if view := ansi.Strip(strings.Join(m.View(80, 24), "\n")); !strings.Contains(view, "off here: @ has no conflict") {
		t.Errorf("view does not say why:\n%s", view)
	}
}

func TestRun_EditingKeys(t *testing.T) {
	t.Parallel()

	m := open(palette.Run)
	press(m, "s", "q", "backspace")
	if m.Query() != "s" {
		t.Errorf("after backspace query = %q", m.Query())
	}

	press(m, "tab")
	if sel, _ := m.Selected(); m.Query() != sel.Action {
		t.Errorf("tab filled %q, selected %q", m.Query(), sel.Action)
	}

	msgs := press(m, "esc")
	if m.IsOpen() || len(msgs) != 0 {
		t.Fatalf("esc: open=%v msgs=%v", m.IsOpen(), msgs)
	}
}

func TestBrowse_KeyExplainsInsteadOfRunning(t *testing.T) {
	t.Parallel()

	m := open(palette.Browse)
	msgs := press(m, "S")
	if len(msgs) != 0 || !m.IsOpen() {
		t.Fatalf("S ran or closed: msgs=%v", msgs)
	}

	if e, ok := m.Explained(); !ok || e.Action != "split" {
		t.Errorf("Explained = %+v %v", e, ok)
	}

	press(m, "]")
	if a := actions(m.Hits()); !slices.Equal(a, []string{"hunk-next"}) {
		t.Errorf("] narrowed to %v", a)
	}

	press(m, "h")
	if e, ok := m.Explained(); !ok || e.Action != "hunk-next" {
		t.Errorf("]h explained %+v %v", e, ok)
	}

	press(m, "z")
	if _, ok := m.Explained(); ok {
		t.Error("an unbound key kept the note")
	}

	if !strings.Contains(ansi.Strip(strings.Join(m.View(80, 24), "\n")), "nothing on z here") {
		t.Error("an unbound key said nothing")
	}

	msgs = press(m, "S", "enter")
	if run, ok := lastRun(msgs); !ok || run.Action != "split" || run.From != palette.Browse {
		t.Errorf("enter after explaining = %+v", msgs)
	}

	if m.IsOpen() {
		t.Error("still open after running")
	}
}

func lastRun(msgs []tea.Msg) (palette.RunMsg, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if r, ok := msgs[i].(palette.RunMsg); ok {
			return r, true
		}
	}

	return palette.RunMsg{}, false
}

func TestBrowse_FilterAndBackOut(t *testing.T) {
	t.Parallel()

	m := open(palette.Browse)
	press(m, "/", "h", "u", "n", "k")
	if a := actions(m.Hits()); len(a) != 2 || a[0] != "hunk-next" {
		t.Errorf("filter hunk = %v", a)
	}

	press(m, "esc")
	if m.Query() != "" || len(m.Hits()) != len(entries()) {
		t.Errorf("esc kept the filter: %q %d", m.Query(), len(m.Hits()))
	}

	msgs := press(m, "?")
	if m.IsOpen() || len(msgs) != 0 {
		t.Errorf("? did not close quietly: %v", msgs)
	}
}

func TestView_LayoutByWidth(t *testing.T) {
	t.Parallel()

	m := open(palette.Run)
	press(m, "s", "p")

	tests := []struct {
		name          string
		want          []string
		width, height int
		layout        palette.Layout
	}{
		{
			name: "drawer", width: 100, height: 30, layout: palette.Drawer,
			want: []string{"╭─ commands", "split", "runs   jj split -i -r @", "flags  -p siblings", "used 3×"},
		},
		{
			name: "strip", width: 80, height: 24, layout: palette.Strip,
			want: []string{"── commands", ": sp▏", "jj split -i -r @"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := m.Resolve(tc.width); got != tc.layout {
				t.Errorf("Resolve(%d) = %v", tc.width, got)
			}

			view := drawn(t, m, tc.width, tc.height)
			for _, w := range tc.want {
				if !strings.Contains(view, w) {
					t.Errorf("view is missing %q:\n%s", w, view)
				}
			}
		})
	}
}

// drawn draws m over a frame of dots, checks it kept the frame's shape, and answers it unstyled.
func drawn(t *testing.T, m *palette.Model, width, height int) string {
	t.Helper()

	base := make([]string, height)
	for i := range base {
		base[i] = strings.Repeat(".", width)
	}

	out := m.Over(base, width)
	if len(out) != height {
		t.Fatalf("Over changed the height to %d", len(out))
	}

	for i, l := range out {
		if w := ansi.StringWidth(l); w != width {
			t.Errorf("line %d is %d wide: %q", i, w, ansi.Strip(l))
		}
	}

	return ansi.Strip(strings.Join(out, "\n"))
}

func TestView_OffReasonRidesWithTheDescription(t *testing.T) {
	t.Parallel()

	es := entries()
	es[3].Why = "a reason long enough that a column sized to it would squeeze every other row"

	m := palette.New(sk(), palette.WithClock(func() time.Time { return now }))
	m.Open(palette.Browse, es)

	view := drawn(t, m, 120, 30)
	if !strings.Contains(view, "squash, keep message") {
		t.Errorf("a long reason squeezed the other rows:\n%s", view)
	}

	if !strings.Contains(view, "resolve · a reason long") {
		t.Errorf("the reason is not beside the description:\n%s", view)
	}
}

func TestView_RectAndWheel(t *testing.T) {
	t.Parallel()

	m := open(palette.Browse)

	if r := m.Rect(120, 30); r.X != 120-palette.DrawerWidth(120) || r.Y != 0 || r.H != 30 {
		t.Errorf("drawer Rect = %+v", r)
	}

	if r := m.Rect(80, 24); r.X != 0 || r.Y+r.H != 24 || r.W != 80 {
		t.Errorf("strip Rect = %+v", r)
	}

	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})

	if sel, _ := m.Selected(); sel.Action != "resolve" {
		t.Errorf("a wheel notch selected %s", sel.Action)
	}

	press(m, "ctrl+d")

	if sel, _ := m.Selected(); sel.Action != "hunk-prev" {
		t.Errorf("ctrl+d in browse selected %s, want the last row", sel.Action)
	}

	if m.Typing() {
		t.Error("browse mode reports typing")
	}

	press(m, "/")

	if !m.Typing() {
		t.Error("/ did not start typing")
	}
}

func TestSetLayout(t *testing.T) {
	t.Parallel()

	m := open(palette.Run)
	m.SetLayout(palette.Drawer)

	if got := m.Resolve(90); got != palette.Drawer {
		t.Errorf("SetLayout(Drawer) resolved %v at 90", got)
	}

	m.SetLayout(palette.Auto)

	if got := m.Resolve(90); got != palette.Strip {
		t.Errorf("SetLayout(Auto) resolved %v at 90", got)
	}
}

func TestView_ClosedDrawsNothing(t *testing.T) {
	t.Parallel()

	m := palette.New(sk())
	base := []string{"a", "b"}

	if got := m.Over(base, 80); !slices.Equal(got, base) || m.View(80, 2) != nil {
		t.Errorf("closed palette drew %q", got)
	}
}

func TestAgo(t *testing.T) {
	t.Parallel()

	for d, want := range map[time.Duration]string{
		time.Second: "just now", 12 * time.Minute: "12m ago", 3 * time.Hour: "3h ago", 50 * time.Hour: "2d ago",
	} {
		if got := palette.Ago(d); got != want {
			t.Errorf("Ago(%v) = %q, want %q", d, got, want)
		}
	}
}

func many() []palette.Entry {
	base := entries()
	out := make([]palette.Entry, 0, 200)

	for i := range 200 {
		e := base[i%len(base)]
		e.Action += string(rune('a' + i%26))
		out = append(out, e)
	}

	return out
}

func BenchmarkRank(b *testing.B) {
	es := many()

	for b.Loop() {
		palette.Rank(es, "sqin", now)
	}
}

func BenchmarkView(b *testing.B) {
	m := palette.New(sk())
	m.Open(palette.Run, many())
	press(m, "s", "q")

	for b.Loop() {
		m.View(120, 40)
	}
}
