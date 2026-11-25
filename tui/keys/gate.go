package keys

import (
	"github.com/kyleking/aragonite/tui/keyhint"
)

// Availability is whether an action can run where the cursor is, and why not. A good Why says
// what is missing and the key that supplies it: "@ has no conflict", "no bookmark on rnzx; b c
// creates one".
type Availability struct {
	Why string
	OK  bool
}

// Allow is an action that can run.
func Allow() Availability { return Availability{OK: true} }

// Refuse is an action that cannot run here, for the reason why.
func Refuse(why string) Availability { return Availability{Why: why} }

// Gate answers Availability for an action on the current screen. A tool writes one switch over
// its own state, and the footer, the legend, the palette, and the key press all read it, so a key
// that does nothing here is drawn Off with its reason and pressing it says the reason instead of
// doing nothing silently. A nil Gate allows everything.
type Gate func(action string) Availability

// Check is the gate's answer for action, Allow when the gate is nil.
func (g Gate) Check(action string) Availability {
	if g == nil {
		return Allow()
	}

	return g(action)
}

// Mark sets Off and Why on h from the gate's answer for action.
func (g Gate) Mark(h keyhint.Hint, action string) keyhint.Hint {
	if a := g.Check(action); !a.OK {
		h.Off, h.Why = true, a.Why
	}

	return h
}

// HintsAt is Hints with each hint the gate refuses marked Off with its reason, which Fit sheds
// first and a legend draws flat.
func (k *Keymap) HintsAt(gate Gate, actions ...string) []keyhint.Hint {
	out := make([]keyhint.Hint, 0, len(actions))

	for _, a := range actions {
		if h, ok := k.Hint(a); ok {
			out = append(out, gate.Mark(h, a))
		}
	}

	return out
}

// LegendAt is Legend with the gate applied: a heading per group, in the order groups first
// appear, then each bound action under it, the ones the gate refuses marked Off with their
// reason. Nothing is dropped, because seeing that a key exists but needs something is how a
// reader learns what it needs.
func (k *Keymap) LegendAt(ctx Context, gate Gate) []keyhint.Hint {
	var groups []string

	byGroup := map[string][]keyhint.Hint{}

	for _, b := range k.bindings {
		if len(b.Keys) == 0 || !b.Contexts.Overlaps(ctx) || b.Group == GroupMoved {
			continue
		}

		if _, ok := byGroup[b.Group]; !ok {
			groups = append(groups, b.Group)
		}

		h := gate.Mark(keyhint.Hint{Key: b.Labels(), What: b.What}, b.Action)
		byGroup[b.Group] = append(byGroup[b.Group], h)
	}

	var out []keyhint.Hint

	for _, g := range groups {
		out = append(out, keyhint.Hint{Head: true, What: g})
		out = append(out, byGroup[g]...)
	}

	return out
}

// Bound is every binding with a key on screen ctx, in the keymap's order, which is what a palette
// lists. Moved bindings are left out, since they are notices rather than things to run.
func (k *Keymap) Bound(ctx Context) []Binding {
	out := make([]Binding, 0, len(k.bindings))

	for _, b := range k.bindings {
		if len(b.Keys) > 0 && b.Contexts.Overlaps(ctx) && b.Group != GroupMoved {
			out = append(out, b)
		}
	}

	return out
}
