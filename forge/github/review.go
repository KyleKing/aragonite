package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kyleking/aragonite/forge"
	"github.com/kyleking/aragonite/vcs"
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
func GetPR(ctx context.Context, repoPath string, number int) (*forge.PullRequest, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath),
		"pr", "view", strconv.Itoa(number),
		"--json", "number,title,state,url,isDraft,mergeStateStatus,headRefName,headRefOid,"+
			"headRepositoryOwner,baseRefName,statusCheckRollup")
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

// PRDiff returns the pull request's unified diff. The bytes are what a review
// anchors against, so callers keep them verbatim rather than re-rendering.
func PRDiff(ctx context.Context, repoPath string, number int) ([]byte, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath),
		"pr", "diff", strconv.Itoa(number), "--patch")
	if err != nil {
		return nil, fmt.Errorf("reading the diff of pull request #%d: %w", number, err)
	}

	return out, nil
}
