package keys_test

import (
	"slices"
	"testing"

	"github.com/kyleking/aragonite/tui/keys"
)

func TestDiffViewCarriesTheReadTick(t *testing.T) {
	t.Parallel()

	i := slices.IndexFunc(keys.DiffView(), func(b keys.Binding) bool { return b.Action == keys.ActionRead })
	if i < 0 {
		t.Fatal("DiffView has no read tick")
	}

	got, want := keys.DiffView()[i], keys.Tick(keys.TickRead)
	if got.Action != want.Action || got.Group != want.Group || got.Contexts != want.Contexts ||
		!slices.Equal(got.Keys, want.Keys) {
		t.Errorf("DiffView's read = %+v, want Tick(TickRead) = %+v", got, want)
	}
}

func TestEveryTickIsSpaceAndNamesItsKind(t *testing.T) {
	t.Parallel()

	for _, b := range keys.Ticks() {
		if !slices.Equal(b.Keys, []string{"space"}) || b.Group != keys.GroupMark || b.Short != b.Action ||
			!keys.IsTick(b.Action) || b.What == "" {
			t.Errorf("tick %+v is not space in the mark group with its kind as the footer word", b)
		}
	}

	if keys.IsTick("file-stage") {
		t.Error("a staging action reads as a tick")
	}
}

// The kinds sit on screens that never meet, so a tool can merge all three onto one space.
func TestTickKindsMergeWithTheSharedTable(t *testing.T) {
	t.Parallel()

	km, err := keys.Merge(shared(), []keys.Binding{keys.Tick(keys.TickSelect), keys.Tick(keys.TickCheck)}, nil)
	if err != nil {
		t.Fatalf("Merge = %v", err)
	}

	for ctx, want := range map[keys.Context]string{
		keys.View: keys.ActionRead, keys.Select: keys.ActionSelect, keys.Split: keys.ActionSelect,
		keys.Log: keys.ActionCheck, keys.List: keys.ActionCheck,
	} {
		if got := keys.NewMatcher(km).Feed(ctx, "space"); got.Action != want {
			t.Errorf("space on %v = %q, want %q", ctx, got.Action, want)
		}
	}
}
