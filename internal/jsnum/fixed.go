package jsnum

import (
	"math"
	"strconv"
	"strings"
)

// exactDigits returns the exact decimal expansion of v, which is finite and positive: its
// significant digits without trailing zeros, and the exponent of the first one.
func exactDigits(v float64) (string, int) {
	// A float64 has at most 767 significant decimal digits; strconv writes them exactly.
	s := strconv.FormatFloat(v, 'e', 800, 64)
	mant, exp, _ := strings.Cut(s, "e")
	digits := strings.TrimRight(strings.Replace(mant, ".", "", 1), "0")
	e, _ := strconv.Atoi(exp)
	return digits, e
}

// roundDigits rounds the exact digits of a number to n digits (n >= 0), halves away from
// zero, as the Number.prototype methods choose the larger n on a tie. It returns the digits,
// padded with zeros to n, and the exponent, one more when rounding carried into a new digit.
func roundDigits(digits string, e, n int) (string, int) {
	if n >= len(digits) {
		return digits + strings.Repeat("0", n-len(digits)), e
	}
	up := digits[n] >= '5'
	out := []byte(digits[:n])
	if up {
		i := n - 1
		for ; i >= 0; i-- {
			if out[i] != '9' {
				out[i]++
				break
			}
			out[i] = '0'
		}
		if i < 0 {
			out = append([]byte{'1'}, out...)[:n]
			e++
		}
	}
	return string(out), e
}

func nonFinite(v float64) (string, bool) {
	switch {
	case math.IsNaN(v):
		return "NaN", true
	case math.IsInf(v, 1):
		return "Infinity", true
	case math.IsInf(v, -1):
		return "-Infinity", true
	}
	return "", false
}

// ToFixed returns v.toFixed(f) for f from 0 to 100.
func ToFixed(v float64, f int) string {
	if s, ok := nonFinite(v); ok {
		return s
	}
	if math.Abs(v) >= 1e21 {
		return Format(v)
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	var m string
	if v == 0 {
		m = "0"
	} else {
		digits, e := exactDigits(v)
		// The digits up to the f-th after the point: e+1 before it.
		n := e + 1 + f
		if n < 0 {
			m = "0"
		} else {
			// v * 10^f rounded to an integer, which is 10^n when rounding carried.
			rounded, re := roundDigits(digits, e, n)
			if re > e {
				rounded = "1" + strings.Repeat("0", n)
			}
			m = strings.TrimLeft(rounded, "0")
			if m == "" {
				m = "0"
			}
		}
	}
	if f == 0 {
		return sign + m
	}
	if len(m) <= f {
		m = strings.Repeat("0", f+1-len(m)) + m
	}
	return sign + m[:len(m)-f] + "." + m[len(m)-f:]
}

func exponent(e int) string {
	if e >= 0 {
		return "e+" + strconv.Itoa(e)
	}
	return "e" + strconv.Itoa(e)
}

// ToExponential returns v.toExponential(f) for f from 0 to 100.
func ToExponential(v float64, f int) string {
	if s, ok := nonFinite(v); ok {
		return s
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	m, e := strings.Repeat("0", f+1), 0
	if v != 0 {
		digits, de := exactDigits(v)
		m, e = roundDigits(digits, de, f+1)
	}
	if f > 0 {
		m = m[:1] + "." + m[1:]
	}
	return sign + m + exponent(e)
}

// ToPrecision returns v.toPrecision(p) for p from 1 to 100.
func ToPrecision(v float64, p int) string {
	if s, ok := nonFinite(v); ok {
		return s
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	m, e := strings.Repeat("0", p), 0
	if v != 0 {
		digits, de := exactDigits(v)
		m, e = roundDigits(digits, de, p)
	}
	if e < -6 || e >= p {
		if p > 1 {
			m = m[:1] + "." + m[1:]
		}
		return sign + m + exponent(e)
	}
	if e == p-1 {
		return sign + m
	}
	if e >= 0 {
		return sign + m[:e+1] + "." + m[e+1:]
	}
	return sign + "0." + strings.Repeat("0", -(e+1)) + m
}
