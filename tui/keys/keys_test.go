package keys_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"
)

func shared() []keys.Binding { return slices.Concat(keys.Global(), keys.DiffView()) }

func TestSharedTableMergesWithoutCollisions(t *testing.T) {
	t.Parallel()

	if _, err := keys.Merge(shared(), nil, nil); err != nil {
		t.Fatalf("Merge(shared) = %v", err)
	}

	for _, verb := range []string{
		keys.VerbRebase, keys.VerbSquash, keys.VerbDuplicate, keys.VerbNew,
		keys.VerbRestore, keys.VerbCompare, keys.VerbSplit,
	} {
		if _, err := keys.Merge(keys.Global(), keys.FlagBindings(verb), nil); err != nil {
			t.Errorf("Merge(global, %s flags) = %v", verb, err)
		}
	}
}

func TestSharedActionsAreUnique(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}

	for _, b := range shared() {
		if seen[b.Action] {
			t.Errorf("action %q defined twice", b.Action)
		}

		seen[b.Action] = true

		if b.What == "" || b.Group == "" || len(b.Keys) == 0 {
			t.Errorf("%q needs a description, a group, and a key: %+v", b.Action, b)
		}
	}
}

func TestMergeRefusesAmbiguity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want      error
		overrides map[string]string
		name      string
		tool      []keys.Binding
	}{
		{
			name: "a shared key for another action",
			tool: []keys.Binding{{Action: "apply", Keys: []string{"n"}, Contexts: keys.View}},
			want: keys.ErrDuplicateKey,
		},
		{
			name: "a prefix bound alone",
			tool: []keys.Binding{{Action: "go", Keys: []string{"g"}}},
			want: keys.ErrDuplicateKey,
		},
		{
			name: "an action defined twice",
			tool: []keys.Binding{{Action: keys.ActionQuit, Keys: []string{"Q"}}},
			want: keys.ErrDuplicateAction,
		},
		{
			name:      "an override for nothing",
			overrides: map[string]string{"nonsense": "x"},
			want:      keys.ErrUnknownAction,
		},
		{
			name:      "an override onto a taken key",
			overrides: map[string]string{keys.ActionQuit: "?"},
			want:      keys.ErrDuplicateKey,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := keys.Merge(shared(), tc.tool, tc.overrides); !errors.Is(err, tc.want) {
				t.Errorf("Merge() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestMergeAllowsAKeyOnScreensThatNeverMeet(t *testing.T) {
	t.Parallel()

	// space is read in a diff being viewed and select in one being edited.
	tool := []keys.Binding{{Action: "select", Keys: []string{"space"}, Contexts: keys.Select}}
	if _, err := keys.Merge(shared(), tool, nil); err != nil {
		t.Fatalf("Merge() = %v", err)
	}

	if got := keys.Disagreements(shared(), tool); len(got) != 0 {
		t.Errorf("Disagreements() = %q, want none", got)
	}
}

func TestOverridesRebindAndUnbind(t *testing.T) {
	t.Parallel()

	km, err := keys.Merge(shared(), nil, map[string]string{
		keys.NextAction(keys.ObjHunk): "J",
		keys.ActionShell:              "",
	})
	if err != nil {
		t.Fatal(err)
	}

	m := keys.NewMatcher(km)
	if got := m.Feed(keys.View, "J"); got.Action != keys.NextAction(keys.ObjHunk) {
		t.Errorf("J = %+v, want hunk-next", got)
	}

	if got := m.Feed(keys.View, "]"); got.Pending {
		if next := m.Feed(keys.View, "h"); next.Action != "" {
			t.Errorf("]h still presses %q after the rebind", next.Action)
		}
	}

	if _, ok := km.Hint(keys.ActionShell); ok {
		t.Error("an unbound action still has a footer hint")
	}
}

func TestMatcherSequences(t *testing.T) {
	t.Parallel()

	km, err := keys.Merge(shared(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		want    string
		presses []string
		ctx     keys.Context
	}{
		{name: "a single key", presses: []string{"j"}, want: keys.ActionLineDown, ctx: keys.View},
		{name: "an alternative", presses: []string{"down"}, want: keys.ActionLineDown, ctx: keys.View},
		{name: "a motion", presses: []string{"]", "h"}, want: "hunk-next", ctx: keys.View},
		{name: "the chapter", presses: []string{"[", "["}, want: "chapter-prev", ctx: keys.View},
		{name: "a fold", presses: []string{"z", "a"}, want: keys.ActionFoldToggle, ctx: keys.View},
		{name: "a view toggle", presses: []string{`\`, "s"}, want: keys.ActionViewSideBySide, ctx: keys.View},
		{name: "go to the top", presses: []string{"g", "g"}, want: keys.ActionTop, ctx: keys.List},
		{name: "n is no motion in the log", presses: []string{"n"}, want: "", ctx: keys.Log},
		{name: "space reads only while viewing", presses: []string{"space"}, want: "", ctx: keys.Select},
		{name: "a broken chord drops", presses: []string{"]", "?"}, want: "", ctx: keys.View},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := keys.NewMatcher(km)

			var got keys.Match
			for i, p := range tc.presses {
				got = m.Feed(tc.ctx, p)
				if i < len(tc.presses)-1 && !got.Pending {
					t.Fatalf("press %d (%q) = %+v, want pending", i, p, got)
				}
			}

			if got.Action != tc.want || got.Pending {
				t.Errorf("%q = %+v, want %q", tc.presses, got, tc.want)
			}

			if m.Pending() != nil {
				t.Errorf("Pending() = %q after the chord ended", m.Pending())
			}
		})
	}
}

func TestMatcherEscCancelsAWaitingPrefix(t *testing.T) {
	t.Parallel()

	km, err := keys.Merge(shared(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	m := keys.NewMatcher(km)
	if got := m.Feed(keys.View, "z"); !got.Pending {
		t.Fatalf("z = %+v, want pending", got)
	}

	if got := m.Feed(keys.View, "esc"); !got.Canceled || got.Action != "" {
		t.Errorf("esc = %+v, want a canceled chord", got)
	}

	if got := m.Feed(keys.View, "esc"); got.Canceled {
		t.Error("esc with nothing waiting reads as a canceled chord rather than back")
	}
}

func TestPanelListsWhatCanFollowThePrefix(t *testing.T) {
	t.Parallel()

	km, err := keys.Merge(keys.Global(), keys.Folds(), nil)
	if err != nil {
		t.Fatal(err)
	}

	m := keys.NewMatcher(km)
	if m.Panel(keys.View) != nil {
		t.Error("a panel with nothing waiting")
	}

	m.Feed(keys.View, "z")

	got := m.Panel(keys.View)

	labels := make([]string, 0, len(got))
	for _, h := range got {
		labels = append(labels, h.Key+" "+h.What)
	}

	want := []string{
		"a fold or unfold this", "i invert every fold", "R open every fold", "M fold everything",
		"z line to the middle", "t line to the top", "b line to the bottom",
	}
	if !slices.Equal(labels, want) {
		t.Errorf("Panel() = %q, want %q", labels, want)
	}

	if rows := keyhint.Panel(keyhint.PanelStyles{}, got, 40); len(rows) != len(want) {
		t.Errorf("keyhint.Panel drew %d rows, want %d", len(rows), len(want))
	}
}

func TestLabelsReadAsTyped(t *testing.T) {
	t.Parallel()

	tests := map[string]keys.Binding{
		"gg":      {Keys: []string{"g g"}},
		"]h":      {Keys: []string{"] h"}},
		`\s`:      {Keys: []string{`\ s`}},
		"ctrl+d":  {Keys: []string{"ctrl+d"}},
		"-k":      {Keys: []string{"- k"}},
		"z space": {Keys: []string{"z space"}},
	}

	for want, b := range tests {
		if got := b.Label(); got != want {
			t.Errorf("Label(%q) = %q, want %q", b.Keys[0], got, want)
		}
	}

	if got := (keys.Binding{Keys: []string{"j", "down"}}).Labels(); got != "j/down" {
		t.Errorf("Labels() = %q, want j/down", got)
	}
}

func TestLegendGroupsInFirstAppearance(t *testing.T) {
	t.Parallel()

	km, err := keys.Merge(keys.Global(), keys.ViewOptions(), nil)
	if err != nil {
		t.Fatal(err)
	}

	var heads []string

	for _, h := range km.Legend(keys.View) {
		if h.Head {
			heads = append(heads, h.What)
		}
	}

	want := []string{keys.GroupGeneral, keys.GroupMove, keys.GroupFind, keys.GroupMotion, keys.GroupView}
	if !slices.Equal(heads, want) {
		t.Errorf("Legend heads = %q, want %q", heads, want)
	}

	for _, h := range km.Legend(keys.Log) {
		if h.Key == "n" {
			t.Error("the log legend lists n as a motion")
		}
	}
}

func TestDisagreementsNameTheClash(t *testing.T) {
	t.Parallel()

	got := keys.Disagreements(shared(), []keys.Binding{
		{Action: "apply", Keys: []string{"a", "] h"}, Contexts: keys.Select},
	})

	if len(got) != 1 || !strings.Contains(got[0], "]h is apply") {
		t.Errorf("Disagreements() = %q, want one naming ]h", got)
	}
}

func TestMotionRepeats(t *testing.T) {
	t.Parallel()

	var m keys.Motion

	if s, _ := m.Resolve(keys.ActionAgain); s != (keys.Step{Object: keys.ObjHunk}) {
		t.Errorf("n before any motion = %+v, want the hunk forward", s)
	}

	if s, _ := m.Resolve(keys.ActionReverse); s != (keys.Step{Object: keys.ObjHunk, Back: true}) {
		t.Errorf("N before any motion = %+v, want the hunk back", s)
	}

	if s, ok := m.Resolve(keys.PrevAction(keys.ObjFile)); !ok || s != (keys.Step{Object: keys.ObjFile, Back: true}) {
		t.Errorf("[f = %+v %v", s, ok)
	}

	if s, _ := m.Resolve(keys.ActionAgain); s != (keys.Step{Object: keys.ObjFile, Back: true}) {
		t.Errorf("n after [f = %+v, want the file back", s)
	}

	if s, _ := m.Resolve(keys.ActionReverse); s != (keys.Step{Object: keys.ObjFile}) {
		t.Errorf("N after [f = %+v, want the file forward", s)
	}

	m.Searched()

	if s, _ := m.Resolve(keys.ActionAgain); s.Object != keys.ObjSearch {
		t.Errorf("n after a search = %+v", s)
	}

	for _, action := range []string{keys.ActionViewCycle, keys.ActionQuit, "page-next"} {
		if _, ok := m.Resolve(action); ok {
			t.Errorf("%q resolved as a motion", action)
		}
	}
}

func TestFlagSetRadioAndToggles(t *testing.T) {
	t.Parallel()

	s := keys.NewFlagSet(keys.VerbSquash)
	if !s.On("t") {
		t.Error("--into is not the squash default")
	}

	if !s.Toggle("B") || s.On("t") || !s.On("B") {
		t.Error("-B did not take the placement from --into")
	}

	s.Toggle("k")
	s.Toggle("u")
	s.Toggle("m")

	if got, want := s.Args(), []string{"--use-destination-message", "--keep-emptied"}; !slices.Equal(got, want) {
		t.Errorf("Args() = %q, want %q", got, want)
	}

	s.Toggle("k")

	if s.On("k") || s.Toggle("Z") {
		t.Error("a toggle did not flip back, or an unknown letter toggled")
	}

	if got := keys.NewFlagSet(keys.VerbCompare); len(got.Args()) != 0 {
		t.Errorf("compare passes %q to jj", got.Args())
	}
}

func TestFlagsAreSpelledAsJJSpellsThem(t *testing.T) {
	t.Parallel()

	for _, verb := range []string{keys.VerbRebase, keys.VerbSquash, keys.VerbSplit} {
		radios := map[string]int{}

		for _, f := range keys.Flags(verb) {
			if f.Default {
				radios[f.Radio]++
			}

			if len(f.Arg) == 2 && f.Arg != "-"+f.Letter {
				t.Errorf("%s -%s passes %s", verb, f.Letter, f.Arg)
			}
		}

		for radio, n := range radios {
			if n > 1 {
				t.Errorf("%s radio %q has %d defaults", verb, radio, n)
			}
		}
	}

	if keys.Flags("nonsense") != nil {
		t.Error("an unknown verb has flags")
	}
}

func TestWithoutLeavesItsArgumentAlone(t *testing.T) {
	t.Parallel()

	all := keys.DiffView()
	n := len(all)

	got := keys.Without(all, "unread-next", "unread-prev")
	if len(got) != n-2 || len(all) != n {
		t.Errorf("Without dropped %d of %d and left %d in the argument", n-len(got), n, len(all))
	}
}

// BenchmarkMatcherFeed is one key press resolved against the whole shared table, which a tool pays
// on every key.
func BenchmarkMatcherFeed(b *testing.B) {
	km, err := keys.Merge(shared(), nil, nil)
	if err != nil {
		b.Fatal(err)
	}

	m := keys.NewMatcher(km)

	for b.Loop() {
		m.Feed(keys.View, "]")
		m.Feed(keys.View, "h")
	}
}
