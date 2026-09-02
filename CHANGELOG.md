## v0.10.0 (2026-09-02)

### Feat

- **tui**: center an overlay as one block rather than line by line

## v0.9.0 (2026-09-01)

### Feat

- **tui**: share the help legend and the faces a screen draws with

## v0.8.0 (2026-09-01)

### Feat

- **display**: render a relative time narrow enough for a table column

## v0.7.0 (2026-09-01)

### Feat

- **vcs**: name a repository's remote branches and its default branch
- **keyhint**: draw a screen's keys one way

## v0.6.0 (2026-09-01)

### Feat

- **forge**: report whether a workflow run finished and passed

## v0.5.1 (2026-09-01)

### Refactor

- **forge**: reduce a run listing to current state per branch

## v0.5.0 (2026-09-01)

### Feat

- **forge**: read Actions runs, jobs, and step timings through gh

## v0.4.1 (2026-09-01)

### Fix

- **forge**: pass search terms after a separator so a negated qualifier survives

## v0.4.0 (2026-09-01)

### Feat

- **forge**: name the repository a pull request read is for

## v0.3.0 (2026-09-01)

### Feat

- **ghcassette**: record and replay gh subprocess calls

## v0.2.1 (2026-08-31)

### Fix

- **forge**: read the cumulative pull request diff, not a patch series

## v0.2.0 (2026-08-29)

### Feat

- **display**: render forge and vcs models as plain text
- **tui**: add the markdown body renderer
- **tui**: add Catppuccin palettes with terminal-background detection
- **tui**: add the expandable region renderer
- **tui**: add the table column-fitting engine

### Fix

- **ci**: serialize Bump Version so duplicate runs cannot race on the tag

## v0.1.1 (2026-08-27)

### Fix

- **lint**: drop nolint:gocritic directives gocritic no longer flags

## v0.1.0 (2026-08-27)

### Feat

- **forge,vcs**: read a pull request by number and guard its checkout
- **forge**: move the gh CLI wrapper out of gh-repo-dashboard
- **vcs**: name the checkout's directory from its summary
- **vcs**: move git and jj behind one interface out of gh-repo-dashboard
- **transport**: add a test seam and mutation guard for HTTP clients
- **forge**: move the pull request model out of gh-repo-dashboard
- **cache**: extract the TTL and disk cache from gh-repo-dashboard

### Refactor

- **forge**: leave the pull request model as data only
