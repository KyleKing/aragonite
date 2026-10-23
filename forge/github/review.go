package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"
)

const (
	devNull            = "/dev/null"
	diffTooLargeMarker = "exceeded the maximum number of files"
	statusAdded        = "added"
	statusRemoved      = "removed"
	tempFileMode       = 0o600
)

var (
	errBlobEncoding     = errors.New("github returned a blob that is not base64")
	errFilesMissing     = errors.New("the files API returned fewer files than the pull request changed")
	errNoSHA            = errors.New("github returned no commit or blob sha")
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
// Past GitHub's 300-file limit it rebuilds the diff from the files API and git
// blobs, and writes no index or mode lines.
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

	pull, err := readPull(ctx, repoPath, repo, number)
	if err != nil {
		return nil, err
	}

	if missing := pull.ChangedFiles - len(files); missing > 0 {
		return nil, fmt.Errorf("%w: %d of %d missing", errFilesMissing, missing, pull.ChangedFiles)
	}

	base, err := mergeBaseSHA(ctx, repoPath, repo, pull.Base.SHA, pull.Head.SHA)
	if err != nil {
		return nil, err
	}

	return renderFileDiffs(ctx, fileSource{repoPath: repoPath, repo: repo, mergeBase: base}, files)
}

type apiRef struct {
	SHA string `json:"sha"`
}

type apiPull struct {
	Base         apiRef `json:"base"`
	Head         apiRef `json:"head"`
	ChangedFiles int    `json:"changed_files"`
}

type apiFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	SHA              string `json:"sha"`
	Patch            string `json:"patch"`
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

func readPull(ctx context.Context, repoPath, repo string, number int) (apiPull, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "api",
		fmt.Sprintf("repos/%s/pulls/%d", repo, number))
	if err != nil {
		return apiPull{}, fmt.Errorf("reading the pull request: %w", err)
	}

	var pull apiPull
	if err := json.Unmarshal(out, &pull); err != nil {
		return apiPull{}, fmt.Errorf("parsing the pull request: %w", err)
	}

	return pull, nil
}

func mergeBaseSHA(ctx context.Context, repoPath, repo, base, head string) (string, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath), "api",
		fmt.Sprintf("repos/%s/compare/%s...%s", repo, base, head), "--jq", ".merge_base_commit.sha")
	if err != nil {
		return "", fmt.Errorf("finding the merge base: %w", err)
	}

	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", fmt.Errorf("%w for %s...%s", errNoSHA, base, head)
	}

	return sha, nil
}

type fileSource struct {
	repoPath  string
	repo      string
	mergeBase string
}

func (s fileSource) blob(ctx context.Context, sha string) ([]byte, error) {
	out, err := runGH(ctx, s.repoPath, vcs.GetGitHubEnv(s.repoPath), "api",
		fmt.Sprintf("repos/%s/git/blobs/%s", s.repo, sha))
	if err != nil {
		return nil, fmt.Errorf("reading blob %s: %w", sha, err)
	}

	var blob struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(out, &blob); err != nil {
		return nil, fmt.Errorf("parsing blob %s: %w", sha, err)
	}

	if blob.Encoding != "base64" {
		return nil, fmt.Errorf("%w: blob %s is %q", errBlobEncoding, sha, blob.Encoding)
	}

	data, err := base64.StdEncoding.DecodeString(blob.Content)
	if err != nil {
		return nil, fmt.Errorf("decoding blob %s: %w", sha, err)
	}

	return data, nil
}

func (s fileSource) atMergeBase(ctx context.Context, path string) ([]byte, error) {
	out, err := runGH(ctx, s.repoPath, vcs.GetGitHubEnv(s.repoPath), "api",
		fmt.Sprintf("repos/%s/contents/%s?ref=%s", s.repo, (&url.URL{Path: path}).EscapedPath(), s.mergeBase),
		"--jq", ".sha")
	if err != nil {
		return nil, fmt.Errorf("reading %s at the merge base: %w", path, err)
	}

	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return nil, fmt.Errorf("%w for %s at the merge base", errNoSHA, path)
	}

	return s.blob(ctx, sha)
}

func renderFileDiffs(ctx context.Context, src fileSource, files []apiFile) ([]byte, error) {
	var b strings.Builder

	for _, f := range files {
		if err := writeFileDiff(ctx, &b, src, f); err != nil {
			return nil, err
		}
	}

	return []byte(b.String()), nil
}

