package github_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge/github"
)

// Queue events come back in order with their reasons, a pull request that
// never queued maps to an empty slice, and pull requests are read in batches.
func TestMergeQueueEvents_ReadsReasonsAndBatchesTheQueries(t *testing.T) {
	t.Parallel()

	var queries int

	ctx := stub(t, func(args []string) ([]byte, error) {
		queries++

		if !strings.Contains(args[len(args)-1], `repository(owner:"o",name:"r")`) {
			t.Errorf("query = %q, want the owner and name", args[len(args)-1])
		}

		return []byte(`{"data":{"repository":{"p1":{"timelineItems":{"nodes":[` +
			`{"__typename":"AddedToMergeQueueEvent","createdAt":"2026-01-01T00:00:00Z"},` +
			`{"__typename":"RemovedFromMergeQueueEvent","createdAt":"2026-01-01T00:02:00Z",` +
			`"reason":"failed_checks"}]}},` +
			`"p2":null}}}`), nil
	})

	numbers := make([]int, 0, 30)
	numbers = append(numbers, 1, 2)

	for n := 100; len(numbers) < 30; n++ {
		numbers = append(numbers, n)
	}

	events, err := github.MergeQueueEvents(ctx, "", "o/r", numbers)
	if err != nil {
		t.Fatal(err)
	}

	if queries != 2 {
		t.Errorf("queries = %d, want 2 batches for 30 pull requests", queries)
	}

	got := events[1]
	if len(got) != 2 || got[0].Type != github.QueueAdded || got[1].Reason != "failed_checks" ||
		got[1].At.Sub(got[0].At) != 2*time.Minute {
		t.Errorf("events = %+v, want an addition then a failed_checks removal two minutes later", got)
	}

	if e, ok := events[2]; !ok || len(e) != 0 {
		t.Errorf("null pull request = %v (present %v), want an empty entry", e, ok)
	}

	if _, err := github.MergeQueueEvents(ctx, "", "no-slash", numbers); err == nil {
		t.Error("a repo without owner/name was accepted")
	}
}
