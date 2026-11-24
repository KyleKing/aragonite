package coach

import (
	"slices"
	"time"
)

// Detector watches the stream of actions and proposes a tip when it sees the long way round. It
// works with the usage log off, since it reads only this session.
type Detector interface {
	Observe(ev Event) (Suggestion, bool)
}

// Repeat proposes tip once action has run n times in a row, such as stepping a diff line by line
// where a hunk motion would do. Any other action restarts the count.
func Repeat(action string, n int, tip Suggestion) *RepeatDetector {
	return &RepeatDetector{action: action, n: n, tip: tip}
}

// RepeatDetector is Repeat's detector.
type RepeatDetector struct {
	action string
	tip    Suggestion
	n, run int
}

// Observe counts a run of the action and proposes the tip at the nth.
func (r *RepeatDetector) Observe(ev Event) (Suggestion, bool) {
	if ev.Action != r.action {
		r.run = 0

		return Suggestion{}, false
	}

	r.run++
	if r.run < r.n {
		return Suggestion{}, false
	}

	r.run = 0

	return r.tip, true
}

// insteadGap is the longest pause between two actions of a sequence Instead still reads as one.
const insteadGap = 30 * time.Second

// Instead proposes tip when the actions in seq run one after another, such as edit then new where
// new alone already starts a change on top.
func Instead(seq []string, tip Suggestion) *InsteadDetector {
	return &InsteadDetector{seq: slices.Clone(seq), tip: tip}
}

// InsteadDetector is Instead's detector.
type InsteadDetector struct {
	last time.Time
	tip  Suggestion
	seq  []string
	at   int
}

// Observe follows the sequence and proposes the tip when it completes.
func (d *InsteadDetector) Observe(ev Event) (Suggestion, bool) {
	if d.at > 0 && ev.At.Sub(d.last) > insteadGap {
		d.at = 0
	}

	d.last = ev.At

	switch {
	case len(d.seq) == 0:
		return Suggestion{}, false
	case ev.Action == d.seq[d.at]:
		d.at++
	case ev.Action == d.seq[0]:
		d.at = 1
	default:
		d.at = 0
	}

	if d.at < len(d.seq) {
		return Suggestion{}, false
	}

	d.at = 0

	return d.tip, true
}

// ViaPalette proposes the key for an action once it has been run from the palette n times, which
// is the palette teaching its own way out: "s squashes without the palette". The key function
// answers the action's key label and word, false for an action with no key.
func ViaPalette(n int, key func(action string) (label, what string, ok bool)) *PaletteDetector {
	return &PaletteDetector{n: n, key: key, runs: map[string]int{}}
}

// PaletteDetector is ViaPalette's detector.
type PaletteDetector struct {
	key  func(string) (string, string, bool)
	runs map[string]int
	n    int
}

// Observe counts palette runs per action and proposes the key at the nth.
func (v *PaletteDetector) Observe(ev Event) (Suggestion, bool) {
	if !ev.Palette {
		return Suggestion{}, false
	}

	v.runs[ev.Action]++
	if v.runs[ev.Action] < v.n {
		return Suggestion{}, false
	}

	label, what, ok := v.key(ev.Action)
	if !ok || label == "" {
		return Suggestion{}, false
	}

	v.runs[ev.Action] = 0

	return Suggestion{
		ID: "palette." + ev.Action, Action: ev.Action, Key: label, What: what,
		Say: "runs " + what + " without opening the palette",
	}, true
}
