package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var errRepoFormat = errors.New("repo must be owner/name")

// Merge queue event types, as QueueEvent.Type reports them.
const (
	QueueAdded   = "added"
	QueueRemoved = "removed"
	QueueMerged  = "merged"
)

// QueueEvent is one merge queue event on a pull request's timeline. Reason is set
// on removals ("merged", "failed_checks", "merge_conflict", and so on).
type QueueEvent struct {
	At     time.Time
	Type   string
	Reason string
}

const (
	mergeQueueBatch = 25
	timelineQuery   = `p%d:pullRequest(number:%d){timelineItems(first:50,` +
		`itemTypes:[ADDED_TO_MERGE_QUEUE_EVENT,REMOVED_FROM_MERGE_QUEUE_EVENT,MERGED_EVENT]){nodes{__typename ` +
		`... on AddedToMergeQueueEvent{createdAt} ... on RemovedFromMergeQueueEvent{createdAt reason} ` +
		`... on MergedEvent{createdAt}}}} `
)

// MergeQueueEvents reads the merge queue events of pull requests, which the REST
// API does not expose, 25 pull requests to a GraphQL query. A pull request with
// no events, or that does not exist, maps to an empty slice. The repo argument
// is owner/name.
func MergeQueueEvents(ctx context.Context, repoPath, repo string, numbers []int) (map[int][]QueueEvent, error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok {
		return nil, fmt.Errorf("%w: %q", errRepoFormat, repo)
	}

	out := make(map[int][]QueueEvent, len(numbers))

	for start := 0; start < len(numbers); start += mergeQueueBatch {
		batch := numbers[start:min(start+mergeQueueBatch, len(numbers))]

		var query strings.Builder

		fmt.Fprintf(&query, "query{repository(owner:%q,name:%q){", owner, name)

		for _, n := range batch {
			fmt.Fprintf(&query, timelineQuery, n, n)
		}

		query.WriteString("}}")

		stdout, err := runGH(ctx, repoPath, nil, "api", "graphql", "-f", "query="+query.String())
		if err != nil {
			return nil, fmt.Errorf("reading merge queue events: %w", err)
		}

		if err := parseQueueEvents(stdout, batch, out); err != nil {
			return nil, err
		}
	}

	return out, nil
}

func parseQueueEvents(stdout []byte, batch []int, out map[int][]QueueEvent) error {
	var payload struct {
		Data struct {
			Repository map[string]*struct {
				TimelineItems struct {
					Nodes []struct {
						CreatedAt time.Time `json:"createdAt"`  //nolint:tagliatelle // GraphQL field name
						Type      string    `json:"__typename"` //nolint:tagliatelle // GraphQL field name
						Reason    string    `json:"reason"`
					} `json:"nodes"`
				} `json:"timelineItems"` //nolint:tagliatelle // GraphQL field name
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal(stdout, &payload); err != nil {
		return fmt.Errorf("parsing merge queue events: %w", err)
	}

	kinds := map[string]string{
		"AddedToMergeQueueEvent": QueueAdded, "RemovedFromMergeQueueEvent": QueueRemoved, "MergedEvent": QueueMerged,
	}

	for _, n := range batch {
		out[n] = nil

		pr := payload.Data.Repository[fmt.Sprintf("p%d", n)]
		if pr == nil {
			continue
		}

		for _, node := range pr.TimelineItems.Nodes {
			out[n] = append(out[n], QueueEvent{At: node.CreatedAt, Type: kinds[node.Type], Reason: node.Reason})
		}
	}

	return nil
}
