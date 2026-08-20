# aragonite

Shared Go packages for kyleking's terminal tools. Aragonite is the mineral coral
skeletons are built from, which puts it alongside calcipy and corallium.

Extracted from [gh-repo-dashboard](https://github.com/KyleKing/gh-repo-dashboard) rather
than designed up front, so every package here has at least one real consumer.

## Packages

| Package | What it holds |
| --- | --- |
| `cache` | Generic TTL cache with a disk store, a registry for package-level caches, and remote-scoped keys so parallel checkouts of one remote share a read |

Planned, once a second consumer needs them:

| Package | Source | Why it is shared |
| --- | --- | --- |
| `forge` | gh-repo-dashboard `internal/github` | GitHub through the `gh` CLI |
| `vcs` | gh-repo-dashboard `internal/vcs` | git and jj behind one interface |
| `filter` | gh-repo-dashboard `internal/filters` | The predicate, query, and sort engine behind both tools' pull request lists |
| `tui/table` | gh-repo-dashboard `internal/ui/table` | Already depends on nothing but lipgloss and uniseg |
| `codeintel` | wavez `internal/codeintel` | Symbols, edges, FTS, and line-to-test coverage in SQLite |

Generic TUI helpers start under `tui/`. They only earn their own module if something
that is not a git tool needs them.

## Consumers

- [gh-repo-dashboard](https://github.com/KyleKing/gh-repo-dashboard)
- [second-look](https://github.com/KyleKing/second-look)

## Local development

Consumers use a `go.work` pointing at a sibling checkout, so nothing has to be published
to iterate across repos. `go.work` is gitignored.

```sh
cd second-look && go work init . ../aragonite
```

[docs/extraction.md](docs/extraction.md) records what the first extraction taught, so the
next one costs less.
