package cache_test

import (
	"testing"
	"time"

	"github.com/kyleking/aragonite/cache"
)

// fakeClock ages a cache without sleeping. One clock belongs to one test, so
// it needs no locking.
type fakeClock struct{ at time.Time }

func newFakeClock() *fakeClock {
	return &fakeClock{at: time.Date(2026, time.August, 11, 9, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time          { return c.at }
func (c *fakeClock) advance(d time.Duration) { c.at = c.at.Add(d) }

// An entry is served as set until its TTL has passed, the last Set wins, and
// Delete and Clear drop what they name and nothing else.
func TestTTLCache_ServesWhatWasSetUntilItsTTL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		act  func(c *cache.TTLCache[string], clock *fakeClock)
		want map[string]string
		name string
	}{
		{
			name: "set then get",
			act:  func(_ *cache.TTLCache[string], _ *fakeClock) {},
			want: map[string]string{"k1": "v1", "k2": "v2"},
		},
		{
			name: "the last set wins",
			act:  func(c *cache.TTLCache[string], _ *fakeClock) { c.Set("k1", cache.NoStamp, "v3") },
			want: map[string]string{"k1": "v3", "k2": "v2"},
		},
		{
			name: "served at exactly its TTL",
			act:  func(_ *cache.TTLCache[string], clock *fakeClock) { clock.advance(5 * time.Minute) },
			want: map[string]string{"k1": "v1", "k2": "v2"},
		},
		{
			name: "gone past its TTL",
			act:  func(_ *cache.TTLCache[string], clock *fakeClock) { clock.advance(5*time.Minute + time.Nanosecond) },
			want: map[string]string{},
		},
		{
			name: "a later set restarts the TTL",
			act: func(c *cache.TTLCache[string], clock *fakeClock) {
				clock.advance(4 * time.Minute)
				c.Set("k2", cache.NoStamp, "v2")
				clock.advance(2 * time.Minute)
			},
			want: map[string]string{"k2": "v2"},
		},
		{
			name: "delete drops one key",
			act:  func(c *cache.TTLCache[string], _ *fakeClock) { c.Delete("k1"); c.Delete("missing") },
			want: map[string]string{"k2": "v2"},
		},
		{
			name: "clear drops every key",
			act:  func(c *cache.TTLCache[string], _ *fakeClock) { c.Clear() },
			want: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clock := newFakeClock()
			c := cache.NewTTLCacheWithClock[string](5*time.Minute, clock.now)
			c.Set("k1", cache.NoStamp, "v1")
			c.Set("k2", cache.NoStamp, "v2")

			tc.act(c, clock)

			for _, key := range []string{"k1", "k2", "missing"} {
				got, ok := c.Get(key, cache.NoStamp)
				want, held := tc.want[key]
				if ok != held || got != want {
					t.Errorf("Get(%q) = %q, %v, want %q, %v", key, got, ok, want, held)
				}
			}
		})
	}

	type data struct {
		Name  string
		Count int
	}

	structs := cache.NewTTLCache[data](time.Minute)
	structs.Set("d", cache.NoStamp, data{Name: "test", Count: 5})

	if got, ok := structs.Get("d", cache.NoStamp); !ok || got != (data{Name: "test", Count: 5}) {
		t.Errorf("a struct value read back as %+v, %v", got, ok)
	}
}

// Fresh serves a value only to the checkout stamp it was written or read
// under, however old it is, and never to a checkout that could not be
// stamped.
func TestTTLCache_FreshFollowsTheStampNotTheTTL(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	c := cache.NewTTLCacheWithClock[int](time.Minute, clock.now)
	written := cache.Stamp{Scope: "/repo", Fingerprint: "a"}

	c.Set("k", written, 7)
	clock.advance(time.Hour)

	for _, tc := range []struct {
		stamp cache.Stamp
		ok    bool
	}{
		{written, true},
		{cache.Stamp{Scope: "/repo", Fingerprint: "b"}, false},
		{cache.Stamp{Scope: "/other", Fingerprint: "a"}, false},
		{cache.Stamp{Scope: "/repo"}, false},
	} {
		if got, ok := c.Fresh("k", tc.stamp); ok != tc.ok || (ok && got != 7) {
			t.Errorf("Fresh(%+v) = %d, %v, want ok %v", tc.stamp, got, ok, tc.ok)
		}
	}

	if _, ok := c.Fresh("missing", written); ok {
		t.Error("Fresh served a key never set")
	}
}

func TestAStampOnlyShortensARemoteValuesLife(t *testing.T) {
	t.Parallel()

	clock := newFakeClock()
	c := cache.NewTTLCacheWithClock[[]prInfo](5*time.Minute, clock.now)

	const key = "github.com/acme/app\x00acme/app:all_prs"

	before := cache.Stamp{Scope: "/repo", Fingerprint: "unpushed"}
	after := cache.Stamp{Scope: "/repo", Fingerprint: "pushed"}
	peer := cache.Stamp{Scope: "/peer-checkout", Fingerprint: "its-own-head"}

	c.Set(key, before, []prInfo{{Number: 1}})

	if _, ok := c.Get(key, before); !ok {
		t.Error("an unchanged checkout refetched the PR list inside the TTL")
	}
	if _, ok := c.Get(key, peer); !ok {
		t.Error("a second checkout of the same remote refetched the PR list")
	}
	if _, ok := c.Get(key, after); ok {
		t.Error("a push left the PR list cached")
	}

	c.Set(key, after, []prInfo{{Number: 2}})
	clock.advance(6 * time.Minute)

	if _, ok := c.Get(key, after); ok {
		t.Error("an unchanged stamp carried the PR list past its TTL")
	}
}

// A checkout that read the entry earlier is the one a later change evicts it
// for; a checkout meeting the entry for the first time is a new reader, not a
// local change, and must not throw away what its peers just paid for.
func TestOneCheckoutsChangeDoesNotEvictForAnother(t *testing.T) {
	t.Parallel()

	c := cache.NewTTLCache[int](5 * time.Minute)
	const key = "shared"

	c.Set(key, cache.Stamp{Scope: "/a", Fingerprint: "a1"}, 7)

	if _, ok := c.Get(key, cache.Stamp{Scope: "/b", Fingerprint: "b1"}); !ok {
		t.Fatal("a first-time reader was treated as a changed checkout")
	}
	if _, ok := c.Get(key, cache.Stamp{Scope: "/b", Fingerprint: "b2"}); ok {
		t.Error("the reader's own later change did not evict")
	}
}
