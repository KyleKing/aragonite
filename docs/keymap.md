# Keymap v2: one meaning per key across jojo, jj-diff and second-look

The canonical spec for the family keymap. `tui/keys` implements the aragonite half (§3.4) and its
tests enforce the principles below; each tool's `TestKeymap_AgreesWithShared` enforces the rest
through `keys.Disagreements` and `keys.RequireReserved`. Decided 2026-11-25; §6 records the
decisions and the options.

Baseline audited: jojo `0e1ffec` (`internal/components/help/bindings.go`, `keymap.go`, `internal/model/chrome.go`,
`files.go`), jj-diff `03ed384` (`internal/model/keymap.go`), second-look `1c79a11` (`internal/tui/keymap.go`, `card.go`,
`view.go`, `listview.go`, `model.go`, `cmd/second-look/{inbox,threads,reviews}screen.go`, `run.go`), aragonite `5d91098`
(`tui/keys`: `Global`, `DiffView`, `SplitColumns`, `ConflictPicks`, `Flags`, `Objects`).
Prior resolution: the v1 keymap spec (2026-11-24) and the later proposal `key-consistency.md` (dogfood finding 21),
neither kept in a repository; v2 supersedes both. "The spec" below means v1.
Tags: **J** jojo, **D** jj-diff, **S** second-look, **A** aragonite. `␣` Space, `⏎` Enter.

## 0. Was this already resolved?

Partly. The spec resolved jojo and jj-diff against aragonite, and both now dispatch from `tui/keys` with
`TestKeymap_AgreesWithShared` guarding them. It did not resolve the three keys you asked about, for two reasons:

1. **The spec exempted second-look's review verbs** (principle 11: "second-look's review verbs are untouched"),
   and `a` (add comment) and `S` (submit) are review verbs. Principle 1 only forbids two meanings *on one
   screen*; two tools never share a screen, so `a` = abandon / add-comment passed it by construction.
2. **The four second-look changes the spec did make (§3.3) never shipped.** second-look does not merge
   aragonite's table at all (it imports `tui/keys` only for the palette), so no test caught the drift:
   `u` is still the look chord, `v` still cycles the view, `ctrl+r` still restages, `y`/`Y` still scroll.

`key-consistency.md` (yesterday's later session) proposed fixes but landed nothing; it also proposed keeping
second-look's `S` as a prefix, which leaves `S` meaning split in two tools and submit in the third. v2 replaces
that recommendation.

## 1. Principle amendments

Principles 2–10 and 12 of the spec stand. Changes:

- **P1 (kept) + P1b (new): family-reserved keys.** Some keys carry one meaning across the family even where a
  tool lacks the concept: the tool leaves them unbound rather than reuse them. The list is the keys whose
  family meaning writes, publishes, or is a gesture every screen shares:
  `u` undo · `ctrl+r` redo · `ctrl+l` reload · `␣` tick · `v`/`V` visual range · `Y` yank · `K` inspect ·
  `a` abandon · `S` split · `P` publish · `i` insert prose · prefixes `g z ] [ \ - b`.
  aragonite enforces it (`keys.Reserved()`, checked by `Disagreements` regardless of context overlap).
  Keys outside the list may still mean different things in different tools (P1 as before), which is how
  `R`, `e`, `x`, `d`, `c`, `s` stay as they are (§2, class 1).
- **P7 (sharpened): `␣` is one concept, "tick the item at point".** A tick is a per-item mark drawn in the
  gutter, toggled by `␣`, cleared or kept by the screen's own rule, and it **never rewrites code or posts
  content**: verbs act on ticks, ticks do not act. Each pane has exactly one tick kind, and the footer names it:
  *check* (jojo log, bookmark list, file list), *select* (jj-diff `-i`, split columns), *read* (diff views,
  outline, second-look queue). Two panes on one screen may tick differently (jojo log + dock) because focus is
  drawn and the footer word changes with it. Consequence: jojo's file-list `␣` = stage, which writes (moves the
  file into the index change), leaves `␣` (§3 J-1).
