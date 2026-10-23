package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"
)

const (
	devNull            = "/dev/null"
	diffTooLargeMarker = "exceeded the maximum number of files"
)

var (
	errFilesMissing     = errors.New("the files API returned fewer files than the pull request changed")
	errPatchWithheld    = errors.New("github withheld the patch for files with changes")
	errUnexpectedStatus = errors.New("unexpected file status")
)

type prByNumberResponse struct {
	HeadRefOid          string `json:"headRefOid"`
	HeadRepositoryOwner struct {
		Login string `json:"login"`
	} `json:"headRepositoryOwner"`
	prResponse
}

// GetPR reads one pull request by number, including the head commit a review
// anchors to. It does not cache: the cached readers key on a branch and a
// remote, which a lookup by number cannot supply, and a review that reads a
// stale head would anchor its comments to the wrong commit.
//
// The repository is named as owner/name in remoteRepo, for a caller holding
// pull requests from several repositories at once or standing outside a
// checkout of this one. Empty leaves the repository to gh's own resolution, which reads the
// remotes of repoPath and picks a fork's upstream correctly.
func GetPR(ctx context.Context, repoPath, remoteRepo string, number int) (*forge.PullRequest, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath),
		against(remoteRepo, "pr", "view", strconv.Itoa(number),
			"--json", "number,title,state,url,isDraft,mergeStateStatus,headRefName,headRefOid,"+
				"headRepositoryOwner,baseRefName,statusCheckRollup")...)
	if err != nil {
		return nil, fmt.Errorf("reading pull request #%d: %w", number, err)
	}

	var resp prByNumberResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parsing gh pr view output: %w", err)
	}

	return &forge.PullRequest{
		Number:        resp.Number,
		Title:         resp.Title,
		State:         resp.State,
		URL:           resp.URL,
		IsDraft:       resp.IsDraft,
		Mergeable:     resp.MergeStateStatus,
		HeadRef:       resp.HeadRefName,
		HeadSHA:       resp.HeadRefOid,
		HeadRepoOwner: resp.HeadRepositoryOwner.Login,
		BaseRef:       resp.BaseRefName,
		Checks:        parseChecks(resp.StatusCheckRollup),
	}, nil
}

// PRDiff returns the pull request's cumulative unified diff against its merge
// base, which is what review comment line numbers are relative to. The bytes
// are what a review anchors against, so callers keep them verbatim rather than
// re-rendering.
//
// Never pass --patch: that returns a format-patch series, one diff per commit,
// where a file touched twice appears twice and the line numbers are the
// intermediate commit's rather than the head's.
//
// Past GitHub's 300-file limit it rebuilds the diff from the files API, which
// carries no index or mode lines.
func PRDiff(ctx context.Context, repoPath, remoteRepo string, number int) ([]byte, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath),
		against(remoteRepo, "pr", "diff", strconv.Itoa(number))...)
	if err != nil && strings.Contains(err.Error(), diffTooLargeMarker) {
		out, err = diffFromFilesAPI(ctx, repoPath, remoteRepo, number)
	}

	if err != nil {
		return nil, fmt.Errorf("reading the diff of pull request #%d: %w", number, err)
	}

	return out, nil
}

func diffFromFilesAPI(ctx context.Context, repoPath, remoteRepo string, number int) ([]byte, error) {
	repo := remoteRepo
	if repo == "" {
		repo = "{owner}/{repo}"
	}

	files, err := listPRFiles(ctx, repoPath, repo, number)
	if err != nil {
		return nil, err
	}

	changed, err := changedFileCount(ctx, repoPath, repo, number)
	if err != nil {
		return nil, err
	}

	if missing := changed - len(files); missing > 0 {
		return nil, fmt.Errorf("%w: %d of %d missing", errFilesMissing, missing, changed)
	}

	return renderFileDiffs(files)
}

type apiFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Patch            string `json:"patch"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
}

func listPRFiles(ctx context.Context, repoPath, repo string, number int) ([]apiFile, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "api", "--paginate",
		fmt.Sprintf("repos/%s/pulls/%d/files?per_page=100", repo, number))
	if err != nil {
		return nil, fmt.Errorf("listing the files: %w", err)
	}

	var files []apiFile

	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var page []apiFile

		err = dec.Decode(&page)
		if errors.Is(err, io.EOF) {
			return files, nil
		}

		if err != nil {
			return nil, fmt.Errorf("parsing the files: %w", err)
		}

		files = append(files, page...)
	}
}

func changedFileCount(ctx context.Context, repoPath, repo string, number int) (int, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "api",
		fmt.Sprintf("repos/%s/pulls/%d", repo, number))
	if err != nil {
		return 0, fmt.Errorf("reading the changed file count: %w", err)
	}

	var pr struct {
		ChangedFiles int `json:"changed_files"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return 0, fmt.Errorf("parsing the changed file count: %w", err)
	}

	return pr.ChangedFiles, nil
}

func renderFileDiffs(files []apiFile) ([]byte, error) {
	var (
		b        strings.Builder
		withheld []string
	)

	for _, f := range files {
		if f.Patch == "" && f.Additions+f.Deletions > 0 {
			withheld = append(withheld, f.Filename)
			continue
		}

		if err := writeFileDiff(&b, f); err != nil {
			return nil, err
		}
	}

	if len(withheld) > 0 {
		return nil, fmt.Errorf("%w: %s", errPatchWithheld, strings.Join(withheld, ", "))
	}

	return []byte(b.String()), nil
}

func writeFileDiff(b *strings.Builder, f apiFile) error {
	renamed := f.Status == "renamed"

	oldName := f.Filename
	if renamed {
		oldName = f.PreviousFilename
	}

	fmt.Fprintf(b, "diff --git a/%s b/%s\n", oldName, f.Filename)
	if renamed {
		fmt.Fprintf(b, "rename from %s\nrename to %s\n", oldName, f.Filename)
	}

	oldSide, newSide := "a/"+oldName, "b/"+f.Filename
	if f.Status == "added" {
		oldSide = devNull
	}

	if f.Status == "removed" {
		newSide = devNull
	}

	if f.Patch != "" {
		fmt.Fprintf(b, "--- %s\n+++ %s\n%s\n", oldSide, newSide, f.Patch)

		return nil
	}

	switch f.Status {
	case "added", "removed", "modified":
		fmt.Fprintf(b, "Binary files %s and %s differ\n", oldSide, newSide)

		return nil
	case "renamed", "changed", "unchanged":
		return nil
	default:
		return fmt.Errorf("%w %q: %s", errUnexpectedStatus, f.Status, f.Filename)
	}
}

// against names the repository when the caller supplied one, so the same call
// works from a checkout of it and from anywhere else. The flag goes last
// because gh accepts --repo on the subcommand rather than on the root command.
func against(remoteRepo string, args ...string) []string {
	if remoteRepo == "" {
		return args
	}

	return append(args, "--repo", remoteRepo)
}
