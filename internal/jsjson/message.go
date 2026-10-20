package jsjson

import (
	"strconv"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// V8Message returns the message of the SyntaxError V8's JSON.parse throws for s, or "" where
// it parses s. It follows V8's JsonParser (src/json/json-parser.cc): where it stops, which of
// its messages it picks there, and how much of the text it quotes.
func V8Message(s string) (message string) {
	p := v8Parser{src: jsstr.UTF16(s)}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(v8Stop); !ok {
				panic(r)
			}
			message = p.message
		}
	}()
	p.parse()
	return ""
}

type v8Token int

const (
	tokEOS v8Token = iota
	tokString
	tokNumber
	tokTrue
	tokFalse
	tokNull
	tokWhitespace
	tokColon
	tokComma
	tokLBrack
	tokRBrack
	tokLBrace
	tokRBrace
	tokIllegal
)

// V8's kMaxContextCharacters, and kMinOriginalSourceLengthForContext.
const (
	contextChars    = 10
	contextMinChars = 2*contextChars + 1
)

type v8Stop struct{}

type v8Parser struct {
	src     []uint16
	i       int
	message string
}

// tokenOf is V8's one_char_json_tokens, with -1 for the end of the text.
func tokenOf(c int) v8Token {
	switch {
	case c < 0:
		return tokEOS
	case c == '"':
		return tokString
	case c == '-' || c >= '0' && c <= '9':
		return tokNumber
	case c == 't':
		return tokTrue
	case c == 'f':
		return tokFalse
	case c == 'n':
		return tokNull
	case c == ' ' || c == '\t' || c == '\r' || c == '\n':
		return tokWhitespace
	case c == ':':
		return tokColon
	case c == ',':
		return tokComma
	case c == '[':
		return tokLBrack
	case c == ']':
		return tokRBrack
	case c == '{':
		return tokLBrace
	case c == '}':
		return tokRBrace
	}
	return tokIllegal
}

func (p *v8Parser) at(i int) int {
	if i < 0 || i >= len(p.src) {
		return -1
	}
	return int(p.src[i])
}

func (p *v8Parser) peek() v8Token { return tokenOf(p.at(p.i)) }

func (p *v8Parser) skipWhitespace() {
	for p.peek() == tokWhitespace {
		p.i++
	}
}

func (p *v8Parser) check(token v8Token) bool {
	p.skipWhitespace()
	if p.peek() != token {
		return false
	}
	p.i++
	return true
}

func (p *v8Parser) expect(token v8Token, template string) {
	if p.peek() != token {
		p.report(p.peek(), template)
	}
	p.i++
}

func (p *v8Parser) expectNext(token v8Token, template string) {
	p.skipWhitespace()
	p.expect(token, template)
}

// location is V8's CalculateFileLocation: the line and column of the cursor, where \r\n
// counts as one line break.
func (p *v8Parser) location() (int, int) {
	line := 1
	lastBreak := 0
	k := 0
	for ; k < p.i; k++ {
		if p.src[k] == '\r' && k < p.i-1 && p.src[k+1] == '\n' {
			k++
		}
		if p.src[k] == '\r' || p.src[k] == '\n' {
			line++
			lastBreak = k + 1
		}
	}
	return line, 1 + k - lastBreak
}

// isSpecial is V8's IsSpecialString: what String() makes of the values people pass by mistake.
func (p *v8Parser) isSpecial() bool {
	switch jsstr.FromUTF16(p.src) {
	case "[object Object]", "undefined", "Infinity", "NaN":
		return true
	}
	return false
}

// report is V8's ReportUnexpectedToken: template, when given, names what went wrong at the
// cursor; otherwise the message names the token found there.
func (p *v8Parser) report(token v8Token, template string) {
	pos := p.i
	if template == "" {
		switch token {
		case tokEOS:
			p.message = "Unexpected end of JSON input"
		case tokNumber:
			template = "Unexpected number in JSON"
		case tokString:
			template = "Unexpected string in JSON"
		default:
			p.message = p.unexpectedToken(pos)
		}
	}
	if template != "" {
		line, column := p.location()
		p.message = template + " at position " + strconv.Itoa(pos) +
			" (line " + strconv.Itoa(line) + " column " + strconv.Itoa(column) + ")"
	}
	panic(v8Stop{})
}

func (p *v8Parser) unexpectedToken(pos int) string {
	n := len(p.src)
	if p.isSpecial() {
		return `"` + jsstr.FromUTF16(p.src) + `" is not valid JSON`
	}
	head := "Unexpected token '" + jsstr.FromUTF16(p.src[pos:pos+1]) + "', "
	switch {
	case n < contextMinChars:
		return head + `"` + jsstr.FromUTF16(p.src) + `" is not valid JSON`
	case pos < contextChars:
		return head + `"` + jsstr.FromUTF16(p.src[:pos+contextChars]) + `"... is not valid JSON`
	case pos < n-contextChars:
		return head + `..."` + jsstr.FromUTF16(p.src[pos-contextChars:pos+contextChars]) + `"... is not valid JSON`
	}
	return head + `..."` + jsstr.FromUTF16(p.src[pos-contextChars:]) + `" is not valid JSON`
}

