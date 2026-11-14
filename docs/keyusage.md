# Key usage

`keyusage` logs the command keys a tool's user presses and reads the log back against the tool's bindings. The point is to find keys nobody presses, so they can be removed or made easier to find. nvim's `utils/usage` (`~/.config/nvim/FEATURE_USAGE_DESIGN.md`) set the pattern.

## Recording

`keyusage.Open(app)` records into `$<APP>_USAGE_DIR`, else `~/Sync/.<app>/usage/` when `~/Sync` exists. With neither, it returns nil and every method does nothing. Files are named `<host>-YYYY-MM.jsonl`, one per host per month, so Syncthing never sees two writers append to one file.

Line movement (`up`, `down`, `j`, `k`) is never recorded or reported. The consumer calls `Key(ctx, msg.String())` in its own dispatch, after prompts and child panes have taken their keys. Typed text never reaches the log, and aragonite needs no key registry.

## CLI invocations

`Recorder.Invoke(root, os.Args[1:])` logs one line per run under the `cli` context: the subcommand path the tool's `Command` tree recognizes, then each flag's name (`tool pr view --web`). Positional arguments and flag values are never written, because they carry paths, repository names, and tokens. Subcommands are read until the first bare word that is not one. `Commands(root)` turns the same tree into bindings for `Keymap[keyusage.CLI]`, so `<tool> keys` reports unused subcommands and flags beside unused keys, and an undeclared flag shows up as a miss.

## Context

The same key does different things on different screens, overlays, and cursor targets. In second-look, `r` replies on the queue and gives a ready verdict on a thread, and `u` is a chord on review but jumps to the next unread on the queue. So every line carries `ctx`, and the consumer names where the key landed (`review`, `review/thread`, `review/help`, `queue/inbox`) before dispatching it. Both reading and counting are per context:

- `Keymap` maps each context to its `[]Binding` (keys as `msg.String()` spells them, what the key does, and a prefix's `Kids`). A chord's second key is matched in the prefix's own context, giving the path `m x`
- `Usage.Overloads` lists every top-level key bound to different things in two or more contexts, with each one's count. This is where muscle memory from one screen misfires on another
- `Usage.Misses` lists keys pressed where nothing is bound, including chord misses such as `a q`. A frequent miss is a key the user expects to exist

Pick context names at the level where meaning changes. A cursor target that changes what a key does (a thread under the cursor) is its own context. A cursor target that only changes what the key acts on is not.

## Surfaces

- **`<tool> keys`.** `keyusage.Write` prints the never-used bindings, the ones gone stale, the misses, the overloads, and any contexts missing from the keymap
- **Brightening.** `keyhint.Hint.Fresh` is a level from `Usage.Level(ctx, path, now)`: unused for 7 days underlines the key, for 14 days bolds it, and for 28 days or never does both. `Line`, `Help`, `Page`, and `Panel` all draw it, so the keys that need learning stand out in the place you look them up

## Not built

Compaction into per-month summaries (add it when a log grows large enough to slow `Load`), and a per-tool denylist beyond line movement (add it when a tool has a key as constant as `j`).
