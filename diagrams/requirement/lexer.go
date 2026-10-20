package requirement

import (
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

type tokens struct {
	types  []int
	texts  []string
	starts []int
}

// Lexer states.
const (
	sInitial = iota
	sString
	sStyle
	sAccTitle
	sAccDescr
	sAccDescrMultiline
)

const longestKeyword = 22

// isStop: characters that end an unquoted string are `:,{<>-=` and line breaks.
func isStop(c int) bool {
	switch c {
	case ':', ',', '\r', '\n', '{', '<', '>', '-', '=':
		return true
	}
	return false
}

var directionWords = [4]string{"tb", "bt", "rl", "lr"}

// tokenize produces the token stream of Mermaid's requirement lexer. Its rules are tried in
// order and the first match wins, so a line that holds a direction statement anywhere is one
// token.
func tokenize(src common.Src) tokens {
	n := len(src)
	var out tokens
	var stack []int
	state := sInitial
	p := 0

	// ciAt matches a lower-case ASCII word at q, ignoring case.
	ciAt := func(q int, word string) bool {
		for i := 0; i < len(word); i++ {
			k := int(word[i])
			c := src.At(q + i)
			if c != k && (k < 97 || k > 122 || (c|32) != k) {
				return false
			}
		}
		return true
	}
	skipSpace := func(q int) int {
		for util.IsSpace(src.At(q)) {
			q++
		}
		return q
	}
	indexOf := func(c uint16, from int) int {
		for q := max(from, 0); q < n; q++ {
			if src[q] == c {
				return q
			}
		}
		return -1
	}

	// Where each kind of direction statement starts and where its keyword ends, found in one
	// pass of /direction\s+(?:(TB)|(BT)|(RL)|LR)/gi.
	var dirStarts, dirEnds [4][]int
	var dirSeen [4]int
	anyDir := false
	lineEnd := -1
	for q := 0; q+9 <= n; {
		if !ciAt(q, "direction") || !util.IsSpace(src.At(q+9)) {
			q++
			continue
		}
		r := skipSpace(q + 9)
		kind := -1
		for k, word := range directionWords {
			if ciAt(r, word) {
				kind = k
				break
			}
		}
		if kind < 0 {
			q++
			continue
		}
		dirStarts[kind] = append(dirStarts[kind], q)
		dirEnds[kind] = append(dirEnds[kind], r+2)
		anyDir = true
		q = r + 2
	}

	emit := func(typ, s, e int) {
		out.types = append(out.types, typ)
		out.texts = append(out.texts, src.String(s, e))
		out.starts = append(out.starts, s)
		p = e
	}
	push := func(s int) {
		stack = append(stack, state)
		state = s
	}
	pop := func() {
		if len(stack) > 0 {
			state = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		}
	}
	// accAt is /<word>\s*<sep>\s*/iy at q.
	accAt := func(q int, word string, sep int) int {
		if !ciAt(q, word) {
			return -1
		}
		q = skipSpace(q + len(word))
		if src.At(q) != sep {
			return -1
		}
		return skipSpace(q + 1)
	}
	lineBreak := func(q int) int {
		if e := indexOf('\n', q); e != -1 {
			return e
		}
		return n
	}

	// direction is `.*direction\s+XX[^\n]*`: the last such statement that starts on this line
	// decides the token.
	direction := func() bool {
		if lineEnd < p {
			lineEnd = n
			for q := p; q < n; q++ {
				if util.IsLineEnd(int(src[q])) {
					lineEnd = q
					break
				}
			}
			for k := range 4 {
				list := dirStarts[k]
				j := dirSeen[k]
				for j < len(list) && list[j] <= lineEnd {
					j++
				}
				dirSeen[k] = j
			}
		}
		for k := range 4 {
			j := dirSeen[k] - 1
			if j >= 0 && dirStarts[k][j] >= p {
				emit(tDirectionTB+k, p, lineBreak(dirEnds[k][j]))
				return true
			}
		}
		return false
	}

	initial := func(c int) bool {
		lower := c | 32
		if lower == 116 && ciAt(p+1, "itle") && util.IsSpace(src.At(p+5)) {
			q := p + 6
			for q < n {
				d := src[q]
				if d == 35 || d == 10 || d == 59 {
					break
				}
				q++
			}
			if q > p+6 {
				emit(tTitle, p, q)
				return true
			}
		}
		if lower == 97 && src.At(p+1)|32 == 99 && src.At(p+2)|32 == 99 {
			if e := accAt(p, "acctitle", ':'); e >= 0 {
				push(sAccTitle)
				emit(tAccTitle, p, e)
				return true
			}
			if e := accAt(p, "accdescr", ':'); e >= 0 {
				push(sAccDescr)
				emit(tAccDescr, p, e)
				return true
			}
			if e := accAt(p, "accdescr", '{'); e >= 0 {
				push(sAccDescrMultiline)
				p = e
				return true
			}
		}
		if anyDir && !util.IsLineEnd(c) && direction() {
			return true
		}

		if c == 10 || (c == 13 && src.At(p+1) == 10) {
			q := p
			for q < n {
				d := src[q]
				if d == 10 {
					q++
				} else if d == 13 && src.At(q+1) == 10 {
					q += 2
				} else {
					break
				}
			}
			emit(tNewline, p, q)
			return true
		}
		if util.IsSpace(c) {
			p++
			for p < n && util.IsSpace(int(src[p])) {
				p++
			}
			return true
		}
		if c == 35 || c == 37 {
			p = lineBreak(p)
			return true
		}

		if util.IsWord(c) {
			q := p + 1
			for q < n && util.IsWord(int(src[q])) {
				q++
			}
			if q-p <= longestKeyword {
				word := make([]byte, q-p)
				for i := range word {
					word[i] = byte(src[p+i])
					if word[i] >= 'A' && word[i] <= 'Z' {
						word[i] += 32
					}
				}
				if keyword, ok := keywords[string(word)]; ok {
					if keyword == tStyle || keyword == tClassDef || keyword == tClass {
						push(sStyle)
					}
					emit(keyword, p, q)
					return true
				}
			}
			for q < n && !isStop(int(src[q])) {
				q++
			}
			out.types = append(out.types, tUnqString)
			out.texts = append(out.texts, jsstr.Trim(src.String(p, q)))
			out.starts = append(out.starts, p)
			p = q
			return true
		}

		switch c {
		case 123:
			emit(tStructStart, p, p+1)
			return true
		case 125:
			emit(tStructStop, p, p+1)
			return true
		case 58:
			if src.At(p+1) == 58 && src.At(p+2) == 58 {
				emit(tStyleSeparator, p, p+3)
			} else {
				emit(tColonSep, p, p+1)
			}
			return true
		case 60:
			if src.At(p+1) != 45 {
				return false
			}
			emit(tEndArrowL, p, p+2)
			return true
		case 45:
			if src.At(p+1) == 62 {
				emit(tEndArrowR, p, p+2)
			} else {
				emit(tLine, p, p+1)
			}
			return true
		case 34:
			push(sString)
			p++
			return true
		case 44:
			emit(tComma, p, p+1)
			return true
		}
		return false
	}

	style := func(c int) bool {
		if util.IsWord(c) {
			q := p + 1
			for q < n && util.IsWord(int(src[q])) {
				q++
			}
			emit(tAlpha, p, q)
			return true
		}
		switch c {
		case 58:
			emit(tColon, p, p+1)
		case 59:
			emit(tSemicolon, p, p+1)
		case 37:
			emit(tPercent, p, p+1)
		case 45:
			emit(tMinus, p, p+1)
		case 35:
			emit(tBrkt, p, p+1)
		case 32:
			p++
		case 34:
			push(sString)
			p++
		case 10:
			pop()
			p++
		case 44:
			emit(tComma, p, p+1)
		default:
			return false
		}
		return true
	}

	for p < n {
		c := int(src[p])
		ok := true
		switch state {
		case sInitial:
			ok = initial(c)
		case sStyle:
			ok = style(c)
		case sString:
			if c == 34 {
				pop()
				p++
			} else {
				e := indexOf('"', p)
				if e == -1 {
					e = n
				}
				emit(tQString, p, e)
			}
		case sAccTitle:
			pop()
			emit(tAccTitleValue, p, lineBreak(p))
		case sAccDescr:
			pop()
			emit(tAccDescrValue, p, lineBreak(p))
		case sAccDescrMultiline:
			if c == 125 {
				pop()
				p++
			} else {
				e := indexOf('}', p)
				if e == -1 {
					e = n
				}
				emit(tAccDescrMultilineValue, p, e)
			}
		}
		if !ok {
			out.types = append(out.types, tError)
			out.texts = append(out.texts, "")
			out.starts = append(out.starts, p)
			return out
		}
	}

	// At the end of input Jison still tries the rules that can match nothing, once.
	last := -1
	switch state {
	case sInitial:
		last = tEOF
	case sString:
		last = tQString
	case sAccTitle:
		last = tAccTitleValue
	case sAccDescr:
		last = tAccDescrValue
	case sAccDescrMultiline:
		last = tAccDescrMultilineValue
	}
	if last != -1 {
		emit(last, n, n)
	}
	emit(tEnd, n, n)
	return out
}
