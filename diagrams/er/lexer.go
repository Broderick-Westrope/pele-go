package er

import (
	"sync"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
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
	sBlock
	sBlockBq
	sAccTitle
	sAccDescr
	sAccDescrMultiline
	sStyle
)

const (
	classText      = 1
	classStyleText = 2
	classAttrStart = 4
	classAttrRest  = 8
)

var charClass = sync.OnceValue(func() [128]uint8 {
	var class [128]uint8
	for c := 0; c < 128; c++ {
		word := (c >= 48 && c <= 57) || (c >= 65 && c <= 90) || (c >= 97 && c <= 122) || c == 95
		letter := word && (c < 48 || c > 57)
		if word {
			class[c] |= classText | classStyleText | classAttrRest
		}
		if letter {
			class[c] |= classAttrStart
		}
	}
	for _, ch := range "-*" {
		class[ch] |= classText | classStyleText | classAttrRest
	}
	class[46] |= classText | classAttrRest
	class[42] |= classAttrStart
	for _, ch := range "[]()," {
		class[ch] |= classAttrRest
	}
	return class
})

// hasClass is CLASS[c] & kind for an ASCII c; past the end of the source (-1) it is false,
// as CLASS[NaN] is undefined upstream.
func hasClass(c, kind int) bool {
	return c >= 0 && c < 128 && int(charClass()[c])&kind != 0
}

// isAttr: Mermaid's attribute pattern is case-insensitive, which lets the micro sign into its
// \u00c0-\uffff range.
func isAttr(c, kind int) bool {
	if c < 0 {
		return false
	}
	if c < 128 {
		return hasClass(c, kind)
	}
	return c >= 0xc0 || c == 0xb5
}

var directionWords = [4]string{"tb", "bt", "rl", "lr"}

