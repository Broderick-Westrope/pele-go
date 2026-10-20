package agentflow

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

// tokens are the lexer's output in parallel: types, texts, start offsets, and four numbers
// per token counted the way Jison counts them: first line, first column, last line, last
// column. Lines start at 1, columns at 0.
type tokens struct {
	types  []int
	texts  []string
	starts []int
	locs   []int
}

// Lexer states.
const (
	sInitial = iota
	sString
	sMdString
	sAccTitle
	sAccDescr
	sAccDescrMultiline
	sDir
	sText
	sEllipseText
	sTrapText
	sEdgeText
	sClick
	sCallbackname
	sCallbackargs
	sShapeData
	sShapeDataStr
)

var nodeChar = func() (t [128]bool) {
	for _, ch := range "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!\"#$%&'*+.`?\\_/" {
		t[ch] = true
	}
	return t
}()

type lexer struct {
	src   common.Src
	n     int
	out   tokens
	stack []int
	state int
	p     int
	line  int
	col   int

	firstGraph bool
	dirNext    int
	atNext     int
	lineEnd    int
	dirFailed  int
	runEnd     int
	runAt      int
	wsFrom     int
	wsTo       int
	blankFrom  int
	blankTo    int
	noLink     int
	noArrow    int
}

// The sticky regexes upstream matches with, as matchers that return the end of the match at
// q, or -1.

func (l *lexer) skipSpace(q int) int {
	for util.IsSpace(l.src.At(q)) {
		q++
	}
	return q
}

// accHead matches /<word>\s*<c>\s*/y: accTitle\s*:\s*, accDescr\s*:\s* and accDescr\s*\{\s*.
func (l *lexer) accHead(q int, w string, c int) int {
	if !l.src.HasPrefix(q, w) {
		return -1
	}
	q = l.skipSpace(q + len(w))
	if l.src.At(q) != c {
		return -1
	}
	return l.skipSpace(q + 1)
}

// spaceAfter matches /<word>\s+/y: call\s+ and click\s+.
func (l *lexer) spaceAfter(q int, w string) int {
	if !l.src.HasPrefix(q, w) || !util.IsSpace(l.src.At(q+len(w))) {
		return -1
	}
	return l.skipSpace(q + len(w))
}

// noDir matches /(?:\r?\n)*\s*\n/y. Whatever the group takes, \s* backs off to the last line
// feed of the run of whitespace at q.
func (l *lexer) noDir(q int) int {
	end := -1
	for j := q; util.IsSpace(l.src.At(j)); j++ {
		if l.src.At(j) == '\n' {
			end = j + 1
		}
	}
	return end
}

// dir matches /\s*(?:LR|RL|TB|BT|TD|BR)\b/y.
func (l *lexer) dir(q int) int {
	q = l.skipSpace(q)
	a, b := l.src.At(q), l.src.At(q+1)
	switch {
	case a == 'L' && b == 'R', a == 'R' && b == 'L', a == 'T' && b == 'B', a == 'B' && b == 'T', a == 'T' && b == 'D', a == 'B' && b == 'R':
	default:
		return -1
	}
	if util.IsWord(l.src.At(q + 2)) {
		return -1
	}
	return q + 2
}

// dirSym matches /\s*[<>^]/y.
func (l *lexer) dirSym(q int) int {
	q = l.skipSpace(q)
	if c := l.src.At(q); c == '<' || c == '>' || c == '^' {
		return q + 1
	}
	return -1
}

// dirV matches /\s*v\b/y.
func (l *lexer) dirV(q int) int {
	q = l.skipSpace(q)
	if l.src.At(q) != 'v' || util.IsWord(l.src.At(q+1)) {
		return -1
	}
	return q + 1
}

var directionWords = [5]string{"TB", "BT", "RL", "LR", "TD"}

