// Package keys is the key vocabulary jojo, jj-diff, and second-look share, held as data: every
// binding is an action name, the key sequences that press it, what it does, and the group a legend
// files it under. A tool merges the shared table with its own verbs into a Keymap, dispatches
// through a Matcher, and draws its help, footer, and pending-prefix panel from the same rows, so
// the three can never disagree about what a key does or drift from what the legend says.
//
// A key sequence is written as key names separated by spaces, each name as Bubble Tea's
// KeyPressMsg.String spells it: "j", "ctrl+d", "space", "] h", "g g", "\\ s". A sequence of more
// than one key starts with a prefix ("]", "[", "g", "z", "\\", "-"), which never acts alone.
package keys

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/kyleking/aragonite/tui/keyhint"
)

// Context is a set of screens a binding applies on. A binding whose Contexts is zero applies on
// every screen, and two bindings conflict only where their contexts overlap.
type Context uint16

// The screens the family shares. A tool reports the one it is on to Matcher.Feed.
const (
	// View is reading a diff.
	View Context = 1 << iota
	// Select is choosing hunks and lines in a diff, as jj-diff's interactive and editor modes do.
	Select
	// Split is the split columns mode.
	Split
	// Conflict is the conflict viewer.
	Conflict
	// Rounds is stepping a change's evolog versions.
	Rounds
	// List is a file list.
	List
	// Picker is choosing a destination for a verb.
	Picker
	// Log is jojo's revision log, where n and N are verbs rather than motions.
	Log
)

// Diff is every screen that shows a diff, which is where the viewing vocabulary applies.
const Diff = View | Select | Split | Conflict | Rounds

// Overlaps reports whether c and o share a screen, a zero context standing for every screen.
func (c Context) Overlaps(o Context) bool {
	return c == 0 || o == 0 || c&o != 0
}

// Binding is one action and the keys that press it.
type Binding struct {
	// Action is the stable name a user rebinds by: "hunk-next", "fold-toggle", "view-sbs".
	Action string
	// What is what the action does, as a legend says it.
	What string
	// Short is the word a footer shows, and What where empty.
	Short string
	// Group is the legend heading the binding is filed under.
	Group string
	// Keys are the sequences that press it, the first being the one a footer shows.
	Keys []string
	// Contexts are the screens it applies on, zero meaning all of them.
	Contexts Context
}

// Label is the binding's first sequence as a reader types it: "gg", "]h", "\s", "ctrl+d". A binding
// with no keys has an empty label.
func (b Binding) Label() string {
	if len(b.Keys) == 0 {
		return ""
	}

	return label(b.Keys[0])
}

// Labels is every sequence of the binding joined by a slash, which is what a legend shows: "j/down".
func (b Binding) Labels() string {
	out := make([]string, 0, len(b.Keys))
	for _, k := range b.Keys {
		out = append(out, label(k))
	}

	return strings.Join(out, "/")
}

// Hint is the binding as a footer draws it, keyed by its first sequence and worded by Short.
func (b Binding) Hint() keyhint.Hint {
	what := b.Short
	if what == "" {
		what = b.What
	}

	return keyhint.Hint{Key: b.Label(), What: what}
}

// label joins a sequence's keys without spaces where every key is one character, so "g g" reads
// "gg", and with them where a named key would otherwise run into the next.
func label(seq string) string {
	parts := strings.Fields(seq)
	for _, p := range parts {
		if len([]rune(p)) > 1 {
			return strings.Join(parts, " ")
		}
	}

	return strings.Join(parts, "")
}

// split turns a written sequence into its keys.
func split(seq string) []string { return strings.Fields(seq) }

// Without returns bindings minus the named actions, for a tool that lacks a concept the shared
// table carries (second-look's review objects, read marks in a tool that keeps none). It never
// modifies its argument.
func Without(bindings []Binding, actions ...string) []Binding {
	return slices.DeleteFunc(slices.Clone(bindings), func(b Binding) bool {
		return slices.Contains(actions, b.Action)
	})
}

// Errors Merge reports, wrapped with the actions and keys involved.
var (
	// ErrDuplicateKey is two actions on one sequence in overlapping contexts, or a sequence that
	// is the prefix of another and so could never complete.
	ErrDuplicateKey = errors.New("key bound twice")
	// ErrDuplicateAction is one action defined twice.
	ErrDuplicateAction = errors.New("action defined twice")
	// ErrUnknownAction is an override naming an action no binding defines.
	ErrUnknownAction = errors.New("unknown action")
)

// Keymap is a merged, collision-free set of bindings. Build it with Merge.
type Keymap struct {
	bindings []Binding
	// seqs holds each binding's sequences already split into keys, so a key press splits nothing.
	seqs [][][]string
}

