package github_test

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/cache"
	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/forge/github"
)

const mutateRepoPath = "/repo"

// A mutation runs one gh call, answers what gh printed, and drops the cached
// pull request views only once gh succeeded.
//
//nolint:paralleltest // asserts against shared global cache state
func TestMutations_RunGHThenInvalidate(t *testing.T) {
	tests := []struct {
		runErr error
		mutate func(ctx context.Context) (string, error)
		name   string
		out    string
		args   string
		want   string
	}{
		{
			name: "create against a base", out: "Creating pull request\nhttps://github.com/acme/app/pull/7\n",
			mutate: func(ctx context.Context) (string, error) {
				return github.CreatePR(ctx, mutateRepoPath, "feature", "main")
			},
			args: "pr create --fill --head feature --base main", want: "https://github.com/acme/app/pull/7",
		},
		{
			name: "create without a base", out: "https://github.com/acme/app/pull/8",
			mutate: func(ctx context.Context) (string, error) {
				return github.CreatePR(ctx, mutateRepoPath, "feature", "")
			},
			args: "pr create --fill --head feature", want: "https://github.com/acme/app/pull/8",
		},
		{
			name: "create fails", runErr: errGHFailed,
			mutate: func(ctx context.Context) (string, error) {
				return github.CreatePR(ctx, mutateRepoPath, "feature", "main")
			},
			args: "pr create --fill --head feature --base main",
		},
		{
			name: "squash merge",
			mutate: func(ctx context.Context) (string, error) {
				return "", github.SquashMergePR(ctx, mutateRepoPath, 42)
			},
			args: "pr merge 42 --squash --delete-branch",
		},
		{
			name: "squash merge fails", runErr: errGHFailed,
			mutate: func(ctx context.Context) (string, error) {
				return "", github.SquashMergePR(ctx, mutateRepoPath, 42)
			},
			args: "pr merge 42 --squash --delete-branch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cache.ClearAll()

			ctx, calls := stubRunGH([]byte(tc.out), tc.runErr)
			github.PRListCache.Set(
				github.PRListCacheKey(mutateRepoPath, testRemoteID, "origin/main"),
				cache.NoStamp,
				[]forge.PullRequest{{Number: 1}},
			)
			github.MergedPRHeadsCache.Set(
				github.MergedPRHeadsCacheKey(mutateRepoPath, testRemoteID),
				cache.NoStamp,
				map[string]string{"feature": "abc123"},
			)

			got, err := tc.mutate(ctx)
			if !errors.Is(err, tc.runErr) || (tc.runErr == nil && err != nil) || got != tc.want {
				t.Fatalf("= %q, %v, want %q, %v", got, err, tc.want, tc.runErr)
			}

			if len(*calls) != 1 || strings.Join((*calls)[0], " ") != tc.args {
				t.Errorf("gh calls = %q, want one: %s", *calls, tc.args)
			}

			_, listHeld := github.CachedPRs(mutateRepoPath, testRemoteID, "origin/main")
			_, headsHeld := github.CachedMergedPRHeads(mutateRepoPath, testRemoteID)

			if wantHeld := tc.runErr != nil; listHeld != wantHeld || headsHeld != wantHeld {
				t.Errorf("caches held list=%v heads=%v, want %v", listHeld, headsHeld, wantHeld)
			}
		})
	}
}

// A mutation changes what every checkout of the remote sees, so every pull
// request view is dropped and nothing else is.
//
//nolint:paralleltest // asserts against shared global cache state
func TestInvalidatePRCaches_DropsEveryPullRequestView(t *testing.T) {
	cache.ClearAll()

	github.PRCache.Set("k", cache.NoStamp, &forge.PullRequest{Number: 1})
	github.PRListCache.Set("k", cache.NoStamp, []forge.PullRequest{{Number: 1}})
	github.PRDetailCache.Set("k", cache.NoStamp, &forge.PRDetail{})
	github.PRPreviewCache.Set("k", cache.NoStamp, &forge.PRPreview{})
	github.MergedPRHeadsCache.Set("k", cache.NoStamp, map[string]string{"a": "b"})
	github.WorkflowCache.Set("k", cache.NoStamp, &forge.WorkflowSummary{Total: 1})

	github.InvalidatePRCaches()

	held := map[string]bool{}
	_, held["pr"] = github.PRCache.Get("k", cache.NoStamp)
	_, held["list"] = github.PRListCache.Get("k", cache.NoStamp)
	_, held["detail"] = github.PRDetailCache.Get("k", cache.NoStamp)
	_, held["preview"] = github.PRPreviewCache.Get("k", cache.NoStamp)
	_, held["merged heads"] = github.MergedPRHeadsCache.Get("k", cache.NoStamp)
	_, held["workflow"] = github.WorkflowCache.Get("k", cache.NoStamp)

	want := map[string]bool{
		"pr": false, "list": false, "detail": false, "preview": false, "merged heads": false, "workflow": true,
	}
	if !maps.Equal(held, want) {
		t.Errorf("held after invalidation = %v, want %v", held, want)
	}
}
