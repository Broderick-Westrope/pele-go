// Package jsnum converts numbers to strings the way JavaScript does.
package jsnum

import (
	"math"
	"strconv"
	"strings"
)

// Format returns String(v): the shortest digits that round-trip, written in plain notation
// for exponents from -7 to 20 and in exponential notation outside them.
func Format(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "Infinity"
	case math.IsInf(v, -1):
		return "-Infinity"
	case v == 0:
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	if v == math.Trunc(v) && v < 1<<53 {
		return sign + strconv.FormatInt(int64(v), 10)
	}

	// 'e' with precision -1 gives the shortest round-tripping digits, d.ddde±xx.
	e := strconv.FormatFloat(v, 'e', -1, 64)
	mant, exp, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	k := len(digits)
	n, _ := strconv.Atoi(exp)
	n++ // the position of the decimal point relative to the digits

	var b strings.Builder
	b.WriteString(sign)
	switch {
	case k <= n && n <= 21:
		b.WriteString(digits)
		b.WriteString(strings.Repeat("0", n-k))
	case 0 < n && n <= 21:
		b.WriteString(digits[:n])
		b.WriteByte('.')
		b.WriteString(digits[n:])
	case -6 < n && n <= 0:
		b.WriteString("0.")
		b.WriteString(strings.Repeat("0", -n))
		b.WriteString(digits)
	default:
		b.WriteString(digits[:1])
		if k > 1 {
			b.WriteByte('.')
			b.WriteString(digits[1:])
		}
		b.WriteByte('e')
		if n-1 >= 0 {
			b.WriteByte('+')
		}
		b.WriteString(strconv.Itoa(n - 1))
	}
	return b.String()
}
