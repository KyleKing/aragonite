package stats_test

import (
	"math"
	"slices"
	"testing"

	"github.com/kyleking/aragonite/stats"
)

func TestPercentile_InterpolatesBetweenRanksAndLeavesTheInputAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values []float64
		p      float64
		want   float64
	}{
		{"empty", nil, 0.5, 0},
		{"single value", []float64{7}, 0.9, 7},
		{"median of an odd sample", []float64{3, 1, 2}, 0.5, 2},
		{"interpolated median", []float64{4, 1, 3, 2}, 0.5, 2.5},
		{"p90", []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 0.9, 9.1},
		{"p above one clamps to the maximum", []float64{1, 2, 3}, 2, 3},
		{"negative p clamps to the minimum", []float64{1, 2, 3}, -1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before := slices.Clone(tt.values)

			if got := stats.Percentile(tt.values, tt.p); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("Percentile(%v, %v) = %v, want %v", tt.values, tt.p, got, tt.want)
			}

			if !slices.Equal(before, tt.values) {
				t.Errorf("Percentile reordered its input: %v, was %v", tt.values, before)
			}
		})
	}
}

func TestMean(t *testing.T) {
	t.Parallel()

	if got := stats.Mean([]float64{1, 2, 6}); got != 3 {
		t.Errorf("Mean = %v, want 3", got)
	}

	if got := stats.Mean(nil); got != 0 {
		t.Errorf("Mean(nil) = %v, want 0", got)
	}
}
