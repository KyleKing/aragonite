package github_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/forge/github"
)

var errTooLargeDiff = errors.New("running gh: exit status 1: could not find pull request diff: " +
	"HTTP 406: Sorry, the diff exceeded the maximum number of files (300). Consider using " +
	"'List pull requests files' API or locally cloning the repository instead.\n" +
	"PullRequest.diff too_large")

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

// The files API pages arrive back to back, each page its own JSON array.
const (
	filesPage1 = `[
		{"filename": "pkg/added.go", "status": "added", "additions": 2, "deletions": 0,
			"patch": "@@ -0,0 +1,2 @@\n+one\n+two"},
		{"filename": "pkg/modified.go", "status": "modified", "additions": 1, "deletions": 1,
			"patch": "@@ -1 +1 @@\n-old\n+new"},
		{"filename": "pkg/removed.go", "status": "removed", "additions": 0, "deletions": 1,
			"patch": "@@ -1 +0,0 @@\n-gone"},
		{"filename": "pkg/renamed.go", "previous_filename": "pkg/was.go", "status": "renamed",
			"additions": 1, "deletions": 0, "patch": "@@ -1 +1,2 @@\n kept\n+more"}
	]`
	filesPage2 = `[
		{"filename": "img/logo.png", "status": "modified", "additions": 0, "deletions": 0},
		{"filename": "img/new.png", "status": "added", "additions": 0, "deletions": 0},
		{"filename": "pkg/moved.go", "previous_filename": "pkg/from.go", "status": "renamed",
			"additions": 0, "deletions": 0}
	]`
)

const rebuiltDiff = `diff --git a/pkg/added.go b/pkg/added.go
--- /dev/null
+++ b/pkg/added.go
@@ -0,0 +1,2 @@
+one
+two
diff --git a/pkg/modified.go b/pkg/modified.go
--- a/pkg/modified.go
+++ b/pkg/modified.go
@@ -1 +1 @@
-old
+new
diff --git a/pkg/removed.go b/pkg/removed.go
--- a/pkg/removed.go
+++ /dev/null
@@ -1 +0,0 @@
-gone
diff --git a/pkg/was.go b/pkg/renamed.go
rename from pkg/was.go
rename to pkg/renamed.go
--- a/pkg/was.go
+++ b/pkg/renamed.go
@@ -1 +1,2 @@
 kept
+more
diff --git a/img/logo.png b/img/logo.png
Binary files a/img/logo.png and b/img/logo.png differ
diff --git a/img/new.png b/img/new.png
Binary files /dev/null and b/img/new.png differ
diff --git a/pkg/from.go b/pkg/moved.go
rename from pkg/from.go
rename to pkg/moved.go
`

func TestPRDiff_FallsBackToFilesPastTheLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		diffErr      error
		files        string
		name         string
		wantDiff     string
		wantErr      string
		changedFiles int
		wantFallback bool
	}{
		{
			name:         "rebuilds the diff from the files API",
			diffErr:      errTooLargeDiff,
			files:        filesPage1 + filesPage2,
			changedFiles: 7,
			wantDiff:     rebuiltDiff,
			wantFallback: true,
		},
		{
			name:    "names every file whose patch GitHub withheld",
			diffErr: errTooLargeDiff,
			files: `[
				{"filename": "big.go", "status": "modified", "additions": 5, "deletions": 0},
				{"filename": "huge.go", "status": "added", "additions": 9, "deletions": 0}
			]`,
			changedFiles: 2,
			wantErr:      "big.go, huge.go",
			wantFallback: true,
		},
		{
			name:    "reports how many changed files the API did not return",
			diffErr: errTooLargeDiff,
			files: `[{"filename": "a.go", "status": "modified", "additions": 1, "deletions": 0,
				"patch": "@@ -1 +1 @@\n-a\n+b"}]`,
			changedFiles: 3,
			wantErr:      "2 of 3 missing",
			wantFallback: true,
		},
		{
			name:    "passes any other gh error through",
			diffErr: errGHFailed,
			wantErr: "reading the diff of pull request #42: gh failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var calls [][]string

			ctx := github.WithGHRunner(t.Context(),
				func(_ context.Context, _ string, _ []string, a ...string) ([]byte, error) {
					calls = append(calls, a)

					switch {
					case a[0] == "pr":
						return nil, tc.diffErr
					case slices.Contains(a, "--paginate"):
						return []byte(tc.files), nil
					default:
						return fmt.Appendf(nil, `{"changed_files":%d}`, tc.changedFiles), nil
					}
				})

			out, err := github.PRDiff(ctx, "", "coverbasedev/irm", 42)

			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
				t.Fatalf("error = %v, want one containing %q", err, tc.wantErr)
			}

			if string(out) != tc.wantDiff {
				t.Errorf("diff = %q, want %q", out, tc.wantDiff)
			}

			usedFiles := slices.ContainsFunc(calls, func(a []string) bool {
				return slices.Contains(a, "--paginate")
			})
			if usedFiles != tc.wantFallback {
				t.Errorf("used the files API = %t, want %t: calls %v", usedFiles, tc.wantFallback, calls)
			}
		})
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
