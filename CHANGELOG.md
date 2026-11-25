## v1.36.0 (2026-11-25)

### Feat

- **vimtext**: add a vim-modal text field for prose typed in place

## v1.35.1 (2026-11-25)

### Fix

- **pane**: close the emulator only after the pump and input copy return

## v1.35.0 (2026-11-25)

### Feat

- **icons**: draw Nerd icons by default in terminals that bundle them

### Fix

- **keyhint**: fill the cell between a hint's key and its word

## v1.34.2 (2026-11-25)

### Fix

- **deps**: update to my_go_template v0.24.4

## v1.34.1 (2026-11-25)

### Fix

- **hermetic**: turn off git auto maintenance in the isolated home

## v1.34.0 (2026-11-25)

### Feat

- **launch**: start a program without losing the first key typed

### Fix

- **coach**: concurrent ledger saves each succeed

## v1.33.0 (2026-11-25)

### Feat

- **keys**: reserved family keys, the space tick, moved-key notices, and the review verbs

## v1.32.0 (2026-11-25)

### Feat

- **seen**: OpenJJ, Store.Union, and Set.Include for marks shared across tools
- **framecheck**: Graph lets a revision graph's lines join its node glyphs

### Fix

- **coach**: a key that dismisses the welcome card no longer also acts

### Perf

- **skin**: benchmark the per-row and per-frame faces, and say to share one Skin
- **keyhint**: Fit draws each hint once, and Memo keeps an unchanged footer

## v1.31.1 (2026-11-24)

### Fix

- **hermetic**: unset git's repository variables and fail a run that changes the real repository

## v1.31.0 (2026-11-24)

### Feat

- **coach**: a first-run welcome card of starter keys from the tool's keymap

## v1.30.0 (2026-11-24)

### Feat

- **hermetic**: run a test binary in a home of its own and fail a run that reaches the real one
- **seen**: migrate marks filed under a prefix of a scope

## v1.29.0 (2026-11-24)

### Feat

- **seen**: resolve a jj workspace to its repository store

## v1.28.0 (2026-11-24)

### Feat

- **seen**: read marks shared across tools

## v1.27.1 (2026-11-24)

### Fix

- **deps**: bubbletea v2.1.2

## v1.27.0 (2026-11-24)

### Feat

- **palette**: SetSkin, so the palette follows a restyle

## v1.26.2 (2026-11-24)

### Fix

- **palette**: explain a key as its column shows it

## v1.26.1 (2026-11-24)

### Fix

- **skin**: On, a cell on a tool's own row face under the selection policy

## v1.26.0 (2026-11-24)

### Feat

- **tui**: a capped palette key column and a clean 16-color selected row
- **framecheck**: a Viewport option for a scrolled body between fixed rows

## v1.25.2 (2026-11-24)

### Fix

- **theme**: keep a diff sign's hue in 256 colors

## v1.25.1 (2026-11-24)

### Fix

- **glyph**: settle a colored circle or square to its shape

## v1.25.0 (2026-11-24)

### Feat

- **tui**: a glyph width policy, unicode icons by default, and settled content

## v1.24.3 (2026-11-24)

### Fix

- **palette**: keep keys whole, give blank right cells to descriptions, and rank an exact name first

## v1.24.2 (2026-11-24)

### Fix

- **skin**: sit a file's heading a ramp step under a changed line's wash

## v1.24.1 (2026-11-24)

### Fix

- **theme**: honor COLORTERM=truecolor inside tmux when detecting the profile

## v1.24.0 (2026-11-24)

### Feat

- **tui**: fit every background to the color profile, and lint frames for alignment

## v1.23.5 (2026-11-24)

### Fix

- **coach**: give the band's word column room for a two-word action
- **palette**: mark a chord's objects off when the chord itself is

## v1.23.4 (2026-11-24)

### Fix

- **palette**: draw no right column when no row has a count or a recent run

## v1.23.3 (2026-11-24)

### Fix

- **palette**: let a tool set the layout after building the palette

## v1.23.2 (2026-11-24)

### Fix

- **palette**: draw an off entry's reason beside its description

## v1.23.1 (2026-11-24)

### Fix

- **coach**: arm the idle timer only when a tip could come, and name keys without a keymap
- **palette**: read a legend's labels as the keys they name, and page like the screen

## v1.23.0 (2026-11-24)

### Feat

- **empty**: say what is empty, why, and the keys out
- **coach**: suggest the next step and tip unused keys, rarely
- **trail**: log each external command a tool runs, copyable as a shell line
- **palette**: one palette for `:` and the `?` legend, with an explainer pane
- **keyusage**: log the action that ran and the presses a screen refused
- **keys**: gate an action with the reason it cannot run here
- **fuzzy**: score a query against text as a ranked subsequence

## v1.22.1 (2026-11-24)

### Fix

- **keys**: word the column moves short enough for an 80-column legend

## v1.22.0 (2026-11-24)

### Feat

- **keys**: add the split columns and conflict viewer tables

## v1.21.0 (2026-11-24)

### Feat

- **keys**: share one key vocabulary across jojo, jj-diff, and second-look

