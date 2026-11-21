package ratelimit_test

import (
	"errors"
	"net/http"
	"strconv"
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

// failingBody fails every read and records whether it was closed.
type failingBody struct {
	err    error
	closed bool
}

func (b *failingBody) Read([]byte) (int, error) { return 0, b.err }

func (b *failingBody) Close() error {
	b.closed = true

	return nil
}

func TestTransport_TurnsAnExhaustedAllowanceIntoATypedError(t *testing.T) {
	t.Parallel()

	now := time.Now()
	resetAt := now.Add(time.Hour).Truncate(time.Second)
	reset := strconv.FormatInt(resetAt.Unix(), 10)

	tests := []struct {
		header http.Header
		want   *ratelimit.Error
		name   string
		status int
		// calls is what the base transport has served after two requests.
		calls int
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
			want:  &ratelimit.Error{Resource: ratelimit.PoolCore, Limit: 5000, RetryAt: resetAt},
			calls: 1,
		},
		{
			name:   "a 429 with no resource is charged to the request's pool",
			status: http.StatusTooManyRequests,
			header: headerOf(map[string]string{
				ratelimit.HeaderRemaining: "0",
				ratelimit.HeaderReset:     reset,
			}),
			want:  &ratelimit.Error{Resource: ratelimit.PoolCore, RetryAt: resetAt},
			calls: 1,
		},
		{
			name:   "secondary limit via Retry-After",
			status: http.StatusTooManyRequests,
			header: headerOf(map[string]string{ratelimit.HeaderRetryAfter: "60", ratelimit.HeaderRemaining: "12"}),
			want:   &ratelimit.Error{Resource: ratelimit.PoolCore},
			calls:  1,
		},
		{
			name:   "a reset already past blocks nothing further",
			status: http.StatusForbidden,
			header: headerOf(map[string]string{
				ratelimit.HeaderRemaining: "0",
				ratelimit.HeaderReset:     strconv.FormatInt(now.Add(-time.Minute).Unix(), 10),
			}),
			want:  &ratelimit.Error{Resource: ratelimit.PoolCore, RetryAt: time.Unix(now.Add(-time.Minute).Unix(), 0)},
			calls: 2,
		},
		{
			name:   "forbidden for permissions, not allowance",
			status: http.StatusForbidden,
			header: headerOf(map[string]string{ratelimit.HeaderRemaining: "4999"}),
			calls:  2,
		},
		{
			name:   "unparsable Retry-After falls back to the remaining count",
			status: http.StatusForbidden,
			header: headerOf(map[string]string{ratelimit.HeaderRetryAfter: "soon"}),
			calls:  2,
		},
		{name: "ordinary success", status: http.StatusOK, calls: 2},
		{
			name:   "a zero remaining count on success is not a limit",
			status: http.StatusOK,
			header: headerOf(map[string]string{ratelimit.HeaderRemaining: "0", ratelimit.HeaderReset: reset}),
			calls:  2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			base := &stub{respond: func(int) *http.Response { return response(tc.status, tc.header) }}
			rt := ratelimit.Transport(base)
			req := request(t, http.MethodGet, userURL)

			resp, err := rt.RoundTrip(req) //nolint:bodyclose // the stub serves http.NoBody
			if tc.want == nil {
				if err != nil || resp == nil || resp.StatusCode != tc.status {
					t.Fatalf("RoundTrip() = %v, %v, want the %d response unchanged", resp, err, tc.status)
				}
			} else {
				checkLimit(t, resp, err, tc.want, now)
			}

			//nolint:bodyclose // the stub serves http.NoBody
			_, second := rt.RoundTrip(req)
			if tc.want != nil && tc.calls == 1 && !errors.Is(second, err) {
				t.Errorf("second RoundTrip() error = %v, want the recorded %v", second, err)
			}

			if base.calls != tc.calls {
				t.Errorf("base transport calls = %d, want %d", base.calls, tc.calls)
			}
		})
	}
}

// checkLimit requires err to be exactly the *Error want describes. A
// Retry-After limit is timed from the call, so its RetryAt is checked as a
// minute past now.
func checkLimit(t *testing.T, resp *http.Response, err error, want *ratelimit.Error, now time.Time) {
	t.Helper()

	bare, ok := err.(*ratelimit.Error) //nolint:errorlint // the error must be the bare *Error, not a wrap of it
	if !ok || resp != nil {
		t.Fatalf("RoundTrip() = %v, %T(%v), want only a bare *ratelimit.Error", resp, err, err)
	}

	got := *bare

	if want.RetryAt.IsZero() {
		if wait := got.RetryAt.Sub(now); wait < time.Minute || wait > time.Minute+5*time.Second {
			t.Errorf("RetryAt = %v, want a minute after %v", got.RetryAt, now)
		}

		got.RetryAt = time.Time{}
	}

	if got.Resource != want.Resource || got.Limit != want.Limit || !got.RetryAt.Equal(want.RetryAt) {
		t.Errorf("error = %+v, want %+v", got, *want)
	}
}

// A body that cannot be drained still yields the limit, joined with the read
// failure, and is closed either way.
var errConnReset = errors.New("connection reset")

func TestTransport_JoinsAFailedDrain(t *testing.T) {
	t.Parallel()

	body := &failingBody{err: errConnReset}
	base := &stub{respond: func(int) *http.Response {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     headerOf(map[string]string{ratelimit.HeaderRetryAfter: "30"}),
			Body:       body,
		}
	}}

	//nolint:bodyclose // the transport closes the body it could not drain
	_, err := ratelimit.Transport(base).RoundTrip(request(t, http.MethodGet, userURL))

	var limitErr *ratelimit.Error
	if !errors.As(err, &limitErr) || !errors.Is(err, errConnReset) {
		t.Errorf("RoundTrip() error = %v, want the limit joined with %v", err, errConnReset)
	}

	if !body.closed {
		t.Error("the undrained body was left open")
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

// kitchen is the wall-clock time a person reads in the message.
func kitchen(at time.Time) string {
	return at.Local().Format(time.Kitchen) //nolint:gosmopolitan // the message is in the reader's own zone
}

func TestError_NamesThePoolLimitAndReset(t *testing.T) {
	t.Parallel()

	retryAt := time.Now().Add(11*time.Minute + 200*time.Millisecond)
	past := time.Now().Add(-time.Hour)
	tests := []struct {
		name string
		want string
		err  ratelimit.Error
	}{
		{
			name: "pending",
			err:  ratelimit.Error{Resource: ratelimit.PoolCore, Limit: 5000, RetryAt: retryAt},
			want: "GitHub core rate limit exhausted (5000/hour); resets at " + kitchen(retryAt) + ", in 11m0s",
		},
		{
			name: "already reset and unnamed",
			err:  ratelimit.Error{RetryAt: past},
			want: "GitHub api rate limit exhausted (0/hour); resets at " + kitchen(past) + ", in 0s",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}
