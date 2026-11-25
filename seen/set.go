package seen

import "sync"

// Set is which hunks have been read. It is keyed by content, so it is not tied to a head commit.
//
// A Set also remembers what this process changed since it was loaded or last saved. Save applies
// those changes on top of what is on disk at the time, rather than writing the whole set back, so
// a mark another process saved in between is kept. Every method is safe to call from several
// goroutines, and the read methods answer as for an empty set on a nil Set.
type Set struct {
	ids     map[ID]bool
	changed map[ID]bool // the read state this process last gave each id since the last load or save
	mu      sync.Mutex
}

// New is an empty set, which is what a caller marking hunks it has just read off a diff starts
// from.
func New() *Set { return &Set{ids: map[ID]bool{}, changed: map[ID]bool{}} }

// Has reports whether a hunk has been read.
func (s *Set) Has(id ID) bool {
	if s == nil {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ids[id]
}

// Toggle flips one hunk and reports what it became.
func (s *Set) Toggle(id ID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	read := !s.ids[id]
	s.set(id, read)

	return read
}

// Mark sets a run of hunks to read or unread at once, which is what marking a whole file or a
// whole change does.
func (s *Set) Mark(read bool, ids ...ID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		s.set(id, read)
	}
}

func (s *Set) set(id ID, read bool) {
	if read {
		s.ids[id] = true
	} else {
		delete(s.ids, id)
	}

	s.changed[id] = read
}

// Count is how many of the given hunks have been read.
func (s *Set) Count(live []Ref) int {
	if s == nil {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	n := 0

	for _, r := range live {
		if s.ids[r.ID] {
			n++
		}
	}

	return n
}

// Unread is the given hunks that have not been read, in the order given: what an only-unread view
// draws and what a next-unread motion steps through.
func (s *Set) Unread(live []Ref) []Ref {
	if s == nil {
		return live
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var out []Ref

	for _, r := range live {
		if !s.ids[r.ID] {
			out = append(out, r)
		}
	}

	return out
}

// FileCount is how much of one file has been read.
type FileCount struct {
	Path  string
	Read  int
	Total int
}

// Files is how much of each file in the given hunks has been read, one entry per file in the
// order the files first appear.
func (s *Set) Files(live []Ref) []FileCount {
	var (
		out []FileCount
		at  = map[string]int{}
	)

	if s != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
	}

	for _, r := range live {
		i, ok := at[r.Path]
		if !ok {
			i = len(out)
			at[r.Path] = i
			out = append(out, FileCount{Path: r.Path})
		}

		out[i].Total++

		if s != nil && s.ids[r.ID] {
			out[i].Read++
		}
	}

	return out
}

// Include adds every hunk o has read to s as already saved rather than as a change of this
// process, so Save never writes them to s's file. It is how a tool lays another tool's marks over
// its own: second-look's own file stays its own, and a hunk jojo marked reads as read. A hunk this
// process has changed in s since its last save keeps the state it was given. A nil o adds nothing.
func (s *Set) Include(o *Set) {
	if o == nil || o == s {
		return
	}

	o.mu.Lock()
	ids := make([]ID, 0, len(o.ids))

	for id := range o.ids {
		ids = append(ids, id)
	}
	o.mu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		if _, pending := s.changed[id]; !pending {
			s.ids[id] = true
		}
	}
}
