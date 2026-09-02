package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Budget is how much of GitHub's hourly allowance is left, per pool.
//
// The pools are separate allowances, not slices of one: a REST read and a
// GraphQL query spend different budgets, so a tool out of one can still make
// the other's calls. Search has its own small allowance again.
type Budget struct {
	Core    Allowance
	GraphQL Allowance
	Search  Allowance
}

// Allowance is one pool's state. Reset is when Remaining goes back to Limit,
// which is what a caller tells someone to wait for.
type Allowance struct {
	Reset     time.Time
	Limit     int
	Remaining int
}

// Covers reports whether the pool can pay for n more calls. A tool about to
// make a burst asks before it starts, since a burst that runs out halfway has
// spent the budget and produced nothing.
func (a Allowance) Covers(n int) bool { return a.Remaining >= n }

// In is how long until the pool refills, and zero once it has.
func (a Allowance) In(now time.Time) time.Duration {
	if !a.Reset.After(now) {
		return 0
	}

	return a.Reset.Sub(now)
}

// Budgets reads what is left of each allowance.
//
// The read is free: GitHub does not count a request for the rate limit against
// the rate limit, so a tool can ask before every burst without the asking being
// part of what it spends.
func Budgets(ctx context.Context, dir string) (Budget, error) {
	out, err := runGH(ctx, dir, nil, "api", "rate_limit")
	if err != nil {
		return Budget{}, fmt.Errorf("reading the rate limit: %w", err)
	}

	var body struct {
		Resources map[string]struct {
			Limit     int   `json:"limit"`
			Remaining int   `json:"remaining"`
			Reset     int64 `json:"reset"`
		} `json:"resources"`
	}

	if err := json.Unmarshal(out, &body); err != nil {
		return Budget{}, fmt.Errorf("reading the rate limit: %w", err)
	}

	pool := func(name string) Allowance {
		r, ok := body.Resources[name]
		if !ok {
			return Allowance{}
		}

		return Allowance{Limit: r.Limit, Remaining: r.Remaining, Reset: time.Unix(r.Reset, 0).UTC()}
	}

	return Budget{Core: pool("core"), GraphQL: pool("graphql"), Search: pool("search")}, nil
}
