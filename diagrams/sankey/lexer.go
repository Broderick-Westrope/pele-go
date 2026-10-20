package sankey

import (
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

const (
	tEnd = iota
	// Not a Jison token: the reference lexer would return the same empty token forever from here.
	tStuck
	tSankey
	tNewline
	tComma
	tDQuote
	tNonEscapedText
	tEscapedText
	tEOF
)

var tokenNames = [...]string{
	"$end",
	"STUCK",
	"SANKEY",
	"NEWLINE",
	"COMMA",
	"DQUOTE",
	"NON_ESCAPED_TEXT",
	"ESCAPED_TEXT",
	"EOF",
}

type tokens struct {
	types  []int
	texts  []string
	starts []int
}

// Lexer states.
const (
	sInitial = iota
	sCSV
	sEscapedText
)

// isTextData is RFC 4180 TEXTDATA: printable ASCII except the double quote and the comma.
func isTextData(c int) bool {
	return c >= 32 && c <= 126 && c != 34 && c != 44
}

// lowerUnit is a code unit as toLowerCase leaves it, where that is ASCII. The Kelvin sign
// is the one character outside ASCII that lowers to an ASCII letter in a single unit.
func lowerUnit(c int) int {
	if c >= 65 && c <= 90 {
		return c + 32
	}
	if c == 0x212a {
		return 'k'
	}
	return c
}

func tokenize(src common.Src) tokens {
	n := len(src)
	var out tokens
	var stack []int
	state := sInitial
	p := 0

	emit := func(typ, s, e int) {
		out.types = append(out.types, typ)
		out.texts = append(out.texts, src.String(s, e))
		out.starts = append(out.starts, s)
		p = e
	}
	// keyword is src.slice(p, p + word.length).toLowerCase() === word with a word boundary
	// after it.
	keyword := func(word string) bool {
		if p+len(word) > n {
			return false
		}
		for k := 0; k < len(word); k++ {
			if lowerUnit(int(src[p+k])) != int(word[k]) {
				return false
			}
		}
		return !util.IsWord(src.At(p + len(word)))
	}

	for p < n {
		c := int(src[p])
		start := p
		q := p
		if state == sEscapedText {
			if c == 34 && src.At(p+1) != 34 {
				if len(stack) > 0 {
					state = stack[len(stack)-1]
					stack = stack[:len(stack)-1]
				}
				emit(tDQuote, p, p+1)
				continue
			}
			for q < n {
				d := int(src[q])
				if isTextData(d) || d == 44 || d == 13 || d == 10 {
					q++
				} else if d == 34 && src.At(q+1) == 34 {
					q += 2
				} else {
					break
				}
			}
			emit(tEscapedText, p, q)
		} else {
			if state == sInitial && c|32 == 115 {
				length := 0
				if keyword("sankey-beta") {
					length = 11
				} else if keyword("sankey") {
					length = 6
				}
				if length > 0 {
					stack = append(stack, state)
					state = sCSV
					emit(tSankey, p, p+length)
					continue
				}
			}
			if c == 10 || (c == 13 && src.At(p+1) == 10) {
				end := p + 2
				if c == 10 {
					end = p + 1
				}
				emit(tNewline, p, end)
				continue
			}
			if c == 44 {
				emit(tComma, p, p+1)
				continue
			}
			if c == 34 {
				stack = append(stack, state)
				state = sEscapedText
				emit(tDQuote, p, p+1)
				continue
			}
			for q < n && isTextData(int(src[q])) {
				q++
			}
			emit(tNonEscapedText, p, q)
		}
		// A character outside the CSV alphabet matches as empty text and is never consumed.
		if q == start {
			emit(tStuck, p, p)
			return out
		}
	}

	if state == sEscapedText {
		emit(tEscapedText, n, n)
	} else {
		emit(tEOF, n, n)
	}
	emit(tEnd, n, n)
	return out
}
