package keys_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kyleking/aragonite/tui/keys"
)

func movedKeymap(t *testing.T, overrides map[string]string) *keys.Keymap {
	t.Helper()

	moves := keys.Moves{"a": keys.ActionInsert, "S a": keys.ActionPublish}

	km, err := keys.Merge(slices.Concat(keys.Global(), keys.Verbs()), moves.Bindings(), overrides)
	if err != nil {
		t.Fatalf("Merge = %v", err)
	}

	return km
}

func TestMovedKeyNamesWhereItsJobWent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		overrides map[string]string
		name      string
		want      string
		press     []string
	}{
		{name: "one key", press: []string{"a"}, want: "a moved to i (comment)"},
		{name: "a chord", press: []string{"S", "a"}, want: "Sa moved to P (publish)"},
		{
			name: "a rebound target", press: []string{"a"}, want: "a moved to c (comment)",
			overrides: map[string]string{keys.ActionInsert: "c"},
		},
		{
			name: "an unbound target", press: []string{"a"}, want: "a moved: insert is unbound",
			overrides: map[string]string{keys.ActionInsert: ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			km := movedKeymap(t, tc.overrides)
			m := keys.NewMatcher(km)

			var got keys.Match
			for _, p := range tc.press {
				got = m.Feed(keys.View, p)
			}

			if !keys.IsMoved(got.Action) {
				t.Fatalf("%q = %+v, want a moved action", tc.press, got)
			}

			mv, ok := km.Move(got.Action)
			if !ok || mv.Notice != tc.want {
				t.Errorf("Move(%q) = %+v %v, want %q", got.Action, mv, ok, tc.want)
			}

			if a := km.WithMoves(nil).Check(got.Action); a.OK || a.Why != tc.want {
				t.Errorf("WithMoves refused with %+v, want %q", a, tc.want)
			}
		})
	}
}

func TestMovedKeysStayOutOfTheLegendPanelAndPalette(t *testing.T) {
	t.Parallel()

	km := movedKeymap(t, nil)

	for _, h := range km.Legend(keys.View) {
		if h.What == keys.GroupMoved || h.Key == "a" {
			t.Errorf("the legend lists %+v", h)
		}
	}

	for _, b := range km.Bound(keys.View) {
		if keys.IsMoved(b.Action) {
			t.Errorf("the palette lists %q", b.Action)
		}
	}

	if got := km.Next(keys.View, []string{"S"}); len(got) != 0 {
		t.Errorf("the S panel = %+v, want nothing", got)
	}

	if a := km.WithMoves(nil).Check(keys.ActionInsert); !a.OK {
		t.Errorf("WithMoves refused the target: %+v", a)
	}

	if _, ok := km.Move(keys.ActionInsert); ok {
		t.Error("an ordinary action reads as moved")
	}
}

func TestMergeRefusesAMoveToNothing(t *testing.T) {
	t.Parallel()

	_, err := keys.Merge(keys.Global(), keys.Moves{"a": "nonsense"}.Bindings(), nil)
	if !errors.Is(err, keys.ErrUnknownAction) {
		t.Errorf("Merge = %v, want ErrUnknownAction", err)
	}
}

func TestMovesBindInKeyOrder(t *testing.T) {
	t.Parallel()

	got := keys.Moves{"y": keys.ActionLineUp, "B": keys.ActionInsert, "D": keys.ActionInsert}.Bindings()

	seqs := make([]string, 0, len(got))
	for _, b := range got {
		seqs = append(seqs, b.Keys[0])
	}

	if want := []string{"B", "D", "y"}; !slices.Equal(seqs, want) {
		t.Errorf("Bindings() keys = %q, want %q", seqs, want)
	}
}
