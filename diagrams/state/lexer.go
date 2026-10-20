package state

import (
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

type tokens struct {
	types  []int
	texts  []string
	starts []int
	// Set when lexing stopped on the grammar's own error rather than on unrecognized text.
	err    string
	hasErr bool
}

// Lexer states, in the order of upstream's S.
const (
	sInitial = iota
	sStruct
	sState
	sStateString
	sStateID
	sScale
	sAccTitle
	sAccDescr
	sAccDescrMultiline
	sClassDef
	sClassDefID
	sClass
	sClassStyle
	sStyle
	sStyleDefStyles
	sNote
	sNoteID
	sNoteText
	sFloatingNote
	sFloatingNoteID
)

// finder finds the next index at or after a position where matchAt holds, remembering the
// answer so that asking at each token costs one scan of the text in total.
func finder(n int, matchAt func(i int) bool) func(q int) int {
	from := 0
	next := -2
	return func(q int) int {
		if next == -2 || q < from || (next != -1 && q > next) {
			next = -1
			for i := max(q, 0); i < n; i++ {
				if matchAt(i) {
					next = i
					break
				}
			}
			from = q
		}
		return next
	}
}

// word is a case-insensitive match of a lowercase ASCII word, as the grammar's /i flag gives.
func word(src common.Src, w string, q int) bool {
	for i := 0; i < len(w); i++ {
		a := int(w[i])
		c := src.At(q + i)
		if c != a && !(a >= 97 && a <= 122 && c == a-32) {
			return false
		}
	}
	return true
}

func tokenize(src common.Src) *tokens {
	n := len(src)
	tk := &tokens{}
	var stack []int
	state := sInitial
	p := 0

	// /direction/gi
	nextDirection := finder(n, func(i int) bool { return word(src, "direction", i) })
	// /<<(?:fork|join|choice)>>|\[\[(?:fork|join|choice)\]\]/gi
	nextStereotype := finder(n, func(i int) bool {
		open, close := "<<", ">>"
		if src[i] == '[' {
			open, close = "[[", "]]"
		}
		if !src.HasPrefix(i, open) {
			return false
		}
		for _, w := range [...]string{"fork", "join", "choice"} {
			if word(src, w, i+2) && src.HasPrefix(i+2+len(w), close) {
				return true
			}
		}
		return false
	})
	// /[\n\r\u2028\u2029]/g
	nextLineEnd := finder(n, func(i int) bool { return util.IsLineEnd(int(src[i])) })
	nextPercent := finder(n, func(i int) bool { return src.HasPrefix(i, "%%") })
	nextBrace := finder(n, func(i int) bool { return src[i] == '{' })

	// The last `direction <dir>` of each kind on the line being lexed, and where its keyword ends.
	dirFrom, dirTo := -1, -1
	dirAt := [4]int{-1, -1, -1, -1}
	var dirEnd [4]int
	// The same for <<fork>>, <<join>>, <<choice>> and their [[ ]] spellings.
	stereoFrom, stereoTo := -1, -1
	stereoAt := [6]int{-1, -1, -1, -1, -1, -1}

	// slice is src.slice(s, e), which is empty when s is past e.
	slice := func(s, e int) string {
		e = min(e, n)
		if s >= e {
			return ""
		}
		return src.String(s, e)
	}
	emit := func(typ, s, e int) {
		tk.types = append(tk.types, typ)
		tk.texts = append(tk.texts, slice(s, e))
		tk.starts = append(tk.starts, s)
		p = e
	}
	emitText := func(typ int, text string, s, e int) {
		tk.types = append(tk.types, typ)
		tk.texts = append(tk.texts, text)
		tk.starts = append(tk.starts, s)
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
	wsEnd := func(q int) int {
		j := q
		for j < n && util.IsSpace(src.At(j)) {
			j++
		}
		return j
	}
	wordEnd := func(q int) int {
		j := q
		for j < n && util.IsWord(src.At(j)) {
			j++
		}
		return j
	}
	eol := func(q int) int {
		if k := src.Index("\n", q); k != -1 {
			return k
		}
		return n
	}
	lineEnd := func(q int) int {
		if k := nextLineEnd(q); k != -1 {
			return k
		}
		return n
	}
	// Rules `.*direction\s+TB[^\n]*` and its three siblings: the first kind, in grammar order,
	// found between q and the end of its line.
	direction := func(q int) int {
		if q < dirFrom || q > dirTo {
			first := nextDirection(q)
			end := n
			if first != -1 {
				end = lineEnd(q)
			}
			dirFrom = q
			dirTo = end
			dirAt = [4]int{-1, -1, -1, -1}
			for d := first; d != -1 && d < end; d = nextDirection(d + 1) {
				w := wsEnd(d + 9)
				if w == d+9 {
					continue
				}
				kind := -1
				switch {
				case word(src, "tb", w):
					kind = 0
				case word(src, "bt", w):
					kind = 1
				case word(src, "rl", w):
					kind = 2
				case word(src, "lr", w):
					kind = 3
				}
				if kind == -1 {
					continue
				}
				dirAt[kind] = d
				dirEnd[kind] = w + 2
			}
		}
		for kind := 0; kind < 4; kind++ {
			if dirAt[kind] >= q {
				return kind
			}
		}
		return -1
	}

	stereotype := func(q int) int {
		if q < stereoFrom || q > stereoTo {
			first := nextStereotype(q)
			end := n
			if first != -1 {
				end = lineEnd(q)
			}
			stereoFrom = q
			stereoTo = end
			stereoAt = [6]int{-1, -1, -1, -1, -1, -1}
			for d := first; d != -1 && d < end; d = nextStereotype(d + 1) {
				letter := src.At(d+2) | 32
				k := 3
				if src.At(d) == 60 {
					k = 0
				}
				switch letter {
				case 102:
				case 106:
					k++
				default:
					k += 2
				}
				stereoAt[k] = d
			}
		}
		for kind := 0; kind < 6; kind++ {
			if stereoAt[kind] >= q {
				return kind
			}
		}
		return -1
	}

	// The grammar's processId(): an id ends at an inline `%%`, and a text that starts with one is dropped.
	emitID := func(s, e int) bool {
		at := nextPercent(s)
		if at == -1 || at+2 > e {
			emit(tID, s, e)
			return true
		}
		if at == s {
			p = e
			return false
		}
		emit(tID, s, at)
		return true
	}

	finish := func(typ int) *tokens {
		tk.types = append(tk.types, typ)
		tk.texts = append(tk.texts, "")
		tk.starts = append(tk.starts, p)
		return tk
	}
	fail := func(msg string) *tokens {
		tk.err = msg
		tk.hasErr = true
		return finish(tError)
	}

	for {
		if p >= n {
			// Jison tries the current state's rules once more on the empty remainder.
			switch state {
			case sInitial:
				emit(tNL, n, n)
			case sAccTitle:
				emit(tAccTitleValue, n, n)
			case sAccDescr:
				emit(tAccDescrValue, n, n)
			case sAccDescrMultiline:
				emit(tAccDescrMultilineValue, n, n)
			case sClassDefID:
				emit(tClassDefStyleOpts, n, n)
			case sClassStyle:
				emit(tStyleClass, n, n)
			case sStyleDefStyles:
				emit(tStyleDefStyleOpts, n, n)
			case sStateString:
				emit(tStateDescr, n, n)
			case sFloatingNote:
				emit(tNoteText, n, n)
			case sStateID, sFloatingNoteID:
				emit(tID, n, n)
			}
			return finish(tEnd)
		}

		c := int(src[p])
		switch state {
		case sInitial, sStruct:
			root := state == sInitial
			lc := c | 32
			if root {
				if lc == 99 && word(src, "click", p) && !util.IsWord(src.At(p+5)) {
					emit(tClick, p, p+5)
					continue
				}
				if lc == 104 && word(src, "href", p) && !util.IsWord(src.At(p+4)) {
					emit(tHref, p, p+4)
					continue
				}
				if c == 34 {
					if end := src.Index(`"`, p+1); end != -1 {
						emit(tString, p, end+1)
						continue
					}
				}
				if lc == 100 && word(src, "default", p) && !util.IsWord(src.At(p+7)) {
					emit(tDefault, p, p+7)
					continue
				}
				if kind := direction(p); kind != -1 {
					emit(tDirectionTB+kind, p, eol(dirEnd[kind]))
					continue
				}
				if c == 10 {
					j := p + 1
					for src.At(j) == 10 {
						j++
					}
					emit(tNL, p, j)
					continue
				}
				if util.IsSpace(c) {
					p = wsEnd(p)
					continue
				}
			} else if c != 10 && util.IsSpace(c) {
				j := p + 1
				for j < n && src[j] != 10 && util.IsSpace(int(src[j])) {
					j++
				}
				p = j
				continue
			}
			if c == 35 || (c == 37 && src.At(p+1) == 37 && src.At(p+2) != 123) {
				p = eol(p)
				continue
			}
			if lc == 115 {
				if root && word(src, "scale", p) {
					if e := wsEnd(p + 5); e > p+5 {
						emit(tScale, p, e)
						push(sScale)
						continue
					}
				}
				if word(src, "style", p) {
					if e := wsEnd(p + 5); e > p+5 {
						emit(tStyle, p, e)
						push(sStyle)
						continue
					}
				}
				if word(src, "state", p) {
					e := wsEnd(p + 5)
					if e > p+5 {
						p = e
						push(sState)
						continue
					}
					if root && word(src, "diagram", p+5) {
						from := p + 12
						if word(src, "-v2", p+12) {
							from = p + 15
						}
						if e = wsEnd(from); e > from {
							emit(tSD, p, e)
							continue
						}
					}
				}
			} else if lc == 99 {
				if word(src, "class", p) {
					def := word(src, "def", p+5)
					from := p + 5
					if def {
						from = p + 8
					}
					if e := wsEnd(from); e > from {
						if def {
							emit(tClassDef, p, e)
							push(sClassDef)
						} else {
							emit(tClass, p, e)
							push(sClass)
						}
						continue
					}
				}
			} else if lc == 97 && root {
				if word(src, "acctitle", p) || word(src, "accdescr", p) {
					title := src.At(p+3)|32 == 116
					q := wsEnd(p + 8)
					d := src.At(q)
					if d == 58 {
						if title {
							emit(tAccTitle, p, wsEnd(q+1))
							push(sAccTitle)
						} else {
							emit(tAccDescr, p, wsEnd(q+1))
							push(sAccDescr)
						}
						continue
					}
					if d == 123 && !title {
						p = wsEnd(q + 1)
						push(sAccDescrMultiline)
						continue
					}
				}
			}
			if root {
				if c == 123 {
					emit(tStructStart, p, p+1)
					push(sStruct)
					continue
				}
			} else {
				if kind := direction(p); kind != -1 {
					emit(tDirectionTB+kind, p, eol(dirEnd[kind]))
					continue
				}
				if c == 125 {
					emit(tStructStop, p, p+1)
					pop()
					continue
				}
				if c == 10 {
					p++
					continue
				}
			}
			if lc == 110 && word(src, "note", p) {
				if e := wsEnd(p + 4); e > p+4 {
					emit(tNote, p, e)
					push(sNote)
					continue
				}
			}
			if root && lc == 104 && word(src, "hide empty description", p) && !util.IsWord(src.At(p+22)) {
				emit(tHideEmpty, p, p+22)
				continue
			}
			if c == 91 && src.At(p+1) == 42 && src.At(p+2) == 93 {
				emit(tEdgeState, p, p+3)
				continue
			}
			j := p
			for j < n {
				d := int(src[j])
				if d == 58 || d == 45 || d == 123 || util.IsSpace(d) {
					break
				}
				j++
			}
			if j > p {
				emitID(p, j)
				continue
			}
			if c == 58 {
				j = p + 1
				for j < n {
					d := int(src[j])
					if d == 10 || d == 59 {
						break
					}
					if d == 58 {
						after := src.At(j + 1)
						if j+1 >= n || after == 58 || after == 10 || after == 59 {
							break
						}
						j++
					}
					j++
				}
				if j > p+1 {
					emitText(tDescr, jsstr.Trim(slice(p, j)), p, j)
					continue
				}
				if src.At(p+1) == 58 && src.At(p+2) == 58 {
					emit(tStyleSeparator, p, p+3)
					continue
				}
			} else if c == 45 && src.At(p+1) == 45 {
				if src.At(p+2) == 62 {
					emit(tArrow, p, p+3)
					continue
				}
				if !root {
					emit(tConcurrent, p, p+2)
					continue
				}
			}
			if !root {
				return finish(tError)
			}
			emit(tInvalid, p, p+1)

		case sState:
			if c != 10 && util.IsSpace(c) {
				j := p + 1
				for j < n && src[j] != 10 && util.IsSpace(int(src[j])) {
					j++
				}
				p = j
				continue
			}
			if c == 35 || (c == 37 && src.At(p+1) == 37 && src.At(p+2) != 123) {
				p = eol(p)
				continue
			}
			if kind := stereotype(p); kind != -1 {
				at := stereoAt[kind]
				choice := kind == 2 || kind == 5
				typ, end := tJoin, at+8
				switch {
				case choice:
					typ, end = tChoice, at+10
				case kind%3 == 0:
					typ = tFork
				}
				emitText(typ, jsstr.Trim(slice(p, at)), p, end)
				pop()
				continue
			}
			if c == 34 {
				p++
				push(sStateString)
				continue
			}
			q := p
			if c == 10 {
				q = wsEnd(p)
			}
			if word(src, "as", q) {
				if e := wsEnd(q + 2); e > q+2 {
					emit(tAs, p, e)
					push(sStateID)
					continue
				}
			}
			if c == 10 {
				p++
				pop()
				continue
			}
			if c == 123 {
				emit(tStructStart, p, p+1)
				pop()
				push(sStruct)
				continue
			}
			if util.IsWord(c) {
				first := wordEnd(p)
				second := wsEnd(first)
				if second > first && util.IsWord(src.At(second)) {
					if brace := nextBrace(second + 1); brace != -1 && brace < lineEnd(second) {
						return fail(`Error: State name must be a single word. Found: "` + jsstr.Trim(slice(p, brace+1)) + `"`)
					}
				}
			}
			j := p + 1
			for j < n {
				d := int(src[j])
				if d == 123 || util.IsSpace(d) {
					break
				}
				j++
			}
			emit(tCompositState, p, j)

		case sStateString:
			if c == 34 {
				p++
				pop()
				continue
			}
			end := src.Index(`"`, p)
			if end == -1 {
				end = n
			}
			emit(tStateDescr, p, end)

		case sStateID:
			j := p
			for j < n {
				d := src[j]
				if d == 10 || d == 123 {
					break
				}
				j++
			}
			if emitID(p, j) {
				pop()
			}

		case sScale:
			if c >= 48 && c <= 57 {
				j := p + 1
				for j < n && src[j] >= 48 && src[j] <= 57 {
					j++
				}
				emit(tWidth, p, j)
				continue
			}
			q := wsEnd(p)
			if q > p && word(src, "width", q) && !util.IsWord(src.At(q+5)) {
				p = q + 5
				pop()
				continue
			}
			return finish(tError)

		case sAccTitle:
			emit(tAccTitleValue, p, eol(p))
			pop()

		case sAccDescr:
			emit(tAccDescrValue, p, eol(p))
			pop()

		case sAccDescrMultiline:
			if c == 125 {
				p++
				pop()
				continue
			}
			end := src.Index("}", p)
			if end == -1 {
				end = n
			}
			emit(tAccDescrMultilineValue, p, end)

		case sClassDef:
			isDefault := word(src, "default", p) && wsEnd(p+7) > p+7
			j := p + 7
			if !isDefault {
				j = wordEnd(p)
			}
			e := wsEnd(j)
			if j == p || e == j {
				return finish(tError)
			}
			if isDefault {
				emit(tDefaultClassDefID, p, e)
			} else {
				emit(tClassDefID, p, e)
			}
			pop()
			push(sClassDefID)

		case sClassDefID:
			emit(tClassDefStyleOpts, p, eol(p))
			pop()

		case sClass:
			j := wordEnd(p)
			if j == p {
				return finish(tError)
			}
			for src.At(j) == 44 {
				k := wsEnd(j + 1)
				e := wordEnd(k)
				if e == k {
					break
				}
				j = e
			}
			emit(tClassEntityIDs, p, j)
			pop()
			push(sClassStyle)

		case sClassStyle:
			emit(tStyleClass, p, eol(p))
			pop()

		case sStyle:
			j := p
			for j < n && (util.IsWord(int(src[j])) || src[j] == 44) {
				j++
			}
			e := wsEnd(j)
			if j == p || e == j {
				return finish(tError)
			}
			emit(tStyleIDs, p, e)
			pop()
			push(sStyleDefStyles)

		case sStyleDefStyles:
			emit(tStyleDefStyleOpts, p, eol(p))
			pop()

		case sNote:
			if word(src, "left of", p) && !util.IsWord(src.At(p+7)) {
				emit(tLeftOf, p, p+7)
				pop()
				push(sNoteID)
				continue
			}
			if word(src, "right of", p) && !util.IsWord(src.At(p+8)) {
				emit(tRightOf, p, p+8)
				pop()
				push(sNoteID)
				continue
			}
			if c == 34 {
				p++
				pop()
				push(sFloatingNote)
				continue
			}
			return finish(tError)

		case sNoteID:
			q := wsEnd(p)
			j := q
			for j < n {
				d := int(src[j])
				if d == 58 || d == 45 || util.IsSpace(d) {
					break
				}
				j++
			}
			if j == q {
				return finish(tError)
			}
			if emitID(p, j) {
				pop()
				push(sNoteText)
			}

		case sNoteText:
			q := wsEnd(p)
			if src.At(q) == 58 {
				j := q + 1
				for j < n {
					d := src[j]
					if d == 58 || d == 10 || d == 59 {
						break
					}
					j++
				}
				if j > q+1 {
					// Mermaid drops two characters here, expecting " :"; with no space before the colon
					// the first character of the note goes too.
					emitText(tNoteText, jsstr.Trim(slice(p+2, j)), p, j)
					pop()
					continue
				}
			}
			k := src.Index("\n", p)
			e := -1
			for k != -1 {
				e = wsEnd(k)
				if word(src, "end note", e) && !util.IsWord(src.At(e+8)) {
					break
				}
				k = src.Index("\n", e)
			}
			if k == -1 {
				return finish(tError)
			}
			emitText(tNoteText, jsstr.Trim(slice(p, e)), p, e+8)
			pop()

		case sFloatingNote:
			q := wsEnd(p)
			if word(src, "as", q) {
				emit(tAs, p, wsEnd(q+2))
				pop()
				push(sFloatingNoteID)
				continue
			}
			if c == 34 {
				p++
				continue
			}
			end := src.Index(`"`, p)
			if end == -1 {
				end = n
			}
			emit(tNoteText, p, end)

		case sFloatingNoteID:
			if emitID(p, eol(p)) {
				pop()
			}
		}
	}
}
