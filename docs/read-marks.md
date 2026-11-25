# Read marks

How a tool wires `seen` in. second-look was the first consumer and its wiring is the reference;
this page is written for jojo, which has the seam ready, and ends with what jj-diff could do later.
The keys are the family keymap's: `␣` marks read while viewing a diff, `]u` / `[u` step to the
next or previous unread thing, and `U` shows only what is unread.

## What the package answers

| Call | Answers |
| --- | --- |
| `seen.Parse(gitDiff)` | every hunk of `git diff` / `jj diff --git` text as `[]Ref{Path, ID, Hunk}`, in patch order |
| `seen.Hunks(d)` | the same from an already parsed `*diff.Diff` |
| `seen.Hunk(d, path, n)` | one hunk's id, rescanning the diff (use `Hunks` for all of them) |
| `store.Unread(scope, id)` | whether a reader has not read that hunk of that scope; a scope whose file cannot be read answers unread |
| `store.Mark(scope, read, live, ids...)` | marks and saves at once, merging with anything another process saved |
| `store.Set(scope)` | the scope's `*Set`, loaded once and kept; `Count(live)`, `Files(live)`, `Unread(live)` count and filter |
| `store.Reload(scope)` | forget the cached set so the next ask rereads the file (after another tool wrote it) |
| `seen.StateDir(app, parts...)` | `$XDG_STATE_HOME/<app>/<parts...>`, else `~/.local/state/...` |
| `seen.RepoKey(path)` | `<base>-<12 hex>` naming one repository under a state directory |
| `seen.JJRepo(dir)` | the jj repository store a workspace (or any directory inside one) belongs to, the path to pass `RepoKey` |
| `seen.Migrate(dir, full)` / `store.Migrate(scope)` | moves the one scope file named by a proper prefix of `full` into `full`'s file (merging if one appeared), under the save lock; for marks filed under a shortened id |

An id is the sha256 of the path, a NUL, and each line as kind byte, text, and newline, which is
exactly what jojo's `internal/readmark.Hunks` already computes, so swapping it for `seen.Parse`
changes no id. `seen`'s tests pin that against marks second-look v0.57.3 wrote.

## jojo

### The store

Open one `*seen.Store` in `cmd/jojo/main.go` and pass it as `model.Options.Reads`. A `*Store`
satisfies `model.ReadState` as it is, because `Unread(scope, hunkID string) bool` is the seam's
signature with plain strings.

```go
dir, err := seen.StateDir("jojo", "seen", seen.RepoKey(repoStore))
if err != nil { /* run without marks: leave Reads nil, noReads answers */ }
reads := seen.Open(dir)
model.New(ctx, client, skin, root, *revset, model.Options{Reads: reads, ...})
```

`repoStore` is the repository's store rather than the workspace root, since every workspace of one
repository shares its change ids and so should share marks. For the main workspace that is
`<root>/.jj/repo`, a directory; in a secondary workspace `.jj/repo` is a file holding the path to
the main one. Read it when it is a file and use the directory otherwise.

The scope is the change id. A mark then survives every rebase, describe, or evolution that leaves
the hunk's text alone, and a hunk whose text changed comes back unread. Text moved to another
change (a squash, a split) is filed under that change and reads as unread there; that is the
honest answer, because nobody read it as part of that change.

Writing needs one more method than the seam has. Declare it where it is consumed, next to
`ReadState`, and keep `noReads` satisfying both:

```go
// ReadMarker records marks. *seen.Store implements it.
type ReadMarker interface {
	Mark(changeID string, read bool, live []seen.Ref, ids ...seen.ID) error
}
```

`Mark` writes a file, so call it from a `tea.Cmd` (copy the change id, the live refs, and the ids
before returning the closure) and send back a message with the error and what to say. `Set` and
`Store` are safe to read from `View` while a command saves.

### Hunks

