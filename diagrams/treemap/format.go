package treemap

import (
	"math"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

// The parts of d3-format's specifier that treemap values use: an optional `$`, `,` for
// thousands separators, a precision, and a type of `f`, `%`, `d`, `e` or none. Fill,
// alignment and width are read and ignored. Anything else falls back to the default, `,`.

// spec is a match of RE_SPEC; "" for a group that did not take part, which is undefined
// upstream and never empty when it takes part.
type spec struct {
	sign, dollar, comma, precision, tilde, typ string
}

// matchSpec is /^(?:.?[<>=^])?([-+( ])?(\$)?0?\d*(,)?(?:\.(\d+))?(~)?([a-zA-Z%])?$/ on
// UTF-16 code units, trying alternatives in the order a backtracking engine does.
func matchSpec(s []uint16) (spec, bool) {
	n := len(s)
	at := func(i int) int {
		if i < n {
			return int(s[i])
		}
		return -1
	}
	isAlign := func(c int) bool { return c == '<' || c == '>' || c == '=' || c == '^' }
	var m spec
	str := func(from, to int) string { return jsstr.FromUTF16(s[from:to]) }

	// The tail from (,)? on: every part is optional and only one way can reach the end.
	tail := func(i int) bool {
		var t spec
		if at(i) == ',' {
			t.comma = ","
			i++
		}
		if at(i) == '.' && util.IsDigit(at(i+1)) {
			j := i + 1
			for util.IsDigit(at(j)) {
				j++
			}
			t.precision = str(i+1, j)
			i = j
		}
		if at(i) == '~' {
			t.tilde = "~"
			i++
		}
		if c := at(i); util.IsLetter(c) || c == '%' {
			t.typ = str(i, i+1)
			i++
		}
		if i != n {
			return false
		}
		m.comma, m.precision, m.tilde, m.typ = t.comma, t.precision, t.tilde, t.typ
		return true
	}
	// 0?\d* then the tail. \d* backs off one digit at a time, but the tail cannot start with
	// a digit, so only the longest run can succeed; 0? taking or not taking the first zero
	// leaves \d* the same end.
	digits := func(i int) bool {
		for util.IsDigit(at(i)) {
			i++
		}
		return tail(i)
	}
	dollar := func(i int) bool {
		if at(i) == '$' {
			if digits(i + 1) {
				m.dollar = "$"
				return true
			}
		}
		m.dollar = ""
		return digits(i)
	}
	sign := func(i int) bool {
		if c := at(i); c == '-' || c == '+' || c == '(' || c == ' ' {
			if dollar(i + 1) {
				m.sign = str(i, i+1)
				return true
			}
		}
		m.sign = ""
		return dollar(i)
	}
	if n > 0 && !util.IsLineEnd(at(0)) && isAlign(at(1)) && sign(2) {
		return m, true
	}
	if isAlign(at(0)) && sign(1) {
		return m, true
	}
	return m, sign(0)
}

func group(digits string) string {
	out := ""
	for i := len(digits); i > 0; i -= 3 {
		part := digits[max(0, i-3):i]
		if out != "" {
			out = part + "," + out
		} else {
			out = part
		}
	}
	return out
}

// trimZeros is body.replace(/\.?0+$/, ”) for a body with a point: the trailing zeros, and the
// point when nothing follows it.
func trimZeros(body string) string {
	end := len(body)
	for end > 0 && body[end-1] == '0' {
		end--
	}
	if end == len(body) {
		return body
	}
	if end > 0 && body[end-1] == '.' {
		end--
	}
	return body[:end]
}

func formatter(specifier string) func(value float64) string {
	// Mermaid documents `$0,0` for a dollar amount with thousands separators.
	if specifier == "$0,0" {
		specifier = "$,"
	}
	m, ok := matchSpec(jsstr.UTF16(specifier))
	if !ok {
		m = spec{comma: ","}
	}
	plus := m.sign == "+"
	comma := m.comma != ""
	precision := -1
	if m.precision != "" {
		precision = int(min(jsnum.Parse(m.precision), 20))
	}
	or := func(fallback int) int {
		if precision == -1 {
			return fallback
		}
		return precision
	}

	return func(value float64) string {
		abs := math.Abs(value)
		suffix := ""
		var body string
		switch m.typ {
		case "f":
			body = jsnum.ToFixed(abs, or(6))
		case "%":
			body = jsnum.ToFixed(abs*100, or(6))
			suffix = "%"
		case "d":
			body = jsnum.ToFixed(jsmath.Round(abs), 0)
		case "e":
			body = jsnum.ToExponential(abs, or(6))
		default:
			body = jsnum.Format(jsnum.Parse(jsnum.ToPrecision(abs, max(1, or(12)))))
		}

		if m.tilde != "" && strings.Contains(body, ".") && !strings.Contains(body, "e") {
			body = trimZeros(body)
		}
		if comma && !strings.Contains(body, "e") {
			if dot := strings.IndexByte(body, '.'); dot == -1 {
				body = group(body)
			} else {
				body = group(body[:dot]) + body[dot:]
			}
		}
		prefix := ""
		if value < 0 {
			prefix = "-"
		} else if plus {
			prefix = "+"
		}
		return prefix + m.dollar + body + suffix
	}
}
