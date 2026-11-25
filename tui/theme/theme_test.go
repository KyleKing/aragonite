package theme_test

import (
	"reflect"
	"testing"

	"github.com/kyleking/aragonite/tui/theme"
)

func TestSelect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want     theme.Palette
		name     string
		override string
		probes   bool
	}{
		{name: "override light", override: "Light", want: theme.Latte()},
		{name: "override latte", override: "latte", want: theme.Latte()},
		{name: "override dark", override: "dark", want: theme.Macchiato()},
		{name: "override ink", override: "ink", want: theme.Ink()},
		{name: "unknown override falls through", override: "solarized", want: theme.Macchiato(), probes: true},
		{name: "empty override probes", override: "", want: theme.Macchiato(), probes: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			probed := false
			dark := func() bool { probed = true; return true }
			if got := theme.Select(tc.override, dark); got != tc.want {
				t.Errorf("Select(%q) = %v, want %v", tc.override, got, tc.want)
			}
			if probed != tc.probes {
				t.Errorf("Select(%q) probed = %v, want %v", tc.override, probed, tc.probes)
			}
		})
	}
}

func TestInkSetsEveryField(t *testing.T) {
	t.Parallel()

	palette := reflect.ValueOf(theme.Ink())
	for i := range palette.NumField() {
		if palette.Field(i).IsNil() {
			t.Errorf("Ink().%s is unset", palette.Type().Field(i).Name)
		}
	}
}

func TestSemanticDrawsFromItsFlavor(t *testing.T) {
	t.Parallel()

	latte, macchiato := theme.Latte().Semantic(), theme.Macchiato().Semantic()
	if latte.Primary != theme.Latte().Mauve {
		t.Errorf("Primary = %v, want Mauve", latte.Primary)
	}
	if latte == macchiato {
		t.Error("Latte and Macchiato produced identical semantic roles")
	}
}

// TestForBackground_HonorsTheOverrideAndQueriesNothing pins the pair a program started under
// tui/launch builds with: Pinned says whether the background is needed at all, and ForBackground
// takes the answer the program learned rather than asking the terminal.
// It sets CATPPUCCIN_THEME, which the process shares, so it runs alone.
func TestForBackground_HonorsTheOverrideAndQueriesNothing(t *testing.T) {
	for _, tc := range []struct {
		dark       theme.Palette
		light      theme.Palette
		override   string
		wantPinned bool
	}{
		{theme.Macchiato(), theme.Latte(), "", false},
		{theme.Macchiato(), theme.Latte(), "solarized", false},
		{theme.Latte(), theme.Latte(), "latte", true},
		{theme.Macchiato(), theme.Macchiato(), "dark", true},
		{theme.Ink(), theme.Ink(), "ink", true},
	} {
		t.Run("override="+tc.override, func(t *testing.T) {
			t.Setenv(theme.EnvOverride, tc.override)

			if got := theme.Pinned(); got != tc.wantPinned {
				t.Errorf("Pinned() = %t, want %t", got, tc.wantPinned)
			}

			if got := theme.ForBackground(true); got != tc.dark {
				t.Errorf("ForBackground(dark) = %v, want %v", got, tc.dark)
			}

			if got := theme.ForBackground(false); got != tc.light {
				t.Errorf("ForBackground(light) = %v, want %v", got, tc.light)
			}
		})
	}
}