- **P11 (replaced): second-look's review verbs join the family.** They keep their keys except where a reserved
  key forces a move. They gain one namespace each: **`i` insert prose** (comment, reply, PR comment),
  **`P` publish** (submit as approve / request changes / comment, post one), **`b` branch** (list, delete).
- **P13 (new): `P` publishes, and never on one key.** Everything that leaves the machine sits on `P` and
  shows what will go out before it goes: jojo push (dry-run table, `y`), jojo outline submit (plan, `⏎`),
  second-look publish (page names the event, the second key sends or opens the existing confirm).
- **P14 (new): esc backs out and never quits.** Esc closes the innermost thing that is open: a prompt, a
  modal, a pending chord, a tick set, a nested screen. At a root screen it does nothing except say `q quits`.
  `q` quits a root, and `ctrl+c` quits from anywhere. Repeated esc is therefore always safe to press.
- **P15 (new): one text-entry contract.** Every field submits on `⏎`, inserts a newline on `ctrl+j`
  (`alt+⏎` too, where the terminal reports it), and cancels on esc. The footer under an open field says so in
  those words: `⏎ save · ctrl+j new line · esc cancel`. A field opens with the cursor at the end of any text it
  already holds. No field sends anything off the machine on `⏎`; that stays behind `P` (P13).

  **The vim layer.** aragonite `tui/vimtext` is the field every tool builds on, a vim layer over this contract. A
  field opens in insert mode, so someone who does not use vim just types, with a badge in the footer
  naming the mode (`INSERT`, `NORMAL`, `VISUAL`, `V-LINE`) and the footer's keys following it:
  `⏎ save · ctrl+j new line · esc normal` in insert mode, `⏎ save · i insert · esc cancel` in normal mode.
  `⏎` saves in every mode, and `o`/`O` open lines in normal mode. Esc backs out one level at a time, so P14
  holds inside the field: insert goes to normal (the cursor steps back one, as in vim), a selection or a
  half-typed operator or count goes back to plain normal mode, and only esc in plain normal mode cancels
  the field, handing the caller the text to keep as a draft or drop. Inside a field vim's keys win over
  the family's reservations, since the field has the keyboard: `u` and `ctrl+r` are the field's own undo
  and redo, `v`/`V` its own visual modes, and `Y` is `y$` as in nvim, not yank-to-clipboard (`WithClipboard`
  copies every yank over OSC 52 instead). `ctrl+e` hands the text to `$EDITOR` and takes it back, keeping
  the draft if the editor fails; `ctrl+c` is never the field's, so it still quits. A tool or user setting
  can turn the layer off (`WithVim(false)`): the same `⏎`, `ctrl+j`, and esc, with esc cancelling from the
  start, which makes the footer `⏎ save · ctrl+j new line · esc cancel`. A one-line field (a bookmark name)
  has no line break, and `j`, `k`, `o`, and `O` do nothing in it.
- **P16 (new): a destructive write confirms only when content would be lost.** `a` abandon (jojo) and `R`
  discard (jj-diff) act on one key when nothing goes with them, such as an empty change or a no-op discard.
  When content would go, they ask on one line, naming what goes and how to get it back:
  `abandon qpvx and its 3 changed files?  y abandon · n keep · u undoes it`. Only `y` acts. Any other key
  cancels and is not replayed. `u` then means `jj undo` of that write in every tool: jj-diff's `u` restores
  the operation recorded before its last write, as jojo's does.

## 2. Audit: every key whose meaning differs

