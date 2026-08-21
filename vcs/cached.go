package vcs

import (
	"strconv"
	"time"

	"github.com/kyleking/aragonite/cache"
)

const defaultTTL = 5 * time.Minute

// BranchCache and CommitCache hold what a checkout's refs and log said, keyed
// by the checkout rather than by the object store it borrows. A worktree and
// its parent read the same refs, but both values are relative to whichever HEAD
// asked: the branch list carries the current-branch marker and the log starts
// at HEAD.
//
//nolint:gochecknoglobals // one process-wide cache each, which is the point of registering them
var (
	BranchCache = cache.NewRegistered[[]BranchInfo](defaultTTL)
	CommitCache = cache.NewRegistered[[]CommitInfo](defaultTTL)
)

// BranchCacheKey builds the branch list cache key for a checkout.
func BranchCacheKey(repoPath string) string {
	return repoPath + "\x00branches"
}

// CommitCacheKey builds the commit log cache key for a checkout, keyed by depth
// because a deeper log is a different value.
func CommitCacheKey(repoPath string, count int) string {
	return repoPath + "\x00commits:" + strconv.Itoa(count)
}

// cachedBranchList serves the checkout's branch list while the stamp proves it
// unchanged, however long ago it was read.
func cachedBranchList(repoPath string, read func() ([]BranchInfo, error)) ([]BranchInfo, error) {
	return stamped(BranchCache, BranchCacheKey(repoPath), repoPath, read)
}

// cachedCommitLog serves the checkout's commit log under the same rule.
func cachedCommitLog(
	repoPath string, count int, read func() ([]CommitInfo, error),
) ([]CommitInfo, error) {
	return stamped(CommitCache, CommitCacheKey(repoPath, count), repoPath, read)
}

// stamped reads through cache c, skipping it entirely for a checkout that
// could not be stamped: a local value nothing can prove fresh must not be
// served from a timer.
//
//nolint:ireturn // T is the cache's own type parameter, not an abstraction leak
func stamped[T any](c *cache.TTLCache[T], key, repoPath string, read func() (T, error)) (T, error) {
	stamp := Stamp(repoPath)
	if cached, ok := c.Fresh(key, stamp); ok {
		return cached, nil
	}

	value, err := read()
	if err != nil {
		var zero T

		return zero, err
	}

	if stamp.Fingerprint != "" {
		c.Set(key, stamp, value)
	}

	return value, nil
}
