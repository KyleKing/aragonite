# gh-sweep as a consumer

What was found while checking whether [gh-sweep](https://github.com/KyleKing/gh-sweep)
should share code with aragonite and gh-repo-dashboard, and what to do about it. gh-sweep
audits a GitHub org or an explicit repo list over the REST/GraphQL APIs; gh-repo-dashboard
manages repos already checked out on disk. They overlap on "GitHub data about a repo," not
on how either one gets it.

## Access strategy: different by design, not by accident

gh-sweep talks to GitHub through `cli/go-gh/v2/pkg/api` in-process: one client built once,
reused across every call in a scan, with go-gh resolving `gh auth login`'s token the same
way `gh` itself does. gh-repo-dashboard shells real `gh` subcommands
(`internal/github/exec.go`), one subprocess per call, with `cmd.Dir` set to the checkout
being read.

Both are the right call for what each tool is given. gh-sweep operates on an org or a repo
list with no filesystem in the picture, so there is no `cwd` to resolve a repo from and no
reason to pay `gh`'s startup cost per request when a persistent client can hold the
connection open across a fleet-wide scan. gh-repo-dashboard operates per local checkout,
and its own `vcs.GetRemoteURL(ctx, repoPath)` / `identity.go` already resolve owner/repo
from an explicit path (go-gh's `repository.Current()` only resolves from process `cwd`,
so it could not replace that path-scoped resolution even if gh-repo-dashboard wanted the
go-gh client generally — repo identity stays exactly where it is).

None of this is a workaround for the other; they are two correct answers to two different
questions.

## Where a shared client would pay off, and where it would not

A go-gh-backed client would still be worth it for gh-repo-dashboard's **read-only** calls
(`pr view`, `pr list`, `run list`, `run view`, and the one raw `api` call already in
`internal/github/workflow.go`): one persistent client instead of a subprocess (and a full
`gh` binary startup) per call, typed decoding instead of parsing `--json` text, fed the
owner/repo the checkout path already resolves locally. Its **write** paths
(`internal/github/mutate.go`, `pr checkout`) should keep shelling `gh`, because those do
more than call the API — `pr checkout` also sets up local branch tracking, which is
squarely `gh`'s own job, not a plain GET/POST this library would replace.

Nobody should build this before gh-repo-dashboard actually adopts it, and it should not be
built as a big-bang cutover: land the client and the model, switch one read-only call site,
confirm nothing broke, then repeat.

## What's already shared, and what's next

| Package | Status | Note |
| --- | --- | --- |
| `cache` | Landed | gh-sweep's own `internal/cache.MemoryManager` was already dead code (no callers) and was deleted rather than migrated — `cache.TTLCache[T]` already covers what it did, and better (stamp-based invalidation, not just a flat TTL) |
| Transport seam + mutation guard | Landing now | See below |
| `forge.PullRequest` / `WorkflowRun` | Not attempted | See "Why the PR model did not move" |

### Transport seam + mutation guard

gh-sweep's `internal/github/transport.go` (~99 lines) is an injectable
`http.RoundTripper` for fake transports in tests, plus a `safetyTransport` that panics on
any real mutating request (`POST`/`PATCH`/`PUT`/`DELETE`) reached during `go test`. It has
no GitHub-domain coupling beyond a host/token pinned for the no-fake-registered case, and
its own comment already says it mirrors a hand-rolled equivalent in gh-lazydispatch's
`exec.RealExecutor` — a third project re-deriving the same pattern is exactly the signal
`docs/extraction.md` describes: this is generic go-gh client-safety infrastructure, not
gh-sweep-specific.

Landing in this pass as a new package (name pending — a short survey of what to call a
"safe transport for a go-gh-backed client" package, distinct from `forge`, is worth doing
in the PR itself rather than pre-deciding here).

### Why the PR model did not move

`forge.PullRequest` already exists, extracted from gh-repo-dashboard, and it is shaped for
a dashboard detail view: `Checks`, `ReviewDecision`, `Reviewers`, `Activity`, state as the
GraphQL-flavored `OPEN`/`CLOSED`/`MERGED` constants `gh --json` returns. gh-sweep's own
`internal/github.PullRequest` is much thinner — `Number`, `Title`, `State`, `Head`, `Base`,
`MergedAt *time.Time`, `ClosedAt *time.Time` — because all it needs is enough to classify
a branch as merged/closed/open for orphan detection, and it reads the REST pulls endpoint,
which represents state as lowercase `open`/`closed` plus separate nullable timestamp
fields rather than a single tri-state enum.

Forcing these into one type means either gh-sweep carrying fields it will never populate
(`Checks`, `Reviewers`, `Activity`) or `forge.PullRequest` gaining a translation layer
between REST's timestamp-pair shape and GraphQL's enum shape. Either is real complexity
bought for no proven need — gh-sweep's orphan detector does not want the richer fields,
and nothing today asks gh-repo-dashboard's PR views to classify merge state from REST
data. Left alone. Revisit only when a real feature needs the same fields from both sides
(the CI-history idea below is the closest candidate).

`forge.WorkflowRun` (`ID`, `Name`, `Status`, `Conclusion`, timestamps) is closer to
gh-sweep's `RunTiming`, which additionally carries per-job/per-step duration breakdowns
`forge.WorkflowRun` does not model. Same call: not attempted, revisit if the CI-history
idea below is ever built.

## Feature-level synergies between gh-sweep and gh-repo-dashboard

Infrastructure sharing (cache, transport) is the easy, low-risk half. The harder question
is whether either tool's actual features belong in the other. Surveyed below; most of
these are "note it and move on," not "go build it" — these are personal projects, so
there's no harm in a longer list of ideas than will ever get built, but KISS/YAGNI still
says don't build ahead of a real want.

- **CI history depth**: gh-repo-dashboard shows the *current* state of a commit's CI
  (`DefaultBranchCI`, `WorkflowSummary` — a snapshot). gh-sweep's `gha-perf` view analyzes
  *historical* run timing: trends, regressions, flaky-test detection over many runs. These
  are complementary depths on the same underlying data. The actual regression/flaky
  algorithms are pure functions over a `[]WorkflowRun`-shaped slice and do not care whether
  the runs came from gh-sweep's REST polling or gh-repo-dashboard's `gh run list` — of
  everything surveyed here, this is the best real DRY candidate, *if* gh-repo-dashboard
  ever wants that depth. Until then, a "view full history in gh-sweep" link from
  gh-repo-dashboard's detail pane is far cheaper than porting the analysis.
- **Namespace-wide watch/notification and orphan-branch audits**: inherently org/account-
  wide, need no local checkout. These stay in gh-sweep; gh-repo-dashboard's whole premise
  is "repos I have on disk," a strict subset of what gh-sweep can see. No action, just a
  boundary worth stating so nobody tries to push these into gh-repo-dashboard later.
- **Cross-referencing remote and local staleness**: gh-sweep's namespace orphan scan finds
  branches whose PR merged/closed on GitHub, whether or not you have them checked out.
  gh-repo-dashboard's `CleanupMerged` only sees branches that exist locally. A gh-sweep
  finding ("this remote branch is gone-merged") could flag a matching *local* stale branch
  in gh-repo-dashboard too. Real integration, not a shared package — would need the two
  tools to agree on an identity format for "this local branch is this remote branch," which
  neither has today. Interesting, not close to buildable.
- **Settings/protection/secrets/webhooks/collaborators/releases/policy audits**: all
  cross-repo governance, no local checkout involved. Squarely gh-sweep's domain, no
  overlap with gh-repo-dashboard expected.
- **Batch task / progress-reporting pattern**: gh-repo-dashboard's `internal/batch` runs a
  task across many local checkouts with incremental progress messages
  (`Loading 8/54`-style). gh-sweep's namespace scanners do the same fan-out-with-progress
  shape across many repos read from the API. A generic "run N tasks concurrently, report
  progress via a channel or Bubble Tea messages" helper is the same kind of pure,
  domain-free infrastructure the transport seam is. Worth a look once transport lands,
  not evaluated in depth here.
- **Search/filter primitives**: gh-repo-dashboard's `internal/filters` is a real predicate
  DSL (typed atoms, recency comparisons, glob matching) built for its repo-list and PR
  views. gh-sweep's home-menu and per-view search is a ~15-line subsequence fuzzy-match,
  which is all it currently needs. Not a candidate to move now — noted only so a future
  gh-sweep feature that wants real query syntax knows where to look first instead of
  growing its own.

## Sequencing

1. Land the transport seam in this pass (aragonite first, then gh-sweep switches its
   import and confirms its test suite is unchanged).
2. Everything else here waits for either a concrete want (CI-history depth, the batch/
   progress helper) or a second real consumer, matching the project's own "planned, once a
   second consumer needs them" rule in the README.
