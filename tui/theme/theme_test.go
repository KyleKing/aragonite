package theme_test

import (
	"testing"

	"github.com/kyleking/aragonite/tui/theme"
)

func TestSelect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		override string
		want     theme.Palette
		probes   bool
	}{
		{"override light", "Light", theme.Latte(), false},
		{"override latte", "latte", theme.Latte(), false},
		{"override dark", "dark", theme.Macchiato(), false},
		{"unknown override falls through", "solarized", theme.Macchiato(), true},
		{"empty override probes", "", theme.Macchiato(), true},
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