// direction matches /.*direction\s+<dir>[^\n]*/y. .* backs off from the end of the line to
// the last place the rest matches; \s+ may then run across lines, and [^\n]* stops only at
// a line feed.
func (l *lexer) direction(q, i int) int {
	lineEnd := q
	for lineEnd < l.n && !util.IsLineEnd(l.src.At(lineEnd)) {
		lineEnd++
	}
	for k := lineEnd; k >= q; k-- {
		if !l.src.HasPrefix(k, "direction") {
			continue
		}
		j := k + len("direction")
		if !util.IsSpace(l.src.At(j)) {
			continue
		}
		j = l.skipSpace(j)
		if !l.src.HasPrefix(j, directionWords[i]) {
			continue
		}
		j += 2
		for j < l.n && l.src[j] != '\n' {
			j++
		}
		return j
	}
	return -1
}

// emptyCall matches /\(\s*\)/y.
func (l *lexer) emptyCall(q int) int {
	if l.src.At(q) != '(' {
		return -1
	}
	q = l.skipSpace(q + 1)
	if l.src.At(q) != ')' {
		return -1
	}
	return q + 1
}

// breaks is s.replace(/\n\s*/g, '<br/>').
func breaks(s string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '\n' {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteString("<br/>")
		i++
		for i < len(s) {
			r, size := jsstr.Decode(s, i)
			if !jsstr.IsSpace(r) {
				break
			}
			i += size
		}
	}
	return b.String()
}

// advance moves over one match. Jison counts \r\n, \r and \n as one line break each, and
// after a break its column stops at the first U+2028 or U+2029.
func (l *lexer) advance(e int) {
	brk := -1
	for k := l.p; k < e; k++ {
		switch l.src[k] {
		case '\n':
			l.line++
			brk = k
		case '\r':
			if k+1 < e && l.src[k+1] == '\n' {
				k++
			}
			l.line++
			brk = k
		}
	}
	if brk < 0 {
		l.col += e - l.p
	} else {
		k := brk + 1
		for k < e && l.src[k] != 0x2028 && l.src[k] != 0x2029 {
			k++
		}
		l.col = k - brk - 1
	}
	l.p = e
}

func (l *lexer) begin(typ int, text string) {
	l.out.types = append(l.out.types, typ)
	l.out.texts = append(l.out.texts, text)
	l.out.starts = append(l.out.starts, l.p)
	l.out.locs = append(l.out.locs, l.line, l.col)
}

func (l *lexer) finish() {
	l.out.locs = append(l.out.locs, l.line, l.col)
}

func (l *lexer) emit(typ, e int) {
	l.begin(typ, l.src.String(l.p, e))
	l.advance(e)
	l.finish()
}

// emitFlat emits a token that cannot hold a line break.
func (l *lexer) emitFlat(typ, e int) {
	l.begin(typ, l.src.String(l.p, e))
	l.col += e - l.p
	l.p = e
	l.finish()
}

// emitChars emits a run that Jison matches one character at a time, where \r\n counts as two
// line breaks.
func (l *lexer) emitChars(typ, e int) {
	l.begin(typ, l.src.String(l.p, e))
	for k := l.p; k < e; k++ {
		if c := l.src[k]; c == '\n' || c == '\r' {
			l.line++
			l.col = 0
		} else {
			l.col++
		}
	}
	l.p = e
	l.finish()
}

func (l *lexer) emitText(typ int, text string, e int) {
	l.begin(typ, text)
	l.advance(e)
	l.finish()
}

func (l *lexer) push(s int) {
	l.stack = append(l.stack, l.state)
	l.state = s
}

func (l *lexer) pop() {
	if n := len(l.stack); n > 0 {
		l.state = l.stack[n-1]
		l.stack = l.stack[:n-1]
	}
}

func (l *lexer) kw(w string) bool {
	return l.src.HasPrefix(l.p, w) && !util.IsWord(l.src.At(l.p+len(w)))
}

// wsEnd is the end of the run of whitespace starting at q. The last run is kept, so asking
// again from inside it costs nothing.
func (l *lexer) wsEnd(q int) int {
	if q >= l.wsFrom && q <= l.wsTo {
		return l.wsTo
	}
	j := q
	for j < l.n && util.IsSpace(l.src.At(j)) {
		j++
	}
	l.wsFrom = q
	l.wsTo = j
	return j
}

