package cache

import "time"

// Snapshot is one named slot's record: memory over an explicit disk store,
// answering how old its value is so a screen can paint last-known state on
// open and refresh behind it. The record's own fields carry whatever schema
// and ownership markers the caller needs; a snapshot that fails the caller's
// checks reads as absent to it, never as zero values.
type Snapshot[T any] struct {
	mem      *TTLCache[T]
	disk     *DiskCache
	upstream string
	key      string
}

// NewSnapshot returns the slot for upstream's file under key, backed by mem
// and disk. Callers that want ClearAll to drop the record pass a registered
// cache — a hard refresh pressed because something looks wrong must not
// repaint a stale record. A nil disk keeps the record for this run alone,
// which is how a caller with no cache directory still repaints after
// navigating away. Slots on different upstreams may share one mem, since the
// upstream is already part of the key.
func NewSnapshot[T any](mem *TTLCache[T], disk *DiskCache, upstream, key string) *Snapshot[T] {
	return &Snapshot[T]{mem: mem, disk: disk, upstream: upstream, key: key}
}

// Load returns the last value saved under this slot and how long ago it was
// written, whatever the TTL says. A value written under a stamp that has
// since moved still misses: that value is wrong, not old. An expired record
// reads from disk and is not seeded back into memory.
//
//nolint:ireturn // T is the record the slot was made for
func (s *Snapshot[T]) Load(stamp Stamp) (T, time.Duration, bool) {
	return persistedStaleFrom(s.disk, s.mem, s.upstream, s.key, stamp)
}

// Save stores value in memory and, when a disk store was given, in upstream's
// cache file. An empty upstream is memory-only: a checkout with no resolvable
// remote shares its record with nobody.
func (s *Snapshot[T]) Save(stamp Stamp, value T) {
	persistTo(s.disk, s.mem, s.upstream, s.key, stamp, value)
}

// Drop removes the slot's record, in memory and on disk, so a later Load
// reads as absent rather than as an old value.
func (s *Snapshot[T]) Drop() {
	s.mem.Delete(s.key)

	if s.disk != nil {
		s.disk.delete(s.upstream, s.key)
	}
}
