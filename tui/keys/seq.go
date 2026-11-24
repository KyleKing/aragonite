package keys

import (
	"slices"

	"github.com/kyleking/aragonite/tui/keyhint"
)

// Match is what one key press came to.
type Match struct {
	// Action is the action the press completed, empty when it completed none.
	Action string
	// Keys is the whole sequence the press ended, so a tool can handle an unbound key itself (a
	// digit, a letter typed into a prompt) when Action is empty and Pending is false.
	Keys []string
	// Pending is a prefix waiting for its next key, which is when a tool shows Matcher.Panel.
	Pending bool
	// Canceled is esc pressed while a prefix waited: the chord is dropped and nothing else
	// happens, since esc backs out one level at a time and the chord is the innermost level.
	Canceled bool
}

// Matcher turns key presses into actions, holding a prefix while it waits for the key that
// completes it.
type Matcher struct {
	keymap  *Keymap
	pending []string
}

// NewMatcher matches presses against keymap.
func NewMatcher(keymap *Keymap) *Matcher {
	return &Matcher{keymap: keymap}
}

// Feed adds one key, named as Bubble Tea's KeyPressMsg.String spells it, on screen ctx. A press that
// completes a sequence answers its action; one that extends a prefix answers Pending; one that does
// neither drops whatever was waiting and answers the keys it ended with no action. A prefix that
// waited for a key it has no binding for is dropped along with that key, as vim drops a broken
// chord, rather than replaying the prefix as a key of its own.
func (m *Matcher) Feed(ctx Context, key string) Match {
	if len(m.pending) > 0 && key == "esc" {
		m.pending = nil

		return Match{Canceled: true, Keys: []string{key}}
	}

	seq := append(slices.Clone(m.pending), key)
	m.pending = nil

	extends := false

	for i, b := range m.keymap.bindings {
		if !b.Contexts.Overlaps(ctx) {
			continue
		}

		for _, keys := range m.keymap.seqs[i] {
			switch {
			case slices.Equal(keys, seq):
				return Match{Action: b.Action, Keys: seq}
			case len(keys) > len(seq) && slices.Equal(keys[:len(seq)], seq):
				extends = true
			}
		}
	}

	if extends {
		m.pending = seq

		return Match{Pending: true, Keys: seq}
	}

	return Match{Keys: seq}
}

// Pending is the prefix waiting for its next key, nil when none is.
func (m *Matcher) Pending() []string { return slices.Clone(m.pending) }

// Reset drops a waiting prefix, for a screen that changes under it.
func (m *Matcher) Reset() { m.pending = nil }

// Panel is the which-key panel for the waiting prefix on screen ctx, nil when nothing waits. Draw it
// with keyhint.Panel and float it with overlay.BottomRight.
func (m *Matcher) Panel(ctx Context) []keyhint.Hint {
	if len(m.pending) == 0 {
		return nil
	}

	return m.keymap.Next(ctx, m.pending)
}