Replace `readmark.Hunks(text)` with `seen.Parse([]byte(text))` in `loadDock` and keep
`readmark.Strip` for drawing. `dockLoadedMsg.hunks` becomes `[]seen.Ref`; the dock needs the map
from a line's `(path, hunk)` to its id, which `Ref.Path` and `Ref.Hunk` give (`diff.Line.Hunk`
numbers hunks across the whole diff from 1, the same as `Ref.Hunk`).

Always pass the change's whole hunk list as `live`, never a filtered one. A save drops marks for
hunks `live` does not carry, so passing the `U` view or a file filter would erase the marks of
everything hidden.

### `␣` in the dock

- On a hunk: `read := reads.Unread(change, id)` and `Mark(change, read, live, id)`, so the key
  toggles. Say `hunk read` / `hunk unread` in the status line
- On a file heading: every id of that file. Mark them all read unless all are already read, then
  all unread (second-look's rule, `allRead`)
- In the outline (`I`), `␣` on a chapter marks the whole change. It needs the change's hunks; when
  the dock has not read that change yet, read its diff first in the same command

After the save lands, refresh the strip with `noteRead(change, hunks)` so the outline and any
footer count move at once.

### `]u` / `[u`

- In the dock: the next or previous hunk heading after the cursor whose id `Unread` answers true,
  through the shared `keys.Motion` so `n` / `N` repeat it. At the end say `no unread hunk below`
  rather than wrapping silently
- In the log and the outline: the next or previous revision whose strip has `Read < Total`. A strip
  is only known once its change's diff was read, so read the stack's diffs in the background when
  the outline opens (`seen.Parse` over a 21,000-line diff takes about 6 ms, and `Hunks` over an
  already parsed one about 2 ms) and fill `strips` as each lands. A revision whose strip is still
  unknown is stepped over, and the status line says how many were not counted yet

### `U`

The dock draws only `set.Unread(live)`, or equivalently the hunks `Unread` answers true for. A hunk
marked read while `U` is on stays drawn until the cursor leaves it, so a mark does not yank the
line out from under the reader; the next redraw after a motion drops it. With nothing unread, show
the empty card (`tui/empty`) saying the change is read, with `U` to show everything.

### Counts

`noteRead` already sums `reads.Unread` into a `readmark.Strip`. With a `*seen.Store` in hand the
same numbers come from `set.Count(live)` and `len(live)`, and `set.Files(live)` gives per-file
`Read`/`Total` for the dock's file list in one pass.

### Another tool wrote

When the screen comes back from a hand-off (`handoff.DoneMsg`, jj-diff run on a change), call
`reads.Reload(change)` for the change it touched and recompute its strip, since jj-diff may have
saved marks. A save from either side merges, so neither loses the other's marks either way.

### Tests

`seen.Open(t.TempDir())` is a real store with nothing in it. To test a mark surviving a rebase, mark
a hunk, rewrite the change so the hunk shifts lines but keeps its text, and assert `Unread` is still
false; change one character of it and assert true.

## jj-diff

jj-diff shows the same marks, read-only, on the revision it views (`-r`, browse and `-i`). It
opens the directory jojo uses, `seen.StateDir("jojo", "seen", seen.RepoKey(repoStore))` with
`repoStore` from `seen.JJRepo`, and takes the scope from the revision's full change id
(`jj log -r <rev> --no-graph -T change_id`), the form jojo files it under since v0.20.0. Both tools
call `store.Migrate(changeID)` before reading a scope, which moves marks an older jojo filed under
`change_id.shortest(8)` into the full id's file: that prefix grows once another change comes to
share it, so a file named by it is lost to a reader that recomputes the prefix. A read hunk's heading
is dimmed behind a read glyph, a file read whole carries one in the file list, `]u` / `[u` step
between unread hunks, and `U` shows only unread hunks. In browse, `␣` marks the hunk read in the
shared store (`seen.OpenJJ`, keymap v2 D-1); its editing modes keep `␣` as select and never mark
there, one tick kind per pane ([keymap.md](keymap.md)). Run as a diff editor
(`--tool jj-diff`) it sees two directories and no change id, so it has nothing to scope marks by and
shows none.