Class **1** = allowed by P1 (different concepts that never share a screen, and no reserved key involved).
Class **2** = violation the spec missed (mostly second-look's review verbs and list screens).
Class **3** = drift since the spec (the spec decided it, the code did not follow).

| Key | J | D | S | A | Class | v2 |
|---|---|---|---|---|---|---|
| `a` | abandon (log, immediate write) | take every side (conflict viewer, in-memory) | add comment chord (review) | — | **2** (S) / 1 (D) | S → `i`; D keeps (modal, in-memory, page drawn) |
| `S` | split in jj-diff (log) | split selection (immediate) | submit chord (review) | Flags `VerbSplit` | **2** | S submit → `P` |
| `␣` | check (log, bookmarks); read (dock, outline); **stage** (files, writes) | select (`-i`, columns); unbound in browse | read (review, queue); "done" (notification) | `read` (View) | 1 under P7, except **3** J files (stage writes) and **3** D browse (spec said read) | §3 J-1, D-1 |
| `u` | undo | undo (not browse) | look chord `u g/i/s/w/v` (review); next row needing a look (lists) | undo | **3** (review, spec §3.3) / **2** (lists) | S review → `\`, S lists → `]u`/`[u` |
| `U` | only unread | only unread | only new (review); previous row needing a look (lists) | only-unread | **2** (lists) | S lists → `[u` |
| `v` | unbound (spec: range-check, not built) | visual (`-i`) | next view preset | visual | **3** | S `v` → `\\`, `v` = visual |
| `ctrl+r` | redo | redo | restage (review); refresh (all 3 lists) | redo | **3** | S → `ctrl+l` |
| `y` / `Y` | duplicate / yank | — | scroll a line up / down | `Y` yank | **3** | S drops `y`/`Y` scroll; `Y` = yank `path:line` |
| `P` | push (log, dry-run); submit stack (outline, plan) | — | post one ready comment **immediately** | — | **2** (no confirm; submit lives on `S`) | `P` = publish page |
| `A` | absorb | — | ask the agent (review); **approve** (inbox, `AA`) | — | **2** inside S (one key, two concepts) | inbox approve → `P a` |
| `r` | rebase picker | — | "ready" verdict (review); **reply** (queue) | — | **2** inside S | queue reply → `i` |
| `m` | reserved (basket) | — | state chord (review); **PR comment in $EDITOR** (inbox) | — | **2** inside S | inbox comment → `i` |
| `e` | `jj edit` (moves @) | edit region (conflict) | edit / reply / write prose | `edit-region` | 1 (shared word "edit"); habit hazard noted §5 | reply leaves `e` for `i`; `e` = edit what exists |
| `t` | reserved | — | **todo verdict** on a thread/comment row, **threads toggle** elsewhere, same screen | — | **2** (two concepts on one screen, cursor-row focus is not drawn) | threads → `\t` |
| `c` | commit (log); commit staged (files) | confirm (select, columns, conflict) | comments toggle | — | 1 | S toggle → `\c` (rides with `\t`); `c` keeps J/D meanings |
| `B` | bisect | — | branches and CI | — | **2** (same concept as J `bb`/`gh`, different key) | S → `bb` |
| `D` | describe in $EDITOR | describe column in $EDITOR | **delete the PR's branch** (remote, confirm) | `describe-editor` | **2** (concept = J `bd`) | S → `bd` |
| `i` | reserved | — | PR info card | — | 1 today; S needs `i` for insert | S info → `gi` |
| `R` | restore picker | discard selection (immediate); reset region | resolve verdict (staged); resolve / thumbs-up (queue, immediate) | `reset-region` | 1 (restore vs resolve never meet; no shared intent) | keep; not reserved (D3 below) |
| `s` | squash picker | squash selection → picker | suggest (review); cycle row order (inbox) | Flags `VerbSquash` | 1 | keep (optional O-1: suggestion into the `i` page) |
| `x` | resolve conflict | `]x` object | skip verdict | `ObjConflict` | 1 | keep |
| `d` | describe | describe column | draft state (review); discard review `dd` (reviews) | `describe` | 1 | keep |
| `E` | diffedit | — | edit a note | — | 1 | keep |
| `F` | sync | — | FIXME into the source (writes worktree) | — | 1 (hazard: a jj habit "sync" pressed in S writes a marker) | keep, note |
| `o` | op log | — | on GitHub | — | 1 (spec-sanctioned) | keep |
| `p` | dock (log); before/after (picker) | — | tour | — | 1 | keep |
| `l` | file list | — | labels (review, inbox); callees (call card) | `side-new` (Diff) | 1 today; **latent 2** once S merges `DiffView` | defer: review labels → `gi` card or `:labels` when S adopts `DiffView` |
| `M` | — | merge tool (conflict) | merge PR (confirm) | `merge-tool` | 1 | keep |
| `L` / `X` / `O` / `T` / `C` / `H` / `I` / `W` | revset / converge / – / – / – / rounds / outline / – | | lockfile / trouble / order / agent / checkout / since a round / outline / no code changed | | 1 | keep |
| `]` `[` bare | prefix | prefix | prefix (review); **switch queue** (lists) | prefix | **2** (P3: bare brackets never act) | S lists → `]q`/`[q`; `1/2/3` stay |
| `z` bare | prefix | prefix | prefix (review); **hide finished lane** (lists) | prefix | **2** minor (P6) | O-3 optional: `za` on the lane heading |
| `gc` | — | — | call graph (review); **clean finished reviews** (reviews list) | `calls` | **2** minor (`g` is go-to, not verbs) | O-3 optional: `:clean` |
| `tab` | next pane | next pane | next decision | switch-pane | 1 (spec exception) | keep |
| `ctrl+o` | jump back | — | jump back; expand tool calls (agent pane) | jump-back | 1 (pane-local) | keep |

Count: 14 class-2 rows (11 in second-look, all outside the spec's scope), 6 class-3 rows (4 second-look §3.3,
jojo file-list `␣`, jj-diff browse `␣`), the rest class 1.

## 3. Changes

### 3.1 second-look (the bulk; every row moves the less-established binding)

| # | Old → New | Screen | Rationale | Migration |
|---|---|---|---|---|
| S-1 | `a` + severity → **`i`** + severity (`ib im in it i?`) | review | `a` is jj's abandon in J, the ecosystem's convention (jjui, lazyjj); a reading habit pressed in jojo's log abandons. `i` is free in all three tools and keeps S's vim grammar (`i` insert, as `a` was append). Same chord page, same severities. | `a` answers "a moved to i (insert a comment)" for one minor release via `keys.Moved`, then unbound (reserved) |
| S-2 | `e` on a thread (reply) → **`i`** on a thread | review | One verb creates prose at point: `i` on a code line comments, on a thread replies, on the PR comments. `e` keeps "edit what already exists" (staged comment, note, review body). | `e` on a thread keeps working one release and says "i replies now" |
| S-3 | `r` reply → **`i`** | conversations queue | `r` is the "ready" verdict on the review screen; one key, two concepts inside S | `r` says "i replies now" one release, then unbound |
| S-4 | `m` PR comment → **`i`** | inbox | `m` is the state chord on the review screen | moved notice |
| S-5 | `S` + `a/r/c` → **`P`** + `a/r/c` | review | `S` is split in J and D; submit is a publish, which P13 puts on `P` beside jojo's push and stack submit. The confirmation sheet after the event key is unchanged. | `S` says "submit moved to P" one release, then unbound (reserved split) |
| S-6 | `P` post one → **`P p`** | review | `P` today posts a ready comment on one key with no confirm, the only network write in the family that does. The page names what goes out; `p` posts it. | `P` alone now opens the page, so the old habit lands one key short of sending: safe by construction |
| S-7 | `A`/`AA` approve → **`P a`** (then the existing confirm) | inbox | `A` is ask-the-agent on the review screen; approve is the same act as `P a` there | `A` says "approve moved to P a" one release |
| S-8 | `B` branches and CI → **`b b`** | review | J's `bb` is the bookmark list; same concept (the branches) | moved notice on `B`, then unbound (J `B` = bisect; S has no bisect, so stays unbound) |
| S-9 | `D` delete branch → **`b d`** (confirm kept) | review | J's `bd` deletes a bookmark; J's `D` is describe-in-editor. Frees an irreversible remote delete from a bare uppercase key | moved notice |
| S-10 | `i` info → **`g i`** | review | `i` is now insert | `i` immediately has its new meaning (S-1); `gi` listed on the `g` page |
| S-11 | `t` threads toggle → **`\t`**; `c` comments toggle → **`\c`** | review | `t` was todo on a thread row and threads elsewhere on one screen. "What is shown" joins the `\` page that S-12 creates | moved notice on bare `t` only off thread rows |
| S-12 | `u g/i/s/w/v` → **`\ g/i/s/w/\`**; `v` next view → **`\\`**; `v`/`V` = visual range | review | spec §3.3, never shipped (class 3) | `u` says "looks moved to \ ; u is undo"; `v` starts visual (V behaves as before) |
| S-13 | `ctrl+r` restage / refresh → **`ctrl+l`** | all screens | spec §3.3 (class 3). Also the three list screens, which the spec missed | `ctrl+r` stays an alias one release with a notice (S has no redo yet) |
| S-14 | `y`/`Y` scroll → dropped; **`Y`** = yank `path:line` | review, lists | spec §3.3 (class 3) | scrolling stays on `ctrl+e/y`, `shift+↑/↓` |
| S-15 | `u`/`U` next/prev row needing a look → **`]u`/`[u`** (`n`/`N` repeat); bare `]`/`[` queue → **`]q`/`[q`** | inbox, queue, reviews | P3 and the family's unread object | moved notices |
| S-16 | merge a `[]keys.Binding` mirror of `defaultKeyMap()` and run `keys.Disagreements` and `keys.RequireReserved` in `TestKeymap_AgreesWithShared` | — | root cause of every class-3 row: S is the one tool no test checks | — |

Result inside S: `r` = ready, `t` = todo, `x` = skip, `R` = resolve, `m` = state, `A` = ask, `i` = insert prose,
`e` = edit prose, `P` = publish, `b` = branch. Every review verb has one key on every S screen.

### 3.2 jojo

| # | Old → New | Rationale | Migration |
|---|---|---|---|
| J-1 | file list `␣` stage → **`␣` check** (`keys.ActionCheck`, the log's tick widened to `List`); `c` commits the checked files (`jj commit <paths> -m`), and `s`/`S` squash and split the checked files (the file under the cursor when none is checked). The staging index is retired (D3), so nothing on the file list writes on `␣` | P7: a tick never writes. This is the v1 spec's row "files `␣` stage, `C` commit staged → `␣` check, `c` → `jj commit <paths>`", which did not land (class 3). Decision D3 | `Deprecated["file-stage"] = "check"` with the footer warning jojo already prints; `Deprecated["file-commit"]` stays `c` |
| J-2 | reserve `i` (insert), `P` publish meaning; no default change | P1b | none |

Nothing on jojo's log, dock, or outline footer changes: `a`, `S`, `P`, `␣` already mean what v2 says.

### 3.3 jj-diff

| # | Old → New | Rationale | Migration |
|---|---|---|---|
| D-1 | browse `␣` unbound → **`␣` read** (mark the hunk read in the shared marks) | spec §2.5 put read on `␣` in D (class 3), and aragonite `seen.OpenJJ` (aragonite `97528fe`, S `2202229`) now makes the marks shared, so jj-diff no longer needs to be read-only. `-i` keeps `␣` = select and never marks read there (one tick kind per pane). Decision D2 | none (new binding) |
| D-2 | none else. `a` take-all stays: the conflict viewer is modal, draws `a all` in its footer, and its picks are in memory until `c` | class 1 | — |

### 3.4 aragonite `tui/keys`

| # | Change | API |
|---|---|---|
| A-1 | One tick on `␣`, three kinds: read (View), select (Select, Split), check (Log, List), all in group `GroupMark` with the kind as the footer word. `DiffView` carries the read tick; a tool adds the other kinds it has and drops its own `␣` binding | `TickKind`, `TickRead`/`TickSelect`/`TickCheck`, `Tick(kind)`, `Ticks()`, `IsTick`, `ActionSelect`, `ActionCheck`, `GroupMark` |
| A-2 | The P1b list as data; `Disagreements` reports a tool binding a reserved key to another action **regardless of context overlap**, and `RequireReserved` checks a whole merged keymap | `Reservation{Keys, Meaning, Actions, Under}`, `Reserved()`, `Unreserved(bindings)`, `RequireReserved(t, km)` |
| A-3 | A moved key answers "`<old>` moved to `<new>` (`<word>`)" for one release, naming the new key as the keymap binds it (so a rebind is what the notice teaches). It is hidden from the legend, the which-key panel, and the palette, and exempt from the reservations | `Moves` (old sequence → action), `Moves.Bindings()`, `Moved(seq, action)`, `MovedAction`, `IsMoved`, `Keymap.Move(action)`, `Keymap.WithMoves(gate)`, `GroupMoved`, `Binding.To` |
| A-4 | The family verbs on reserved keys, as one opt-in table, and the queue object for list screens | `Verbs()`: `insert` (`i`), `publish` (`P`), `branches` (`b b`), `branch-delete` (`b d`), `info` (`g i`); `ObjQueue` (`]q`/`[q`, `Contexts: List`), `Object.Contexts` |

`Reserved()` admits these actions on each key, and nothing else on any screen:

| Key | Meaning | Actions allowed | Chords under it |
|---|---|---|---|
| `u` | undo | `undo` | no |
| `ctrl+r` | redo | `redo` | no |
| `ctrl+l` | reload | `reload` | no |
| `␣` | tick | `read`, `select`, `check` | no |
| `v`, `V` | visual range | `visual` | no |
| `Y` | yank | `yank` | no |
| `K` | inspect | `inspect` | no |
| `a` | abandon | `abandon`, `take-all` (jj-diff's conflict viewer, D-2) | no |
| `S` | split | `split` | no |
| `P` | publish | `publish`, `push`, `submit` (jojo's, which already open their page) | yes (`P a`) |
| `i` | insert prose | `insert` | yes (`i b`) |
| `g` `z` `]` `[` `\` `b` | prefix | none | yes |
| `-` | prefix | `context-less` (bare `-` outside the split columns) | yes |

## 4. Footers at 100 columns, before → after

Ruler: `....|....1....|....2....|....3....|....4....|....5....|....6....|....7....|....8....|....9....|....0`

**jojo** (unchanged except the file list)

```
log, before = after
parse*  @  enter diff  p dock  n new  d describe  s squash  r rebase  e edit  a abandon  …  ? help
dock, before = after
@  ]h hunk  space read  ]u unread  ]f file  U unread  n next  ]r round  H rounds  \s sbs  esc back
outline, before = after
@  ]] chapter  [[ previous chapter  enter dock  space read  ]u unread  P submit  ]h hunk  …  ? help
file list, before
enter diff  space stage  s squash  R restore  c commit  x resolve  esc close
file list, after (J-1)
enter diff  space check  s squash…  c commit checked  R restore  x resolve  esc close
```

**jj-diff** (browse gains `␣`; the rest unchanged)

```
browse, before
  Browse  @   j/k scroll   ]h hunk   ]u unread   U hide read   ]f file   w ws   \s sbs   ? help
