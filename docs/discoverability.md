# Discoverability: wiring a tool

How a tool in the family (jojo, jj-diff, second-look) adopts the shared discoverability layer.
second-look is the pilot and the reference for each step. The packages:

| Package | Answers |
| --- | --- |
| `tui/keys` (`Gate`, `Availability`, `HintsAt`, `LegendAt`, `Bound`) | What can I do here, and why not |
| `tui/palette` | How do I find the thing I half-remember (`:`), and what does this key do (`?`) |
| `tui/coach` | What should I do next, and what am I not using that would help |
| `trail` | What did that just run |
| `tui/empty` | Why is this empty, and how do I get out |
| `keyusage` (`Action`, `Gated`, `ActionStat`, `Actions`) | What has this reader learned (opt-in) |
| `fuzzy` | The scorer under the palette, usable alone for any finder |

## 1. Gates: no silent no-ops

Write one switch over the tool's own state that answers every action that can be unavailable:

```go
func (m *Model) gate(action string) keys.Availability {
    switch action {
    case "resolve":
        if !m.cursorConflicted() {
            return keys.Refuse("@ has no conflict; ]x finds one")
        }
    case "push":
        if m.bookmarkAtCursor() == "" {
            return keys.Refuse("no bookmark here; b c creates one")
        }
    }

    return keys.Allow()
}
```

A good reason says what is missing and the key that supplies it. Then:

- Footer: `km.HintsAt(m.gate, actions...)` instead of `km.Hints(...)`. `keyhint.Fit` sheds Off
  hints first and draws them flat
- Legend and palette: `palette.FromKeymap(km, ctx, m.gate, about)` (below) marks them Off with
  `Why`; `km.LegendAt(ctx, m.gate)` does the same for a plain legend
- The press: before dispatching, `if a := m.gate(action); !a.OK { m.say(label + "  " + a.Why);
  m.usage.Gated(ctx, action); return }`. The status line says why instead of nothing happening

A tool with no `keys.Keymap` (second-look today) sets `keyhint.Hint.Off` and `Why` itself from the
same switch, keyed by its own key labels.

## 2. The palette: `:` runs, `?` explains

One `*palette.Model` per screen, built once with `palette.New(skin)`.

```go
// entries are the screen's actions, taught and gated. Palette-only actions (arrange, metaedit,
// git remote, :tips, :next) are appended as Entry values with no Keys.
func (m *Model) entries() []palette.Entry {
    es := palette.FromKeymap(m.keymap, m.context(), m.gate, about)
    es = append(es, paletteOnly...)

    return palette.WithSeen(es, m.coach.Ledger(), false) // recency from the local ledger
}
```

- `about` is a `map[string]palette.About` keyed by action: the command it runs (jojo's
  `Binding.Cmd` column moves here), its flags, git-word aliases (`"squash": {Also: {"amend",
  "fixup"}}`, `"new": {Also: {"stash", "checkout"}}`, `"edit": {Also: {"checkout"}}`,
  `"describe": {Also: {"commit", "reword"}}`), prose, and related keys
- With the opt-in log on, `palette.WithSeen(es, usage.Actions(ctx), true)` adds counts
- `:` opens `m.palette.Open(palette.Run, m.entries())`; `?` opens `palette.Browse`
- While open, route every key and paste to `cmd := m.palette.Update(msg)` and nothing else
- Draw it last: `lines = m.palette.Over(lines, width)` over the body rows. It is a drawer with the
  explainer pane from 100 columns and a strip over the bottom rows below
- Handle the messages it sends:
  - `palette.RunMsg{Action, Key, Flags, From}`: dispatch the action by name (never replay a key,
    which breaks under rebinding), log `usage.Action(ctx, action, "")`, call
    `m.coach.Observe(coach.Event{Action: action, Palette: true, At: now})`, and say
    `ran squash · next time s` so the palette teaches its own way out. `Flags` carries `-u`-style
    words typed after the name, for a verb that opens a picker with them preset
  - `palette.RefusedMsg{Action, Why}`: log `usage.Gated`. The palette already shows the reason
  - Closing sends nothing; ask `m.palette.IsOpen()` after `Update` if the screen cares
- `palette.Note(skin, entry, width, now)` is the explainer as lines, for a tool that answers "what
  does this key do" in its own status area too

A tool whose legend is a `[]keyhint.Hint` tree rather than a keymap uses `palette.FromLegend(tree)`.
Each entry's `Action` is then its key sequence (`"] h"`), which the tool replays. Labels read as the
keys they name (`palette.Sequences`: `ctrl+d/u` is ctrl+d or ctrl+u, `gg/G` is g then g, or G), and
a prefix row marked Off marks its whole page Off with its reason. A tool that draws inside a margin
calls `m.palette.SetLayout(...)` on resize from the terminal's own width, since `Auto` judges the
width it is handed. second-look's `internal/tui/discover.go` is the worked example of all of it.

## 3. The coach: next step and tips

```go
m.coach = coach.New(coach.Config[State]{
    Ledger:    ledger,           // coach.OpenLedger("jojo"); Save() on quit
    Keymap:    m.keymap,         // fills Key and What from Action, so rebinds follow
    Seen:      usage.Actions(keyusage.AnyContext), // optional, when the log is on
    Rules:     rules,            // the tool's what-next table
    Detectors: []coach.Detector{
        coach.Repeat("line-down", 8, coach.Suggestion{ID: "tip.hunk-next", Action: "hunk-next",
            Say: "jumps a hunk at a time"}),
        coach.Instead([]string{"edit", "new"}, coach.Suggestion{ID: "tip.new-alone", Action: "new",
            Say: "alone already starts a change on top of the one under the cursor"}),
        coach.ViaPalette(3, keyOf),
    },
    Tips: unlearned,             // pool of Suggestion, tier 1 first
})
```

