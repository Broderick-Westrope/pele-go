package class

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

// Lexer states, upstream's const enum S.
const (
	sInitial = iota
	sString
	sBqstring
	sGeneric
	sCallbackName
	sCallbackArgs
	sAccTitle
	sAccDescr
	sAccDescrMultiline
	sCls
	sClassBody
	sNamespace
	sNamespaceBody
)

// atEnd is the token each state gives at the end of input, before the parser sees the end
// itself.
var atEnd = [...]int{
	tEof,
	tStr,
	tEof,
	tGenerictype,
	tCallbackName,
	tCallbackArgs,
	tAccTitleValue,
	tAccDescrValue,
	tAccDescrMultilineValue,
	tEof,
	tEofInStruct,
	tEof,
	tEofInStruct,
}

var directionWords = [4]string{"TB", "BT", "RL", "LR"}

func indexOf(src common.Src, word string, from int) int {
	for q := max(from, 0); q+len(word) <= len(src); q++ {
		if startsWith(src, word, q) {
			return q
		}
	}
	return -1
}

// startsWith is src.startsWith(word, p) for an ASCII word.
func startsWith(src common.Src, word string, p int) bool {
	if p < 0 || p+len(word) > len(src) {
		return false
	}
	for i := 0; i < len(word); i++ {
		if src[p+i] != uint16(word[i]) {
			return false
		}
	}
	return true
}

// matchDirection is the sticky /.*direction\s+XX[^\n]*/y at p, for XX one of directionWords,
// with lineEnd the first line terminator at or after p. It returns the end of the match, or
// -1. The greedy .* backs off from the end of the line, so the last "direction" on the line
// that the pattern fits is the one matched.
func matchDirection(src common.Src, p, lineEnd int, word string) int {
	n := len(src)
	for q := lineEnd - 1; q >= p; q-- {
		if !startsWith(src, "direction", q) {
			continue
		}
		r := q + 9
		for r < n && util.IsSpace(int(src[r])) {
			r++
		}
		if r == q+9 || !startsWith(src, word, r) {
			continue
		}
		e := r + 2
		for e < n && src[e] != '\n' {
			e++
		}
		return e
	}
	return -1
}

