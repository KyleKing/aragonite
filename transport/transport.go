// Package transport gives an HTTP-based API client (go-gh's REST/GraphQL
// clients, or anything else built on http.RoundTripper) a test seam and a
// safety net in one: a fake transport tests can register, and a guard that
// panics on a real mutating request reaching the network when no fake is
// registered.
package transport

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
)

var (
	testTransportMu sync.RWMutex
	testTransport   http.RoundTripper
)

// SetTestTransport routes every client built through Current afterward
// through rt, so a test never reaches the real network. It returns a
// restore function and panics when called outside `go test`.
func SetTestTransport(rt http.RoundTripper) func() {
	if !testing.Testing() {
		panic("transport.SetTestTransport is test-only")
	}

	testTransportMu.Lock()
	previous := testTransport
	testTransport = rt
	testTransportMu.Unlock()

	return func() {
		testTransportMu.Lock()
		testTransport = previous
		testTransportMu.Unlock()
	}
}

func currentTestTransport() http.RoundTripper {
	testTransportMu.RLock()
	defer testTransportMu.RUnlock()

	return testTransport
}

// Current returns the registered fake transport, or base wrapped in the
// mutation guard when none is registered. Call it while building a client
// under `go test` so every request goes through one or the other; a
// production build has no reason to call it at all.
func Current(base http.RoundTripper) http.RoundTripper {
	if rt := currentTestTransport(); rt != nil {
		return rt
	}

	return safetyTransport{base: base}
}

// safetyTransport panics on a mutating request during tests, so a test that
// forgot to register a fake fails loudly before it can touch a real
// resource, rather than silently reaching the network.
type safetyTransport struct {
	base http.RoundTripper
}

func (s safetyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if testing.Testing() && isMutatingMethod(req.Method) {
		panic(fmt.Sprintf(
			"SAFETY VIOLATION: attempted real %s %s during a test.\n"+
				"This could mutate a real remote resource!\n"+
				"Register a fake via transport.SetTestTransport before this call runs.",
			req.Method, req.URL,
		))
	}

	return s.base.RoundTrip(req) //nolint:wrapcheck // transparent proxy: http.Client wraps this in *url.Error itself
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodDelete, http.MethodPatch, http.MethodPost, http.MethodPut:
		return true
	default:
		return false
	}
}
