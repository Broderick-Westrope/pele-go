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
