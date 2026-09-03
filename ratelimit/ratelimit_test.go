package ratelimit_test

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/aragonite/ratelimit"
)

const (
	userURL = "https://api.github.com/user"
	gqlURL  = "https://api.github.com/graphql"
)

type stub struct {
	respond func(call int) *http.Response
	calls   int
}

func (s *stub) RoundTrip(*http.Request) (*http.Response, error) {
	s.calls++

	return s.respond(s.calls), nil
}

// headerOf canonicalizes keys the way a parsed response does, which a literal
// http.Header map does not.
func headerOf(pairs map[string]string) http.Header {
	header := http.Header{}
	for key, value := range pairs {
		header.Set(key, value)
	}

	return header
}

func response(status int, header http.Header) *http.Response {
	if header == nil {
		header = http.Header{}
	}

	return &http.Response{StatusCode: status, Header: header, Body: http.NoBody}
}

func request(t *testing.T, method, url string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, url, http.NoBody)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}

	return req
}

func TestTransport(t *testing.T) {
	t.Parallel()

	now := time.Now()
	reset := strconv.FormatInt(now.Add(time.Hour).Unix(), 10)

	tests := []struct {
		header    http.Header
		name      string
		status    int
		wantLimit bool
	}{
		{
			name:   "exhausted core allowance",
			status: http.StatusForbidden,
			header: headerOf(map[string]string{
				ratelimit.HeaderRemaining: "0",
				ratelimit.HeaderReset:     reset,
				ratelimit.HeaderLimit:     "5000",
				ratelimit.HeaderResource:  ratelimit.PoolCore,
			}),
			wantLimit: true,
		},
		{
			name:      "secondary limit via Retry-After",
			status:    http.StatusTooManyRequests,
			header:    headerOf(map[string]string{ratelimit.HeaderRetryAfter: "60"}),
			wantLimit: true,
		},
		{
			name:   "forbidden for permissions, not allowance",
			status: http.StatusForbidden,
			header: headerOf(map[string]string{ratelimit.HeaderRemaining: "4999"}),
		},
		{name: "ordinary success", status: http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			base := &stub{respond: func(int) *http.Response { return response(tc.status, tc.header) }}
			rt := ratelimit.Transport(base)
			req := request(t, http.MethodGet, userURL)

			_, err := rt.RoundTrip(req) //nolint:bodyclose // the stub serves http.NoBody

			var limitErr *ratelimit.Error
			if got := errors.As(err, &limitErr); got != tc.wantLimit {
				t.Fatalf("RoundTrip() rate-limit error = %v (err %v), want %v", got, err, tc.wantLimit)
			}

			if !tc.wantLimit {
				return
			}

			// A second call must not spend a request to relearn the same window.
			//nolint:bodyclose // the stub serves http.NoBody
			if _, err := rt.RoundTrip(req); !errors.As(err, &limitErr) {
				t.Fatalf("second RoundTrip() error = %v, want the recorded *Error", err)
			}

			if base.calls != 1 {
				t.Errorf("base transport calls = %d, want 1", base.calls)
			}
		})
	}
}

// The pools are separate allowances, so being out of core must not stop a
// GraphQL query that still has its own.
func TestTransportBlocksPerPool(t *testing.T) {
	t.Parallel()

	now := time.Now()

	base := &stub{respond: func(call int) *http.Response {
		if call == 1 {
			return response(http.StatusForbidden, headerOf(map[string]string{
				ratelimit.HeaderRemaining: "0",
				ratelimit.HeaderReset:     strconv.FormatInt(now.Add(time.Hour).Unix(), 10),
				ratelimit.HeaderResource:  ratelimit.PoolCore,
			}))
		}

		return response(http.StatusOK, nil)
	}}

	rt := ratelimit.Transport(base)

	//nolint:bodyclose // the stub serves http.NoBody
	if _, err := rt.RoundTrip(request(t, http.MethodGet, userURL)); err == nil {
		t.Fatal("RoundTrip() error = nil, want the core allowance exhausted")
	}

	resp, err := rt.RoundTrip(request(t, http.MethodPost, gqlURL)) //nolint:bodyclose // the stub serves http.NoBody
	if err != nil {
		t.Fatalf("GraphQL RoundTrip() error = %v, want it unaffected by the core pool", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GraphQL status = %d, want 200", resp.StatusCode)
	}

	//nolint:bodyclose // the stub serves http.NoBody
	if _, err := rt.RoundTrip(request(t, http.MethodGet, userURL)); err == nil {
		t.Error("second core RoundTrip() error = nil, want it still blocked")
	}
}

func TestPoolFor(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"/graphql":             ratelimit.PoolGraphQL,
		"/api/graphql":         ratelimit.PoolGraphQL,
		"/search/repositories": ratelimit.PoolSearch,
		"/repos/acme/widgets":  ratelimit.PoolCore,
		"/user":                ratelimit.PoolCore,
	}

	for path, want := range tests {
		if got := ratelimit.PoolFor(path); got != want {
			t.Errorf("PoolFor(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestErrorNamesTheResetTime(t *testing.T) {
	t.Parallel()

	limitErr := &ratelimit.Error{Resource: ratelimit.PoolCore, Limit: 5000, RetryAt: time.Now().Add(11 * time.Minute)}

	for _, want := range []string{"core", "5000", "resets at"} {
		if !strings.Contains(limitErr.Error(), want) {
			t.Errorf("Error() = %q, want it to mention %s", limitErr.Error(), want)
		}
	}
}