// reportCharacter is V8's ReportUnexpectedCharacter.
func (p *v8Parser) reportCharacter(c int) {
	token := tokIllegal
	if c < 0 || c <= 0xff {
		token = tokenOf(c)
	}
	p.report(token, "")
}

func (p *v8Parser) parse() {
	// Whether each open container is an object; V8 keeps them on a stack of its own too.
	var stack []bool
	p.skipWhitespace()
	for {
		p.skipWhitespace()
		switch p.peek() {
		case tokString:
			p.i++
			p.scanString()
		case tokNumber:
			p.scanNumber()
		case tokLBrace:
			p.i++
			if p.check(tokRBrace) {
				break
			}
			stack = append(stack, true)
			p.expectNext(tokString, "Expected property name or '}' in JSON")
			p.scanString()
			p.expectNext(tokColon, "Expected ':' after property name in JSON")
			continue
		case tokLBrack:
			p.i++
			if p.check(tokRBrack) {
				break
			}
			stack = append(stack, false)
			continue
		case tokTrue:
			p.scanLiteral("true")
		case tokFalse:
			p.scanLiteral("false")
		case tokNull:
			p.scanLiteral("null")
		default:
			p.reportCharacter(p.at(p.i))
		}
		for {
			if len(stack) == 0 {
				if !p.check(tokEOS) {
					p.report(p.peek(), "Unexpected non-whitespace character after JSON")
				}
				return
			}
			object := stack[len(stack)-1]
			if p.check(tokComma) {
				if object {
					p.expectNext(tokString, "Expected double-quoted property name in JSON")
					p.scanString()
					p.expectNext(tokColon, "Expected ':' after property name in JSON")
				}
				break
			}
			if object {
				p.expect(tokRBrace, "Expected ',' or '}' after property value in JSON")
			} else {
				p.expect(tokRBrack, "Expected ',' or ']' after array element in JSON")
			}
			stack = stack[:len(stack)-1]
		}
	}
}

// scanLiteral is V8's ScanLiteral, with the cursor on the first character of word.
func (p *v8Parser) scanLiteral(word string) {
	remaining := len(p.src) - p.i
	if remaining >= len(word) {
		match := true
		for k := 1; k < len(word); k++ {
			if p.src[p.i+k] != uint16(word[k]) {
				match = false
				break
			}
		}
		if match {
			p.i += len(word)
			return
		}
	}
	p.i++
	for k := 0; k < min(len(word)-1, remaining-1); k++ {
		if p.at(p.i) != int(word[1+k]) {
			p.reportCharacter(p.at(p.i))
		}
		p.i++
	}
	p.report(tokEOS, "")
}

func isDecimal(c int) bool { return c >= '0' && c <= '9' }

// scanNumber is V8's ParseJsonNumber, with the cursor on the sign or the first digit.
func (p *v8Parser) scanNumber() {
	c := p.at(p.i)
	if c == '-' {
		p.i++
		c = p.at(p.i)
	}
	if c == '0' {
		p.i++
		// A leading zero is only allowed alone before the point or the exponent.
		if isDecimal(p.at(p.i)) {
			p.report(tokNumber, "")
		}
	} else {
		start := p.i
		for isDecimal(p.at(p.i)) {
			p.i++
		}
		if p.i == start {
			p.report(tokIllegal, "No number after minus sign in JSON")
		}
	}
	if p.at(p.i) == '.' {
		p.i++
		if !isDecimal(p.at(p.i)) {
			p.report(tokIllegal, "Unterminated fractional number in JSON")
		}
		for isDecimal(p.at(p.i)) {
			p.i++
		}
	}
	if c := p.at(p.i); c == 'e' || c == 'E' {
		p.i++
		if c := p.at(p.i); c == '-' || c == '+' {
			p.i++
		}
		if !isDecimal(p.at(p.i)) {
			p.report(tokIllegal, "Exponent part is missing a number in JSON")
		}
		for isDecimal(p.at(p.i)) {
			p.i++
		}
	}
}

// scanString is V8's ScanJsonString, with the cursor past the opening quote.
func (p *v8Parser) scanString() {
	for {
		c := p.at(p.i)
		switch {
		case c < 0:
			p.report(tokIllegal, "Unterminated string in JSON")
		case c == '"':
			p.i++
			return
		case c == '\\':
			p.i++
			e := p.at(p.i)
			if e < 0 || e > 0xff {
				p.reportCharacter(e)
			}
			switch e {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			case 'u':
				for range 4 {
					p.i++
					if d := p.at(p.i); d < 0 || hexValue(uint16(d)) < 0 {
						p.report(tokIllegal, "Bad Unicode escape in JSON")
					}
				}
			default:
				p.report(tokIllegal, "Bad escaped character in JSON")
			}
		case c < 0x20:
			p.report(tokIllegal, "Bad control character in string literal in JSON")
		}
		p.i++
	}
}
