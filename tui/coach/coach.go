// Package coach answers two questions a screen should answer without a manual: what should I do
// next, and what am I not using that would help. A tool writes what-next rules over its own state
// and the coach ranks the ones that hold; detectors watch the stream of actions for a long way
// round, and a pool of unlearned keys stands by. Tips are rate limited so they stay rare enough to
// read: one at a time, none in a session's first minute, only when the reader is idle, at most one
// per ten minutes and four per session, three days before the same tip again, and retired for good
// once its action is used or it has been shown three times untaken.
//
// It renders in two layouts. Inline (the default) puts the one next step in a status line and a
// tip in the footer's leading slot until the next key. Band is opt-in: a labeled "next" band above
// the footer naming each suggestion, why, and the command it runs.
//
// Welcome is the first-run card, shown once per reader and reopened by `:welcome`.
package coach

import (
	"cmp"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/aragonite/tui/keys"
)

// Tone is how much a suggestion matters.
type Tone int

const (
	// Do is something wrong or blocked: a conflict, a stale working copy.
	Do Tone = iota
	// Next is the natural next step: describe the change, then start the next one.
	Next
	// Could is an option worth knowing about here.
	Could
	// Tip is a key the reader is not using that would help.
	Tip
)

// String is the tone's label as a status line shows it.
func (t Tone) String() string {
	switch t {
	case Do:
		return "do"
	case Next:
		return "next"
	case Could:
		return "could"
	case Tip:
		return "tip"
	}

	return "next"
}

// Suggestion is one thing the coach points at.
type Suggestion struct {
	// ID is stable across sessions, which is what the ledger remembers a tip by:
	// "jojo.describe-before-new".
	ID string
	// Action is what taking the suggestion runs. A rebind changes Key and not this.
	Action string
	// Key is the label of the key that runs it, filled from the keymap: "d", "]x".
	Key string
	// What is the action's word: "describe".
	What string
	// Say is the state that prompts it, or for a tip what the key does: "@ has 2 files and no
	// description".
	Say string
	// Why is a line more for the band: "it has changes and no message".
	Why string
	// Cmd is the command taking it would run, for the band: "jj describe -r @".
	Cmd string
	// Tone is how much it matters.
	Tone Tone
}

// Rule is one tool-supplied what-next rule over the tool's own state type S.
type Rule[S any] struct {
	// When reports whether the rule holds.
	When func(S) bool
	// Say words the state that prompts it.
	Say func(S) string
	// Why is optional: a line more for the band.
	Why    func(S) string
	ID     string
	Action string
	// Key and What name the key that takes it, for a tool with no keys.Keymap to fill them from.
	Key  string
	What string
	Cmd  string
	Tone Tone
	// Priority orders rules of one tone, the highest first.
	Priority int
}

// Limits are how rare tips stay. Rules are never limited: a next step that holds is always shown.
type Limits struct {
	// Warmup is how long into a session before the first tip.
	Warmup time.Duration
	// Idle is how long without a key before a tip shows.
	Idle time.Duration
	// Every is the least time between two tips.
	Every time.Duration
	// Cooldown is the least time before one tip shows again.
	Cooldown time.Duration
	// PerSession is the most tips one session shows.
	PerSession int
	// RetireAfter is how many showings without being taken retire a tip.
	RetireAfter int
}

// The limits the family ships with.
const (
	defaultIdle        = 2 * time.Second
	defaultEvery       = 10 * time.Minute
	defaultCooldown    = 3 * 24 * time.Hour
	defaultPerSession  = 4
	defaultRetireAfter = 3
)

// DefaultLimits are the limits the family ships with: none in the first minute, idle for two
// seconds, ten minutes apart, four a session, three days before one repeats, retired after three
// showings untaken.
func DefaultLimits() Limits {
	return Limits{
		Warmup:      time.Minute,
		Idle:        defaultIdle,
		Every:       defaultEvery,
		Cooldown:    defaultCooldown,
		PerSession:  defaultPerSession,
		RetireAfter: defaultRetireAfter,
	}
}

// Seen is past use read from another source, such as keyusage.Usage.Actions, which a tip consults
// so a key used before the ledger existed is not taught as new.
type Seen interface {
	Count(action string) int
}