func tokenize(src common.Src) tokens {
	n := len(src)
	var out tokens
	var stack []int
	state := sInitial
	p := 0
	dirNext := indexOf(src, "direction", 0)
	lineEnd := -1
	dirFailed := 0

	at := func(q int) int {
		if q < 0 || q >= n {
			return -1
		}
		return int(src[q])
	}
	emit := func(typ, s, e int) bool {
		out.types = append(out.types, typ)
		out.texts = append(out.texts, jsstr.FromUTF16(src[s:e]))
		out.starts = append(out.starts, s)
		p = e
		return true
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
	kw := func(word string) bool { return startsWith(src, word, p) && !util.IsWord(at(p+len(word))) }
	skipWs := func(q int) int {
		for q < n && util.IsSpace(int(src[q])) {
			q++
		}
		return q
	}
	until := func(ch uint16) int {
		for e := p; e < n; e++ {
			if src[e] == ch {
				return e
			}
		}
		return n
	}

	// A run of whitespace is a NEWLINE up to its last line feed; the rest is skipped.
	space := func(leaves bool) {
		q := p
		k := -1
		for ; q < n && util.IsSpace(int(src[q])); q++ {
			if src[q] == 10 {
				k = q
			}
		}
		if k < 0 {
			p = q
			return
		}
		if leaves {
			pop()
		}
		emit(tNewline, p, k+1)
	}

	// The keywords that every state recognizes, even inside generics and backtick names.
	keyword := func(c int) bool {
		switch c {
		case 99:
			if kw("cssClass") {
				return emit(tCssclass, p, p+8)
			}
			if kw("callback") {
				return emit(tCallback, p, p+8)
			}
			if kw("click") {
				return emit(tClick, p, p+5)
			}
		case 108:
			if kw("link") {
				return emit(tLink, p, p+4)
			}
		case 110:
			if kw("note for") {
				return emit(tNoteFor, p, p+8)
			}
			if kw("note") {
				return emit(tNote, p, p+4)
			}
		case 60:
			if at(p+1) == 60 {
				return emit(tAnnotationStart, p, p+2)
			}
		case 62:
			if at(p+1) == 62 {
				return emit(tAnnotationEnd, p, p+2)
			}
		case 104:
			if kw("href") {
				return emit(tHref, p, p+4)
			}
		}
		return false
	}

	commonToken := func(c int) bool {
		if keyword(c) {
			return true
		}
		d := at(p + 1)
		switch c {
		case 34:
			push(sString)
			p++
			return true
		case 126:
			push(sGeneric)
			p++
			return true
		case 96:
			push(sBqstring)
			p++
			return true
		case 95:
			if kw("_self") {
				return emit(tLinkTarget, p, p+5)
			}
			if kw("_blank") {
				return emit(tLinkTarget, p, p+6)
			}
			if kw("_parent") {
				return emit(tLinkTarget, p, p+7)
			}
			if kw("_top") {
				return emit(tLinkTarget, p, p+4)
			}
		case 60:
			if d == 124 {
				return emit(tExtension, p, p+2)
			}
			return emit(tDependency, p, p+1)
		case 124:
			return d == 62 && emit(tExtension, p, p+2)
		case 62:
			return emit(tDependency, p, p+1)
		case 42:
			return emit(tComposition, p, p+1)
		case 111:
			if !util.IsWord(d) {
				return emit(tAggregation, p, p+1)
			}
		case 40:
			return d == 41 && emit(tLollipop, p, p+2)
		case 45:
			if d == 45 {
				return emit(tLine, p, p+2)
			}
			return emit(tMinus, p, p+1)
		case 46:
			if d == 46 {
				return emit(tDottedLine, p, p+2)
			}
			return emit(tDot, p, p+1)
		case 58:
			q := p + 1
			for q < n {
				ch := src[q]
				if ch == 58 || ch == 10 || ch == 59 {
					break
				}
				q++
			}
			if q > p+1 {
				return emit(tLabel, p, q)
			}
			return d == 58 && at(p+2) == 58 && emit(tStyleSeparator, p, p+3)
		case 43:
			return emit(tPlus, p, p+1)
		case 37:
			return emit(tPct, p, p+1)
		case 61:
			return emit(tEquals, p, p+1)
		case 91:
			return emit(tSqs, p, p+1)
		case 93:
			return emit(tSqe, p, p+1)
		case 33, 35, 36, 38, 39, 44, 63, 92, 47:
			return emit(tPunctuation, p, p+1)
		}
		if util.IsWord(c) {
			q := p + 1
			for util.IsWord(at(q)) {
				q++
			}
			return emit(tAlpha, p, q)
		}
		return util.IsUnicodeLetter(c) && emit(tUnicodeText, p, p+1)
	}

	direction := func() bool {
		if dirNext == -1 || p < dirFailed {
			return false
		}
		if dirNext < p {
			dirNext = indexOf(src, "direction", p)
		}
		if dirNext == -1 {
			return false
		}
		if lineEnd < p {
			lineEnd = n
			for q := p; q < n; q++ {
				if util.IsLineEnd(int(src[q])) {
					lineEnd = q
					break
				}
			}
		}
		if dirNext >= lineEnd {
			return false
		}
		for i, word := range directionWords {
			if e := matchDirection(src, p, lineEnd, word); e >= 0 {
				return emit(tDirectionTb+i, p, e)
			}
		}
		// No direction statement starts anywhere in the rest of this line either.
		dirFailed = lineEnd
		return false
	}

	initial := func(c int) bool {
		if direction() {
			return true
		}
		if util.IsSpace(c) {
			space(false)
			return true
		}
		switch c {
		case 37:
			if at(p+1) == 37 {
				e := until('\n')
				for e < n && (src[e] == 10 || src[e] == 13) {
					e++
				}
				p = e
				return true
			}
		case 97:
			title := startsWith(src, "accTitle", p)
			if !title && !startsWith(src, "accDescr", p) {
				break
			}
			q := skipWs(p + 8)
			d := at(q)
			if d == 58 {
				typ, next := tAccDescr, sAccDescr
				if title {
					typ, next = tAccTitle, sAccTitle
				}
				emit(typ, p, skipWs(q+1))
				push(next)
				return true
			}
			if d == 123 && !title {
				p = skipWs(q + 1)
				push(sAccDescrMultiline)
				return true
			}
		case 99:
			if kw("classDiagram-v2") {
				return emit(tClassDiagram, p, p+15)
			}
			if kw("classDiagram") {
				return emit(tClassDiagram, p, p+12)
			}
			if startsWith(src, "call", p) && util.IsSpace(at(p+4)) {
				p = skipWs(p + 4)
				push(sCallbackName)
				return true
			}
			if kw("classDef") {
				return emit(tClassdef, p, p+8)
			}
			if kw("class") {
				push(sCls)
				return emit(tClass, p, p+5)
			}
		case 91:
			if startsWith(src, "*]", p+1) {
				return emit(tEdgeState, p, p+3)
			}
		case 115:
			if kw("style") {
				return emit(tStyle, p, p+5)
			}
		case 110:
			if kw("namespace") {
				push(sNamespace)
				return emit(tNamespace, p, p+9)
			}
		case 44:
			return emit(tComma, p, p+1)
		case 35:
			return emit(tBrkt, p, p+1)
		}
		return commonToken(c) || (c == 58 && emit(tColon, p, p+1))
	}

	for {
		if p >= n {
			out.types = append(out.types, atEnd[state])
			out.texts = append(out.texts, "")
			out.starts = append(out.starts, n)
			break
		}
		c := int(src[p])
		ok := true
		switch state {
		case sInitial:
			ok = initial(c)

		case sCls:
			switch {
			case util.IsSpace(c):
				space(true)
			case c == 125:
				pop()
				pop()
				emit(tStructStop, p, p+1)
			case c == 123:
				push(sClassBody)
				emit(tStructStart, p, p+1)
			default:
				ok = commonToken(c)
			}

		case sClassBody:
			switch {
			case c == 34:
				push(sString)
				p++
			case c == 125:
				pop()
				emit(tStructStop, p, p+1)
			case c == 91 && startsWith(src, "*]", p+1):
				emit(tEdgeState, p, p+3)
			case c == 123:
				emit(tOpenInStruct, p, p+1)
			case c == 10:
				p++
			default:
				q := p + 1
				for q < n {
					ch := src[q]
					if ch == 123 || ch == 125 || ch == 10 {
						break
					}
					q++
				}
				emit(tMember, p, q)
			}

		case sNamespace:
			switch {
			case util.IsSpace(c):
				space(true)
			case c == 123:
				push(sNamespaceBody)
				emit(tStructStart, p, p+1)
			case c == 125:
				// The brace is read again in the state the namespace was opened from.
				pop()
			case c == 110 && kw("namespace"):
				push(sNamespace)
				emit(tNamespace, p, p+9)
			default:
				ok = commonToken(c)
			}

		case sNamespaceBody:
			switch {
			case util.IsSpace(c):
				space(false)
			case c == 125:
				pop()
				emit(tStructStop, p, p+1)
			case c == 110 && kw("namespace"):
				push(sNamespace)
				emit(tNamespace, p, p+9)
			case c == 91 && startsWith(src, "*]", p+1):
				emit(tEdgeState, p, p+3)
			case c == 99 && kw("class"):
				push(sCls)
				emit(tClass, p, p+5)
			default:
				ok = commonToken(c)
			}

		case sString:
			if c == 34 {
				pop()
				p++
			} else {
				emit(tStr, p, until('"'))
			}

		case sGeneric:
			switch {
			case c == 34:
				push(sString)
				p++
			case keyword(c):
			case c == 126:
				pop()
				p++
			default:
				emit(tGenerictype, p, until('~'))
			}

		case sBqstring:
			switch {
			case c == 34:
				push(sString)
				p++
			case keyword(c):
			case c == 126:
				push(sGeneric)
				p++
			case c == 96:
				pop()
				p++
			default:
				emit(tBquoteStr, p, until('`'))
			}

		case sCallbackName:
			if c == 40 {
				q := skipWs(p + 1)
				pop()
				if at(q) == 41 {
					p = q + 1
				} else {
					push(sCallbackArgs)
					p++
				}
			} else {
				emit(tCallbackName, p, until('('))
			}

		case sCallbackArgs:
			if c == 41 {
				pop()
				p++
			} else {
				emit(tCallbackArgs, p, until(')'))
			}

		case sAccTitle, sAccDescr:
			typ := tAccDescrValue
			if state == sAccTitle {
				typ = tAccTitleValue
			}
			emit(typ, p, until('\n'))
			pop()

		case sAccDescrMultiline:
			if c == 125 {
				pop()
				p++
			} else {
				emit(tAccDescrMultilineValue, p, until('}'))
			}
		}

		if !ok {
			out.types = append(out.types, tError)
			out.texts = append(out.texts, "")
			out.starts = append(out.starts, p)
			return out
		}
	}

	out.types = append(out.types, tEnd)
	out.texts = append(out.texts, "")
	out.starts = append(out.starts, n)
	return out
}
