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

| Package | Source |
| --- | --- |
| `forge` | gh-repo-dashboard `internal/github`, GitHub through the `gh` CLI |
| `vcs` | gh-repo-dashboard `internal/vcs`, git and jj behind one interface |
| `codeintel` | wavez `internal/codeintel`, per-project SQLite store of symbols, edges, FTS, and line-to-test coverage |

## Consumers

- [gh-repo-dashboard](https://github.com/KyleKing/gh-repo-dashboard)
- [second-look](https://github.com/KyleKing/second-look)

## Local development

Consumers use a `go.work` pointing at a sibling checkout, so nothing has to be published
to iterate across repos. `go.work` is gitignored.

```sh
cd second-look && go work init . ../aragonite
```

[docs/gh-repo-dashboard-cutover.patch](docs/gh-repo-dashboard-cutover.patch) is the
`internal/cache` cutover, verified green against gh-repo-dashboard's full suite in a
throwaway worktree. Apply it when the extraction lands.
