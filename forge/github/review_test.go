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

	pr, err := github.GetPR(ctx, "/repo", 42)
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

	out, err := github.PRDiff(ctx, "/repo", 42)
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
