package keys

import (
	"slices"
	"testing"
)

// Reservation is a key that means one thing across the family even in a tool that lacks the
// concept: such a tool leaves it unbound rather than reuse it. These are the keys whose family
// meaning writes, publishes, or is a gesture every screen shares, which is where a habit carried
// from one tool into another does damage.
type Reservation struct {
	// Keys is the reserved sequence, written as a Binding's keys are.
	Keys string
	// Meaning is what it means across the family, as a failure names it.
	Meaning string
	// Actions are the actions that may sit on Keys exactly. A prefix has none, or the one shared
	// binding the table already gives its bare key (- is less context outside the split columns).
	Actions []string
	// Under is true where a longer sequence starting with Keys is the key's own business: every
	// prefix, and the verbs whose chords follow them (i b, P a). It is false where a chord would
	// turn the key into a prefix, which is how u stopped being undo in second-look's review.
	Under bool
}

// Reserved is the family-reserved keys (keymap principle P1b): undo, redo, reload, the tick,
// visual range, yank, inspect, abandon, split, publish, insert, and the prefixes g z ] [ \ - b.
// Each names the actions any tool may bind on it, and Unreserved fails any other.
func Reserved() []Reservation {
	one := func(keys, meaning string, actions ...string) Reservation {
		return Reservation{Keys: keys, Meaning: meaning, Actions: actions}
	}
	verb := func(keys, meaning string, actions ...string) Reservation {
		return Reservation{Keys: keys, Meaning: meaning, Actions: actions, Under: true}
	}
	prefix := func(keys string, actions ...string) Reservation {
		return Reservation{Keys: keys, Meaning: "a prefix, which never acts alone", Actions: actions, Under: true}
	}

	return []Reservation{
		one("u", "undo", ActionUndo),
		one("ctrl+r", "redo", ActionRedo),
		one("ctrl+l", "reload", ActionReload),
		one("space", "tick the item at point (read, select, or check)", ActionRead, ActionSelect, ActionCheck),
		one("v", "visual range", ActionVisual),
		one("V", "visual range", ActionVisual),
		one("Y", "yank a reference", ActionYank),
		one("K", "inspect", ActionInspect),
		// The conflict viewer's take-all stays on a: it is modal, draws "a all" in its own footer,
		// and its picks are in memory until c confirms them.
		one("a", "abandon", "abandon", ActionTakeAll),
		one("S", "split", "split"),
		// jojo's push and stack submit are publishes that already open their page first.
		verb("P", "publish, after a page naming what goes out", ActionPublish, "push", "submit"),
		verb("i", "insert prose at point", ActionInsert),
		prefix("g"),
		prefix("z"),
		prefix("]"),
		prefix("["),
		prefix(`\`),
		prefix("-", ActionLessContext),
		prefix("b"),
	}
}

// Unreserved lists every binding that puts a reserved key to another use, whatever screen it is
// on: two tools never share a screen, so context cannot excuse a reserved key the way it excuses
// an ordinary one. A Moved binding is exempt, since it exists to say the key's old job left.
func Unreserved(bindings []Binding) []string {
	var out []string

	reserved := Reserved()

	for _, b := range bindings {
		if b.Group == GroupMoved {
			continue
		}

		for _, k := range b.Keys {
			if why := reservedClash(reserved, b.Action, split(k)); why != "" {
				out = append(out, label(k)+" is "+b.Action+" here; "+why)
			}
		}
	}

	return out
}

// reservedClash is why action may not sit on seq, empty when it may.
func reservedClash(reserved []Reservation, action string, seq []string) string {
	for _, r := range reserved {
		rk := split(r.Keys)

		switch {
		case slices.Equal(seq, rk):
			if !slices.Contains(r.Actions, action) {
				return label(r.Keys) + " is reserved for " + r.Meaning
			}
		case len(seq) > len(rk) && slices.Equal(seq[:len(rk)], rk) && !r.Under:
			return label(r.Keys) + " is reserved for " + r.Meaning + " and may not start a chord"
		}
	}

	return ""
}

// RequireReserved fails t for every binding in km that puts a reserved key to another use. Each
// tool calls it from TestKeymap_AgreesWithShared on every keymap it builds, beside Disagreements,
// so a key cannot drift from its family meaning in the one tool nothing else checks.
func RequireReserved(tb testing.TB, km *Keymap) {
	tb.Helper()

	for _, why := range Unreserved(km.Bindings()) {
		tb.Error(why)
	}
}
