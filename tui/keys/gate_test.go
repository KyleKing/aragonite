package keys_test

import (
	"testing"

	"github.com/kyleking/aragonite/tui/keyhint"
	"github.com/kyleking/aragonite/tui/keys"
)

func gated() (*keys.Keymap, keys.Gate) {
	tool := []keys.Binding{
		{Action: "resolve", Keys: []string{"x"}, What: "resolve conflicts", Group: "Change"},
		{Action: "squash", Keys: []string{"s"}, What: "squash into", Group: "Change"},
		{Action: "push", Keys: []string{"P"}, What: "push", Group: "Remote", Contexts: keys.Log},
		{Action: "unbound", What: "palette only", Group: "Change"},
	}

	km, err := keys.Merge(keys.Global(), tool, nil)
	if err != nil {
		panic(err)
	}

	gate := func(action string) keys.Availability {
		if action == "resolve" {
			return keys.Refuse("@ has no conflict")
		}

		return keys.Allow()
	}

	return km, gate
}

func TestGate_NilAllows(t *testing.T) {
	t.Parallel()

	var g keys.Gate
	if a := g.Check("anything"); !a.OK || a.Why != "" {
		t.Errorf("nil Gate.Check = %+v, want OK", a)
	}
}

func TestHintsAt_MarksRefusedOff(t *testing.T) {
	t.Parallel()

	km, gate := gated()

	tests := []struct {
		name, action string
		want         keyhint.Hint
	}{
		{
			name: "refused", action: "resolve",
			want: keyhint.Hint{Key: "x", What: "resolve conflicts", Off: true, Why: "@ has no conflict"},
		},
		{name: "allowed", action: "squash", want: keyhint.Hint{Key: "s", What: "squash into"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := km.HintsAt(gate, tc.action)
			if len(got) != 1 || got[0].Key != tc.want.Key || got[0].Off != tc.want.Off || got[0].Why != tc.want.Why {
				t.Errorf("HintsAt(%s) = %+v, want %+v", tc.action, got, tc.want)
			}
		})
	}

	if got := km.HintsAt(gate, "unbound"); len(got) != 0 {
		t.Errorf("HintsAt(unbound) = %+v, want none", got)
	}
}

func TestLegendAt_KeepsRefusedRowsWithReason(t *testing.T) {
	t.Parallel()

	km, gate := gated()
	legend := km.LegendAt(keys.View, gate)

	var found bool

	for _, h := range legend {
		if h.Key == "x" {
			found = true

			if !h.Off || h.Why != "@ has no conflict" {
				t.Errorf("legend row x = %+v, want Off with its reason", h)
			}
		}

		if h.Key == "P" {
			t.Errorf("legend on View carries the Log-only push: %+v", h)
		}
	}

	if !found {
		t.Fatal("legend dropped the refused row")
	}

	plain := km.Legend(keys.View)
	if len(plain) != len(legend) {
		t.Errorf("Legend has %d rows, LegendAt %d: the gate must not drop rows", len(plain), len(legend))
	}
}

func TestBound_OnlyKeyedBindingsInContext(t *testing.T) {
	t.Parallel()

	km, _ := gated()

	for _, b := range km.Bound(keys.View) {
		if b.Action == "unbound" || b.Action == "push" {
			t.Errorf("Bound(View) carries %s", b.Action)
		}
	}

	log := km.Bound(keys.Log)
	if !containsAction(log, "push") {
		t.Error("Bound(Log) is missing push")
	}
}

func containsAction(bs []keys.Binding, action string) bool {
	for _, b := range bs {
		if b.Action == action {
			return true
		}
	}

	return false
}

func BenchmarkLegendAt(b *testing.B) {
	km, gate := gated()

	for b.Loop() {
		km.LegendAt(keys.View, gate)
	}
}