func (l *lexer) blankEnd(q int) int {
	if q >= l.blankFrom && q <= l.blankTo {
		return l.blankTo
	}
	j := l.blanks(q)
	l.blankFrom = q
	l.blankTo = j
	return j
}

// arrowEnd is the end of `\s*--+[x>]\s*` starting at q, or -1.
func (l *lexer) arrowEnd(q int) int {
	j := l.wsEnd(q)
	if j == l.noArrow || l.src.At(j) != '-' || l.src.At(j+1) != '-' {
		return -1
	}
	k := j + 2
	for l.src.At(k) == '-' {
		k++
	}
	if t := l.src.At(k); t == 'x' || t == '>' {
		return l.wsEnd(k + 1)
	}
	l.noArrow = j
	return -1
}

func (l *lexer) link() bool {
	j := l.wsEnd(l.p)
	if j == l.noLink || l.src.At(j) != '-' {
		return false
	}
	d := l.src.At(j + 1)
	k := j + 2
	if d == '-' {
		for l.src.At(k) == '-' {
			k++
		}
		if t := l.src.At(k); t == 'x' || t == '>' {
			l.emit(tLink, l.wsEnd(k+1))
		} else {
			l.push(sEdgeText)
			l.emit(tStartLink, l.wsEnd(j+2))
		}
		return true
	}
	if d == '.' {
		for l.src.At(k) == '.' {
			k++
		}
		if l.src.At(k) == '-' {
			l.emit(tLink, l.wsEnd(k+1))
			return true
		}
	}
	l.noLink = j
	return false
}

// commentEnd is the end of `%%` not followed by `{`, through the end of the line, or -1.
func (l *lexer) commentEnd(q int) int {
	if l.src.At(q) != '%' || l.src.At(q+1) != '%' || l.src.At(q+2) == '{' {
		return -1
	}
	return l.indexFrom('\n', q+2)
}

func (l *lexer) quote() {
	if l.src.At(l.p+1) == '`' {
		l.push(sMdString)
		l.advance(l.p + 2)
	} else {
		l.push(sString)
		l.advance(l.p + 1)
	}
}

func (l *lexer) opener(c int) bool {
	p := l.p
	d := l.src.At(p + 1)
	switch c {
	case '(':
		switch {
		case d == '-':
			l.push(sEllipseText)
			l.emitFlat(tEllipseStart, p+2)
		case d == '[':
			l.push(sText)
			l.emitFlat(tStadiumstart, p+2)
		case d == '(' && l.src.At(p+2) == '(':
			l.push(sText)
			l.emitFlat(tDoublecirclestart, p+3)
		default:
			l.push(sText)
			l.emitFlat(tPs, p+1)
		}
		return true
	case '[':
		switch d {
		case '[':
			l.push(sText)
			l.emitFlat(tSubroutinestart, p+2)
		case '(':
			l.push(sText)
			l.emitFlat(tCylinderstart, p+2)
		case '/':
			l.push(sTrapText)
			l.emitFlat(tTrapstart, p+2)
		case '\\':
			l.push(sTrapText)
			l.emitFlat(tInvtrapstart, p+2)
		default:
			l.push(sText)
			l.emitFlat(tSqs, p+1)
		}
		return true
	case '|':
		l.push(sText)
		l.emitFlat(tPipe, p+1)
		return true
	case '{':
		l.push(sText)
		l.emitFlat(tDiamondStart, p+1)
		return true
	}
	return false
}

func (l *lexer) nodeString() bool {
	q := l.p
	for q < l.n {
		ch := l.src.At(q)
		if ch < 128 && nodeChar[ch] {
			q++
		} else if ch == '-' && q+1 < l.n {
			if d := l.src.At(q + 1); d == '>' || d == '-' || d == '.' {
				break
			}
			q++
		} else {
			break
		}
	}
	if q == l.p {
		return false
	}
	l.emitFlat(tNodeString, q)
	return true
}

func (l *lexer) blanks(from int) int {
	e := from
	for e < l.n {
		if c := l.src[e]; c != ' ' && c != '\t' {
			break
		}
		e++
	}
	return e
}

