package jsnum

import (
	"math"
	"math/big"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// ParseInt returns parseInt(s) with no radix: the digits at the start of s after JS
// whitespace and an optional sign, read as hex after 0x and as decimal otherwise. No
// digits is NaN.
func ParseInt(s string) float64 {
	s = jsstr.TrimStart(s)
	negative := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		negative = s[0] == '-'
		s = s[1:]
	}
	base := 10
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		base = 16
		s = s[2:]
	}
	end := 0
	for end < len(s) {
		if d := digitValue(s[end]); d < 0 || d >= base {
			break
		}
		end++
	}
	if end == 0 {
		return math.NaN()
	}
	// Exact, then rounded once, as V8 does.
	n, _ := new(big.Int).SetString(s[:end], base)
	v, _ := new(big.Float).SetInt(n).Float64()
	if negative {
		return -v
	}
	return v
}
