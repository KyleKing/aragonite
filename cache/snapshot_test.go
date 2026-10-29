package cache_test

import (
	"testing"
	"time"

	"github.com/kyleking/aragonite/cache"
)

type rowState struct {
	Branch  string `json:"branch"`
	Version int    `json:"version"`
	Open    int    `json:"open"`
}

const rowKey = "row"

func row() rowState { return rowState{Version: 1, Branch: "main", Open: 3} }

// A second run reads back what the first one left, with the age it carries,
// and a slot under another upstream sees none of it.
func TestSnapshotSurvivesAProcessBoundary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	stamp := cache.Stamp{Scope: "/repo", Fingerprint: "head-a"}

	cache.NewSnapshot(cache.NewTTLCache[rowState](time.Hour), cache.NewDiskCache(dir), upstream, rowKey).
		Save(stamp, row())

	got, _, ok := cache.NewSnapshot(cache.NewTTLCache[rowState](time.Hour), cache.NewDiskCache(dir),
		upstream, rowKey).Load(stamp)
	if !ok || got != row() {
		t.Fatalf("Load = %+v, %v", got, ok)
	}

	if _, _, ok := cache.NewSnapshot(cache.NewTTLCache[rowState](time.Hour), cache.NewDiskCache(dir),
		"github.com/other/repo", rowKey).Load(stamp); ok {
		t.Error("another upstream read this one's record")
	}
}

// A snapshot paints whatever is held, fresh or long past its TTL, but never
// a record written under a checkout that has since moved.
func TestSnapshotLoad(t *testing.T) {
	t.Parallel()

	written := cache.Stamp{Scope: "/repo", Fingerprint: "head-a"}
	moved := cache.Stamp{Scope: "/repo", Fingerprint: "head-b"}

	tests := []struct {
		name   string
		stamp  cache.Stamp
		wait   time.Duration
		wantOK bool
	}{
		{"fresh", written, time.Minute, true},
		{"stale", written, 10 * time.Hour, true},
		{"past retention", written, 8 * 24 * time.Hour, false},
		{"stamp moved", moved, 10 * time.Hour, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			clock := newFakeClock()
			mem := func() *cache.TTLCache[rowState] {
				return cache.NewTTLCacheWithClock[rowState](time.Hour, clock.now)
			}

			cache.NewSnapshot(mem(), cache.NewDiskCache(dir), upstream, rowKey).Save(written, row())
			clock.advance(tc.wait)

			got, age, ok := cache.NewSnapshot(mem(), cache.NewDiskCache(dir), upstream, rowKey).
				Load(tc.stamp)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v, want %v", ok, tc.wantOK)
			}

			if !ok {
				return
			}
			if got != row() {
				t.Errorf("value = %+v", got)
			}
			if age != tc.wait {
				t.Errorf("age = %v, want %v", age, tc.wait)
			}
		})
	}
}

// With no disk store the slot still answers within a run, which is what a
// return to a painted screen needs.
func TestSnapshotWithoutDiskIsMemoryOnly(t *testing.T) {
	t.Parallel()

	mem := cache.NewTTLCache[rowState](time.Hour)
	cache.NewSnapshot(mem, nil, upstream, rowKey).Save(cache.NoStamp, row())

	if _, _, ok := cache.NewSnapshot(mem, nil, upstream, rowKey).Load(cache.NoStamp); !ok {
		t.Error("a memory-only record did not read back")
	}
}

// Drop makes the record read as absent in memory and on disk, not as old.
func TestSnapshotDrop(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mem := cache.NewTTLCache[rowState](time.Hour)

	cache.NewSnapshot(mem, cache.NewDiskCache(dir), upstream, rowKey).Save(cache.NoStamp, row())
	cache.NewSnapshot(mem, cache.NewDiskCache(dir), upstream, rowKey).Drop()

	if _, _, ok := cache.NewSnapshot(cache.NewTTLCache[rowState](time.Hour),
		cache.NewDiskCache(dir), upstream, rowKey).Load(cache.NoStamp); ok {
		t.Error("a dropped record still reads back")
	}
}