// Config builds a Coach.
type Config[S any] struct {
	// Start is when the session began, time.Now when zero.
	Start time.Time
	// Ledger is where tips and runs are remembered; nil keeps nothing across sessions.
	Ledger *Ledger
	// Seen is optional past use beyond the ledger.
	Seen Seen
	// Keymap fills a suggestion's Key and What from its Action where they are empty, so a rebind
	// moves the key a suggestion names.
	Keymap *keys.Keymap
	// Rules are the what-next rules, in the order that breaks ties.
	Rules []Rule[S]
	// Detectors watch the action stream for a long way round.
	Detectors []Detector
	// Tips are the unlearned-key tips, in the order to teach them: tier by tier.
	Tips []Suggestion
	// Limits default to DefaultLimits when zero.
	Limits Limits
}

// Coach is one session's coach for a screen whose state is S.
type Coach[S any] struct {
	keymap  *keys.Keymap
	start   time.Time
	lastKey time.Time
	lastTip time.Time
	armed   time.Time
	ledger  *Ledger
	seen    Seen
	showing *Suggestion
	rules   []Rule[S]
	dets    []Detector
	pool    []Suggestion
	queue   []Suggestion
	lim     Limits
	shown   int
}

// New is a coach for one session.
func New[S any](cfg Config[S]) *Coach[S] {
	if cfg.Start.IsZero() {
		cfg.Start = time.Now()
	}

	if cfg.Limits == (Limits{}) {
		cfg.Limits = DefaultLimits()
	}

	cfg.Ledger.Began(cfg.Start)

	return &Coach[S]{
		keymap: cfg.Keymap, start: cfg.Start, lastKey: cfg.Start, ledger: cfg.Ledger, seen: cfg.Seen,
		rules: cfg.Rules, dets: cfg.Detectors, pool: cfg.Tips, lim: cfg.Limits,
	}
}

// Ledger is the coach's ledger, for a tool to save on quit and read recency from.
func (c *Coach[S]) Ledger() *Ledger { return c.ledger }

// All is every rule that holds for s and whose action the gate allows, Do first, then Next, then
// Could, each tone by priority, ties in rule order. A refused action is left to the gate's own
// reason rather than suggested.
func (c *Coach[S]) All(s S, gate keys.Gate) []Suggestion {
	type ranked struct {
		Suggestion

		priority, order int
	}

	var held []ranked

	for i, r := range c.rules {
		if r.When == nil || !r.When(s) || !gate.Check(r.Action).OK {
			continue
		}

		sg := c.keyed(Suggestion{ID: r.ID, Action: r.Action, Key: r.Key, What: r.What, Cmd: r.Cmd, Tone: r.Tone})
		if r.Say != nil {
			sg.Say = r.Say(s)
		}

		if r.Why != nil {
			sg.Why = r.Why(s)
		}

		held = append(held, ranked{Suggestion: sg, priority: r.Priority, order: i})
	}

	slices.SortStableFunc(held, func(a, b ranked) int {
		return cmp.Or(cmp.Compare(a.Tone, b.Tone), cmp.Compare(b.priority, a.priority), cmp.Compare(a.order, b.order))
	})

	out := make([]Suggestion, len(held))
	for i := range held {
		out[i] = held[i].Suggestion
	}

	return out
}

// Next is the one next step for s, which the inline layout shows in its status line.
func (c *Coach[S]) Next(s S, gate keys.Gate) (Suggestion, bool) {
	all := c.All(s, gate)
	if len(all) == 0 {
		return Suggestion{}, false
	}

	return all[0], true
}

// Event is one thing the reader did.
type Event struct {
	At     time.Time
	Action string
	// Palette marks an action run from the palette rather than its key.
	Palette bool
	// Motion marks a movement key (a line down): detectors see it, since a long run of them is
	// what a Repeat detector watches for, but the ledger does not count it as a command run.
	Motion bool
}

// Press records a key press of any kind, which clears a showing tip (any key dismisses one) and
// restarts the idle clock.
func (c *Coach[S]) Press(at time.Time) {
	c.showing = nil
	c.lastKey = at
}

// Observe records an action that ran: the ledger counts it, a key press for it marks taken any tip
// that pointed at it (a palette run does not, since the tip may be teaching the key), and detectors
// that see a long way round queue a tip.
func (c *Coach[S]) Observe(ev Event) {
	c.Press(ev.At)

	if !ev.Motion {
		c.ledger.Ran(ev.Action, ev.At)
	}

	if !ev.Palette && !ev.Motion {
		c.ledger.Took(ev.Action)
	}

	for _, d := range c.dets {
		sg, ok := d.Observe(ev)
		if ok && !slices.ContainsFunc(c.queue, func(q Suggestion) bool { return q.ID == sg.ID }) {
			c.queue = append(c.queue, sg)
		}
	}
}

