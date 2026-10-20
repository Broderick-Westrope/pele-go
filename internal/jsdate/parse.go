package jsdate

import (
	"math"
	"time"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// Parse is new Date(s) in loc, through V8's date parser (src/date/dateparser*): ES5 ISO
// date-time strings, then the legacy forms browsers accept, such as "Jan 2 2024 10:00 PM".
func Parse(s string, loc *time.Location) Date {
	var out [outputSize]float64
	if !parseDate(jsstr.UTF16(s), &out) {
		return Date{t: math.NaN(), loc: loc}
	}
	date := makeDate(makeDay(out[outYear], out[outMonth], out[outDay]),
		makeTime(out[outHour], out[outMinute], out[outSecond], out[outMillisecond]))
	if math.IsNaN(out[outUTCOffset]) {
		date = localToUTC(loc, date)
	} else {
		date -= out[outUTCOffset] * 1000
	}
	return Date{t: timeClip(date), loc: loc}
}

const (
	outYear = iota
	outMonth
	outDay
	outHour
	outMinute
	outSecond
	outMillisecond
	outUTCOffset
	outputSize
)

// none marks a missing value, as V8's kNone (kMaxInt) does.
const none = math.MaxInt32

const maxSignificantDigits = 9

func between(x, lo, hi int) bool { return uint32(x-lo) <= uint32(hi-lo) }

// reader is V8's InputReader: ch is the current code unit, 0 past the end (and at a NUL,
// which V8 also takes for the end).
type reader struct {
	src   []uint16
	index int
	ch    uint32
}

func newReader(src []uint16) *reader {
	r := &reader{src: src}
	r.next()
	return r
}

func (r *reader) next() {
	r.ch = 0
	if r.index < len(r.src) {
		r.ch = uint32(r.src[r.index])
	}
	r.index++
}

func (r *reader) readUnsignedNumeral() int {
	n, i := 0, 0
	for r.ch == '0' {
		r.next()
	}
	for r.isDigit() {
		if i < maxSignificantDigits {
			n = n*10 + int(r.ch) - '0'
		}
		i++
		r.next()
	}
	return n
}

func (r *reader) readWord(prefix *[3]uint32) int {
	n := 0
	for ; r.isAlphaOrAbove() && !isWhiteSpace(r.ch); r.next() {
		if n < len(prefix) {
			prefix[n] = r.ch | 0x20
		}
		n++
	}
	return n
}

func (r *reader) skip(c uint32) bool {
	if r.ch == c {
		r.next()
		return true
	}
	return false
}

func (r *reader) skipWhiteSpace() bool {
	if isWhiteSpace(r.ch) || r.ch == '\n' || r.ch == '\r' || r.ch == 0x2028 || r.ch == 0x2029 {
		r.next()
		return true
	}
	return false
}

func (r *reader) skipParentheses() bool {
	if r.ch != '(' {
		return false
	}
	balance := 0
	for {
		if r.ch == ')' {
			balance--
		} else if r.ch == '(' {
			balance++
		}
		r.next()
		if balance <= 0 || r.ch == 0 {
			return true
		}
	}
}

func (r *reader) isEnd() bool          { return r.ch == 0 }
func (r *reader) isDigit() bool        { return r.ch >= '0' && r.ch <= '9' }
func (r *reader) isAlphaOrAbove() bool { return r.ch >= 'A' }

// isWhiteSpace is V8's IsWhiteSpace: the ECMAScript WhiteSpace production, without line
// terminators.
func isWhiteSpace(c uint32) bool {
	return c != '\n' && c != '\r' && c != 0x2028 && c != 0x2029 && jsstr.IsSpace(rune(c))
}

type keywordType int

const (
	kwInvalid keywordType = iota
	kwMonthName
	kwTimeZoneName
	kwTimeSeparator
	kwAMPM
)

const (
	tagInvalid = -6 + iota
	tagUnknown
	tagWhiteSpace
	tagNumber
	tagSymbol
	tagEndOfInput
)

// token is V8's DateToken. A tag of 0 or more is a keyword of that keywordType.
type token struct {
	tag, length, value int
}

func (t token) isInvalid() bool                  { return t.tag == tagInvalid }
func (t token) isNumber() bool                   { return t.tag == tagNumber }
func (t token) isSymbol(c byte) bool             { return t.tag == tagSymbol && t.value == int(c) }
func (t token) isWhiteSpace() bool               { return t.tag == tagWhiteSpace }
func (t token) isEndOfInput() bool               { return t.tag == tagEndOfInput }
func (t token) isKeyword() bool                  { return t.tag >= 0 }
func (t token) isKeywordType(k keywordType) bool { return t.tag == int(k) }
func (t token) isFixedLengthNumber(n int) bool {
	return t.isNumber() && t.length == n
}
func (t token) isASCIISign() bool {
	return t.tag == tagSymbol && (t.value == '-' || t.value == '+')
}
func (t token) asciiSign() int { return 44 - t.value }
func (t token) isKeywordZ() bool {
	return t.isKeywordType(kwTimeZoneName) && t.length == 1 && t.value == 0
}

var (
	invalidToken    = token{tag: tagInvalid, value: -1}
	endOfInputToken = token{tag: tagEndOfInput, value: -1}
)

// keywords is V8's KeywordTable: a three-letter prefix, its type and value.
var keywords = [...]struct {
	prefix [3]uint32
	kind   keywordType
	value  int
}{
	{[3]uint32{'j', 'a', 'n'}, kwMonthName, 1},
	{[3]uint32{'f', 'e', 'b'}, kwMonthName, 2},
	{[3]uint32{'m', 'a', 'r'}, kwMonthName, 3},
	{[3]uint32{'a', 'p', 'r'}, kwMonthName, 4},
	{[3]uint32{'m', 'a', 'y'}, kwMonthName, 5},
	{[3]uint32{'j', 'u', 'n'}, kwMonthName, 6},
	{[3]uint32{'j', 'u', 'l'}, kwMonthName, 7},
	{[3]uint32{'a', 'u', 'g'}, kwMonthName, 8},
	{[3]uint32{'s', 'e', 'p'}, kwMonthName, 9},
	{[3]uint32{'o', 'c', 't'}, kwMonthName, 10},
	{[3]uint32{'n', 'o', 'v'}, kwMonthName, 11},
	{[3]uint32{'d', 'e', 'c'}, kwMonthName, 12},
	{[3]uint32{'a', 'm', 0}, kwAMPM, 0},
	{[3]uint32{'p', 'm', 0}, kwAMPM, 12},
	{[3]uint32{'u', 't', 0}, kwTimeZoneName, 0},
	{[3]uint32{'u', 't', 'c'}, kwTimeZoneName, 0},
	{[3]uint32{'z', 0, 0}, kwTimeZoneName, 0},
	{[3]uint32{'g', 'm', 't'}, kwTimeZoneName, 0},
	{[3]uint32{'c', 'd', 't'}, kwTimeZoneName, -5},
	{[3]uint32{'c', 's', 't'}, kwTimeZoneName, -6},
	{[3]uint32{'e', 'd', 't'}, kwTimeZoneName, -4},
	{[3]uint32{'e', 's', 't'}, kwTimeZoneName, -5},
	{[3]uint32{'m', 'd', 't'}, kwTimeZoneName, -6},
	{[3]uint32{'m', 's', 't'}, kwTimeZoneName, -7},
	{[3]uint32{'p', 'd', 't'}, kwTimeZoneName, -7},
	{[3]uint32{'p', 's', 't'}, kwTimeZoneName, -8},
	{[3]uint32{'t', 0, 0}, kwTimeSeparator, 0},
}

// lookupKeyword returns the type and value of a word; a word longer than three letters only
// matches a month name.
func lookupKeyword(prefix [3]uint32, length int) (keywordType, int) {
	for _, k := range keywords {
		if k.prefix == prefix && (length <= 3 || k.kind == kwMonthName) {
			return k.kind, k.value
		}
	}
	return kwInvalid, 0
}

// tokenizer is V8's DateStringTokenizer.
type tokenizer struct {
	in   *reader
	peek token
}

func newTokenizer(in *reader) *tokenizer {
	t := &tokenizer{in: in}
	t.peek = t.scan()
	return t
}

func (t *tokenizer) next() token {
	result := t.peek
	t.peek = t.scan()
	return result
}

func (t *tokenizer) skipSymbol(c byte) bool {
	if t.peek.isSymbol(c) {
		t.peek = t.scan()
		return true
	}
	return false
}

func (t *tokenizer) scan() token {
	in := t.in
	pre := in.index
	if in.isEnd() {
		return endOfInputToken
	}
	if in.isDigit() {
		n := in.readUnsignedNumeral()
		return token{tag: tagNumber, length: in.index - pre, value: n}
	}
	for _, c := range [...]uint32{':', '-', '+', '.', ')'} {
		if in.skip(c) {
			return token{tag: tagSymbol, length: 1, value: int(c)}
		}
	}
	if in.isAlphaOrAbove() && !isWhiteSpace(in.ch) {
		var prefix [3]uint32
		length := in.readWord(&prefix)
		kind, value := lookupKeyword(prefix, length)
		return token{tag: int(kind), length: length, value: value}
	}
	if in.skipWhiteSpace() {
		return token{tag: tagWhiteSpace, length: in.index - pre, value: -1}
	}
	if in.skipParentheses() {
		return token{tag: tagUnknown, length: 1, value: -1}
	}
	in.next()
	return token{tag: tagUnknown, length: 1, value: -1}
}

func readMilliseconds(t token) int {
	number, length := t.value, t.length
	if length == 1 {
		number *= 100
	} else if length == 2 {
		number *= 10
	} else if length > 3 {
		if length > maxSignificantDigits {
			length = maxSignificantDigits
		}
		factor := 1
		for ; length > 3; length-- {
			factor *= 10
		}
		number /= factor
	}
	return number
}

type tzComposer struct{ sign, hour, minute int }

func (tz *tzComposer) set(offsetInHours int) {
	tz.sign = 1
	if offsetInHours < 0 {
		tz.sign = -1
	}
	tz.hour = offsetInHours * tz.sign
	tz.minute = 0
}

func (tz *tzComposer) setSign(sign int) {
	tz.sign = 1
	if sign < 0 {
		tz.sign = -1
	}
}

func (tz *tzComposer) isExpecting(n int) bool {
	return tz.hour != none && tz.minute == none && between(n, 0, 59)
}
func (tz *tzComposer) isUTC() bool   { return tz.hour == 0 && tz.minute == 0 }
func (tz *tzComposer) isEmpty() bool { return tz.hour == none }

func (tz *tzComposer) write(out *[outputSize]float64) bool {
	if tz.sign == none {
		out[outUTCOffset] = math.NaN()
		return true
	}
	if tz.hour == none {
		tz.hour = 0
	}
	if tz.minute == none {
		tz.minute = 0
	}
	// V8 adds these as unsigned 32-bit numbers, which wrap, and wants a Smi: 32 bits in
	// Node, which builds without pointer compression.
	total := uint32(tz.hour)*3600 + uint32(tz.minute)*60
	if total > math.MaxInt32 {
		return false
	}
	seconds := int(total)
	if tz.sign < 0 {
		seconds = -seconds
	}
	out[outUTCOffset] = float64(seconds)
	return true
}

type timeComposer struct {
	comp       [4]int
	index      int
	hourOffset int
}

func (tc *timeComposer) isEmpty() bool { return tc.index == 0 }
func (tc *timeComposer) isExpecting(n int) bool {
	return (tc.index == 1 && between(n, 0, 59)) || (tc.index == 2 && between(n, 0, 59)) ||
		(tc.index == 3 && between(n, 0, 999))
}

func (tc *timeComposer) add(n int) bool {
	if tc.index < len(tc.comp) {
		tc.comp[tc.index] = n
		tc.index++
		return true
	}
	return false
}

func (tc *timeComposer) addFinal(n int) bool {
	if !tc.add(n) {
		return false
	}
	for tc.index < len(tc.comp) {
		tc.comp[tc.index] = 0
		tc.index++
	}
	return true
}

func (tc *timeComposer) write(out *[outputSize]float64) bool {
	for tc.index < len(tc.comp) {
		tc.comp[tc.index] = 0
		tc.index++
	}
	hour, minute, second, ms := tc.comp[0], tc.comp[1], tc.comp[2], tc.comp[3]
	if tc.hourOffset != none {
		if !between(hour, 0, 12) {
			return false
		}
		hour %= 12
		hour += tc.hourOffset
	}
	if !between(hour, 0, 23) || !between(minute, 0, 59) || !between(second, 0, 59) || !between(ms, 0, 999) {
		if hour != 24 || minute != 0 || second != 0 || ms != 0 {
			return false
		}
	}
	out[outHour], out[outMinute], out[outSecond], out[outMillisecond] = float64(hour), float64(minute), float64(second), float64(ms)
	return true
}

type dayComposer struct {
	comp       [3]int
	index      int
	namedMonth int
	isoDate    bool
}

func (dc *dayComposer) isEmpty() bool { return dc.index == 0 }

func (dc *dayComposer) add(n int) bool {
	if dc.index < len(dc.comp) {
		dc.comp[dc.index] = n
		dc.index++
		return true
	}
	return false
}

func isDay(x int) bool { return between(x, 1, 31) }

func (dc *dayComposer) write(out *[outputSize]float64) bool {
	if dc.index < 1 {
		return false
	}
	for dc.index < len(dc.comp) {
		dc.comp[dc.index] = 1
		dc.index++
	}
	year, month, day := 0, none, none
	if dc.namedMonth == none {
		if dc.isoDate || (dc.index == 3 && !isDay(dc.comp[0])) {
			year, month, day = dc.comp[0], dc.comp[1], dc.comp[2]
		} else {
			month, day = dc.comp[0], dc.comp[1]
			if dc.index == 3 {
				year = dc.comp[2]
			}
		}
	} else {
		month = dc.namedMonth
		if dc.index == 1 {
			day = dc.comp[0]
		} else if !isDay(dc.comp[0]) {
			year, day = dc.comp[0], dc.comp[1]
		} else {
			day, year = dc.comp[0], dc.comp[1]
		}
	}
	if !dc.isoDate {
		if between(year, 0, 49) {
			year += 2000
		} else if between(year, 50, 99) {
			year += 1900
		}
	}
	if year < math.MinInt32 || year > math.MaxInt32 || !between(month, 1, 12) || !isDay(day) {
		return false
	}
	out[outYear], out[outMonth], out[outDay] = float64(year), float64(month-1), float64(day)
	return true
}

func parseDate(src []uint16, out *[outputSize]float64) bool {
	scanner := newTokenizer(newReader(src))
	tz := tzComposer{sign: none, hour: none, minute: none}
	tc := timeComposer{hourOffset: none}
	day := dayComposer{namedMonth: none}

	next := parseES5DateTime(scanner, &day, &tc, &tz)
	if next.isInvalid() {
		return false
	}
	hasReadNumber := !day.isEmpty()
	for tok := next; !tok.isEndOfInput(); tok = scanner.next() {
		switch {
		case tok.isNumber():
			hasReadNumber = true
			n := tok.value
			if scanner.skipSymbol(':') {
				if scanner.skipSymbol(':') {
					if !tc.isEmpty() {
						return false
					}
					tc.add(n)
					tc.add(0)
				} else {
					if !tc.add(n) {
						return false
					}
					if scanner.peek.isSymbol('.') {
						scanner.next()
					}
				}
			} else if scanner.skipSymbol('.') && tc.isExpecting(n) {
				tc.add(n)
				if !scanner.peek.isNumber() {
					return false
				}
				ms := readMilliseconds(scanner.next())
				if ms < 0 {
					return false
				}
				tc.addFinal(ms)
			} else if tz.isExpecting(n) {
				tz.minute = n
			} else if tc.isExpecting(n) {
				tc.addFinal(n)
				// The time must end here: the end, white space, "Z", "+" or "-".
				peek := scanner.peek
				if !peek.isEndOfInput() && !peek.isWhiteSpace() && !peek.isKeywordZ() && !peek.isASCIISign() {
					return false
				}
			} else {
				if !day.add(n) {
					return false
				}
				scanner.skipSymbol('-')
			}
		case tok.isKeyword():
			kind, value := keywordType(tok.tag), tok.value
			if kind == kwAMPM && !tc.isEmpty() {
				tc.hourOffset = value
			} else if kind == kwMonthName {
				day.namedMonth = value
				scanner.skipSymbol('-')
			} else if kind == kwTimeZoneName && hasReadNumber {
				tz.set(value)
			} else {
				// Garbage words are only allowed before the first number, and apart from it.
				if hasReadNumber || scanner.peek.isNumber() {
					return false
				}
			}
		case tok.isASCIISign() && (tz.isUTC() || !tc.isEmpty()):
			tz.setSign(tok.asciiSign())
			n, length := 0, 0
			if scanner.peek.isNumber() {
				t := scanner.next()
				length, n = t.length, t.value
			}
			hasReadNumber = true
			switch {
			case scanner.peek.isSymbol(':'):
				tz.hour, tz.minute = n, none
			case length == 2 || length == 1:
				tz.hour, tz.minute = n, 0
			case length == 4 || length == 3:
				tz.hour, tz.minute = n/100, n%100
			default:
				return false
			}
		case (tok.isASCIISign() || tok.isSymbol(')')) && hasReadNumber:
			return false
		}
	}
	return day.write(out) && tc.write(out) && tz.write(out)
}

// parseES5DateTime reads as much of an ES5 date-time string as there is. It returns the
// token the legacy parser goes on from, the end of input when the string was one, or an
// invalid token when it cannot be a date.
func parseES5DateTime(scanner *tokenizer, day *dayComposer, tc *timeComposer, tz *tzComposer) token {
	if scanner.peek.isASCIISign() {
		sign := scanner.next()
		if !scanner.peek.isFixedLengthNumber(6) {
			return sign
		}
		year := scanner.next().value
		if sign.asciiSign() < 0 && year == 0 {
			return sign
		}
		day.add(sign.asciiSign() * year)
	} else if scanner.peek.isFixedLengthNumber(4) {
		day.add(scanner.next().value)
	} else {
		return scanner.next()
	}
	if scanner.skipSymbol('-') {
		if !scanner.peek.isFixedLengthNumber(2) || !between(scanner.peek.value, 1, 12) {
			return scanner.next()
		}
		day.add(scanner.next().value)
		if scanner.skipSymbol('-') {
			if !scanner.peek.isFixedLengthNumber(2) || !isDay(scanner.peek.value) {
				return scanner.next()
			}
			day.add(scanner.next().value)
		}
	}
	if !scanner.peek.isKeywordType(kwTimeSeparator) {
		if !scanner.peek.isEndOfInput() {
			return scanner.next()
		}
	} else {
		scanner.next()
		if !scanner.peek.isFixedLengthNumber(2) || !between(scanner.peek.value, 0, 24) {
			return invalidToken
		}
		// 24:00, 24:00:00 and 24:00:00.000 are midnight at the end of the day.
		hourIs24 := scanner.peek.value == 24
		tc.add(scanner.next().value)
		if !scanner.skipSymbol(':') {
			return invalidToken
		}
		if !scanner.peek.isFixedLengthNumber(2) || !between(scanner.peek.value, 0, 59) || (hourIs24 && scanner.peek.value > 0) {
			return invalidToken
		}
		tc.add(scanner.next().value)
		if scanner.skipSymbol(':') {
			if !scanner.peek.isFixedLengthNumber(2) || !between(scanner.peek.value, 0, 59) || (hourIs24 && scanner.peek.value > 0) {
				return invalidToken
			}
			tc.add(scanner.next().value)
			if scanner.skipSymbol('.') {
				if !scanner.peek.isNumber() || (hourIs24 && scanner.peek.value > 0) {
					return invalidToken
				}
				tc.add(readMilliseconds(scanner.next()))
			}
		}
		if scanner.peek.isKeywordZ() {
			scanner.next()
			tz.set(0)
		} else if scanner.peek.isSymbol('+') || scanner.peek.isSymbol('-') {
			tz.setSign(1)
			if scanner.next().value == '-' {
				tz.setSign(-1)
			}
			if scanner.peek.isFixedLengthNumber(4) {
				hourmin := scanner.next().value
				hour, minute := hourmin/100, hourmin%100
				if !between(hour, 0, 23) || !between(minute, 0, 59) {
					return invalidToken
				}
				tz.hour, tz.minute = hour, minute
			} else {
				if !scanner.peek.isFixedLengthNumber(2) || !between(scanner.peek.value, 0, 23) {
					return invalidToken
				}
				tz.hour = scanner.next().value
				if !scanner.skipSymbol(':') {
					return invalidToken
				}
				if !scanner.peek.isFixedLengthNumber(2) || !between(scanner.peek.value, 0, 59) {
					return invalidToken
				}
				tz.minute = scanner.next().value
			}
		}
		if !scanner.peek.isEndOfInput() {
			return invalidToken
		}
	}
	// Date-only forms without an offset are UTC; date-time forms are local time.
	if tz.isEmpty() && tc.isEmpty() {
		tz.set(0)
	}
	day.isoDate = true
	return endOfInputToken
}
