package flowchart

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

// tokens are the lexer's output: types, texts and start offsets in parallel.
type tokens struct {
	types  []int
	texts  []string
	starts []int
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
	sThickEdgeText
	sDottedEdgeText
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

// The sticky regexes upstream matches with, as matchers that return the end of the match at
// q, or -1. None of them can backtrack into a different match: each is noted where that
// takes a moment's thought.

func (l *lexer) skipSpace(q int) int {
	for util.IsSpace(l.src.At(q)) {
		q++
	}
	return q
}

// word matches a literal at q.
func (l *lexer) word(q int, w string) int {
	if l.src.HasPrefix(q, w) {
		return q + len(w)
	}
	return -1
}

// accHead matches /<word>\s*<c>\s*/y: accTitle\s*:\s*, accDescr\s*:\s* and accDescr\s*\{\s*.
func (l *lexer) accHead(q int, w string, c int) int {
	q = l.word(q, w)
	if q < 0 {
		return -1
	}
	q = l.skipSpace(q)
	if l.src.At(q) != c {
		return -1
	}
	return l.skipSpace(q + 1)
}

// spaceAfter matches /<word>\s+/y: call\s+ and click\s+.
func (l *lexer) spaceAfter(q int, w string) int {
	q = l.word(q, w)
	if q < 0 || !util.IsSpace(l.src.At(q)) {
		return -1
	}
	return l.skipSpace(q)
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
	for lineEnd < len(l.src) && !util.IsLineEnd(l.src.At(lineEnd)) {
		lineEnd++
	}
	for k := lineEnd; k >= q; k-- {
		j := l.word(k, "direction")
		if j < 0 || !util.IsSpace(l.src.At(j)) {
			continue
		}
		j = l.word(l.skipSpace(j), directionWords[i])
		if j < 0 {
			continue
		}
		for j < len(l.src) && l.src[j] != '\n' {
			j++
		}
		return j
	}
	return -1
}

// link matches /\s*[xo<]?<c>{2,}[<c>xo>]\s*/y, for c - and =: the run of c gives one back
// for the last character when nothing else follows it.
func (l *lexer) link(q, c int) int {
	q = l.skipSpace(q)
	if d := l.src.At(q); d == 'x' || d == 'o' || d == '<' {
		q++
	}
	k := 0
	for l.src.At(q+k) == c {
		k++
	}
	if d := l.src.At(q + k); k >= 2 && (d == 'x' || d == 'o' || d == '>') {
		return l.skipSpace(q + k + 1)
	}
	if k >= 3 {
		return l.skipSpace(q + k)
	}
	return -1
}

// startLink matches /\s*[xo<]?<c><c>\s*/y, and with c . the dotted /\s*[xo<]?-\.\s*/y.
func (l *lexer) startLink(q int, first, second int) int {
	q = l.skipSpace(q)
	if d := l.src.At(q); d == 'x' || d == 'o' || d == '<' {
		q++
	}
	if l.src.At(q) != first || l.src.At(q+1) != second {
		return -1
	}
	return l.skipSpace(q + 2)
}

// dottedLink matches /\s*[xo<]?-?\.+-[xo>]?\s*/y.
func (l *lexer) dottedLink(q int) int {
	q = l.skipSpace(q)
	if d := l.src.At(q); d == 'x' || d == 'o' || d == '<' {
		q++
	}
	if l.src.At(q) == '-' {
		q++
	}
	if l.src.At(q) != '.' {
		return -1
	}
	for l.src.At(q) == '.' {
		q++
	}
	if l.src.At(q) != '-' {
		return -1
	}
	q++
	if d := l.src.At(q); d == 'x' || d == 'o' || d == '>' {
		q++
	}
	return l.skipSpace(q)
}

// invisibleLink matches /\s*~~~+\s*/y.
func (l *lexer) invisibleLink(q int) int {
	q = l.skipSpace(q)
	k := 0
	for l.src.At(q+k) == '~' {
		k++
	}
	if k < 3 {
		return -1
	}
	return l.skipSpace(q + k)
}

// newline matches /(?:\r?\n)+/y.
func (l *lexer) newline(q int) int {
	end := -1
	for {
		j := q
		if l.src.At(j) == '\r' {
			j++
		}
		if l.src.At(j) != '\n' {
			return end
		}
		q = j + 1
		end = q
	}
}

// space matches /[^\S\n\r]+/y.
func (l *lexer) space(q int) int {
	j := q
	for c := l.src.At(j); util.IsSpace(c) && c != '\n' && c != '\r'; c = l.src.At(j) {
		j++
	}
	if j == q {
		return -1
	}
	return j
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

// The link matchers by the state they end: RE_LINK, RE_THICK_LINK and RE_DOTTED_LINK.
func (l *lexer) linkRe(state, q int) int {
	switch state {
	case sEdgeText:
		return l.link(q, '-')
	case sThickEdgeText:
		return l.link(q, '=')
	}
	return l.dottedLink(q)
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

type lexer struct {
	src   common.Src
	n     int
	out   tokens
	stack []int
	state int
	p     int

	firstGraph bool
	dirNext    int
	atNext     int
	lineEnd    int
	dirFailed  int
	runEnd     int
	runAt      int
	wsFrom     int
	wsTo       int
}

func (l *lexer) emit(typ, s, e int) {
	l.out.types = append(l.out.types, typ)
	l.out.texts = append(l.out.texts, l.src.String(s, e))
	l.out.starts = append(l.out.starts, s)
	l.p = e
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

// spaceRunEnd is where the run of whitespace at q ends, remembered for the next call.
func (l *lexer) spaceRunEnd(q int) int {
	j := q
	if q >= l.wsFrom && q <= l.wsTo {
		j = l.wsTo
	} else if util.IsSpace(l.src.At(q)) {
		for j < l.n && util.IsSpace(l.src.At(j)) {
			j++
		}
		l.wsFrom = q
		l.wsTo = j
	}
	return j
}

// linkChar is the first character after any run of whitespace starting at q, skipping one
// optional x, o or <.
func (l *lexer) linkChar(q int) int {
	j := l.spaceRunEnd(q)
	d := l.src.At(j)
	if d == 'x' || d == 'o' || d == '<' {
		return l.src.At(j + 1)
	}
	return d
}

func (l *lexer) invisAhead(q int) bool {
	return l.src.At(l.spaceRunEnd(q)) == '~'
}

func (l *lexer) quote() {
	if l.src.At(l.p+1) == '`' {
		l.push(sMdString)
		l.p += 2
	} else {
		l.push(sString)
		l.p++
	}
}

func (l *lexer) invisLink() bool {
	if !l.invisAhead(l.p) {
		return false
	}
	e := l.invisibleLink(l.p)
	if e < 0 {
		return false
	}
	l.emit(tLink, l.p, e)
	return true
}

func (l *lexer) opener(c int) bool {
	p := l.p
	d := l.src.At(p + 1)
	switch c {
	case '(':
		switch {
		case d == '-':
			l.push(sEllipseText)
			l.emit(tEllipseStart, p, p+2)
		case d == '[':
			l.push(sText)
			l.emit(tStadiumstart, p, p+2)
		case d == '(' && l.src.At(p+2) == '(':
			l.push(sText)
			l.emit(tDoublecirclestart, p, p+3)
		default:
			l.push(sText)
			l.emit(tPs, p, p+1)
		}
		return true
	case '[':
		switch d {
		case '[':
			l.push(sText)
			l.emit(tSubroutinestart, p, p+2)
		case '(':
			l.push(sText)
			l.emit(tCylinderstart, p, p+2)
		case '/':
			l.push(sTrapText)
			l.emit(tTrapstart, p, p+2)
		case '\\':
			l.push(sTrapText)
			l.emit(tInvtrapstart, p, p+2)
		default:
			l.push(sText)
			l.emit(tSqs, p, p+1)
		}
		return true
	case '|':
		l.push(sText)
		l.emit(tPipe, p, p+1)
		return true
	case '{':
		l.push(sText)
		l.emit(tDiamondStart, p, p+1)
		return true
	}
	return false
}

func (l *lexer) graph(length int) {
	if l.firstGraph {
		l.firstGraph = false
		l.push(sDir)
	}
	l.emit(tGraph, l.p, l.p+length)
}

func (l *lexer) nodeString() bool {
	q := l.p
	for q < l.n {
		ch := l.src.At(q)
		if ch < 128 && nodeChar[ch] {
			q++
		} else if ch == '-' && q+1 < l.n {
			d := l.src.At(q + 1)
			if d == '>' || d == '-' || d == '.' {
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
	l.emit(tNodeString, l.p, q)
	return true
}

func (l *lexer) linkToken(c int) bool {
	p := l.p
	d := l.linkChar(p)
	switch d {
	case '-':
		if e := l.link(p, '-'); e >= 0 {
			l.emit(tLink, p, e)
			return true
		}
		if e := l.startLink(p, '-', '-'); e >= 0 {
			l.push(sEdgeText)
			l.emit(tStartLink, p, e)
			return true
		}
		if e := l.dottedLink(p); e >= 0 {
			l.emit(tLink, p, e)
			return true
		}
		if e := l.startLink(p, '-', '.'); e >= 0 {
			l.push(sDottedEdgeText)
			l.emit(tStartLink, p, e)
			return true
		}
	case '=':
		if e := l.link(p, '='); e >= 0 {
			l.emit(tLink, p, e)
			return true
		}
		if e := l.startLink(p, '=', '='); e >= 0 {
			l.push(sThickEdgeText)
			l.emit(tStartLink, p, e)
			return true
		}
	case '.':
		if e := l.dottedLink(p); e >= 0 {
			l.emit(tLink, p, e)
			return true
		}
	}
	return c != '-' && c != '=' && c != '.' && l.invisLink()
}

func (l *lexer) initial(c int) bool {
	p := l.p
	switch c {
	case 'a':
		if l.src.HasPrefix(p, "acc") {
			if e := l.accHead(p, "accTitle", ':'); e >= 0 {
				l.push(sAccTitle)
				l.emit(tAccTitle, p, e)
				return true
			}
			if e := l.accHead(p, "accDescr", ':'); e >= 0 {
				l.push(sAccDescr)
				l.emit(tAccDescr, p, e)
				return true
			}
			if e := l.accHead(p, "accDescr", '{'); e >= 0 {
				l.push(sAccDescrMultiline)
				l.p = e
				return true
			}
		}
	case '@':
		if l.src.At(p+1) == '{' {
			l.push(sShapeData)
			l.out.types = append(l.out.types, tShapeData)
			l.out.texts = append(l.out.texts, "")
			l.out.starts = append(l.out.starts, p)
			l.p += 2
			return true
		}
	case 'c':
		if e := l.spaceAfter(p, "call"); e >= 0 {
			l.push(sCallbackname)
			l.p = e
			return true
		}
		if l.kw("classDef") {
			l.emit(tClassdef, p, p+8)
			return true
		}
		if l.kw("class") {
			l.emit(tClass, p, p+5)
			return true
		}
		if e := l.spaceAfter(p, "click"); e >= 0 {
			l.push(sClick)
			l.p = e
			return true
		}
	case '"':
		l.quote()
		return true
	case 's':
		if l.kw("style") {
			l.emit(tStyle, p, p+5)
			return true
		}
		if l.kw("swimlane-beta") {
			l.graph(13)
			return true
		}
		if l.kw("subgraph") {
			l.emit(tSubgraph, p, p+8)
			return true
		}
	case 'd':
		if l.kw("default") {
			l.emit(tDefault, p, p+7)
			return true
		}
	case 'l':
		if l.kw("linkStyle") {
			l.emit(tLinkstyle, p, p+9)
			return true
		}
	case 'i':
		if l.kw("interpolate") {
			l.emit(tInterpolate, p, p+11)
			return true
		}
	case 'h':
		if l.src.HasPrefix(p, "href") && util.IsSpace(l.src.At(p+4)) {
			l.emit(tHref, p, p+5)
			return true
		}
	case 'f':
		if l.kw("flowchart-elk") {
			l.graph(13)
			return true
		}
		if l.kw("flowchart") {
			l.graph(9)
			return true
		}
	case 'g':
		if l.kw("graph") {
			l.graph(5)
			return true
		}
	case 'e':
		if l.kw("end") {
			e := p + 3
			for e < l.n && util.IsSpace(l.src.At(e)) {
				e++
			}
			l.emit(tEnd, p, e)
			return true
		}
	case '_':
		for _, target := range [...]string{"_self", "_blank", "_parent", "_top"} {
			if l.kw(target) {
				l.emit(tLinkTarget, p, p+len(target))
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
						l.emit(tDirectionTB+i, p, e)
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
				ch := l.src.At(q)
				if ch == '"' || util.IsSpace(ch) {
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
			l.emit(tLinkID, p, l.runAt+1)
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
		l.emit(tNum, p, q)
		return true
	}

	switch c {
	case '#':
		l.emit(tBrkt, p, p+1)
		return true
	case ':':
		if l.src.At(p+1) == ':' && l.src.At(p+2) == ':' {
			l.emit(tStyleSeparator, p, p+3)
		} else {
			l.emit(tColon, p, p+1)
		}
		return true
	case '&':
		l.emit(tAmp, p, p+1)
		return true
	case ';':
		l.emit(tSemi, p, p+1)
		return true
	case ',':
		l.emit(tComma, p, p+1)
		return true
	case '*':
		l.emit(tMult, p, p+1)
		return true
	case 'x', 'o':
		return l.linkToken(c) || l.nodeString()
	case '<':
		if l.linkToken(c) {
			return true
		}
		l.emit(tTagstart, p, p+1)
		return true
	case '-':
		if l.linkToken(c) || l.nodeString() {
			return true
		}
		l.emit(tMinus, p, p+1)
		return true
	case '=':
		return l.linkToken(c)
	case '.':
		return l.linkToken(c) || l.nodeString()
	case '~':
		return l.invisLink()
	case '(':
		return l.opener(c)
	case '[':
		if l.src.At(p+1) == '|' {
			l.emit(tVertexWithPropsStart, p, p+2)
			return true
		}
		return l.opener(c)
	case '|', '{':
		return l.opener(c)
	case '>':
		l.push(sText)
		l.emit(tTagend, p, p+1)
		return true
	case '^':
		l.emit(tUp, p, p+1)
		return true
	case '\\':
		if l.src.At(p+1) == '|' {
			l.emit(tSep, p, p+2)
			return true
		}
		return l.nodeString()
	case 'v':
		if !util.IsWord(l.src.At(p + 1)) {
			l.emit(tDown, p, p+1)
			return true
		}
		return l.nodeString()
	}

	if util.IsSpace(c) {
		if l.linkToken(c) {
			return true
		}
		if e := l.newline(p); e >= 0 {
			l.emit(tNewline, p, e)
			return true
		}
		if e := l.space(p); e >= 0 {
			l.emit(tSpace, p, e)
			return true
		}
		return false
	}

	if c < 128 {
		return l.nodeString()
	}
	if util.IsUnicodeLetter(c) {
		l.emit(tUnicodeText, p, p+1)
		return true
	}
	return false
}

func (l *lexer) edgeText(c, a, b int) bool {
	if c == '"' {
		l.quote()
		return true
	}
	p := l.p
	if d := l.linkChar(p); d == a || d == b {
		if e := l.linkRe(l.state, p); e >= 0 {
			l.pop()
			l.emit(tLink, p, e)
			return true
		}
	}
	if c == a && (a != '-' || l.src.At(p+1) == '-') {
		return false
	}
	q := p + 1
	for q < l.n {
		ch := l.src.At(q)
		if ch == '"' {
			break
		}
		if ch == a {
			if a != '-' || l.src.At(q+1) == '-' {
				break
			}
		} else if ch == 'x' || ch == 'o' || ch == '<' || ch == b || util.IsSpace(ch) {
			if dd := l.linkChar(q); (dd == a || dd == b) && l.linkRe(l.state, q) >= 0 {
				break
			}
		}
		q++
	}
	l.emit(tEdgeText, p, q)
	return true
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
		firstGraph: true,
		dirNext:    src.Index("direction", 0),
		atNext:     src.Index("@", 0),
		lineEnd:    -1,
		runAt:      -1,
		wsFrom:     -1,
		wsTo:       -1,
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
				l.emit(typ, n, n)
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
					l.p += 2
				} else {
					l.pop()
					l.p++
				}
			} else {
				l.emit(tStr, p, l.indexFrom('"', p))
			}

		case sMdString:
			switch c {
			case '`':
				if l.src.At(p+1) == '"' {
					l.pop()
					l.p += 2
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
				l.emit(tMdStr, p, q)
			}

		case sAccTitle, sAccDescr:
			e := l.indexFrom('\n', p)
			typ := tAccDescrValue
			if l.state == sAccTitle {
				typ = tAccTitleValue
			}
			l.pop()
			l.emit(typ, p, e)

		case sAccDescrMultiline:
			if c == '}' {
				l.pop()
				l.p++
			} else {
				l.emit(tAccDescrMultilineValue, p, l.indexFrom('}', p))
			}

		case sDir:
			if c == '"' {
				l.quote()
			} else if e := l.noDir(p); e >= 0 {
				l.pop()
				l.emit(tNodir, p, e)
			} else if e := l.dir(p); e >= 0 {
				l.pop()
				l.emit(tDir, p, e)
			} else if e := l.dirSym(p); e >= 0 {
				l.pop()
				l.emit(tDir, p, e)
			} else if e := l.dirV(p); e >= 0 {
				l.pop()
				l.emit(tDir, p, e)
			} else {
				ok = l.invisLink() || l.opener(c)
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
					l.emit(tStadiumend, p, p+2)
				case ']':
					l.emit(tSubroutineend, p, p+2)
				default:
					l.emit(tSqe, p, p+1)
				}
			case ')':
				l.pop()
				if d == ']' {
					l.emit(tCylinderend, p, p+2)
				} else if d == ')' && l.src.At(p+2) == ')' {
					l.emit(tDoublecircleend, p, p+3)
				} else {
					l.emit(tPe, p, p+1)
				}
			case '|':
				l.pop()
				l.emit(tPipe, p, p+1)
			case '}':
				l.pop()
				l.emit(tDiamondStop, p, p+1)
			default:
				if (c == '~' || util.IsSpace(c)) && l.invisLink() {
					break
				}
				q := p + 1
				for q < n {
					ch := l.src.At(q)
					if ch == '[' || ch == ']' || ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '|' || ch == '"' {
						break
					}
					q++
				}
				l.emit(tText, p, q)
			}

		case sEllipseText:
			d := l.src.At(p + 1)
			switch {
			case c == '"':
				l.quote()
			case (c == '~' || util.IsSpace(c)) && l.invisLink():
			case (c == '-' || c == '/' || c == ')') && d == ')':
				l.pop()
				l.emit(tEllipseEnd, p, p+2)
			case c == '(' || c == '[' || c == '{':
				l.opener(c)
			case c == ')' || c == ']' || c == '}':
				ok = false
			default:
				q := p + 1
				for q < n {
					ch := l.src.At(q)
					if ch == '"' || ch == '(' || ch == ')' || ch == '[' || ch == ']' || ch == '{' || ch == '}' {
						break
					}
					if (ch == '-' || ch == '/') && l.src.At(q+1) == ')' {
						break
					}
					if (ch == '~' || util.IsSpace(ch)) && l.invisAhead(q) && l.invisibleLink(q) >= 0 {
						break
					}
					q++
				}
				l.emit(tText, p, q)
			}

		case sTrapText:
			ok = l.trapText(c)

		case sEdgeText:
			ok = l.edgeText(c, '-', -1)
		case sThickEdgeText:
			ok = l.edgeText(c, '=', -1)
		case sDottedEdgeText:
			ok = l.edgeText(c, '.', '-')

		case sClick:
			if c == '"' {
				l.quote()
			} else if util.IsSpace(c) {
				l.pop()
				l.p++
			} else {
				q := p + 1
				for q < n && !util.IsSpace(l.src.At(q)) {
					q++
				}
				l.emit(tClick, p, q)
			}

		case sCallbackname:
			if c == '(' {
				e := l.emptyCall(p)
				l.pop()
				if e >= 0 {
					l.p = e
				} else {
					l.push(sCallbackargs)
					l.p++
				}
			} else {
				l.emit(tCallbackname, p, l.indexFrom('(', p))
			}

		case sCallbackargs:
			if c == ')' {
				l.pop()
				l.p++
			} else {
				l.emit(tCallbackargs, p, l.indexFrom(')', p))
			}

		case sShapeData:
			switch c {
			case '"':
				l.push(sShapeDataStr)
				l.emit(tShapeData, p, p+1)
			case '}':
				l.pop()
				l.p++
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
				l.emit(tShapeData, p, q)
			}

		case sShapeDataStr:
			if c == '"' {
				l.pop()
				l.emit(tShapeData, p, p+1)
			} else {
				e := l.indexFrom('"', p)
				l.out.types = append(l.out.types, tShapeData)
				l.out.texts = append(l.out.texts, breaks(l.src.String(p, e)))
				l.out.starts = append(l.out.starts, p)
				l.p = e
			}
		}

		if !ok {
			l.out.types = append(l.out.types, tError)
			l.out.texts = append(l.out.texts, "")
			l.out.starts = append(l.out.starts, l.p)
			return l.out
		}
	}

	l.out.types = append(l.out.types, tInputEnd)
	l.out.texts = append(l.out.texts, "")
	l.out.starts = append(l.out.starts, n)
	return l.out
}

func (l *lexer) trapText(c int) bool {
	p := l.p
	d := l.src.At(p + 1)
	if c == '"' {
		l.quote()
		return true
	}
	if (c == '~' || util.IsSpace(c)) && l.invisLink() {
		return true
	}
	switch c {
	case '(':
		if d == '-' || d == '[' || (d == '(' && l.src.At(p+2) == '(') {
			l.opener(c)
		} else if d == ']' {
			l.pop()
			l.emit(tTrapend, p, p+2)
		} else {
			l.opener(c)
		}
	case '[', '{':
		l.opener(c)
	case '\\':
		if d == ']' {
			l.pop()
			l.emit(tTrapend, p, p+2)
		} else {
			l.emit(tText, p, p+1)
		}
	case '/':
		if d == ']' {
			l.pop()
			l.emit(tInvtrapend, p, p+2)
		} else {
			l.emit(tText, p, p+1)
		}
	case ']', ')':
		if d != ']' {
			return false
		}
		l.pop()
		l.emit(tTrapend, p, p+2)
	case '}':
		return false
	default:
		if (c == '?' || c == '=') && d == ']' {
			l.pop()
			l.emit(tTrapend, p, p+2)
			return true
		}
		q := p + 1
		for q < l.n {
			ch := l.src.At(q)
			if ch == '\\' || ch == '[' || ch == ']' || ch == '(' || ch == ')' || ch == '{' || ch == '}' || ch == '/' {
				break
			}
			q++
		}
		l.emit(tText, p, q)
	}
	return true
}