browse, after (D-1)
  Browse  @   j/k scroll   ]h hunk   space read   ]u unread   U hide read   ]f file   ? help
-i, before = after
  Interactive  ▸ next  nothing selected yet · space select   v lines   s squash   S split   ? help
resolve, before = after
  Resolve  @   1–9 side   0 base   a all   c confirm   ]x conflict   u undo   ? help
```

**second-look**

```
review, before
 j/k line  ] go to  v code  m mark  e write  a add  i info  S submit  ? help  esc queue  q quit
review, after
 j/k line  ] go to  \\ code  m mark  e write  i comment  P publish  ? help  esc queue  q quit
review with a checkout and an agent, before
 j/k line  ] go to  v code  m mark  e write  a add  i info  B branches  T agent  S submit  ? help
review with a checkout and an agent, after
 j/k line  ] go to  \\ code  m mark  e write  i comment  bb branches  T agent  P publish  ? help
review, on a thread row, before / after (card foot)
   e reply   r ready   t todo   x skip   R resolve   , react   za fold
   i reply   r ready   t todo   x skip   R resolve   , react   za fold
chord pages, before
   a: b blocker  m major  n minor  t nit  ? question      S: a approve  r request changes  c comment
chord pages, after
   i: b blocker  m major  n minor  t nit  ? question      P: a approve  r request changes  c comment
                                                             p post this comment now
