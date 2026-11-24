package seen_test

import (
	"slices"
	"testing"

	"github.com/kyleking/aragonite/seen"
)

func TestSet_CountsPerFileAndUnread(t *testing.T) {
	t.Parallel()

	live := []seen.Ref{
		{Path: "a.go", Hunk: 1, ID: "a1"},
		{Path: "b.go", Hunk: 2, ID: "b1"},
		{Path: "a.go", Hunk: 3, ID: "a2"},
		{Path: "c.go", Hunk: 4, ID: "c1"},
	}

	set := seen.New()
	set.Mark(true, "a1", "c1")

	if read := set.Toggle("b1"); !read {
		t.Error("toggling an unread hunk did not read it")
	}

	if read := set.Toggle("b1"); read {
		t.Error("toggling a read hunk did not unread it")
	}

	want := []seen.FileCount{
		{Path: "a.go", Read: 1, Total: 2}, {Path: "b.go", Total: 1}, {Path: "c.go", Read: 1, Total: 1},
	}
	if got := set.Files(live); !slices.Equal(got, want) {
		t.Errorf("Files = %+v, want %+v", got, want)
	}

	if got := set.Count(live); got != 2 {
		t.Errorf("Count = %d, want 2", got)
	}

	if got := set.Unread(live); !slices.Equal(got, []seen.Ref{live[1], live[2]}) {
		t.Errorf("Unread = %+v", got)
	}
}

// A nil set is the "remember nothing" default: nothing read, everything unread.
func TestSet_NilIsEmpty(t *testing.T) {
	t.Parallel()

	var s *seen.Set

	live := []seen.Ref{{Path: "a.go", Hunk: 1, ID: "a1"}}

	if s.Has("a1") || s.Count(live) != 0 || len(s.Unread(live)) != 1 {
		t.Error("a nil set answered as if something were read")
	}

	if got := s.Files(live); !slices.Equal(got, []seen.FileCount{{Path: "a.go", Total: 1}}) {
		t.Errorf("Files = %+v", got)
	}
}
