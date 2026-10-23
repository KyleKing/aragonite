package github_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/forge/github"
)

const fakeMergeBase = "merge-base-sha"

var (
	errTooLargeDiff = errors.New("running gh: exit status 1: could not find pull request diff: " +
		"HTTP 406: Sorry, the diff exceeded the maximum number of files (300). Consider using " +
		"'List pull requests files' API or locally cloning the repository instead.\n" +
		"PullRequest.diff too_large")
	errMissingBlob  = errors.New("the fake has no blob for that sha")
	errMissingFile  = errors.New("the fake has no file at the merge base for that path")
	errNotMergeBase = errors.New("the fake was asked to read a ref other than the merge base")
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
		{"filename": "img/logo.png", "status": "modified", "sha": "logo-new"},
		{"filename": "img/new.png", "status": "added", "sha": "png-new"},
		{"filename": "pkg/moved.go", "previous_filename": "pkg/from.go", "status": "renamed",
			"sha": "moved-sha"}
	]`
	blobFiles = `[
		{"filename": "src/app.ts", "status": "modified", "sha": "app-new"},
		{"filename": "src/new.ts", "status": "added", "sha": "new-blob"},
		{"filename": "src/gone.ts", "status": "removed"},
		{"filename": "scripts/run.sh", "status": "modified", "sha": "run-same"}
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

const blobDiff = `diff --git a/src/app.ts b/src/app.ts
--- a/src/app.ts
+++ b/src/app.ts
@@ -1,3 +1,3 @@
 one
-two
+two changed
 three
diff --git a/src/new.ts b/src/new.ts
--- /dev/null
+++ b/src/new.ts
@@ -0,0 +1 @@
+export {}
diff --git a/src/gone.ts b/src/gone.ts
--- a/src/gone.ts
+++ /dev/null
@@ -1 +0,0 @@
-bye
diff --git a/scripts/run.sh b/scripts/run.sh
`

// diffCase is one way the files API fallback ends: a rebuilt diff, an error, or a pass-through.
type diffCase struct {
	diffErr      error
	files        string
	atBase       map[string]string
	blobs        map[string]string
	name         string
	wantDiff     string
	wantErr      string
	changedFiles int
	wantFallback bool
}

var fallbackCases = []diffCase{
	{
		name:         "rebuilds the diff from the files API",
		diffErr:      errTooLargeDiff,
		files:        filesPage1 + filesPage2,
		changedFiles: 7,
		atBase:       map[string]string{"img/logo.png": "logo-old", "pkg/from.go": "moved-sha"},
		blobs: map[string]string{
			"logo-old":  "\x89PNG\x00\x01",
			"logo-new":  "\x89PNG\x00\x02",
			"png-new":   "\x89PNG\x00\x03",
			"moved-sha": "same\n",
		},
		wantDiff:     rebuiltDiff,
		wantFallback: true,
	},
	{
		name:         "rebuilds withheld text files and a mode-only change from their blobs",
		diffErr:      errTooLargeDiff,
		files:        blobFiles,
		changedFiles: 4,
		atBase: map[string]string{
			"src/app.ts":     "app-old",
			"src/gone.ts":    "gone-base",
			"scripts/run.sh": "run-same",
		},
		blobs: map[string]string{
			"app-old":   "one\ntwo\nthree\n",
			"app-new":   "one\ntwo changed\nthree\n",
			"new-blob":  "export {}\n",
			"gone-base": "bye\n",
			"run-same":  "echo hi\n",
		},
		wantDiff:     blobDiff,
		wantFallback: true,
	},
	{
		name:    "reports how many changed files the API did not return",
		diffErr: errTooLargeDiff,
		files: `[{"filename": "a.go", "status": "modified",
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

func endpointOf(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "repos/") {
			return arg
		}
	}

	return ""
}

func contentsAtMergeBase(c diffCase, endpoint string) ([]byte, error) {
	rest := endpoint[strings.Index(endpoint, "/contents/")+len("/contents/"):]

	file, ref, _ := strings.Cut(rest, "?ref=")
	if ref != fakeMergeBase {
		return nil, fmt.Errorf("%w: %q", errNotMergeBase, ref)
	}

	sha, ok := c.atBase[file]
	if !ok {
		return nil, fmt.Errorf("%w: %q", errMissingFile, file)
	}

	return []byte(sha + "\n"), nil
}

func blobResponse(c diffCase, endpoint string) ([]byte, error) {
	sha := endpoint[strings.LastIndex(endpoint, "/")+1:]

	content, ok := c.blobs[sha]
	if !ok {
		return nil, fmt.Errorf("%w: %q", errMissingBlob, sha)
	}

	return fmt.Appendf(nil, `{"content":"%s","encoding":"base64"}`,
		base64.StdEncoding.EncodeToString([]byte(content))), nil
}

// fakeDiffGH answers the reads PRDiff makes for c and records each call in calls.
func fakeDiffGH(calls *[][]string, c diffCase) github.GHRunner {
	return func(_ context.Context, _ string, _ []string, a ...string) ([]byte, error) {
		*calls = append(*calls, a)

		endpoint := endpointOf(a)

		switch {
		case a[0] == "pr":
			return nil, c.diffErr
		case slices.Contains(a, "--paginate"):
			return []byte(c.files), nil
		case strings.Contains(endpoint, "/compare/"):
			return []byte(fakeMergeBase + "\n"), nil
		case strings.Contains(endpoint, "/contents/"):
			return contentsAtMergeBase(c, endpoint)
		case strings.Contains(endpoint, "/git/blobs/"):
			return blobResponse(c, endpoint)
		default:
			return fmt.Appendf(nil, `{"changed_files":%d,"base":{"sha":"base-tip"},"head":{"sha":"head-tip"}}`,
				c.changedFiles), nil
		}
	}
}

func TestPRDiff_FallsBackToFilesPastTheLimit(t *testing.T) {
	t.Parallel()

	for _, tc := range fallbackCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var calls [][]string

			ctx := github.WithGHRunner(t.Context(), fakeDiffGH(&calls, tc))

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
