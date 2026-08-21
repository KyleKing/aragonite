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
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	URL       string `json:"url"`
	IsDraft   bool   `json:"is_draft"`
	Mergeable string `json:"mergeable,omitempty"`
	HeadRef   string `json:"head_ref"`
	// HeadSHA is the head branch's tip commit. A review anchors to it, so a
	// reader that only lists pull requests leaves it empty rather than paying
	// for it.
	HeadSHA         string       `json:"head_sha,omitempty"`
	HeadRepoOwner   string       `json:"head_repo_owner,omitempty"`
	BaseRef         string       `json:"base_ref"`
	Checks          ChecksStatus `json:"checks"`
	ReviewDecision  string       `json:"review_decision,omitempty"`
	ApprovedBy      []string     `json:"approved_by,omitempty"`
	ChangesRequests int          `json:"changes_requests,omitempty"`
	Reviewers       []string     `json:"reviewers,omitempty"`
	Activity        *PRActivity  `json:"activity,omitempty"`
	// Repo, Author, and UpdatedAt are carried by rows a saved search produced,
	// where the list spans repositories and the owner of a pull request is not
	// the person reading it. A row read for one repo's own panel leaves them
	// empty.
	Repo      string    `json:"repo,omitempty"`
	Author    string    `json:"author,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
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
	Author string    `json:"author"`
	At     time.Time `json:"at"`
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
	Name        string
	Workflow    string
	Status      string
	Conclusion  string
	StartedAt   time.Time
	CompletedAt time.Time
}

// PRComment is a single issue comment on a pull request.
type PRComment struct {
	Author    string
	Body      string
	CreatedAt time.Time
}

// PRDetail holds the full detail view state for a single pull request.
type PRDetail struct {
	PullRequest
	Body      string
	Author    string
	Assignees []string
	Reviewers []string
	CreatedAt time.Time
	UpdatedAt time.Time
	Additions int
	Deletions int
	// Comments counts issue comments on the pull request; LatestComment is
	// the most recent of them, nil when there are none.
	Comments      int
	LatestComment *PRComment
	CheckDetails  []CheckDetail
	ReviewsURL    string
}

// WorkflowRun summarizes a single CI workflow run.
type WorkflowRun struct {
	ID         int64
	Name       string
	Status     string
	Conclusion string
	URL        string
	CreatedAt  time.Time
	UpdatedAt  time.Time
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
	ID          int64     `json:"id"`
	Workflow    string    `json:"workflow"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	URL         string    `json:"url"`
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	FailingJobs []string  `json:"failing_jobs,omitempty"`
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
	Reviewers      []string
	ReviewDecision string
	Additions      int
	Deletions      int
}
