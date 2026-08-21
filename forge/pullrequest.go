// Package forge holds the pull request model shared by every tool that reads a
// code host. Today that host is GitHub, read through the gh CLI.
package forge

import (
	"strings"
	"time"
)

// Display values shared by PullRequest/ChecksStatus/WorkflowSummary and the views
// that render them, so both sides compare against the same constant.
const (
	PRStatusClosed         = "CLOSED"
	PRStatusMerged         = "MERGED"
	PRStatusOpen           = "OPEN"
	ReviewApproved         = "approved"
	ReviewChangesRequested = "changes requested"
	StatusCompleted        = "completed"
	StatusFailing          = "failing"
	StatusPassing          = "passing"
)

// PullRequest summarizes a pull request for the repo list and detail views.
type PullRequest struct {
	UpdatedAt       time.Time    `json:"updated_at,omitzero"`
	Activity        *PRActivity  `json:"activity,omitempty"`
	HeadRef         string       `json:"head_ref"`
	URL             string       `json:"url"`
	State           string       `json:"state"`
	Mergeable       string       `json:"mergeable,omitempty"`
	Title           string       `json:"title"`
	HeadSHA         string       `json:"head_sha,omitempty"`
	HeadRepoOwner   string       `json:"head_repo_owner,omitempty"`
	BaseRef         string       `json:"base_ref"`
	Author          string       `json:"author,omitempty"`
	ReviewDecision  string       `json:"review_decision,omitempty"`
	Repo            string       `json:"repo,omitempty"`
	ApprovedBy      []string     `json:"approved_by,omitempty"`
	Reviewers       []string     `json:"reviewers,omitempty"`
	Checks          ChecksStatus `json:"checks"`
	ChangesRequests int          `json:"changes_requests,omitempty"`
	Number          int          `json:"number"`
	IsDraft         bool         `json:"is_draft"`
}

// FromFork reports whether the pull request's head branch lives in someone
// else's fork rather than in owner's own repository. A fork's head ref shares
// a namespace with local branches ("master" is common), so a name match alone
// is not evidence the branch is here.
func (p PullRequest) FromFork(owner string) bool {
	return p.HeadRepoOwner != "" && owner != "" && !strings.EqualFold(p.HeadRepoOwner, owner)
}

// HeadLabel names where the head branch lives, qualifying it with the owner
// when the pull request comes from a fork.
func (p PullRequest) HeadLabel(owner string) string {
	if p.FromFork(owner) {
		return p.HeadRepoOwner + ":" + p.HeadRef
	}

	return p.HeadRef
}

// MatchesUpstream reports whether upstream (a branch's "remote/name" tracking
// ref) points at this pull request's head branch, so a local branch never has
// to share its name with the head ref to prove it holds the same pull
// request. A fork's head ref lives in a different remote, so it never
// matches on upstream alone.
func (p PullRequest) MatchesUpstream(owner, upstream string) bool {
	if p.FromFork(owner) || upstream == "" {
		return false
	}

	_, name, found := strings.Cut(upstream, "/")
	if !found {
		name = upstream
	}

	return name == p.HeadRef
}

// PRActivity is the most recent comment or review on a pull request: the
// signal for who a pull request is waiting on.
type PRActivity struct {
	At     time.Time `json:"at"`
	Author string    `json:"author"`
}

// NeedsReviewer reports whether an open, non-draft pull request has nobody
// currently requested to review it, the case GitHub's own reviewDecision
// leaves unflagged since it only tracks reviews already submitted.
func (p PullRequest) NeedsReviewer() bool {
	return p.State == PRStatusOpen && !p.IsDraft && len(p.Reviewers) == 0
}

// ChecksStatus tallies a pull request's CI check outcomes.
type ChecksStatus struct {
	Total   int `json:"total"`
	Passing int `json:"passing"`
	Failing int `json:"failing"`
	Pending int `json:"pending"`
	Skipped int `json:"skipped"`
}

// CheckDetail is a single CI check on a pull request.
type CheckDetail struct {
	StartedAt   time.Time
	CompletedAt time.Time
	Name        string
	Workflow    string
	Status      string
	Conclusion  string
}

// PRComment is a single issue comment on a pull request.
type PRComment struct {
	CreatedAt time.Time
	Author    string
	Body      string
}

// PRDetail holds the full detail view state for a single pull request.
type PRDetail struct {
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LatestComment *PRComment
	ReviewsURL    string
	Body          string
	Author        string
	Assignees     []string
	CheckDetails  []CheckDetail
	Reviewers     []string
	PullRequest
	Deletions int
	Comments  int
	Additions int
}

// WorkflowRun summarizes a single CI workflow run.
type WorkflowRun struct {
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Name       string
	Status     string
	Conclusion string
	URL        string
	ID         int64
}

// DefaultBranchCI is the CI state of a repo's default branch head: the latest
// run of each workflow on that commit.
type DefaultBranchCI struct {
	Branch    string          `json:"branch"`
	SHA       string          `json:"sha"`
	Workflows []CIWorkflowRun `json:"workflows"`
}

// CIWorkflowRun is one workflow's latest run on a commit.
type CIWorkflowRun struct {
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Workflow    string    `json:"workflow"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	URL         string    `json:"url"`
	FailingJobs []string  `json:"failing_jobs,omitempty"`
	ID          int64     `json:"id"`
}

// WorkflowSummary aggregates the CI workflow runs for a commit. Every run
// lands in exactly one of Passing, Skipped, Canceled, Failing, or
// InProgress, so the five sum to Total.
type WorkflowSummary struct {
	Runs       []WorkflowRun
	Total      int
	Passing    int
	Skipped    int
	Canceled   int
	Failing    int
	InProgress int
}

// PRPreview is the little a PRs-tab row needs to show under the table: enough
// to judge whether the pull request is worth opening, and nothing that costs
// GitHub a second query to answer.
type PRPreview struct {
	Body           string
	ReviewDecision string
	Reviewers      []string
	Additions      int
	Deletions      int
}
