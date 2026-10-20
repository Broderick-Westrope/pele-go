package xychart

import (
	"math"

	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
)

// tickStep is a step of 1, 2 or 5 times a power of ten that divides span into about count
// parts. A step below one comes back as the negative of its inverse, which is a whole
// number, so that tick values are found by division and print without rounding noise (0.3,
// not 0.30000000000000004).
func tickStep(span, count float64) float64 {
	raw := span / max(count, 1)
	power := math.Floor(jsmath.Log10(raw))
	unit := raw / jsmath.Pow(10, power)
	factor := 1.0
	switch {
	case unit >= 7.07:
		factor = 10
	case unit >= 3.16:
		factor = 5
	case unit >= 1.41:
		factor = 2
	}
	if power < 0 {
		return -jsmath.Pow(10, -power) / factor
	}
	return float64(factor * jsmath.Pow(10, power))
}

func tickIndex(step, value float64) float64 {
	if step > 0 {
		return value / step
	}
	return float64(value * -step)
}

func tickValue(step, index float64) float64 {
	if step > 0 {
		return float64(index * step)
	}
	return index / -step
}

// ticksBetween returns the multiples of a step from lo to hi, both included.
func ticksBetween(lo, hi, step float64) []float64 {
	first := jsmath.Round(tickIndex(step, lo))
	if tickValue(step, first) < lo {
		first++
	}
	last := jsmath.Round(tickIndex(step, hi))
	if tickValue(step, last) > hi {
		last--
	}
	var out []float64
	for i := first; i <= last && len(out) < 100; i++ {
		out = append(out, tickValue(step, i))
	}
	return out
}
