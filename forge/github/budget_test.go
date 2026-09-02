package github_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge/github"
)

// The pools are separate allowances rather than slices of one, which is the
// whole reason to read them: a tool out of core can still make its GraphQL
// calls, and one about to burst can ask whether the burst is affordable.
func TestBudgetsReadsEachPoolSeparately(t *testing.T) {
	t.Parallel()

	reset := time.Date(2026, time.September, 2, 5, 0, 0, 0, time.UTC)
	payload := `{"resources":{
		"core":{"limit":5000,"remaining":0,"reset":` + itoa(reset.Unix()) + `},
		"graphql":{"limit":5000,"remaining":5000,"reset":` + itoa(reset.Unix()) + `},
		"search":{"limit":30,"remaining":29,"reset":` + itoa(reset.Unix()) + `}}}`

	var asked string

	ctx := github.WithRunner(t.Context(),
		func(_ context.Context, _ string, _ []string, a ...string) ([]byte, error) {
			asked = strings.Join(a, " ")

			return []byte(payload), nil
		})

	got, err := github.Budgets(ctx, "/repo")
	if err != nil {
		t.Fatal(err)
	}

	if asked != "api rate_limit" {
		t.Errorf("asked %q, want the rate_limit endpoint, which is the one read GitHub does not charge for", asked)
	}

	if got.Core.Remaining != 0 || got.GraphQL.Remaining != 5000 || got.Search.Remaining != 29 {
		t.Errorf("read %+v, want the three pools apart", got)
	}

	if got.Core.Covers(1) {
		t.Error("an exhausted pool said it covers a call")
	}

	if !got.GraphQL.Covers(70) {
		t.Error("a full pool said it cannot cover a burst")
	}

	if got.Core.In(reset.Add(-time.Minute)) != time.Minute {
		t.Errorf("the wait is %v, want a minute", got.Core.In(reset.Add(-time.Minute)))
	}

	if got.Core.In(reset.Add(time.Minute)) != 0 {
		t.Error("a pool past its reset still reported a wait")
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
