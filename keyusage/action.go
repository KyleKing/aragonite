package keyusage

import (
	"cmp"
	"slices"
	"time"
)

// The events that name an action rather than a key.
const (
	// Act is an action that ran, however it was reached: its key, a chord, or a palette.
	Act = "act"
	// Gated is a key pressed for an action that could not run where the cursor was.
	Gated = "gated"
)

// AnyContext is the context ActionStat and Actions read to count an action across every
// screen it ran on.
const AnyContext = ""

// Action records that action ran in ctx, reached by key ("" when it was not a key press, such as a
// palette run). Log it after the action's key or chord has been logged with Key, so the key report
// keeps counting keys and this line adds the action, which survives a rebind.
func (r *Recorder) Action(ctx, action, key string) {
	r.put(Entry{S: ctx, E: Act, A: action, W: key})
}

// Gated records a press for action that the screen refused, which is a key the reader expected to
// work. A gated key pressed often is a reason the screen should say more, or offer it sooner.
func (r *Recorder) Gated(ctx, action string) {
	r.put(Entry{S: ctx, E: Gated, A: action})
}

// tallyAction counts an act or gated line, by its context and across all of them, and reports
// whether the line was one.
func (u *Usage) tallyAction(e Entry) bool {
	var into map[string]map[string]Stat

	switch e.E {
	case Act:
		into = u.actions
	case Gated:
		into = u.gated
	default:
		return false
	}

	if e.A == "" {
		u.Skipped++

		return true
	}

	bump(into, e.S, e.A, e.T)
	bump(into, AnyContext, e.A, e.T)

	return true
}

// ActionStat is how often action ran in ctx and when it last did, AnyContext counting every
// screen. Logs written before actions were recorded answer zero, so a reader falls back to Stat by
// key for those.
func (u Usage) ActionStat(ctx, action string) Stat { return u.actions[ctx][action] }

// GatedStat is how often action was pressed in ctx where it could not run.
func (u Usage) GatedStat(ctx, action string) Stat { return u.gated[ctx][action] }

// Unused is the actions in order that never ran in ctx.
func (u Usage) Unused(ctx string, actions []string) []string {
	var out []string

	for _, a := range actions {
		if u.actions[ctx][a].Count == 0 {
			out = append(out, a)
		}
	}

	return out
}

// Actions is the log's view of one context by action name, which is what a palette and a coach
// read past use through.
func (u Usage) Actions(ctx string) Actions { return Actions{stats: u.actions[ctx]} }

// Actions answers past use of actions in one context.
type Actions struct{ stats map[string]Stat }

// Last is when action last ran, zero when never.
func (a Actions) Last(action string) time.Time { return a.stats[action].Last }

// Count is how many times action ran.
func (a Actions) Count(action string) int { return a.stats[action].Count }

// Refusals lists every action pressed where it could not run, most frequent first, per context.
func (u Usage) Refusals() []Row {
	var rows []Row

	for ctx, actions := range u.gated {
		if ctx == AnyContext {
			continue
		}

		for action, s := range actions {
			rows = append(rows, Row{Ctx: ctx, Path: action, Stat: s})
		}
	}

	slices.SortFunc(rows, func(a, b Row) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Ctx, b.Ctx), cmp.Compare(a.Path, b.Path))
	})

	return rows
}
