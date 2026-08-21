//nolint:testpackage // exercises currentTestTransport and safetyTransport directly
package transport

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func recordingTransport(body string, paths *[]string) roundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		if paths != nil {
			*paths = append(*paths, req.Method+" "+req.URL.Path)
		}

		return jsonResponse(body), nil
	}
}

func TestSetTestTransportRestore(t *testing.T) { //nolint:paralleltest // mutates the shared global test transport
	rt := recordingTransport(`{}`, nil)

	restore := SetTestTransport(rt)
	if currentTestTransport() == nil {
		t.Fatal("expected transport to be registered")
	}

	restore()

	if currentTestTransport() != nil {
		t.Error("expected transport to be restored to nil")
	}
}

//nolint:paralleltest // mutates the shared global test transport
func TestCurrentPrefersRegisteredFake(t *testing.T) {
	var paths []string
	restore := SetTestTransport(recordingTransport(`{"ok":true}`, &paths))
	defer restore()

	rt := Current(http.DefaultTransport)

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "https://example.com/thing", http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	if len(paths) != 1 || paths[0] != "GET /thing" {
		t.Errorf("requests = %v, want [GET /thing]", paths)
	}
}

func TestSafetyTransportPanicsOnMutation(t *testing.T) {
	t.Parallel()

	mutating := []string{http.MethodDelete, http.MethodPatch, http.MethodPost, http.MethodPut}
	for _, method := range mutating {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			guard := safetyTransport{base: recordingTransport(`{}`, nil)}
			req, err := http.NewRequestWithContext(
				context.Background(), method, "https://example.com/repos/acme/widgets/git/refs/heads/x", http.NoBody,
			)
			if err != nil {
				t.Fatalf("NewRequestWithContext() error = %v", err)
			}

			defer func() {
				if recover() == nil {
					t.Errorf("expected panic for %s request", method)
				}
			}()

			//nolint:bodyclose // the guard panics before a response exists
			resp, err := guard.RoundTrip(req)
			t.Fatalf("RoundTrip returned without panic: resp=%v err=%v", resp, err)
		})
	}
}

func TestSafetyTransportAllowsReads(t *testing.T) {
	t.Parallel()

	guard := safetyTransport{base: recordingTransport(`{"ok":true}`, nil)}
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "https://example.com/user", http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}

	resp, err := guard.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	var payload map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if !payload["ok"] {
		t.Error("expected fake body to pass through the guard")
	}
}

func TestCurrentGuardsWhenNoFakeRegistered(t *testing.T) {
	t.Parallel()

	rt := Current(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{}`), nil
	}))

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodDelete, "https://example.com/repos/acme/widgets", http.NoBody,
	)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Error("expected panic for a real mutating request with no fake registered")
		}
	}()

	//nolint:bodyclose // the guard panics before a response exists
	resp, err := rt.RoundTrip(req)
	t.Fatalf("RoundTrip returned without panic: resp=%v err=%v", resp, err)
}