## v1.20.3 (2026-11-24)

### Perf

- **diff**: pair lines for mechanical hunks only when a hunk could be one

## v1.20.2 (2026-11-24)

### Perf

- **diff**: fold mechanical hunks in one pass without building a Replacer per line
- **overview**: draw the track, the thumb, and each stock mark once per bar

## v1.20.1 (2026-11-24)

### Perf

- **pen**: draw a lipgloss style without running its renderer per run
- **highlight**: lex in the background, resolve grammars once, and lex both sides at once

## v1.20.0 (2026-11-23)

### Feat

- **icons**: add Nerd Font file and state icons with a unicode fallback
- **skin**: share diff faces, a file band, and labeled-rule headings
- **overview**: add a typed overview track for a frame's right edge
- **diff**: add AlignRun to lay a change run out side by side

### Fix

- **diff**: keep short lines from anchoring the line pairing

## v1.19.0 (2026-11-15)

### Feat

- **keyhint**: pin hints so help and quit outlast the status
- **overlay**: draw a modal over a dimmed live frame

## v1.18.0 (2026-11-14)

### Feat

- add highlight, generated, and diff.MechanicalHunks for tools that draw diffs

## v1.17.0 (2026-11-14)

### Feat

- **vcs**: count the commits a history walk covers
- **vcs**: let a history walk diff blobs past a size as binary
- **vcs**: read blobs across calls through one long-lived cat-file process

## v1.16.0 (2026-11-14)

### Feat

- **keyusage**: record CLI subcommands and flag names and report unused ones

## v1.15.0 (2026-11-14)

### Feat

- **keyusage**: skip line movement and grade brightening at 7, 14, and 28 days

## v1.14.0 (2026-11-14)

### Feat

- **keyusage**: log command keys per context and report unused ones

## v1.13.1 (2026-11-10)

### Fix

- **vcs**: mark GIT binary patch sections as binary
- **vcs**: stop LogHistory from blocking on an early break
- **vcs**: read binary file paths from diff headers
- **vcs**: stop patch bytes from splitting git log records

## v1.13.0 (2026-11-10)

### Feat

- **diff**: match moved lines across removed and added sequences

## v1.12.1 (2026-11-10)

### Fix

- **vcs**: read ---/+++ lines as headers only before a file's first hunk
- **vcs**: skip submodule entries in TreeEntries
- **vcs**: place pure-insertion hunks after their anchor line

## v1.12.0 (2026-11-10)

### Feat

- **vcs**: capture added and removed line text on CommitFile

## v1.11.0 (2026-11-10)

### Feat

- add diff package for unified-diff parsing and anchoring

## v1.10.1 (2026-11-09)

### Fix

- **vcs**: key BlobContents by the requested sha, not the resolved one

## v1.10.0 (2026-11-09)

### Feat

- **vcs**: stream per-commit history, diffs, trees, and blobs for analysis tools

### Fix

- **vcs**: spell out jj blame timestamp format for jj 0.47

## v1.9.0 (2026-10-30)

### Feat

- **tui/table**: add ScrollWindow to keep a selection visible
- **tui/box**: draw content in a bordered rectangle of an exact size
- **tui/commandbar**: add a generic command registry with tab completion

## v1.8.0 (2026-10-30)

### Feat

- **forge/github**: read run and job timings and merge queue events
- **stats**: add Percentile and Mean for timing summaries

## v1.7.0 (2026-10-29)

### Feat

- **preview**: render a standalone detail pane in one call

## v1.6.0 (2026-10-29)

### Feat

- **vcs**: resolve a checkout's remote identity without a subprocess
- **cache**: add a Snapshot slot for last-known state, and take the app name for the user store

## v1.5.2 (2026-10-29)

### Feat

- **tui**: export the grid's side width for callers clamping scroll

## v1.5.1 (2026-10-29)

### Feat

- **forge**: rebuild diffs the line-limit error too

## v1.5.0 (2026-10-29)

### Feat

- **tui**: add a preview package for the sections-and-detail pane

## v1.4.0 (2026-10-28)

### Feat

- **frame**: space a tab count off its label

## v1.3.0 (2026-10-26)

### Feat

- **frame**: draw a strip's hints bare and its panel on a Mantle surface that keeps muted text readable

## v1.2.0 (2026-10-26)

### Feat

- **frame**: tab count chips, a strip header with an underlining rule, a raised panel, and click hit-testing
- **overlay**: report the cells a blitted box covers so a click outside it can close it

## v1.1.0 (2026-10-26)

### Feat

- **split**: place a detail region beside, below, or not at all and join it with main

## v1.0.0 (2026-10-26)

### Feat

- **keyhint**: add a which-key Panel and overlay.BottomRight to float it
- **skin**: remove CursorBar, draw Selection in reverse video under NO_COLOR

## v0.24.0 (2026-10-25)

### Feat

- **handoff**: run another tool with the terminal and report its exit
- **keyhint**: draw hints bare with a set gap
- **skin**: badge, hairline, and selection band faces
- **table**: breakpoints, styled cells, and a group rule

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
