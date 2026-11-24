package keys

import "strings"

// Object is a thing the ] and [ prefixes step between, so a new kind of thing to jump to costs a
// letter after the prefix rather than a new key.
type Object struct {
	// Key is the letter after ] or [.
	Key string
	// Noun names the object in actions ("hunk" makes "hunk-next" and "hunk-prev") and in legends.
	Noun string
	// Review marks an object only second-look has (comments, threads, problems, stops, asks), which
	// the other tools leave out of their keymaps rather than draw dead.
	Review bool
}

// The objects' nouns, which a tool switches on after Motion.Resolve.
const (
	ObjHunk     = "hunk"
	ObjFile     = "file"
	ObjChapter  = "chapter"
	ObjUnread   = "unread"
	ObjRound    = "round"
	ObjConflict = "conflict"
	ObjComment  = "comment"
	ObjThread   = "thread"
	ObjProblem  = "problem"
	ObjStop     = "stop"
	ObjAsk      = "ask"
	// ObjSearch is the last / search, which n and N step through once one has run.
	ObjSearch = "search"
)

// Objects is every object the family steps between, hunk first because it is the default motion.
func Objects() []Object {
	return []Object{
		{Key: "h", Noun: ObjHunk},
		{Key: "f", Noun: ObjFile},
		{Key: "]", Noun: ObjChapter},
		{Key: "u", Noun: ObjUnread},
		{Key: "r", Noun: ObjRound},
		{Key: "x", Noun: ObjConflict},
		{Key: "c", Noun: ObjComment, Review: true},
		{Key: "t", Noun: ObjThread, Review: true},
		{Key: "p", Noun: ObjProblem, Review: true},
		{Key: "s", Noun: ObjStop, Review: true},
		{Key: "a", Noun: ObjAsk, Review: true},
	}
}

// NextAction is the forward motion's action name for noun.
func NextAction(noun string) string { return noun + "-next" }

// PrevAction is the backward motion's action name for noun.
func PrevAction(noun string) string { return noun + "-prev" }

// Motions is the ]x and [x pair for each object, on every screen that shows a diff. The chapter
// object is ]] and [[, the doubled bracket reading as "the next one up".
func Motions(objects []Object) []Binding {
	out := make([]Binding, 0, 2*len(objects))

	for _, o := range objects {
		back := o.Key
		if back == "]" {
			back = "["
		}

		out = append(out,
			Binding{
				Action: NextAction(o.Noun), Keys: []string{"] " + o.Key}, Group: GroupMotion,
				What: "next " + o.Noun, Short: o.Noun, Contexts: Diff,
			},
			Binding{
				Action: PrevAction(o.Noun), Keys: []string{"[ " + back}, Group: GroupMotion,
				What: "previous " + o.Noun, Contexts: Diff,
			},
		)
	}

	return out
}

// Step is one motion: the object to move to and which way.
type Step struct {
	Object string
	Back   bool
}

// Motion remembers the last ] or [ motion, or search, so n and N can repeat it forward and back. Its
// zero value repeats the hunk motion, so n and N step hunks before any motion has been made.
type Motion struct {
	last Step
}

// Resolve answers the step an action asks for, false when the action is not a motion. A ] or [
// action is remembered for later repeats; ActionAgain repeats the last one and ActionReverse runs it
// the other way, leaving what is remembered alone.
func (m *Motion) Resolve(action string) (Step, bool) {
	switch action {
	case ActionAgain:
		return m.Last(), true
	case ActionReverse:
		s := m.Last()
		s.Back = !s.Back

		return s, true
	}

	if noun, ok := strings.CutSuffix(action, "-next"); ok && isObject(noun) {
		m.last = Step{Object: noun}

		return m.last, true
	}

	if noun, ok := strings.CutSuffix(action, "-prev"); ok && isObject(noun) {
		m.last = Step{Object: noun, Back: true}

		return m.last, true
	}

	return Step{}, false
}

func isObject(noun string) bool {
	if noun == ObjSearch {
		return true
	}

	for _, o := range Objects() {
		if o.Noun == noun {
			return true
		}
	}

	return false
}

// Searched makes n and N step through search hits, which is what a search the user just ran means
// them to do.
func (m *Motion) Searched() { m.last = Step{Object: ObjSearch} }

// Last is the step n repeats, the hunk forward until a motion has been made.
func (m *Motion) Last() Step {
	if m.last.Object == "" {
		return Step{Object: ObjHunk}
	}

	return m.last
}
