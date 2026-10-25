## v0.23.0 (2026-10-25)

### Feat

- **cache**: read a persisted value past its TTL, with its age
- **region**: size to content under a height ceiling
- **table**: paint a row's gutters in its own style
- **frame**: a header, body, and pinned footer shared by every screen
- **keyhint**: fit hints to a width by priority, dimming those that do not apply

## v0.22.0 (2026-10-25)

### Feat

- **theme**: lighten Ink's base and the surfaces above it

## v0.21.0 (2026-10-23)

### Feat

- **card**: a card drawn inline with a captioned border
- **skin**: selection, card, and edge faces
- **theme**: tint a background toward an accent, and mix two colors
- **theme**: add the Ink flavor

### Fix

- **forge**: diff the files GitHub sends without a patch
- **forge**: read a pull request past 300 files from the files API

## v0.20.0 (2026-10-15)

### Feat

- **skin**: tab and tab-active faces for the strips three tools draw

## v0.19.1 (2026-10-15)

### Fix

- **keyhint**: a column's prose indents to the description column

## v0.19.0 (2026-10-14)

### Feat

- **tui**: pane runs a program on a pty inside the frame

## v0.18.1 (2026-10-14)

### Fix

- **keyhint**: a bracket key wears its face, not the brackets

## v0.18.0 (2026-10-10)

### Feat

- **tui**: float a legend as a titled column box

## v0.17.0 (2026-10-10)

### Feat

- **keyhint**: render a legend as paged chip groups

## v0.16.0 (2026-10-07)

### Feat

- **vcs**: blame a file at a named revision

## v0.15.1 (2026-09-11)

### Fix

- **vcs**: recover from a diverged upstream in PullFastForward

## v0.15.0 (2026-09-07)

### Feat

- **filter**: add a generic predicate and scoped-query package

## v0.14.0 (2026-09-06)

### Feat

- **vcs**: attribute each line of a file to the change that introduced it

## v0.13.0 (2026-09-04)

### Feat

- **keyhint**: draw a heading over a group of hints

## v0.12.1 (2026-09-03)

### Fix

- **ratelimit**: use real time in transport tests instead of a fixed past date

## v0.12.0 (2026-09-02)

### Feat

- **ratelimit**: fail usefully when a GitHub allowance runs out

## v0.11.0 (2026-09-02)

### Feat

- **github**: read what is left of each rate-limit pool

## v0.10.1 (2026-09-02)

### Fix

- **vcs**: read a colocated jj checkout's head through git

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