func (l *lexer) initial(c int) bool {
	p := l.p
	switch c {
	case 'a':
		if l.src.HasPrefix(p, "acc") {
			if e := l.accHead(p, "accTitle", ':'); e >= 0 {
				l.push(sAccTitle)
				l.emit(tAccTitle, e)
				return true
			}
			if e := l.accHead(p, "accDescr", ':'); e >= 0 {
				l.push(sAccDescr)
				l.emit(tAccDescr, e)
				return true
			}
			if e := l.accHead(p, "accDescr", '{'); e >= 0 {
				l.push(sAccDescrMultiline)
				l.advance(e)
				return true
			}
		}
		if l.kw("agentflow-beta") {
			if l.firstGraph {
				l.firstGraph = false
				l.push(sDir)
			}
			l.emitFlat(tGraph, p+14)
			return true
		}
	case '@':
		if l.src.At(p+1) == '{' {
			l.push(sShapeData)
			l.emitText(tShapeData, "", p+2)
			return true
		}
	case 'c':
		if e := l.spaceAfter(p, "call"); e >= 0 {
			l.push(sCallbackname)
			l.advance(e)
			return true
		}
		if l.kw("classDef") {
			l.emitFlat(tClassdef, p+8)
			return true
		}
		if l.kw("class") {
			l.emitFlat(tClass, p+5)
			return true
		}
		if e := l.spaceAfter(p, "click"); e >= 0 {
			l.push(sClick)
			l.advance(e)
			return true
		}
		if l.kw("connector") {
			l.emitFlat(tConnector, p+9)
			return true
		}
	case '"':
		l.quote()
		return true
	case 's':
		if l.kw("style") {
			l.emitFlat(tStyle, p+5)
			return true
		}
	case 'd':
		if l.kw("default") {
			l.emitFlat(tDefault, p+7)
			return true
		}
	case 'l':
		if l.kw("linkStyle") {
			l.emitFlat(tLinkstyle, p+9)
			return true
		}
	case 'i':
		if l.kw("interpolate") {
			l.emitFlat(tInterpolate, p+11)
			return true
		}
	case 'h':
		if l.src.HasPrefix(p, "href") && util.IsSpace(l.src.At(p+4)) {
			l.emit(tHref, p+5)
			return true
		}
	case 'f':
		if l.kw("flow") {
			l.emitFlat(tFlow, p+4)
			return true
		}
	case 'g':
		if l.kw("global") {
			l.emitFlat(tGlobal, l.blanks(p+6))
			return true
		}
	case 'e':
		if l.kw("end") {
			l.emitFlat(tEnd, l.blanks(p+3))
			return true
		}
	case '_':
		for _, target := range [...]string{"_self", "_blank", "_parent", "_top"} {
			if l.kw(target) {
				l.emitFlat(tLinkTarget, p+len(target))
				return true
			}
		}
	}

	if l.dirNext != -1 && p >= l.dirFailed {
		if l.dirNext < p {
			l.dirNext = l.src.Index("direction", p)
		}
		if l.dirNext != -1 {
			if l.lineEnd < p {
				l.lineEnd = p
				for l.lineEnd < l.n && !util.IsLineEnd(l.src.At(l.lineEnd)) {
					l.lineEnd++
				}
			}
			if l.dirNext < l.lineEnd {
				for i := range 5 {
					if e := l.direction(p, i); e >= 0 {
						l.emit(tDirectionTB+i, e)
						return true
					}
				}
				// No direction statement starts anywhere in the rest of this line either.
				l.dirFailed = l.lineEnd
			}
		}
	}

	// A link id is a run of non-space characters up to its last `@` that is not followed by `{` or `"`.
	if l.atNext != -1 && c != '"' && !util.IsSpace(c) {
		if p >= l.runEnd {
			if l.atNext < p {
				l.atNext = l.src.Index("@", p)
			}
			q := p
			for q < l.n {
				if ch := l.src.At(q); ch == '"' || util.IsSpace(ch) {
					break
				}
				q++
			}
			l.runEnd = q
			l.runAt = -1
			if l.atNext != -1 && l.atNext < q {
				for k := min(q, l.n-1) - 1; k >= l.atNext; k-- {
					if l.src.At(k) != '@' {
						continue
					}
					if after := l.src.At(k + 1); after != '{' && after != '"' {
						l.runAt = k
						break
					}
				}
			}
		}
		if l.runAt > p {
			l.emitFlat(tLinkID, l.runAt+1)
			return true
		}
	}

	if c >= '0' && c <= '9' {
		q := p + 1
		for q < l.n {
			if ch := l.src.At(q); ch < '0' || ch > '9' {
				break
			}
			q++
		}
		l.emitFlat(tNum, q)
		return true
	}

	switch c {
	case '#':
		l.emitFlat(tBrkt, p+1)
		return true
	case ':':
		if l.src.At(p+1) == ':' && l.src.At(p+2) == ':' {
			l.emitFlat(tStyleSeparator, p+3)
		} else {
			l.emitFlat(tColon, p+1)
		}
		return true
	case '&':
		l.emitFlat(tAmp, p+1)
		return true
	case ';':
		l.emitFlat(tSemi, p+1)
		return true
	case ',':
		l.emitFlat(tComma, p+1)
		return true
	case '*':
		l.emitFlat(tMult, p+1)
		return true
	case '<':
		l.emitFlat(tTagstart, p+1)
		return true
	case '-':
		if l.link() || l.nodeString() {
			return true
		}
		l.emitFlat(tMinus, p+1)
		return true
	case '(':
		return l.opener(c)
	case '[':
		if l.src.At(p+1) == '|' {
			l.emitFlat(tVertexWithPropsStart, p+2)
			return true
		}
		return l.opener(c)
	case '|', '{':
		return l.opener(c)
	case '>':
		l.push(sText)
		l.emitFlat(tTagend, p+1)
		return true
	case '^':
		l.emitFlat(tUp, p+1)
		return true
	case '\\':
		if l.src.At(p+1) == '|' {
			l.emitFlat(tSep, p+2)
			return true
		}
		return l.nodeString()
	case 'v':
		if !util.IsWord(l.src.At(p + 1)) {
			l.emitFlat(tDown, p+1)
			return true
		}
		return l.nodeString()
	case '%':
		if e := l.commentEnd(p); e >= 0 {
			l.emit(tComment, e)
			return true
		}
		return l.nodeString()
	}

	if util.IsSpace(c) {
		if l.link() {
			return true
		}
		if c == ' ' || c == '\t' {
			if e := l.commentEnd(l.blankEnd(p)); e >= 0 {
				l.emit(tComment, e)
				return true
			}
		}
		e := p
		for {
			if l.src.At(e) == '\n' {
				e++
			} else if l.src.At(e) == '\r' && l.src.At(e+1) == '\n' {
				e += 2
			} else {
				break
			}
		}
		if e > p {
			l.emit(tNewline, e)
		} else {
			l.emit(tSpace, p+1)
		}
		return true
	}

	if c < 128 {
		return l.nodeString()
	}
	if util.IsUnicodeLetter(c) {
		l.emitFlat(tUnicodeText, p+1)
		return true
	}
	return false
}

