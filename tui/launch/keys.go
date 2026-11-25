package launch

import (
	"slices"

	tea "charm.land/bubbletea/v2"
)

// Keys holds the key presses a model receives before its first data has landed, which would
// otherwise move a cursor over no rows. The zero value holds; [Keys.Release] stops it and hands
// back what it held. A ctrl+c is never held, so a load that never answers can still be left.
//
// A model keeps one as a field and asks it first for every key press:
//
//	case tea.KeyPressMsg:
//		if m.early.Hold(msg) {
//			return m, nil
//		}
//	case loadedMsg:
//		m.apply(msg)
//		held := m.early.Release()
//		return launch.Replay(m, held)
//
// It is safe in a model held by value: a copy never appends into another copy's keys.
type Keys struct {
	held  []tea.KeyPressMsg
	ready bool
}

// Hold keeps msg for later and reports true while the model is not ready, unless msg is ctrl+c.
// Once released it holds nothing.
func (k *Keys) Hold(msg tea.KeyPressMsg) bool {
	if k.ready || msg.String() == keyQuit {
		return false
	}

	k.held = append(slices.Clip(k.held), msg)

	return true
}

// Release marks the model ready and returns the keys held until now, in the order they were
// pressed. Every later call returns nothing, so it can be called on each load and only the first
// replays anything.
func (k *Keys) Release() []tea.KeyPressMsg {
	held := k.held
	k.held, k.ready = nil, true

	return held
}

// Ready reports whether [Keys.Release] has been called.
func (k *Keys) Ready() bool { return k.ready }
