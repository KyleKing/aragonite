# aragonite

Shared Go packages for kyleking's terminal tools. Aragonite is the mineral coral
skeletons are built from, which puts it alongside calcipy and corallium.

Extracted from [gh-repo-dashboard](https://github.com/KyleKing/gh-repo-dashboard) rather
than designed up front, so every package here has at least one real consumer.

The data packages hold data and predicates only. The rendering packages under `display`
and `tui/` take their styles as an argument and carry no vocabulary of their own, so a
consumer keeps naming its own colors and styles while sharing the layout, wrapping, and
measurement underneath. [DESIGN.md](DESIGN.md) has the layering rule in full.

## Packages

| Package | What it holds |
| --- | --- |
| `cache` | Generic TTL cache with a disk store, a registry for package-level caches, and remote-scoped keys so parallel checkouts of one remote share a read |
| `display` | `forge` and `vcs` models as plain text: relative times, status summaries, review glyphs, and the em-dash placeholder. Importing both is what keeps `forge` and `vcs` from importing each other |
| `forge` | The pull request model shared by every tool that reads a code host: `PullRequest`, its detail and preview forms, checks, and workflow runs |
| `forge/github` | GitHub through the `gh` CLI: pull requests, reviews, comments, search, workflow runs, and the caches typed on them. A second host is a sibling directory, not a rename |
| `transport` | A test seam and mutation guard for an `http.RoundTripper`-based API client: register a fake transport in tests, or get a guard that panics on a real mutating request when none is registered |
| `tui/markdown` | Markdown and raw HTML flattened to terminal lines, folding `<details>` to its summary so a bot's changelog costs one line |
| `tui/region` | The block a list opens beneath itself: a rule, label/value facts, a body, and a captioned divider |
| `tui/table` | Fits columns to an available width and pads cells to it, measuring in display cells so wide glyphs never shift a row |
| `tui/theme` | Catppuccin Latte and Macchiato palettes, terminal-background detection with a `CATPPUCCIN_THEME` override, and an eight-role semantic view over a palette |
| `vcs` | git and jj behind one interface, with the working-tree summary, branches, commits, stashes, worktrees, diff, checkout identity, and stamp |

Planned, once a second consumer needs them:

| Package | Source | Why it is shared |
| --- | --- | --- |
| `filter` | gh-repo-dashboard `internal/filters` | The predicate, query, and sort engine behind both tools' pull request lists |
| `codeintel` | wavez `internal/codeintel` | Symbols, edges, FTS, and line-to-test coverage in SQLite |

Generic TUI helpers start under `tui/`. They only earn their own module if something
that is not a git tool needs them.

## Consumers

- [gh-repo-dashboard](https://github.com/KyleKing/gh-repo-dashboard)
- [second-look](https://github.com/KyleKing/second-look)
- [gh-sweep](https://github.com/KyleKing/gh-sweep) (`transport` and `tui/theme`; see
  [docs/gh-sweep.md](docs/gh-sweep.md) for why `cache` and `forge` were not a fit)

## Local development

Consumers use a `go.work` pointing at a sibling checkout, so nothing has to be published
to iterate across repos. `go.work` is gitignored.

```sh
cd second-look && go work init . ../aragonite
```

[docs/extraction.md](docs/extraction.md) records what the first extraction taught, so the
next one costs less.