// indexFrom is src.indexOf(c, p), or the end of src when c is not there.
func (l *lexer) indexFrom(c uint16, p int) int {
	for i := p; i < l.n; i++ {
		if l.src[i] == c {
			return i
		}
	}
	return l.n
}

func tokenize(src common.Src) tokens {
	l := &lexer{
		src:        src,
		n:          len(src),
		line:       1,
		firstGraph: true,
		dirNext:    src.Index("direction", 0),
		atNext:     src.Index("@", 0),
		lineEnd:    -1,
		runAt:      -1,
		wsFrom:     -1,
		wsTo:       -1,
		blankFrom:  -1,
		blankTo:    -1,
		noLink:     -1,
		noArrow:    -1,
	}
	n := l.n
	for {
		if l.p >= n {
			typ := -1
			switch l.state {
			case sInitial:
				typ = tEOF
			case sAccTitle:
				typ = tAccTitleValue
			case sAccDescr:
				typ = tAccDescrValue
			case sAccDescrMultiline:
				typ = tAccDescrMultilineValue
			case sClick:
				typ = tClick
			case sCallbackname:
				typ = tCallbackname
			case sCallbackargs:
				typ = tCallbackargs
			}
			if typ != -1 {
				l.emitFlat(typ, n)
			}
			break
		}

		p := l.p
		c := l.src.At(p)
		ok := true

		switch l.state {
		case sInitial:
			ok = l.initial(c)

		case sString:
			if c == '"' {
				if l.src.At(p+1) == '`' {
					l.push(sMdString)
					l.advance(p + 2)
				} else {
					l.pop()
					l.advance(p + 1)
				}
			} else {
				l.emit(tStr, l.indexFrom('"', p))
			}

		case sMdString:
			switch c {
			case '`':
				if l.src.At(p+1) == '"' {
					l.pop()
					l.advance(p + 2)
				} else {
					ok = false
				}
			case '"':
				l.quote()
			default:
				q := p + 1
				for q < n {
					if ch := l.src.At(q); ch == '`' || ch == '"' {
						break
					}
					q++
				}
				l.emit(tMdStr, q)
			}

		case sAccTitle, sAccDescr:
			e := l.indexFrom('\n', p)
			typ := tAccDescrValue
			if l.state == sAccTitle {
				typ = tAccTitleValue
			}
			l.pop()
			l.emit(typ, e)

		case sAccDescrMultiline:
			if c == '}' {
				l.pop()
				l.advance(p + 1)
			} else {
				l.emit(tAccDescrMultilineValue, l.indexFrom('}', p))
			}

		case sDir:
			if c == '"' {
				l.quote()
			} else if e := l.noDir(p); e >= 0 {
				l.pop()
				l.emit(tNodir, e)
			} else if e := l.dir(p); e >= 0 {
				l.pop()
				l.emit(tDir, e)
			} else if e := l.dirSym(p); e >= 0 {
				l.pop()
				l.emit(tDir, e)
			} else if e := l.dirV(p); e >= 0 {
				l.pop()
				l.emit(tDir, e)
			} else {
				ok = l.opener(c)
			}

		case sText:
			d := l.src.At(p + 1)
			switch c {
			case '"':
				l.quote()
			case '(', '[', '{':
				l.opener(c)
			case ']':
				l.pop()
				switch d {
				case ')':
					l.emitFlat(tStadiumend, p+2)
				case ']':
					l.emitFlat(tSubroutineend, p+2)
				default:
					l.emitFlat(tSqe, p+1)
				}
			case ')':
				l.pop()
				if d == ']' {
					l.emitFlat(tCylinderend, p+2)
				} else if d == ')' && l.src.At(p+2) == ')' {
					l.emitFlat(tDoublecircleend, p+3)
				} else {
					l.emitFlat(tPe, p+1)
				}
			case '|':
				l.pop()
				l.emitFlat(tPipe, p+1)
			case '}':
				l.pop()
				l.emitFlat(tDiamondStop, p+1)
			case '%':
				// Jison drops the lone `%` from Mermaid's text pattern, so only a comment may start with one.
				if e := l.commentEnd(p); e >= 0 {
					l.advance(e)
				} else {
					ok = false
				}
			default:
				q := p + 1
				for q < n {
					ch := l.src.At(q)
					if ch == '[' || ch == ']' || ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '|' || ch == '"' || ch == '%' {
						break
					}
					q++
				}
				l.emit(tText, q)
			}

		case sEllipseText:
			d := l.src.At(p + 1)
			if c == '"' {
				l.quote()
			} else if e := l.commentEnd(p); c == '%' && e >= 0 {
				l.advance(e)
			} else if (c == '-' || c == '/' || c == ')') && d == ')' {
				l.pop()
				l.emitFlat(tEllipseEnd, p+2)
			} else if c == '(' || c == '[' || c == '{' {
				l.opener(c)
			} else if c == ')' || c == ']' || c == '}' {
				ok = false
			} else {
				q := p + 1
				for q < n {
					ch := l.src.At(q)
					if ch == '"' || ch == '(' || ch == ')' || ch == '[' || ch == ']' || ch == '{' || ch == '}' {
						break
					}
					if (ch == '-' || ch == '/') && l.src.At(q+1) == ')' {
						break
					}
					if ch == '%' && l.src.At(q+1) == '%' && l.src.At(q+2) != '{' {
						break
					}
					q++
				}
				l.emitChars(tText, q)
			}

		case sTrapText:
			ok = l.trapText(c)

		case sEdgeText:
			ok = l.edgeText(c)

		case sClick:
			if c == '"' {
				l.quote()
			} else if util.IsSpace(c) {
				l.pop()
				l.advance(p + 1)
			} else {
				q := p + 1
				for q < n && !util.IsSpace(l.src.At(q)) {
					q++
				}
				l.emitFlat(tClick, q)
			}

		case sCallbackname:
			if c == '(' {
				e := l.emptyCall(p)
				l.pop()
				if e >= 0 {
					l.advance(e)
				} else {
					l.push(sCallbackargs)
					l.advance(p + 1)
				}
			} else {
				l.emit(tCallbackname, l.indexFrom('(', p))
			}

		case sCallbackargs:
			if c == ')' {
				l.pop()
				l.advance(p + 1)
			} else {
				l.emit(tCallbackargs, l.indexFrom(')', p))
			}

		case sShapeData:
			switch c {
			case '"':
				l.push(sShapeDataStr)
				l.emitFlat(tShapeData, p+1)
			case '}':
				l.pop()
				l.advance(p + 1)
			case '^':
				ok = false
			default:
				q := p + 1
				for q < n {
					if ch := l.src.At(q); ch == '}' || ch == '^' || ch == '"' {
						break
					}
					q++
				}
				l.emit(tShapeData, q)
			}

		case sShapeDataStr:
			if c == '"' {
				l.pop()
				l.emitFlat(tShapeData, p+1)
			} else {
				e := l.indexFrom('"', p)
				l.emitText(tShapeData, breaks(l.src.String(p, e)), e)
			}
		}

		if !ok {
			l.begin(tError, "")
			l.finish()
			return l.out
		}
	}

	l.begin(tInputEnd, "")
	l.finish()
	return l.out
}

