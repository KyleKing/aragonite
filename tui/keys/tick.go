package keys

import "slices"

// GroupMark is the legend group every tick is filed under, whatever its kind.
const GroupMark = "Mark"

// TickKind is what a tick means in one pane: one of TickRead, TickSelect, or TickCheck. Its value
// is the action name a tick of that kind dispatches on, so a footer can say the kind in one word.
//
// A tick is the family's one meaning for space: a per-item mark drawn in the gutter, toggled by
// space, kept or cleared by the screen's own rule. It never rewrites code or posts content. Verbs
// act on ticks; ticks do not act. Each pane has exactly one kind, and two panes on one screen may
// tick differently because focus is drawn and the footer word changes with it.
type TickKind string

// The tick kinds and the panes each belongs to.
const (
	// TickRead marks the item read in the shared read marks: diff views, an outline, a review queue.
	TickRead TickKind = ActionRead
	// TickSelect selects the item for the verb that follows: jj-diff's -i and the split columns.
	TickSelect TickKind = ActionSelect
	// TickCheck checks the item for a batch verb: jojo's log, bookmark list, and file list.
	TickCheck TickKind = ActionCheck
)

// The tick actions. ActionRead is in DiffView, since every diff view reads; the other two are a
// tool's to add with Tick where it has the pane.
const (
	ActionSelect = "select"
	ActionCheck  = "check"
)

// tickKinds is every kind in the order Ticks lists them.
func tickKinds() []TickKind { return []TickKind{TickRead, TickSelect, TickCheck} }

// Tick is the space binding for kind on the screens that kind belongs to by default: read while
// viewing a diff, select in the select and split modes, check in the log and a list. A tool widens
// or narrows Contexts for its own panes. Tick(TickRead) is the binding DiffView already carries, so
// a tool merging DiffView adds only the other kinds it has.
func Tick(kind TickKind) Binding {
	b := Binding{Action: string(kind), Keys: []string{"space"}, Short: string(kind), Group: GroupMark}

	switch kind {
	case TickRead:
		b.What, b.Contexts = "mark the hunk read", View
	case TickSelect:
		b.What, b.Contexts = "select the hunk, or in visual the lines", Select|Split
	case TickCheck:
		b.What, b.Contexts = "check the item for a batch verb", Log|List
	}

	return b
}

// Ticks is every tick kind's binding, the catalog the reserved space key is checked against.
// It is not a table to merge whole, since DiffView already holds the read tick.
func Ticks() []Binding {
	out := make([]Binding, 0, len(tickKinds()))
	for _, k := range tickKinds() {
		out = append(out, Tick(k))
	}

	return out
}

// IsTick reports whether action is one of the tick kinds, which is what space may press.
func IsTick(action string) bool { return slices.Contains(tickKinds(), TickKind(action)) }
