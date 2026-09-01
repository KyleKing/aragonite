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
func PRDiff(ctx context.Context, repoPath, remoteRepo string, number int) ([]byte, error) {
	out, err := runGH(ctx, repoPath, vcs.GetGitHubEnv(repoPath),
		against(remoteRepo, "pr", "diff", strconv.Itoa(number))...)
	if err != nil {
		return nil, fmt.Errorf("reading the diff of pull request #%d: %w", number, err)
	}

	return out, nil
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