// Merge joins the shared table with a tool's own bindings and applies a user's overrides, which map
// an action to the one sequence that should press it ("" unbinds it). Anything ambiguous is an
// error rather than a silent winner: an action defined twice, an override for an action that does
// not exist, or two actions reachable by the same keys on one screen.
func Merge(shared, tool []Binding, overrides map[string]string) (*Keymap, error) {
	all := make([]Binding, 0, len(shared)+len(tool))
	seen := make(map[string]bool, cap(all))

	for _, b := range slices.Concat(shared, tool) {
		if seen[b.Action] {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateAction, b.Action)
		}

		seen[b.Action] = true
		b.Keys = slices.Clone(b.Keys)
		all = append(all, b)
	}

	for action, seq := range overrides {
		i := slices.IndexFunc(all, func(b Binding) bool { return b.Action == action })
		if i < 0 {
			return nil, fmt.Errorf("%w: %q", ErrUnknownAction, action)
		}

		all[i].Keys = nil
		if strings.TrimSpace(seq) != "" {
			all[i].Keys = []string{seq}
		}
	}

	if err := collisions(all); err != nil {
		return nil, err
	}

	seqs := make([][][]string, len(all))
	for i, b := range all {
		for _, k := range b.Keys {
			seqs[i] = append(seqs[i], split(k))
		}
	}

	return &Keymap{bindings: all, seqs: seqs}, nil
}

func collisions(all []Binding) error {
	for i, a := range all {
		for _, b := range all[i+1:] {
			if !a.Contexts.Overlaps(b.Contexts) {
				continue
			}

			for _, ka := range a.Keys {
				for _, kb := range b.Keys {
					if prefixes(split(ka), split(kb)) {
						return fmt.Errorf("%w: %q (%s) and %q (%s)",
							ErrDuplicateKey, label(ka), a.Action, label(kb), b.Action)
					}
				}
			}
		}
	}

	return nil
}

// prefixes reports whether one sequence equals the other or starts it.
func prefixes(a, b []string) bool {
	n := min(len(a), len(b))

	return n > 0 && slices.Equal(a[:n], b[:n])
}

// Bindings is every binding in the keymap, in the order Merge was given them.
func (k *Keymap) Bindings() []Binding { return slices.Clone(k.bindings) }

// Binding is the binding for action, false when the keymap has none.
func (k *Keymap) Binding(action string) (Binding, bool) {
	for _, b := range k.bindings {
		if b.Action == action {
			return b, true
		}
	}

	return Binding{}, false
}

// Hint is action's footer hint, false when it is unbound, so a rebound key changes the footer and an
// unbound one drops out of it.
func (k *Keymap) Hint(action string) (keyhint.Hint, bool) {
	b, ok := k.Binding(action)
	if !ok || len(b.Keys) == 0 {
		return keyhint.Hint{}, false
	}

	return b.Hint(), true
}

// Hints is the footer hints for actions in order, skipping any the keymap leaves unbound.
func (k *Keymap) Hints(actions ...string) []keyhint.Hint {
	out := make([]keyhint.Hint, 0, len(actions))

	for _, a := range actions {
		if h, ok := k.Hint(a); ok {
			out = append(out, h)
		}
	}

	return out
}

// Legend is the help for one screen: a heading per group, in the order groups first appear, then
// each bound action under it keyed by every sequence that presses it.
func (k *Keymap) Legend(ctx Context) []keyhint.Hint {
	var groups []string

	byGroup := map[string][]keyhint.Hint{}

	for _, b := range k.bindings {
		if len(b.Keys) == 0 || !b.Contexts.Overlaps(ctx) {
			continue
		}

		if _, ok := byGroup[b.Group]; !ok {
			groups = append(groups, b.Group)
		}

		byGroup[b.Group] = append(byGroup[b.Group], keyhint.Hint{Key: b.Labels(), What: b.What})
	}

	var out []keyhint.Hint

	for _, g := range groups {
		out = append(out, keyhint.Hint{Head: true, What: g})
		out = append(out, byGroup[g]...)
	}

	return out
}

// Next is what may follow prefix on a screen, one hint per next key, which is the which-key panel a
// tool shows while a prefix waits. A key that opens a deeper prefix is described by the group of
// what lies under it.
func (k *Keymap) Next(ctx Context, prefix []string) []keyhint.Hint {
	var out []keyhint.Hint

	seen := map[string]bool{}

	for _, b := range k.bindings {
		if !b.Contexts.Overlaps(ctx) {
			continue
		}

		for _, seq := range b.Keys {
			keys := split(seq)
			if len(keys) <= len(prefix) || !slices.Equal(keys[:len(prefix)], prefix) {
				continue
			}

			next := keys[len(prefix)]
			if seen[next] {
				continue
			}

			seen[next] = true

			h := keyhint.Hint{Key: next, What: b.What}
			if len(keys) > len(prefix)+1 {
				h.What = b.Group
				h.Kids = []keyhint.Hint{{Key: keys[len(prefix)+1], What: b.What}}
			}

			out = append(out, h)
		}
	}

	return out
}