// Showing is the tip on screen, false when none is.
func (c *Coach[S]) Showing() (Suggestion, bool) {
	if c.showing == nil {
		return Suggestion{}, false
	}

	return *c.showing, true
}

// Retire stops tip id from showing again and takes it off the screen.
func (c *Coach[S]) Retire(id string) {
	c.ledger.Retire(id)

	if c.showing != nil && c.showing.ID == id {
		c.showing = nil
	}
}

// Tip is the tip to show at now, if the limits allow one. A busy screen (a text field, a picker,
// or an overlay open) holds tips back. A tip once shown stays until the next key press.
func (c *Coach[S]) Tip(now time.Time, busy bool) (Suggestion, bool) {
	if c.showing != nil {
		return *c.showing, true
	}

	if busy || !c.allowed(now) {
		return Suggestion{}, false
	}

	sg, ok := c.pick(now)
	if !ok {
		return Suggestion{}, false
	}

	sg = c.keyed(sg)
	sg.Tone = Tip
	c.ledger.Shown(sg.ID, sg.Action, now)
	c.showing, c.lastTip = &sg, now
	c.shown++

	return sg, true
}

func (c *Coach[S]) allowed(now time.Time) bool {
	switch {
	case c.ledger.TipsOff():
		return false
	case now.Sub(c.start) < c.lim.Warmup, now.Sub(c.lastKey) < c.lim.Idle:
		return false
	case !c.lastTip.IsZero() && now.Sub(c.lastTip) < c.lim.Every:
		return false
	}

	return c.shown < c.lim.PerSession
}

// pick is the first queued detector tip, then the first pool tip whose action has not been used,
// that the ledger has not retired, taken, worn out, or shown within the cooldown.
func (c *Coach[S]) pick(now time.Time) (Suggestion, bool) {
	for i, sg := range slices.Concat(c.queue, c.pool) {
		if !c.fresh(&sg, now) || (i >= len(c.queue) && c.used(sg.Action)) {
			continue
		}

		if i < len(c.queue) {
			c.queue = slices.Delete(c.queue, i, i+1)
		}

		return sg, true
	}

	return Suggestion{}, false
}

func (c *Coach[S]) fresh(sg *Suggestion, now time.Time) bool {
	m := c.ledger.tip(sg.ID)

	switch {
	case m.Retired, m.Taken, m.Shown >= c.lim.RetireAfter:
		return false
	case !m.Last.IsZero() && now.Sub(m.Last) < c.lim.Cooldown:
		return false
	}

	return true
}

// used reports whether action has run before, in the ledger or the usage log, which makes a tip
// teaching it moot.
func (c *Coach[S]) used(action string) bool {
	return c.ledger.Count(action) > 0 || (c.seen != nil && c.seen.Count(action) > 0)
}

// keyed fills the suggestion's key and word from the keymap where they are empty.
func (c *Coach[S]) keyed(sg Suggestion) Suggestion {
	if c.keymap == nil || sg.Action == "" {
		return sg
	}

	b, ok := c.keymap.Binding(sg.Action)
	if !ok {
		return sg
	}

	if sg.Key == "" {
		sg.Key = b.Label()
	}

	if sg.What == "" {
		sg.What = b.Hint().What
	}

	return sg
}

// IdleMsg arrives when a tip may have become possible, which is when a tool asks Tip again.
type IdleMsg struct{ At time.Time }

// Wake is WakeAt now.
func (c *Coach[S]) Wake() tea.Cmd { return c.WakeAt(time.Now()) }

// WakeAt is the command that sends IdleMsg at the earliest moment a tip could show, which a tool
// returns after each key press and after each IdleMsg that showed nothing. It is nil while one is
// already on its way, during the session's warmup (so a short session, a test among them, never
// arms a timer), while a tip is showing, and once no tip can come this session: tips off, the
// session's allowance spent, or nothing left to teach.
func (c *Coach[S]) WakeAt(now time.Time) tea.Cmd {
	switch {
	case c.armed.After(now), c.showing != nil, now.Sub(c.start) < c.lim.Warmup:
		return nil
	case c.ledger.TipsOff(), c.shown >= c.lim.PerSession, len(c.queue)+len(c.pool) == 0:
		return nil
	}

	due := c.lastKey.Add(c.lim.Idle)
	if !c.lastTip.IsZero() {
		due = latest(due, c.lastTip.Add(c.lim.Every))
	}

	c.armed = latest(due, now)

	return tea.Tick(c.armed.Sub(now), func(t time.Time) tea.Msg { return IdleMsg{At: t} })
}

func latest(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}

	return b
}