conversations queue, before
  enter read  space mark read  r reply  R resolve  o GitHub  tab group  / filter  ? help  q quit
conversations queue, after
  enter read  space read  i reply  R resolve  o GitHub  tab group  ]u unread  / filter  ? help
inbox, before
  enter review   C check out   m comment   A approve   M merge   l labels   o GitHub   ? help
inbox, after
  enter review   C check out   i comment   Pa approve   M merge   l labels   o GitHub   ? help
reviews, before
  enter open   C checkout   d discard   gc clean   ctrl+r refresh   ? help
reviews, after
  enter open   C checkout   d discard   gc clean   ctrl+l refresh   ? help      (O-3: gc → :clean)
```

Side by side for the three keys you named, after v2:

| Key | jojo | jj-diff | second-look |
|---|---|---|---|
| `a` | abandon | (conflict viewer only) take every side | unbound, reserved |
| `S` | split, in jj-diff | split the selection | unbound, reserved |
| `␣` | tick: check (log, files), read (dock, outline) | tick: select (`-i`, columns), read (browse) | tick: read (review, queue) |
| `P` | push / submit stack | — | publish page (approve, request changes, comment, post one) |
| `i` | reserved | — | insert prose: comment, reply, PR comment |

## 5. What v2 deliberately leaves different, and why

- `R` restore (J, D) vs resolve (S). Both are each tool's strongest mnemonic, the concepts share no intent (no one
  restores in a review or resolves a thread in jj), and S's review `R` only stages. Not reserved.
- `e` `jj edit` (J) vs edit prose (S). Shared word, and `jj edit` is undoable and visible on the graph. The habit
  hazard (S's most frequent key pressed with jojo's log focused moves `@`) is the residual risk of v2; it is
  smaller than `a`'s and moving jj's `e` costs the jj mnemonic.
- `c` commit (J) / confirm (D) / comments view (S, until S-11 moves it to `\c`). After S-11 bare `c` is unbound
  in S's review; it is not reserved, so a later S feature may not take it without revisiting.
- `s` squash (J, D) vs suggest (S). Optional O-1 folds suggest into the `i` page (`i s`) and reserves `s`.
- `a` take-all in jj-diff's conflict viewer: modal, in-memory, drawn in its footer.

## 6. Decisions

Decided by the maintainer on 2026-11-25:

- **D1. The comment key is `i`.** `i` + severity comments (`ib im in it i?`), `i` on a thread replies, and `i`
  comments on the pull request from the inbox. second-look's PR info card moves to `gi`. `c` was the
  review-ecosystem mnemonic (Gerrit) but is jojo's commit and jj-diff's confirm, so it would have traded one
  cross-tool meaning for another.
- **D2. jj-diff writes read marks (D-1).** Browse `␣` marks the hunk read in the shared store
  (`seen.OpenJJ`); `-i` and the split columns keep `␣` = select and never mark read there.
- **D3. jojo's staging index is retired (J-1).** `␣` checks files on the file list and never writes; `s` and
  `S` act on the checked files, and `c` commits them (`jj commit <paths>`).

The two options, decided with D1–D3 by the doc's own lean:

- **O-1 is declined for now.** Suggest stays on `s` in second-look and `s` is not reserved (§5); folding
  suggest into the `i` page (`i s`) can come later without moving anything else.
- **O-3 is taken.** The reservations force it: a bare `z` is a prefix acting alone (P6), which
  `keys.Unreserved` fails, and `gc` on the reviews list is the shared `gc` call graph on a zero context, which
  `keys.Disagreements` fails. So the lane toggle becomes `za` on the lane heading and clean becomes `:clean`.

## 7. Order of work

1. aragonite: A-1..A-4 (one minor). This turns every remaining disagreement into a failing test downstream.
2. second-look: S-16 first (the test goes red), then S-1..S-15 in one release with `keys.Moved` notices; README
   keys table, `helpTree`, welcome card, and `demo/*.tape` recordings in the same change (the tapes press `a`
   and `S`).
3. jj-diff: D-1 (after D2).
4. jojo: J-1 (after D3).
5. Next minor of each: drop the `Moved` notices; the keys become unbound and reserved.
