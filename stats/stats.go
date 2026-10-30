// Package stats holds the small summaries tools repeat over timings: percentiles
// and means of a sample of float64 values.
package stats

import (
	"math"
	"sort"
)

// Percentile reads p (0 to 1) out of values by linear interpolation between the
// two nearest ranks, and returns 0 for an empty sample. It sorts a copy, so the
// caller's order survives.
func Percentile(values []float64, p float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	if n == 1 {
		return sorted[0]
	}

	rank := min(max(p, 0), 1) * float64(n-1)
	lo, hi := int(math.Floor(rank)), int(math.Ceil(rank))

	if lo == hi {
		return sorted[lo]
	}

	return sorted[lo] + (rank-float64(lo))*(sorted[hi]-sorted[lo])
}

// Mean is the arithmetic mean of values, or 0 for an empty sample.
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}
