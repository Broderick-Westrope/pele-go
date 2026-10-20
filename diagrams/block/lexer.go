package block

import (
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
	sString
	sMDString
	sNode
	sBlockArrow
	sArrowDir
	sLLabel
	sClassDef
	sClassDefID
	sClass
	sClassStyle
	sStyleStmnt
	sStyleDefinition
	sAccTitle
	sAccDescr
	sAccDescrMultiline
)

var directions = [...]string{"right", "left", "x", "y", "up", "down"}

// idStop is the characters that end a node id: ( ) [ { } < > - : = and whitespace.
var idStop = func() (stop [128]bool) {
	for _, c := range []int{9, 10, 11, 12, 13, 32, 40, 41, 45, 58, 60, 61, 62, 91, 123, 125} {
		stop[c] = true
	}
	return stop
}()

func tokenize(src common.Src) tokens {
	n := len(src)
	var out tokens
	var stack []int
	state := sInitial
	p := 0

	put := func(typ int, text string, end int) bool {
		out.types = append(out.types, typ)
		out.texts = append(out.texts, text)
		out.starts = append(out.starts, p)
		p = end
		return true
	}
	emit := func(typ int, end int) bool { return put(typ, src.String(p, end), end) }
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
	ws := func(q int) int {
		for q < n && util.IsSpace(int(src[q])) {
			q++
		}
		return q
	}
	digits := func(q int) int {
		for q < n {
			if c := src[q]; c < 48 || c > 57 {
				break
			}
			q++
		}
		return q
	}
	word := func(q int) int {
		for q < n && util.IsWord(int(src[q])) {
			q++
		}
		return q
	}
	keyword := func(text string) bool { return src.HasPrefix(p, text) && !util.IsWord(src.At(p+len(text))) }

	quote := func() bool {
		if src.At(p+1) == 96 {
			push(sMDString)
			p += 2
		} else {
			push(sString)
			p++
		}
		return true
	}

	// End of `.+-[xo>]?` at q, or -1.
	dotted := func(q int) int {
		if src.At(q) != 46 {
			return -1
		}
		q++
		for src.At(q) == 46 {
			q++
		}
		if src.At(q) != 45 {
			return -1
		}
		if c := src.At(q + 1); c == 120 || c == 111 || c == 62 {
			return q + 2
		}
		return q + 1
	}

	// End of a complete link at q, before its trailing whitespace, or -1.
	linkEnd := func(q int) int {
		if c := src.At(q); c == 120 || c == 111 || c == 60 {
			q++
		}
		line := src.At(q)
		if line == 46 {
			return dotted(q)
		}
		if line != 45 && line != 61 {
			return -1
		}
		k := q + 1
		for src.At(k) == line {
			k++
		}
		if k-q == 1 {
			if line == 45 {
				return dotted(k)
			}
			return -1
		}
		if head := src.At(k); head == 120 || head == 111 || head == 62 {
			return k + 1
		}
		if k-q >= 3 {
			return k
		}
		return -1
	}

	link := func() bool {
		if e := linkEnd(p); e >= 0 {
			return emit(tLink, ws(e))
		}
		q := p
		if src.At(p) == 60 {
			q = p + 1
		}
		a := src.At(q)
		b := src.At(q + 1)
		if (a == 45 && (b == 45 || b == 46)) || (a == 61 && b == 61) {
			push(sLLabel)
			return emit(tStartLink, ws(q+2))
		}
		return false
	}

	lineState := func(typ int) bool {
		e := src.Index("\n", p)
		if e == -1 {
			e = n
		}
		pop()
		return emit(typ, e)
	}

	// `\w+(,\s*\w+)*`
	idList := func(typ, next int) bool {
		k := word(p)
		if k == p {
			return false
		}
		for src.At(k) == 44 {
			from := ws(k + 1)
			to := word(from)
			if to == from {
				break
			}
			k = to
		}
		state = next
		return emit(typ, k)
	}

	initial := func(c int) bool {
		if util.IsSpace(c) {
			p = ws(p + 1)
			return true
		}
		var q int
		switch c {
		case 98:
			if src.HasPrefix(p, "block") {
				if keyword("block-beta") {
					return emit(tKey, p+10)
				}
				if src.At(p+5) == 58 {
					return emit(tIDBlock, p+6)
				}
				if !util.IsWord(src.At(p + 5)) {
					return emit(tKey, p+5)
				}
			}
		case 99:
			if src.HasPrefix(p, "columns") {
				q = ws(p + 7)
				if q > p+7 {
					if src.HasPrefix(q, "auto") && !util.IsWord(src.At(q+4)) {
						return put(tColumns, "-1", q+4)
					}
					if e := digits(q); e > q {
						return put(tColumns, src.String(q, e), e)
					}
				}
			} else if src.HasPrefix(p, "class") {
				def := src.HasPrefix(p+5, "Def")
				from := p + 5
				if def {
					from = p + 8
				}
				q = ws(from)
				if q > from {
					if def {
						push(sClassDef)
						return emit(tClassDef, q)
					}
					push(sClass)
					return emit(tClass, q)
				}
			}
		case 34:
			return quote()
		case 115:
			if src.HasPrefix(p, "space") {
				if src.At(p+5) == 58 {
					if e := digits(p + 6); e > p+6 {
						return put(tSpaceBlock, src.String(p+6, e), e)
					}
				}
				if !util.IsWord(src.At(p + 5)) {
					return put(tSpaceBlock, "1", p+5)
				}
			} else if src.HasPrefix(p, "style") {
				q = ws(p + 5)
				if q > p+5 {
					push(sStyleStmnt)
					return emit(tStyle, q)
				}
			}
		case 100:
			if keyword("default") {
				return emit(tDefault, p+7)
			}
		case 108:
			if keyword("linkStyle") {
				return emit(tLinkStyle, p+9)
			}
		case 105:
			if keyword("interpolate") {
				return emit(tInterpolate, p+11)
			}
		case 97:
			if src.HasPrefix(p, "accTitle") {
				q = ws(p + 8)
				if src.At(q) == 58 {
					push(sAccTitle)
					return emit(tAccTitle, ws(q+1))
				}
			} else if src.HasPrefix(p, "accDescr") {
				q = ws(p + 8)
				d := src.At(q)
				if d == 58 {
					push(sAccDescr)
					return emit(tAccDescr, ws(q+1))
				}
				if d == 123 {
					push(sAccDescrMultiline)
					p = ws(q + 1)
					return true
				}
			}
		case 101:
			if keyword("end") {
				return emit(tEnd, ws(p+3))
			}
		case 45:
			if src.At(p+1) == 41 {
				push(sNode)
				return emit(tNodeDStart, p+2)
			}
			return link()
		case 61:
			return link()
		case 60:
			if src.At(p+1) == 91 {
				push(sBlockArrow)
				return emit(tBlockArrowStart, p+2)
			}
			return link()
		case 40:
			d := src.At(p + 1)
			size := 1
			if d == 45 || d == 91 {
				size = 2
			} else if d == 40 {
				size = 2
				if src.At(p+2) == 40 {
					size = 3
				}
			}
			push(sNode)
			return emit(tNodeDStart, p+size)
		case 41, 123:
			push(sNode)
			if src.At(p+1) == c {
				return emit(tNodeDStart, p+2)
			}
			return emit(tNodeDStart, p+1)
		case 62:
			push(sNode)
			return emit(tNodeDStart, p+1)
		case 91:
			d := src.At(p + 1)
			push(sNode)
			if d == 91 || d == 124 || d == 40 || d == 92 || d == 47 {
				return emit(tNodeDStart, p+2)
			}
			return emit(tNodeDStart, p+1)
		case 58:
			if e := digits(p + 1); e > p+1 {
				return put(tSize, src.String(p+1, e), e)
			}
			return false
		}

		q = p
		for q < n {
			ch := int(src[q])
			if ch < 128 && idStop[ch] || ch >= 128 && util.IsSpace(ch) {
				break
			}
			q++
		}
		return q > p && emit(tNodeID, q)
	}

	nodeEnd := func(c int) bool {
		d := src.At(p + 1)
		size := 0
		switch c {
		case 34:
			return quote()
		case 40:
			switch {
			case d == 45:
				size = 2
			case d == 40 && src.At(p+2) == 40:
				size = 3
			case d == 40:
				size = 2
			default:
				size = 1
			}
		case 41:
			switch {
			case d == 41 && src.At(p+2) == 41:
				size = 3
			case d == 41 || d == 93:
				size = 2
			default:
				size = 1
			}
		case 125:
			size = 1
			if d == 125 {
				size = 2
			}
		case 45:
			if d == 41 {
				size = 2
			}
		case 93:
			size = 1
			if d == 93 || d == 41 || d == 62 {
				size = 2
			}
		case 92, 47:
			if d == 93 {
				size = 2
			}
		}
		if size == 0 {
			return false
		}
		pop()
		return emit(tNodeDEnd, p+size)
	}

	arrowDir := func(c int) bool {
		if c == 41 {
			pop()
			pop()
			return put(tBlockArrowEnd, "]>", ws(p+1))
		}
		from := p
		if c == 44 {
			from = p + 1
		}
		from = ws(from)
		for _, dir := range directions {
			if src.HasPrefix(from, dir) {
				e := ws(from + len(dir))
				start := p
				if c == 44 {
					start = from
				}
				return put(tDir, src.String(start, e), e)
			}
		}
		return false
	}

	for {
		if p >= n {
			typ := -1
			switch state {
			case sInitial:
				typ = tEOF
			case sString:
				typ = tStr
			case sClassDefID:
				typ = tClassDefStyleOpts
			case sClassStyle:
				typ = tStyleClass
			case sStyleDefinition:
				typ = tStyleDefinitionData
			case sAccTitle:
				typ = tAccTitleValue
			case sAccDescr:
				typ = tAccDescrValue
			case sAccDescrMultiline:
				typ = tAccDescrMultilineValue
			}
			p = n
			if typ != -1 {
				put(typ, "", n)
			}
			break
		}

		c := int(src[p])
		ok := true

		switch state {
		case sInitial:
			ok = initial(c)

		case sString:
			if c == 34 {
				pop()
				p++
			} else {
				e := src.Index(`"`, p)
				if e == -1 {
					e = n
				}
				emit(tStr, e)
			}

		case sMDString:
			if c == 96 {
				ok = src.At(p+1) == 34
				if ok {
					pop()
					p += 2
				}
			} else if c == 34 {
				ok = false
			} else {
				q := p + 1
				for q < n {
					if ch := src[q]; ch == 96 || ch == 34 {
						break
					}
					q++
				}
				emit(tMDStr, q)
			}

		case sNode:
			ok = nodeEnd(c)

		case sBlockArrow:
			if c == 34 {
				quote()
			} else if c == 93 && src.At(p+1) == 62 {
				q := ws(p + 2)
				ok = src.At(q) == 40
				if ok {
					push(sArrowDir)
					p = q + 1
				}
			} else {
				ok = false
			}

		case sArrowDir:
			ok = arrowDir(c)

		case sLLabel:
			if c == 34 {
				if src.At(p+1) == 96 {
					quote()
				} else {
					emit(tLinkLabel, p+1)
					push(sString)
				}
			} else {
				e := linkEnd(ws(p))
				ok = e >= 0
				if ok {
					pop()
					emit(tLink, ws(e))
				}
			}

		case sClassDef:
			typ := tDefaultClassDefID
			k := p
			if src.HasPrefix(p, "DEFAULT") {
				k = p + 7
			}
			q := ws(k)
			if k == p || q == k {
				typ = tClassDefID
				k = word(p)
				q = ws(k)
			}
			ok = k > p && q > k
			if ok {
				state = sClassDefID
				emit(typ, q)
			}

		case sClassDefID:
			lineState(tClassDefStyleOpts)

		case sClass:
			ok = idList(tClassEntityIDs, sClassStyle)

		case sClassStyle:
			lineState(tStyleClass)

		case sStyleStmnt:
			ok = idList(tStyleEntityIDs, sStyleDefinition)

		case sStyleDefinition:
			lineState(tStyleDefinitionData)

		case sAccTitle:
			lineState(tAccTitleValue)

		case sAccDescr:
			lineState(tAccDescrValue)

		case sAccDescrMultiline:
			if c == 125 {
				pop()
				p++
			} else {
				e := src.Index("}", p)
				if e == -1 {
					e = n
				}
				emit(tAccDescrMultilineValue, e)
			}
		}

		if !ok {
			out.types = append(out.types, tError)
			out.texts = append(out.texts, "")
			out.starts = append(out.starts, p)
			return out
		}
	}

	out.types = append(out.types, tEndOfInput)
	out.texts = append(out.texts, "")
	out.starts = append(out.starts, n)
	return out
}
