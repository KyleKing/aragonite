package theme_test

import (
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
