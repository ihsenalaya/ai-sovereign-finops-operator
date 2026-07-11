package predictor

import (
	"math"
	"sort"
)

// QuantileBound returns the empirical quantile bound for a sorted or unsorted
// sample of predicted output token counts. It is intentionally simple for the
// initial research scaffold.
func QuantileBound(samples []int, q float64) int {
	if len(samples) == 0 {
		return 0
	}
	if q <= 0 {
		q = 0
	}
	if q >= 1 {
		q = 1
	}
	cp := append([]int(nil), samples...)
	sort.Ints(cp)
	idx := int(float64(len(cp)-1) * q)
	return cp[idx]
}

// MeanStdBound returns a conservative integer bound using mean + z * stddev.
// This is a lightweight scaffold for the future risk-bounded reservation mode.
func MeanStdBound(samples []int, z float64) int {
	if len(samples) == 0 {
		return 0
	}
	mean := meanInts(samples)
	var variance float64
	for _, s := range samples {
		d := float64(s) - mean
		variance += d * d
	}
	variance /= float64(len(samples))
	stddev := math.Sqrt(variance)
	bound := mean + z*stddev
	if bound < 0 {
		return 0
	}
	return int(math.Ceil(bound))
}