func tokenize(src common.Src) tokens {
	n := len(src)
	var out tokens
	state := sInitial
	p := 0
	dirNext := -2
	dirFailed := 0
	lineEnd := -1
	tildeNext := -2
	tildeLine := -1
	tildeLast := -1
	wsNext := -1

	emit := func(typ, s, e int) {
		out.types = append(out.types, typ)
		out.texts = append(out.texts, src.String(s, e))
		out.starts = append(out.starts, s)
		p = e
	}
	// ciAt matches a lower-case keyword at q, ignoring case.
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
	kw := func(word string) bool { return ciAt(p, word) }
	kwb := func(word string) bool { return kw(word) && !util.IsWord(src.At(p+len(word))) }
	lower := func(q int) int {
		c := src.At(q)
		if c >= 65 && c <= 90 {
			return c + 32
		}
		return c
	}
	nextLineEnd := func(q int) int {
		for ; q < n; q++ {
			if util.IsLineEnd(int(src[q])) {
				return q
			}
		}
		return n
	}
	indexOf := func(c uint16, from int) int {
		for q := max(from, 0); q < n; q++ {
			if src[q] == c {
				return q
			}
		}
		return -1
	}
	digits := func(q int) int {
		for util.IsDigit(src.At(q)) {
			q++
		}
		return q
	}
	skipSpace := func(q int) int {
		for util.IsSpace(src.At(q)) {
			q++
		}
		return q
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
	// directionAt is /.*direction\s+<word>[^\n]*/iy at q. The greedy .* makes the last
	// "direction" on the line that is followed by the word the one that matches.
	directionAt := func(q int, word string) int {
		end := nextLineEnd(q)
		for at := end - 9; at >= q; at-- {
			if !ciAt(at, "direction") {
				continue
			}
			r := at + 9
			if !util.IsSpace(src.At(r)) {
				continue
			}
			r = skipSpace(r)
			if !ciAt(r, word) {
				continue
			}
			if e := indexOf('\n', r+2); e != -1 {
				return e
			}
			return n
		}
		return -1
	}
	findDirection := func(from int) int {
		for q := from; q+9 <= n; q++ {
			if ciAt(q, "direction") {
				return q
			}
		}
		return -1
	}

	// A direction statement swallows its whole line, from wherever the lexer stands on it.
	direction := func() bool {
		if dirNext == -1 {
			return false
		}
		if dirNext < p {
			dirNext = findDirection(p)
			if dirNext == -1 {
				return false
			}
		}
		if p < dirFailed {
			return false
		}
		if lineEnd < p {
			lineEnd = nextLineEnd(p)
		}
		if dirNext >= lineEnd {
			return false
		}
		for i, word := range directionWords {
			if e := directionAt(p, word); e >= 0 {
				emit(tDirectionTB+i, p, e)
				return true
			}
		}
		dirFailed = lineEnd
		return false
	}

	quoted := func() {
		e := indexOf('"', p+1)
		if e == -1 {
			emit(tChar, p, p+1)
			return
		}
		name := e > p+1
		for q := p + 1; name && q < e; q++ {
			c := src[q]
			if c == 37 || c == 92 || c == 13 || c == 10 || c == 11 || c == 8 {
				name = false
			}
		}
		if name {
			emit(tEntityName, p, e+1)
		} else {
			emit(tWord, p, e+1)
		}
	}

	number := func(c int) {
		d := src.At(p + 1)
		if d == 43 && (c == 49 || c == 48) {
			if c == 49 {
				emit(tOneOrMore, p, p+2)
			} else {
				emit(tZeroOrMore, p, p+2)
			}
			return
		}
		q := digits(p + 1)
		if src.At(q) == 46 && util.IsDigit(src.At(q+1)) {
			emit(tDecimalNum, p, digits(q+2))
			return
		}
		if c == 49 {
			if util.IsSpace(d) {
				r := skipSpace(p + 2)
				f := src.At(r)
				if util.IsWord(f) || f == 34 || f == 39 {
					emit(tOnlyOne, p, p+1)
					return
				}
			} else if d == 45 || d == 46 {
				f := src.At(p + 2)
				if f == 45 || f == 46 {
					emit(tOnlyOne, p, p+1)
					return
				}
			}
			if !util.IsWord(d) {
				emit(tEntityOne, p, p+1)
				return
			}
		}
		emit(tNum, p, q)
	}

	keyword := func(lc int) bool {
		d := src.At(p + 1)
		switch lc {
		case 101:
			if kwb("erdiagram") {
				emit(tErDiagram, p, p+9)
			} else if kwb("end") {
				emit(tEndKw, p, skipSpace(p+3))
			} else {
				return false
			}
			return true
		case 115:
			if kwb("style") {
				state = sStyle
				emit(tStyle, p, p+5)
			} else if kwb("subgraph") {
				emit(tSubgraph, p, p+8)
			} else {
				return false
			}
			return true
		case 99:
			if kwb("classdef") {
				state = sStyle
				emit(tClassDef, p, p+8)
			} else if kwb("class") {
				emit(tClass, p, p+5)
			} else {
				return false
			}
			return true
		case 111:
			switch {
			case kwb("one or zero"):
				emit(tZeroOrOne, p, p+11)
			case kwb("one or more") || kwb("one or many"):
				emit(tOneOrMore, p, p+11)
			case kwb("one"):
				emit(tOnlyOne, p, p+3)
			case kwb("only one"):
				emit(tOnlyOne, p, p+8)
			case d == 124:
				emit(tZeroOrOne, p, p+2)
			case d == 123:
				emit(tZeroOrMore, p, p+2)
			case kwb("optionally to"):
				emit(tNonIdentifying, p, p+13)
			default:
				return false
			}
			return true
		case 122:
			if kwb("zero or one") {
				emit(tZeroOrOne, p, p+11)
			} else if kwb("zero or more") || kwb("zero or many") {
				emit(tZeroOrMore, p, p+12)
			} else {
				return false
			}
			return true
		case 109:
			if kw("many(0)") {
				emit(tZeroOrMore, p, p+7)
			} else if kw("many(1)") {
				emit(tOneOrMore, p, p+7)
			} else if kwb("many") {
				emit(tZeroOrMore, p, p+4)
			} else {
				return false
			}
			return true
		case 116:
			if !kwb("to") {
				return false
			}
			emit(tIdentifying, p, p+2)
			return true
		case 117:
			if d != 46 && d != 45 && d != 124 {
				return false
			}
			emit(tMdParent, p, p+1)
			return true
		case 124:
			if lower(p+1) == 111 && !util.IsWord(src.At(p+2)) {
				emit(tZeroOrOne, p, p+2)
			} else if d == 124 {
				emit(tOnlyOne, p, p+2)
			} else if d == 123 {
				emit(tOneOrMore, p, p+2)
			} else {
				return false
			}
			return true
		case 125:
			if lower(p+1) == 111 && !util.IsWord(src.At(p+2)) {
				emit(tZeroOrMore, p, p+2)
			} else if d == 124 {
				emit(tOneOrMore, p, p+2)
			} else {
				return false
			}
			return true
		case 46:
			if d != 46 && d != 45 {
				return false
			}
			emit(tNonIdentifying, p, p+2)
			return true
		case 45:
			if d == 45 {
				emit(tIdentifying, p, p+2)
			} else if d == 46 {
				emit(tNonIdentifying, p, p+2)
			} else {
				return false
			}
			return true
		}
		return false
	}

	initial := func(c int) {
		if (c == 97 || c == 65) && kw("acc") {
			if e := accAt(p, "acctitle", ':'); e >= 0 {
				state = sAccTitle
				emit(tAccTitle, p, e)
				return
			}
			if e := accAt(p, "accdescr", ':'); e >= 0 {
				state = sAccDescr
				emit(tAccDescr, p, e)
				return
			}
			if e := accAt(p, "accdescr", '{'); e >= 0 {
				state = sAccDescrMultiline
				p = e
				return
			}
		}
		if direction() {
			return
		}

		switch c {
		case 32, 9, 13:
			e := p + 1
			for d := src.At(e); d == 32 || d == 9 || d == 13; d = src.At(e) {
				e++
			}
			p = e
			return
		case 10:
			e := p + 1
			for src.At(e) == 10 {
				e++
			}
			emit(tNewline, p, e)
			return
		case 34:
			quoted()
			return
		case 123:
			state = sBlock
			emit(tBlockStart, p, p+1)
			return
		case 35:
			emit(tBrkt, p, p+1)
			return
		case 44:
			emit(tComma, p, p+1)
			return
		case 58:
			if src.At(p+1) == 58 && src.At(p+2) == 58 {
				emit(tStyleSeparator, p, p+3)
			} else {
				emit(tColon, p, p+1)
			}
			return
		case 91:
			emit(tSQS, p, p+1)
			return
		case 93:
			emit(tSQE, p, p+1)
			return
		}
		if util.IsDigit(c) {
			number(c)
			return
		}
		lc := c
		if c >= 65 && c <= 90 {
			lc = c + 32
		}
		if keyword(lc) {
			return
		}
		if c >= 128 || hasClass(c, classText) {
			e := p + 1
			for d := src.At(e); d >= 128 || hasClass(d, classText); d = src.At(e) {
				e++
			}
			emit(tUnicodeText, p, e)
			return
		}
		emit(tChar, p, p+1)
	}

	// generic is Mermaid's generic type rule: a run of non-space characters holding a `~`,
	// through the last `~` on the line.
	generic := func() bool {
		if tildeNext == -1 {
			return false
		}
		if tildeNext < p {
			tildeNext = indexOf('~', p)
			if tildeNext == -1 {
				return false
			}
		}
		if wsNext < p {
			q := p
			for q < n && !util.IsSpace(int(src[q])) {
				q++
			}
			wsNext = q
		}
		if tildeNext >= wsNext {
			return false
		}
		if tildeLine < p {
			tildeLine = nextLineEnd(p)
			tildeLast = -1
			// lastIndexOf('~', tildeLine - 1), whose position is clamped to 0.
			for q := min(max(tildeLine-1, 0), n-1); q >= 0; q-- {
				if src[q] == '~' {
					tildeLast = q
					break
				}
			}
		}
		if tildeLast <= tildeNext {
			return false
		}
		e := tildeLast + 1
		for e < n && !util.IsSpace(int(src[e])) {
			e++
		}
		emit(tAttributeWord, p, e)
		return true
	}

	block := func(c int) {
		if util.IsSpace(c) {
			p = skipSpace(p + 1)
			return
		}
		lc := c | 32
		if (lc == 112 || lc == 102 || lc == 117) && src.At(p+1)|32 == 107 && !util.IsWord(src.At(p+2)) {
			emit(tAttributeKey, p, p+2)
			return
		}
		if generic() {
			return
		}
		if isAttr(c, classAttrStart) {
			e := p + 1
			for e < n && isAttr(int(src[e]), classAttrRest) {
				e++
			}
			emit(tAttributeWord, p, e)
			return
		}
		if c == 96 {
			state = sBlockBq
			p++
			return
		}
		if c == 34 {
			if e := indexOf('"', p+1); e != -1 {
				emit(tComment, p, e+1)
				return
			}
		} else if c == 125 {
			state = sInitial
			emit(tBlockStop, p, p+1)
			return
		}
		emit(tChar, p, p+1)
	}

	orEnd := func(e int) int {
		if e == -1 {
			return n
		}
		return e
	}

	for p < n {
		c := int(src[p])
		switch state {
		case sInitial:
			initial(c)

		case sBlock:
			block(c)

		case sBlockBq:
			if c == 96 {
				state = sBlock
				p++
			} else {
				emit(tAttributeWord, p, orEnd(indexOf('`', p)))
			}

		case sAccTitle, sAccDescr:
			typ := tAccDescrValue
			if state == sAccTitle {
				typ = tAccTitleValue
			}
			emit(typ, p, orEnd(indexOf('\n', p)))
			state = sInitial

		case sAccDescrMultiline:
			if c == 125 {
				state = sInitial
				p++
			} else {
				emit(tAccDescrMultilineValue, p, orEnd(indexOf('}', p)))
			}

		case sStyle:
			switch {
			case c == 10:
				e := p + 1
				for src.At(e) == 10 {
					e++
				}
				state = sInitial
				emit(tNewline, p, e)
			case util.IsSpace(c):
				p = skipSpace(p + 1)
			case c == 58:
				emit(tColon, p, p+1)
			case c == 44:
				emit(tComma, p, p+1)
			case c == 35:
				emit(tBrkt, p, p+1)
			case c == 59:
				emit(tSemi, p, p+1)
			case c >= 128 || hasClass(c, classStyleText):
				e := p + 1
				for d := src.At(e); d >= 128 || hasClass(d, classStyleText); d = src.At(e) {
					e++
				}
				emit(tStyleText, p, e)
			default:
				out.types = append(out.types, tError)
				out.texts = append(out.texts, "")
				out.starts = append(out.starts, p)
				return out
			}
		}
	}

	// At the end of input Jison's INITIAL state yields EOF, the title and description states
	// yield an empty value, and the block and style states yield nothing.
	last := -1
	switch state {
	case sInitial:
		last = tEOF
	case sAccTitle:
		last = tAccTitleValue
	case sAccDescr:
		last = tAccDescrValue
	case sAccDescrMultiline:
		last = tAccDescrMultilineValue
	}
	if last != -1 {
		out.types = append(out.types, last)
		out.texts = append(out.texts, "")
		out.starts = append(out.starts, n)
	}
	out.types = append(out.types, tEnd)
	out.texts = append(out.texts, "")
	out.starts = append(out.starts, n)
	return out
}
