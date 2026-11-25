package keys_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/tui/keys"
)

// everyShared is every table aragonite ships, which must keep the reservations it states.
func everyShared() []keys.Binding {
	all := slices.Concat(shared(), keys.SplitColumns(), keys.ConflictPicks(), keys.Verbs(),
		[]keys.Binding{keys.Tick(keys.TickSelect), keys.Tick(keys.TickCheck)})

	for _, verb := range []string{
		keys.VerbRebase, keys.VerbSquash, keys.VerbDuplicate, keys.VerbNew,
		keys.VerbRestore, keys.VerbCompare, keys.VerbSplit,
	} {
		all = append(all, keys.FlagBindings(verb)...)
	}

	return all
}

func TestSharedTablesKeepTheirReservations(t *testing.T) {
	t.Parallel()

	if got := keys.Unreserved(everyShared()); len(got) != 0 {
		t.Errorf("Unreserved(shared tables) = %q", got)
	}

	km, err := keys.Merge(slices.Concat(shared(), keys.Verbs()), nil, nil)
	if err != nil {
		t.Fatalf("Merge(shared, Verbs) = %v", err)
	}

	keys.RequireReserved(t, km)
}

func TestReservedKeysRefuseOtherUses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		b    keys.Binding
		bad  bool
	}{
		{"the old add-comment chord", keys.Binding{Action: "comment-blocker", Keys: []string{"a b"}}, true},
		{"submit on S", keys.Binding{Action: "submit-approve", Keys: []string{"S a"}}, true},
		{"the look chord on u", keys.Binding{Action: "look-grammar", Keys: []string{"u g"}}, true},
		{"refresh on ctrl+r", keys.Binding{Action: "refresh", Keys: []string{"ctrl+r"}}, true},
		{"a tick that writes", keys.Binding{Action: "file-stage", Keys: []string{"space"}, Contexts: keys.List}, true},
		{"a bare z", keys.Binding{Action: "hide-lane", Keys: []string{"z"}}, true},
		{"a bare bracket", keys.Binding{Action: "queue-next", Keys: []string{"]"}}, true},
		{"post on one key", keys.Binding{Action: "post", Keys: []string{"P"}}, true},
		{"scroll on Y", keys.Binding{Action: "scroll-down", Keys: []string{"Y"}}, true},
		{"info on i", keys.Binding{Action: "info", Keys: []string{"i"}}, true},
		{"the insert chord", keys.Binding{Action: "comment-blocker", Keys: []string{"i b"}}, false},
		{"a publish chord", keys.Binding{Action: "publish-approve", Keys: []string{"P a"}}, false},
		{"jojo's push", keys.Binding{Action: "push", Keys: []string{"P"}, Contexts: keys.Log}, false},
		{"a check tick", keys.Binding{Action: keys.ActionCheck, Keys: []string{"space"}, Contexts: keys.List}, false},
		{"under a prefix", keys.Binding{Action: "bookmarks", Keys: []string{"b b"}}, false},
		{"take every side", keys.ConflictPicks()[2], false},
		{"a moved key", keys.Moved("a", keys.ActionInsert), false},
		{"an unreserved key", keys.Binding{Action: "ready", Keys: []string{"r"}}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := keys.Unreserved([]keys.Binding{tc.b})
			if bad := len(got) != 0; bad != tc.bad {
				t.Errorf("Unreserved(%+v) = %q, want a clash: %v", tc.b, got, tc.bad)
			}
		})
	}
}

func TestDisagreementsReportReservedKeysOnAnyScreen(t *testing.T) {
	t.Parallel()

	// No shared binding is on the conflict viewer's a, so only the reservation can catch this.
	got := keys.Disagreements(shared(), []keys.Binding{
		{Action: "add-comment", Keys: []string{"a"}, Contexts: keys.Rounds},
	})

	if len(got) != 1 || !strings.Contains(got[0], "reserved for abandon") {
		t.Errorf("Disagreements() = %q, want one naming abandon", got)
	}
}

func TestEveryReservationIsWellFormed(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}

	for _, r := range keys.Reserved() {
		if seen[r.Keys] || r.Keys == "" || r.Meaning == "" {
			t.Errorf("reservation %+v is repeated or unnamed", r)
		}

		seen[r.Keys] = true
	}

	for _, want := range []string{
		"u", "ctrl+r", "ctrl+l", "space", "v", "V", "Y", "K", "a", "S", "P", "i",
		"g", "z", "]", "[", `\`, "-", "b",
	} {
		if !seen[want] {
			t.Errorf("%q is not reserved", want)
		}
	}
}
