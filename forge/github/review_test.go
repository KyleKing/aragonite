package github_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/forge/github"
)

func TestGetPR_CarriesHeadSHA(t *testing.T) {
	t.Parallel()

	var args []string

	ctx := github.WithGHRunner(t.Context(),
		func(_ context.Context, _ string, _ []string, a ...string) ([]byte, error) {
			args = a

			return []byte(`{"number":42,"title":"t","state":"OPEN","headRefName":"feat",
				"headRefOid":"abc123","headRepositoryOwner":{"login":"someone"},"baseRefName":"main"}`), nil
		})

	pr, err := github.GetPR(ctx, "/repo", "", 42)
	if err != nil {
		t.Fatal(err)
	}

	if pr.HeadSHA != "abc123" {
		t.Errorf("HeadSHA = %q, want abc123", pr.HeadSHA)
	}
	if pr.HeadRepoOwner != "someone" {
		t.Errorf("HeadRepoOwner = %q, want someone", pr.HeadRepoOwner)
	}
	if !strings.Contains(strings.Join(args, " "), "headRefOid") {
		t.Errorf("gh was not asked for headRefOid: %v", args)
	}
}

func TestPRDiff_ReturnsPatchVerbatim(t *testing.T) {
	t.Parallel()

	patch := "diff --git a/x b/x\n@@ -1 +1 @@\n-old\n+new\n"

	var args []string

	ctx := github.WithGHRunner(t.Context(),
		func(_ context.Context, _ string, _ []string, a ...string) ([]byte, error) {
			args = a

			return []byte(patch), nil
		})

	out, err := github.PRDiff(ctx, "/repo", "", 42)
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != patch {
		t.Errorf("diff = %q, want %q", out, patch)
	}

	// --patch returns a format-patch series numbered per commit, which anchors
	// a review comment to the wrong line.
	if slices.Contains(args, "--patch") {
		t.Errorf("gh was asked for a patch series: %v", args)
	}
}

func TestReview_NamesTheRepositoryWhenGiven(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		remoteRepo string
		want       []string
	}{
		{name: "from a checkout of it", remoteRepo: "", want: nil},
		{name: "from anywhere", remoteRepo: "acme/app", want: []string{"--repo", "acme/app"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var args []string

			ctx := github.WithGHRunner(t.Context(),
				func(_ context.Context, _ string, _ []string, a ...string) ([]byte, error) {
					args = a

					return []byte("diff --git a/x b/x\n"), nil
				})

			if _, err := github.PRDiff(ctx, "/repo", tc.remoteRepo, 42); err != nil {
				t.Fatal(err)
			}

			if tc.want == nil {
				if slices.Contains(args, "--repo") {
					t.Errorf("gh named a repository it was not given: %v", args)
				}

				return
			}

			// The flag belongs to the subcommand rather than to gh itself, so it
			// has to follow "pr diff" rather than lead it.
			if !slices.Equal(args[len(args)-2:], tc.want) {
				t.Errorf("args = %v, want them to end with %v", args, tc.want)
			}
		})
	}
}
