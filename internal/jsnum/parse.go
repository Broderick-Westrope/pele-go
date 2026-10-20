package jsnum

import (
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// Parse returns Number(s): s without surrounding JS whitespace as a decimal literal, a
// 0x, 0o or 0b integer, or Infinity. The empty string is 0. Anything else is NaN.
func Parse(s string) float64 {
	s = jsstr.Trim(s)
	if s == "" {
		return 0
	}
	if len(s) > 2 && s[0] == '0' {
		base := 0
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			return parseRadix(s[2:], base)
		}
	}
	body := s
	if body[0] == '+' || body[0] == '-' {
		body = body[1:]
	}
	if body == "Infinity" {
		if s[0] == '-' {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	if !isDecimalLiteral(body) {
		return math.NaN()
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil && !strings.Contains(err.Error(), "range") {
		return math.NaN()
	}
	return v
}

func parseRadix(digits string, base int) float64 {
	if digits == "" {
		return math.NaN()
	}
	for i := 0; i < len(digits); i++ {
		if d := digitValue(digits[i]); d < 0 || d >= base {
			return math.NaN()
		}
	}
	// Exact, then rounded once, as JS does for large values.
	n, _ := new(big.Int).SetString(digits, base)
	v, _ := new(big.Float).SetInt(n).Float64()
	return v
}

func digitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	}
	return -1
}

// isDecimalLiteral reports whether s is digits, digits.digits, digits. or .digits, each
// with an optional exponent.
func isDecimalLiteral(s string) bool {
	i := 0
	intDigits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		intDigits++
	}
	fracDigits := 0
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			fracDigits++
		}
	}
	if intDigits == 0 && fracDigits == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		expDigits := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			expDigits++
		}
		if expDigits == 0 {
			return false
		}
	}
	return i == len(s)
}
