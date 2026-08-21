# Contributing to aragonite

## Setup

Prerequisites: Go (see `go.mod`), [mise](https://mise.jdx.dev/), [hk](https://hk.jdx.dev/)

```bash
mise install
hk install --mise
mise run ci
```

## Tasks

Shared tasks live in `.config/mise/conf.d/template.toml` (managed by the copier template).
Project-specific tasks go in additional `.config/mise/conf.d/*.toml` files.

mise loads `conf.d/*.toml` files in alphabetical order, and a task defined in more
than one file resolves to whichever file loaded last. Name your project file so it
sorts after `template.toml` (`user.toml` works; `project.toml` does not, since
`p` < `t`) or a same-named task override will silently do nothing.

| Command | Description |
|---------|-------------|
| `mise run bench` | Run benchmarks |
| `mise run build` | Build packages |
| `mise run ci` | Full CI check (tests + build) |
| `mise run clean` | Clean build artifacts |
| `mise run demo` | Generate VHS demo recordings (needs [vhs](https://github.com/charmbracelet/vhs) on `PATH`; it is not pinned in `[tools]`) |
| `mise run format` | Auto-fix lint and formatting |
| `mise run hooks` | Run git hooks |
| `mise run lint` | Run linter |
| `mise run test` | Run tests with coverage |
| `mise run test:coverage-min` | Fail below the 70% coverage threshold |
| `mise run test:view-coverage` | Open the coverage report in a browser |
| `mise tasks` | List all available tasks |

## Code Guidelines

Follow [AGENTS.md](AGENTS.md) for code organization, testing patterns, and error handling.
[docs/go-best-practices.md](docs/go-best-practices.md) carries the worked examples.

Linting is configured in `.golangci.toml` with 40+ rules. Run `mise run format` to auto-fix.

## Git Workflow

Conventional commits enforced via [commitizen](https://commitizen-tools.github.io/commitizen/):

```
<type>(<scope>): <subject>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Git hooks run automatically via hk on commit and push.



## Releases

```bash
gh release create v1.0.0 --generate-notes
```


## Troubleshooting

```bash
mise install --force   # Reinstall tools
hk install --mise --force  # Reinstall hooks
go test -v -run TestName ./package  # Debug specific test
go test ./... -update  # Refresh golden fixtures, where the project has them
```

[docs/troubleshooting.md](docs/troubleshooting.md) covers the toolchain failures that
look like project bugs: an empty `GOPROXY` from a corrupt mise Go install, a
`compile` loaded from a second `GOROOT`, and golden fixtures rewritten on commit.