func (l *lexer) trapText(c int) bool {
	p := l.p
	d := l.src.At(p + 1)
	if c == '"' {
		l.quote()
		return true
	}
	switch c {
	case '(':
		if d == ']' {
			l.pop()
			l.emitFlat(tTrapend, p+2)
		} else {
			l.opener(c)
		}
	case '[', '{':
		l.opener(c)
	case '\\':
		if d == ']' {
			l.pop()
			l.emitFlat(tTrapend, p+2)
		} else {
			l.emitFlat(tText, p+1)
		}
	case '/':
		if d == ']' {
			l.pop()
			l.emitFlat(tInvtrapend, p+2)
		} else {
			l.emitFlat(tText, p+1)
		}
	case ']', ')':
		if d != ']' {
			return false
		}
		l.pop()
		l.emitFlat(tTrapend, p+2)
	case '}':
		return false
	case '%':
		e := l.commentEnd(p)
		if e < 0 {
			return false
		}
		l.advance(e)
	default:
		if (c == '?' || c == '=') && d == ']' {
			l.pop()
			l.emitFlat(tTrapend, p+2)
			return true
		}
		q := p + 1
		for q < l.n {
			ch := l.src.At(q)
			if ch == '\\' || ch == '[' || ch == ']' || ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '/' || ch == '%' {
				break
			}
			q++
		}
		l.emit(tText, q)
	}
	return true
}

func (l *lexer) edgeText(c int) bool {
	if c == '"' {
		l.quote()
		return true
	}
	p := l.p
	if e := l.arrowEnd(p); e >= 0 {
		l.pop()
		l.emit(tLink, e)
		return true
	}
	if c == '-' && l.src.At(p+1) == '-' {
		return false
	}
	q := p + 1
	for q < l.n {
		ch := l.src.At(q)
		if ch == '"' {
			break
		}
		if ch == '-' {
			if l.src.At(q+1) == '-' {
				break
			}
		} else if util.IsSpace(ch) && l.arrowEnd(q) >= 0 {
			break
		}
		q++
	}
	l.emitChars(tEdgeText, q)
	return true
}