- On every key press: `m.coach.Press(now)` (any key clears a showing tip), and return
  `m.coach.WakeAt(now)`, which arms one `IdleMsg` for the earliest moment a tip could show and is
  nil during the first minute, so tests and short sessions never run a timer
- On every action that ran: `m.coach.Observe(coach.Event{Action: a, At: now})`
- On `coach.IdleMsg`: `m.coach.Tip(msg.At, busy)` where busy is any text field, picker, or
  overlay being open, then `ledger.Save()` (the idle lull is the time to write it), and return
  `m.coach.WakeAt(msg.At)` to arm the next one
- Inline layout (default): `coach.Line(skin, next, width)` in the status line while a rule holds,
  and `coach.TipLine(skin, tip, width)` in the footer's leading slot until the next key.
  `coach.Mark(hints, next)` brightens the suggested key in the footer
- Band layout (opt-in, config `coach.layout = "band"` read by `coach.ParseLayout`, or the `:next`
  palette command toggling it for the session): `coach.BandLines(skin, m.coach.All(state, gate),
  tipOrNil, width)` above the footer, taking no rows when it has nothing to say
- `:tips off` is `ledger.SetTipsOff(true)`. A tip retires itself once its action is used by key
  or after three showings

## 4. The command log

Route every external command through one `*trail.Trail` (`trail.New(0)`): `t.Run(ctx, dir, write,
argv...)`, or `r := t.Start(argv, dir, write)` then `r.Done(err, stderr)` around a runner the
tool already has. Then:

- Each write's status line is `trail.Echo("squashed into rnzx", &call, "u undo")`
- `gm` lists `t.Calls(writesOnly)` with `trail.Lines(skin, calls, width, sel)`, Enter shows
  `call.Stderr` in full, and `Y` copies `call.Shell()`
- jojo's single `!` failure buffer becomes `t.Last(func(c *trail.Call) bool { return c.Failed() })`

## 5. Empty states and first run

Where a list is empty, draw `empty.Card(skin, empty.Empty{Title, Why, Hints}, width, height)` in
place of the body. Name what is empty, why, and two or three keys out.

A tool's first launch shows `coach.Welcome`, a card of the five or so keys that start a reader off
plus a line each on `?` and `:`:

```go
m.welcome = &coach.Welcome{
    Title:    "Welcome to jj-diff",
    Lead:     "Read a change hunk by hunk, and move what you select into another change.",
    Starters: coach.StartersFrom(km, keys.ActionRead, "squash", "split", keys.NextAction(keys.ObjHunk)),
}
m.welcome.HelpKeysFrom(km)   // ? and : follow a rebind too
m.welcome.OpenIfDue(ledger)  // the ledger's first session, not yet dismissed
```

- Starters come from the keymap, so a rebind changes the card. A tool with no keymap fills
  `Starters` with `keyhint.Hint` values from its own table
- On a key press, before anything else: `if !m.welcome.Key(ledger, msg.String(), now) { return }`.
  Any key only dismisses the card, so a first guess (`a`, `S`) never runs over a screen the
  reader has not read; the help and palette keys dismiss it and open the legend or palette.
  Dismissing records `welcomed` in the ledger, so it shows once per reader (`Save()` on quit as
  ever). A ledger from before the card (an existing reader) is not greeted
- Draw it last: `lines = m.welcome.Over(skin, lines, width, height)`. Below 60x16 `Over` draws
  nothing and the tool puts `m.welcome.Line(skin, width)` in its footer's leading slot instead
- Append `coach.WelcomeEntry()` to the palette-only commands and open it on
  `RunMsg{Action: coach.WelcomeAction}`, so `:welcome` shows it again
- A test harness that drives the binary seeds `{"welcomed": "..."}` into the state directory's
  `coach.json`, or every pty test opens behind the card

## 6. Usage logging (opt-in)

Keep calling `Recorder.Key` for every command key. Add `Recorder.Action(ctx, action, key)` once an
action has run (key empty for a palette run) and `Recorder.Gated(ctx, action)` for a refused press.
Readers that predate these lines skip them.

## Per tool

**jj-diff**: drop `ActionPalette` from `unsupported`; gates from `selectionAllowed` and
`hasCurrentHunk`; `?` becomes the palette in browse mode over `km.Bound(ctx)` (its help overlay
goes); the trail wraps `internal/jj` and `writes.go`'s notice uses `trail.Echo`; the empty state
`view.go:40` becomes an `empty.Card` naming the revision and suggesting `-r @-`; jj-diff starts
recording keyusage under its own app name, or under jojo's with a `jj-diff/` context prefix when
embedded.

**jojo**: adopt `tui/keys` first (keymap.md step 3). Then `internal/components/palette` is replaced
by `tui/palette` (Run mode), `help.go`'s legend by Browse mode, `Binding.Cmd` moves into the
`About` map, `rowHints` becomes coach rules (the what-next table in the design), the `!` buffer and
statusbar echo come from the trail, and an empty revset draws an `empty.Card`.
