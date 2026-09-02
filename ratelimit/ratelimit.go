// Package ratelimit keeps a GitHub API client from spending requests to
// rediscover that it has none left.
//
// GitHub answers an exhausted allowance with a 403 whose body reads like a
// permissions failure, so a client that does not inspect the headers reports
// the wrong cause and a retry loop spends a request per attempt learning the
// same thing. Transport turns that response into a typed error naming when the
// allowance returns, and refuses further requests to that pool until it does.
//
// The pools are separate allowances rather than slices of one, so being out of
// core does not stop a GraphQL query. Pair this with a proactive read of what
// is left (forge/github.Budgets) to decide whether a burst is affordable
// before starting it.
package ratelimit

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Header names GitHub sets on every API response.
const (
	HeaderRemaining  = "X-RateLimit-Remaining"
	HeaderReset      = "X-RateLimit-Reset"
	HeaderLimit      = "X-RateLimit-Limit"
	HeaderResource   = "X-RateLimit-Resource"
	HeaderRetryAfter = "Retry-After"
)

// Pool names one of GitHub's separate hourly allowances.
const (
	PoolCore    = "core"
	PoolGraphQL = "graphql"
	PoolSearch  = "search"
)

// Error reports an exhausted allowance and when it returns. Retrying before
// RetryAt fails without spending a request, so a caller should show the time
// rather than offering an immediate retry.
type Error struct {
	RetryAt  time.Time
	Resource string
	Limit    int
}

func (e *Error) Error() string {
	resource := e.Resource
	if resource == "" {
		resource = "api"
	}

	wait := time.Until(e.RetryAt).Round(time.Second)
	if wait < 0 {
		wait = 0
	}

	//nolint:gosmopolitan // a wall-clock time a person reads is only useful in their own zone
	return fmt.Sprintf("GitHub %s rate limit exhausted (%d/hour); resets at %s, in %s",
		resource, e.Limit, e.RetryAt.Local().Format(time.Kitchen), wait)
}

// Transport wraps base so an exhausted allowance surfaces as *Error and later
// requests to that pool fail immediately until it resets. Requests to a pool
// with allowance left are unaffected.
func Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	return &transport{base: base, now: time.Now, blocked: map[string]*Error{}}
}

type transport struct {
	base    http.RoundTripper
	now     func() time.Time
	blocked map[string]*Error
	mu      sync.Mutex
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	resource := PoolFor(req.URL.Path)
	if blocked := t.current(resource); blocked != nil {
		return nil, blocked
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err //nolint:wrapcheck // transparent proxy: http.Client wraps this in *url.Error itself
	}

	limitErr := Exhausted(resp, t.now())
	if limitErr == nil {
		return resp, nil
	}

	if limitErr.Resource == "" {
		limitErr.Resource = resource
	}

	t.block(limitErr)

	if err := drain(resp); err != nil {
		return nil, errors.Join(limitErr, err)
	}

	return nil, limitErr
}

func (t *transport) current(resource string) *Error {
	t.mu.Lock()
	defer t.mu.Unlock()

	blocked, found := t.blocked[resource]
	if !found {
		return nil
	}

	if !t.now().Before(blocked.RetryAt) {
		delete(t.blocked, resource)

		return nil
	}

	return blocked
}

func (t *transport) block(limitErr *Error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.blocked[limitErr.Resource] = limitErr
}

// PoolFor names the allowance a request path spends.
func PoolFor(path string) string {
	switch {
	case strings.HasSuffix(path, "/graphql"):
		return PoolGraphQL
	case strings.HasPrefix(path, "/search/"):
		return PoolSearch
	default:
		return PoolCore
	}
}

// Exhausted returns an error only when resp says the allowance is spent, and
// nil otherwise. A 403 for permissions carries no remaining-count header and
// must reach the caller unchanged rather than be reported as a rate limit.
func Exhausted(resp *http.Response, now time.Time) *Error {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return nil
	}

	limitErr := &Error{
		Resource: resp.Header.Get(HeaderResource),
		Limit:    intHeader(resp.Header.Get(HeaderLimit)),
	}

	// A secondary limit answers with Retry-After and no remaining count.
	if after := intHeader(resp.Header.Get(HeaderRetryAfter)); after > 0 {
		limitErr.RetryAt = now.Add(time.Duration(after) * time.Second)

		return limitErr
	}

	if resp.Header.Get(HeaderRemaining) != "0" {
		return nil
	}

	limitErr.RetryAt = time.Unix(int64(intHeader(resp.Header.Get(HeaderReset))), 0)

	return limitErr
}

func intHeader(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}

	return parsed
}

func drain(resp *http.Response) error {
	if resp.Body == nil {
		return nil
	}

	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return errors.Join(err, resp.Body.Close())
	}

	return resp.Body.Close() //nolint:wrapcheck // an io error on an abandoned body needs no context
}
