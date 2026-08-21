package github_test

import (
	"os"
	"testing"

	"github.com/kyleking/aragonite/cache"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/forge/github"
	"github.com/kyleking/aragonite/vcs"
)

const (
	upstream = "github.com/kyleking/aragonite"
	listKey  = "prs"
)

func samplePRs() []forge.PullRequest {
	return []forge.PullRequest{
		{Number: 7, Title: "Cache pull requests on disk", State: "OPEN", HeadRef: "cache", Checks: forge.ChecksStatus{
			Total: 2, Passing: 2,
		}},
		{Number: 4, Title: "Stamp the checkout", State: "OPEN", HeadRef: "stamp"},
	}
}

// Refresh is pressed because something looks wrong; a stale pull request state
// left on disk would survive it.
//
//nolint:paralleltest // installs the process-wide store ClearAll drains
func TestClearAllDropsTheInstalledDiskCache(t *testing.T) {
	dir := t.TempDir()
	store := cache.NewDiskCache(dir)

	cache.SetDiskCache(store)
	t.Cleanup(func() { cache.SetDiskCache(nil) })

	cache.Persist(github.PRListCache, upstream, listKey, cache.NoStamp, samplePRs())

	if _, err := os.Stat(cache.DiskPath(store, upstream)); err != nil {
		t.Fatalf("the installed store never wrote the file: %v", err)
	}

	cache.ClearAll()

	if _, err := os.Stat(cache.DiskPath(store, upstream)); !os.IsNotExist(err) {
		t.Errorf("refresh left the file on disk: %v", err)
	}
	if _, ok := cache.Persisted(github.PRListCache, upstream, listKey, cache.NoStamp); ok {
		t.Error("refresh served a value from the cache it just cleared")
	}
}

// ClearAll reaches caches registered from another package, which is the only
// reason the registry exists rather than a list one package maintains.
//
//nolint:paralleltest // drains the process-wide registry other tests populate
func TestClearAllReachesEveryRegisteredCache(t *testing.T) {
	github.PRCache.Set("test", cache.NoStamp, nil)
	github.WorkflowCache.Set("test", cache.NoStamp, nil)
	vcs.BranchCache.Set("test", cache.NoStamp, nil)
	vcs.CommitCache.Set("test", cache.NoStamp, nil)

	cache.ClearAll()

	_, pr := github.PRCache.Get("test", cache.NoStamp)
	_, workflow := github.WorkflowCache.Get("test", cache.NoStamp)
	_, branch := vcs.BranchCache.Get("test", cache.NoStamp)
	_, commit := vcs.CommitCache.Get("test", cache.NoStamp)

	if pr || workflow || branch || commit {
		t.Error("expected every registered cache to be cleared")
	}
}
