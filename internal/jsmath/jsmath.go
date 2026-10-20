// Package jsmath has the Math functions whose results differ from Go's math package.
package jsmath

import "math"

// Round rounds half-way values up, towards positive infinity, as Math.round does. It keeps the
// sign of zero: values in [-0.5, -0] round to -0.
func Round(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v == 0 {
		return v
	}
	t := math.Trunc(v)
	if v >= 0 {
		if v-t >= 0.5 {
			return t + 1
		}
		return t
	}
	if t-v > 0.5 {
		return t - 1
	}
	if t == 0 {
		return math.Copysign(0, -1)
	}
	return t
}

// Hypot is Math.hypot as V8 computes it (builtins/math.tq): each value over the largest,
// squared and summed with Kahan compensation. Go's math.Hypot rounds differently.
func Hypot(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	largest := 0.0
	nan := false
	for _, v := range values {
		if math.IsNaN(v) {
			nan = true
		} else if a := math.Abs(v); a > largest {
			largest = a
		}
	}
	switch {
	case math.IsInf(largest, 1):
		return largest
	case nan:
		return math.NaN()
	case largest == 0:
		return 0
	}
	sum, compensation := 0.0, 0.0
	for _, v := range values {
		if math.IsNaN(v) {
			continue
		}
		n := math.Abs(v) / largest
		summand := float64(n*n) - compensation
		preliminary := sum + summand
		compensation = (preliminary - sum) - summand
		sum = preliminary
	}
	return float64(math.Sqrt(sum) * largest)
}
