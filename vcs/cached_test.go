package vcs_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kyleking/aragonite/cache"
	"github.com/kyleking/aragonite/vcs"
)

type fakeClock struct {
	t  time.Time
	mu sync.Mutex
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Now()} }

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.t = c.t.Add(d)
}

// A worktree borrows its parent's object store, but the branch list and commit
// log it reads are relative to its own HEAD, so it keeps its own entry rather
// than reading the answer the parent wrote.
func TestObjectStoreKeysSeparateAWorktreeFromItsParent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	parent := filepath.Join(root, "app")
	worktree := filepath.Join(root, "app-wt")
	other := filepath.Join(root, "lib")

	for _, dir := range []string{filepath.Join(parent, ".git"), other, worktree} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}

	pointer := "gitdir: " + filepath.Join(parent, ".git", "worktrees", "app-wt") + "\n"
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte(pointer), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := vcs.CheckoutIdentity(worktree); got != parent {
		t.Fatalf("the fixture is not a worktree of the parent: identity = %q, want %q", got, parent)
	}

	branchKey := vcs.BranchCacheKey(parent)
	commitKey := vcs.CommitCacheKey(parent, 10)

	vcs.BranchCache.Set(branchKey, cache.NoStamp, []vcs.BranchInfo{{Name: "main"}})
	vcs.CommitCache.Set(commitKey, cache.NoStamp, []vcs.CommitInfo{{Subject: "init"}})

	t.Cleanup(func() {
		vcs.BranchCache.Delete(branchKey)
		vcs.CommitCache.Delete(commitKey)
	})

	if _, ok := vcs.BranchCache.Get(vcs.BranchCacheKey(worktree), cache.NoStamp); ok {
		t.Error("a worktree read its parent's branch list, whose current-branch marker is the parent's")
	}
	if _, ok := vcs.CommitCache.Get(vcs.CommitCacheKey(worktree, 10), cache.NoStamp); ok {
		t.Error("a worktree read its parent's commit log, which starts at the parent's HEAD")
	}
	if _, ok := vcs.CommitCache.Get(vcs.CommitCacheKey(parent, 20), cache.NoStamp); ok {
		t.Error("a different log depth read the 10-commit entry")
	}
	if _, ok := vcs.BranchCache.Get(vcs.BranchCacheKey(other), cache.NoStamp); ok {
		t.Error("an unrelated checkout read the parent's branch list")
	}
}

// A branch list read from an unchanged checkout is still correct however old
// it is, so the stamp replaces the TTL rather than sitting under it.
func TestAnUnchangedStampCarriesALocalValuePastItsTTL(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	c := cache.NewTTLCacheWithClock[[]vcs.BranchInfo](5*time.Minute, clock.now)

	stamp := cache.Stamp{Scope: "/repo", Fingerprint: "head-a"}
	moved := cache.Stamp{Scope: "/repo", Fingerprint: "head-b"}
	key := vcs.BranchCacheKey("/repo")

	c.Set(key, stamp, []vcs.BranchInfo{{Name: "main"}})
	clock.advance(time.Hour)

	if got, ok := c.Fresh(key, stamp); !ok || len(got) != 1 {
		t.Errorf("an unchanged checkout refetched its branch list: %+v, hit=%v", got, ok)
	}
	if _, ok := c.Fresh(key, moved); ok {
		t.Error("a commit left the branch list cached")
	}
	if _, ok := c.Fresh(key, cache.NoStamp); ok {
		t.Error("an unstampable checkout was served a local value")
	}
	if _, ok := c.Get(key, stamp); ok {
		t.Error("the TTL is still the ceiling for Get, whatever the stamp says")
	}
}
