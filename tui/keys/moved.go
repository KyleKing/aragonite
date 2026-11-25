package keys

import (
	"maps"
	"slices"
	"strings"
)

// GroupMoved holds the bindings Moved makes. The legend, the which-key panel, and the palette leave
// it out, and Unreserved exempts it, because a moved key is a notice rather than something to run.
const GroupMoved = "Moved"

// movedPrefix starts the action name of every Moved binding.
const movedPrefix = "moved:"

// Moves is one release's moved keys, each old sequence mapped to the action that now does its job.
// A tool ships it for one release so an old habit answers with where the job went, then deletes
// the entry and the key is unbound (and, where it is reserved, stays so).
type Moves map[string]string

// Bindings is a Moved binding for each entry, in key order so a keymap built from it is stable.
func (m Moves) Bindings() []Binding {
	out := make([]Binding, 0, len(m))
	for _, seq := range slices.Sorted(maps.Keys(m)) {
		out = append(out, Moved(seq, m[seq]))
	}

	return out
}

// Moved is a hidden binding on seq that says its job moved to action. Pressing it matches
// MovedAction(seq); the tool's gate, wrapped by Keymap.WithMoves, refuses it with the notice, so a
// press says where the job went instead of doing nothing. A tool that keeps the old key working
// for the release reads Keymap.Move and runs To after showing the notice. Contexts is zero; set
// it on the result to move a key on some screens only.
func Moved(seq, action string) Binding {
	return Binding{
		Action: MovedAction(seq), Keys: []string{seq}, What: label(seq) + " moved to " + action,
		Group: GroupMoved, To: action,
	}
}

// MovedAction is the action a Moved binding on seq dispatches on.
func MovedAction(seq string) string { return movedPrefix + strings.Join(split(seq), " ") }

// IsMoved reports whether action is a Moved binding's.
func IsMoved(action string) bool { return strings.HasPrefix(action, movedPrefix) }

// Move is where a moved key's job went: the action and the notice a press shows.
type Move struct {
	// To is the action that does the job now.
	To string
	// Notice is the sentence a press shows: "a moved to i (comment)". It names the new key as the
	// keymap binds it, so a reader's rebind is what the notice teaches.
	Notice string
}

// Move answers action's Move, false when action is not a Moved binding in k.
func (k *Keymap) Move(action string) (Move, bool) {
	b, ok := k.Binding(action)
	if !ok || b.To == "" {
		return Move{}, false
	}

	old := b.Label()

	to, ok := k.Binding(b.To)
	if !ok || len(to.Keys) == 0 {
		return Move{To: b.To, Notice: old + " moved: " + b.To + " is unbound"}, true
	}

	word := to.Short
	if word == "" {
		word = to.What
	}

	return Move{To: b.To, Notice: old + " moved to " + to.Label() + " (" + word + ")"}, true
}

// WithMoves wraps g so every Moved binding is refused with its notice, which is how the footer,
// the legend, and a key press all say it without the tool writing a case per moved key.
func (k *Keymap) WithMoves(g Gate) Gate {
	return func(action string) Availability {
		if m, ok := k.Move(action); ok {
			return Refuse(m.Notice)
		}

		return g.Check(action)
	}
}
