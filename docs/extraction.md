# Extracting a package

What the `cache` extraction taught, so the next one costs less.

## The seam

Split on consumer coupling rather than on file boundaries. `internal/cache` looked like
one package and was two: generic machinery (the TTL cache, the disk store, the registry,
remote scoping) and gh-repo-dashboard's own concrete cache instances, TTL constants, and
key helpers, which name `models` types the library must never know about.

Only the generic half moves. The consumer keeps a thin `internal/cache` that aliases the
library's types and wraps its generic functions, so no call site outside the package
changes. That kept the cutover to seven files and left the other thirteen packages
untouched.

## Tests move with the code, except the ones that do not

31 of 33 cache tests moved and passed unchanged once their payload types were swapped for
local stand-ins, because the cache is generic over its value and only requires that it
round-trips through `encoding/json`.

Two stayed behind. They exercise the concrete `BranchCache` and `CommitCache` and call
`vcs.CheckoutIdentity`, which makes them tests of the consumer's cache wiring rather than
of the library.

## A library needs a test seam

The consumer's remaining tests needed a clock they could advance, a disk store with a
small byte budget, and a store pointed at their own directory. All three lived in the
package's `export_test.go`, which is invisible outside it.

`cache/testing.go` promotes them to real API. A package that keeps its test seams private
forces every consumer to either sleep in tests or not test at all.

## Verify in a throwaway worktree

`git worktree add --detach` a copy, cut it over there, run the full suite, export the diff
as a patch, and remove the worktree. The consumer's working tree is never touched and the
result is a patch you can apply once you decide to land it.