func writeFileDiff(ctx context.Context, b *strings.Builder, src fileSource, f apiFile) error {
	renamed := f.Status == "renamed"

	oldName := f.Filename
	if renamed {
		oldName = f.PreviousFilename
	}

	fmt.Fprintf(b, "diff --git a/%s b/%s\n", oldName, f.Filename)
	if renamed {
		fmt.Fprintf(b, "rename from %s\nrename to %s\n", oldName, f.Filename)
	}

	if f.Patch != "" {
		oldSide, newSide := diffSides(f, oldName)
		fmt.Fprintf(b, "--- %s\n+++ %s\n%s\n", oldSide, newSide, f.Patch)

		return nil
	}

	switch f.Status {
	case statusAdded, statusRemoved, "modified", "renamed", "changed", "unchanged":
		return writeBlobDiff(ctx, b, src, f, oldName)
	default:
		return fmt.Errorf("%w %q: %s", errUnexpectedStatus, f.Status, f.Filename)
	}
}

func writeBlobDiff(ctx context.Context, b *strings.Builder, src fileSource, f apiFile, oldName string) error {
	hunks, binary, err := diffBlobs(ctx, src, f, oldName)
	if err != nil {
		return err
	}

	oldSide, newSide := diffSides(f, oldName)

	switch {
	case binary:
		fmt.Fprintf(b, "Binary files %s and %s differ\n", oldSide, newSide)
	case len(hunks) > 0:
		fmt.Fprintf(b, "--- %s\n+++ %s\n%s", oldSide, newSide, hunks)
	}

	return nil
}

func diffBlobs(ctx context.Context, src fileSource, f apiFile, oldName string) ([]byte, bool, error) {
	dir, err := os.MkdirTemp("", "aragonite-diff-")
	if err != nil {
		return nil, false, fmt.Errorf("creating a temp dir: %w", err)
	}

	hunks, binary, err := diffInDir(ctx, dir, src, f, oldName)

	return hunks, binary, errors.Join(err, os.RemoveAll(dir))
}

func diffInDir(ctx context.Context, dir string, src fileSource, f apiFile, oldName string) ([]byte, bool, error) {
	oldPath, newPath := devNull, devNull

	if f.Status != statusAdded {
		data, err := src.atMergeBase(ctx, oldName)
		if err != nil {
			return nil, false, err
		}

		if oldPath, err = writeTemp(dir, "old", data); err != nil {
			return nil, false, err
		}
	}

	if f.Status != statusRemoved {
		data, err := src.blob(ctx, f.SHA)
		if err != nil {
			return nil, false, err
		}

		if newPath, err = writeTemp(dir, "new", data); err != nil {
			return nil, false, err
		}
	}

	out, err := gitNoIndex(ctx, dir, oldPath, newPath)
	if err != nil {
		return nil, false, err
	}

	hunks, binary := splitGitDiff(out)

	return hunks, binary, nil
}

func writeTemp(dir, name string, data []byte) (string, error) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, tempFileMode); err != nil {
		return "", fmt.Errorf("writing %s: %w", name, err)
	}

	return path, nil
}

// git exits 1 when the files differ, which is an answer rather than a failure.
func gitNoIndex(ctx context.Context, dir, oldPath, newPath string) ([]byte, error) {
	args := []string{"diff", "--no-index", "--no-color", "--no-ext-diff", "--unified=3", oldPath, newPath}
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- fixed git binary, paths this package wrote
	cmd.Dir = dir

	out, err := cmd.Output()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return out, nil
	}

	if err != nil {
		return nil, fmt.Errorf("running git diff --no-index: %w", err)
	}

	return out, nil
}

func splitGitDiff(out []byte) ([]byte, bool) {
	if bytes.Contains(out, []byte("\nBinary files ")) {
		return nil, true
	}

	if at := bytes.Index(out, []byte("\n@@ ")); at >= 0 {
		return out[at+1:], false
	}

	return nil, false
}

func diffSides(f apiFile, oldName string) (string, string) {
	oldSide, newSide := "a/"+oldName, "b/"+f.Filename
	if f.Status == statusAdded {
		oldSide = devNull
	}

	if f.Status == statusRemoved {
		newSide = devNull
	}

	return oldSide, newSide
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
