package gantt

// The parts of dayjs 1.11 that Mermaid's gantt code relies on, with the plugins it loads
// (custom parse format, advanced format, ISO week) and the English locale: strict parsing
// against a format, formatting, and calendar arithmetic, all in local time. Quirks are kept,
// because a date is accepted only when formatting the parsed result gives back the input.

import (
	"math"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsdate"
	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

var monthNames = [...]string{"January", "February", "March", "April", "May", "June", "July", "August", "September",
	"October", "November", "December"}

var weekdayNames = [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

const (
	msDay  = 86400000.0
	msWeek = 7 * msDay
)

// num is String(n).
func num(n float64) string { return jsnum.Format(n) }

func padNum(value float64, length int) string {
	s := num(value)
	if len(s) >= length {
		return s
	}
	return strings.Repeat("0", length-len(s)) + s
}

// ordinal is n with its English suffix, as dayjs's locale writes it: s[(v - 20) % 10] ||
// s[v] || s[0].
func ordinal(n float64) string {
	suffixes := [...]string{"th", "st", "nd", "rd"}
	v := math.Mod(n, 100)
	suffix := suffixes[0]
	if k := math.Mod(v-20, 10); k >= 0 && k <= 3 {
		suffix = suffixes[int(k)]
	} else if v >= 1 && v <= 3 {
		suffix = suffixes[int(v)]
	}
	return num(n) + suffix
}

func utcOffset(date jsdate.Date) float64 {
	offset := -jsmath.Round(date.TimezoneOffset()/15) * 15
	if offset == 0 {
		return 0
	}
	return offset
}

func zone(date jsdate.Date) string {
	offset := utcOffset(date)
	minutes := math.Abs(offset)
	sign := "-"
	if offset >= 0 {
		sign = "+"
	}
	return sign + padNum(math.Floor(minutes/60), 2) + ":" + padNum(math.Mod(minutes, 60), 2)
}

// isoWeekday is date.getDay() || 7.
func isoWeekday(date jsdate.Date) float64 {
	if d := date.Day(); d != 0 && !math.IsNaN(d) {
		return d
	}
	return 7
}

func addDays(date jsdate.Date, days float64) jsdate.Date {
	next := date
	next.SetDate(next.Date() + days)
	return next
}

func daysInMonth(date jsdate.Date) float64 {
	last := jsdate.Local(date.Location(), date.FullYear(), date.Month()+1, 0, 0, 0, 0, 0)
	last.SetHours(23, 59, 59, 999)
	return last.Date()
}

// setCalendar sets the month or the year, keeping the day of the month unless the new month
// is shorter.
func setCalendar(date jsdate.Date, apply func(d *jsdate.Date)) jsdate.Date {
	next := date
	next.SetDate(1)
	apply(&next)
	next.SetDate(math.Min(date.Date(), daysInMonth(next)))
	return next
}

// add adds amount of a dayjs unit to date.
func add(date jsdate.Date, amount float64, unit string) jsdate.Date {
	switch unit {
	case "M":
		return setCalendar(date, func(d *jsdate.Date) { d.SetMonth(date.Month() + amount) })
	case "y":
		return setCalendar(date, func(d *jsdate.Date) { d.SetFullYear(date.FullYear() + amount) })
	case "d":
		return addDays(date, jsmath.Round(amount))
	case "w":
		return addDays(date, jsmath.Round(7*amount))
	case "h":
		return jsdate.New(date.Time()+float64(amount*3600000), date.Location())
	case "m":
		return jsdate.New(date.Time()+float64(amount*60000), date.Location())
	case "s":
		return jsdate.New(date.Time()+float64(amount*1000), date.Location())
	default:
		return jsdate.New(date.Time()+amount, date.Location())
	}
}

func weekThursday(date jsdate.Date) jsdate.Date {
	return addDays(date, 4-isoWeekday(date))
}

func isoWeek(date jsdate.Date) float64 {
	thursday := weekThursday(date)
	jan1 := jsdate.Local(date.Location(), thursday.FullYear(), 0, 1, 0, 0, 0, 0)
	weekday := isoWeekday(jan1)
	shift := 4 - weekday
	if weekday > 4 {
		shift += 7
	}
	first := addDays(jan1, shift)
	zoneDelta := float64((utcOffset(first) - utcOffset(thursday)) * 60000)
	weeks := (thursday.Time() - first.Time() - zoneDelta) / msWeek
	if weeks < 0 {
		weeks = math.Ceil(weeks)
		if weeks == 0 {
			weeks = 0
		}
	} else {
		weeks = math.Floor(weeks)
	}
	return weeks + 1
}

// tokenRule is one alternative of a format regex: an exact string, or a letter repeated up
// to max times.
type tokenRule struct {
	exact  string
	letter byte
	max    int
}

func (r tokenRule) match(s string, i int) int {
	if r.exact != "" {
		if strings.HasPrefix(s[i:], r.exact) {
			return len(r.exact)
		}
		return 0
	}
	n := 0
	for n < r.max && i+n < len(s) && s[i+n] == r.letter {
		n++
	}
	return n
}

func exact(s string) tokenRule       { return tokenRule{exact: s} }
func upTo(c byte, max int) tokenRule { return tokenRule{letter: c, max: max} }
func literalAt(s string, i int) (int, bool) {
	if s[i] != '[' {
		return 0, false
	}
	j := strings.IndexByte(s[i+1:], ']')
	if j <= 0 {
		return 0, false
	}
	return j + 2, true
}

// replaceTokens is s.replace(re, fn) for a regex of the form /\[([^\]]+)]|alternatives/g:
// fn gets each match, and literal is set for a bracketed one. The alternatives are ASCII, so
// the scan can go byte by byte over WTF-8.
func replaceTokens(s string, rules []tokenRule, fn func(match string, literal bool) string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if n, ok := literalAt(s, i); ok {
			b.WriteString(fn(s[i:i+n], true))
			i += n
			continue
		}
		matched := false
		for _, r := range rules {
			if n := r.match(s, i); n > 0 {
				b.WriteString(fn(s[i:i+n], false))
				i += n
				matched = true
				break
			}
		}
		if !matched {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// advancedRules is RE_ADVANCED without its literal: Q|wo|ww|w|WW|W|zzz|z|gggg|GGGG|Do|X|x|k{1,2}|S.
var advancedRules = []tokenRule{exact("Q"), exact("wo"), exact("ww"), exact("w"), exact("WW"), exact("W"), exact("zzz"),
	exact("z"), exact("gggg"), exact("GGGG"), exact("Do"), exact("X"), exact("x"), upTo('k', 2), exact("S")}

// formatRules is RE_FORMAT without its literal:
// YYYY|YY|M{1,4}|D{1,2}|d{1,4}|H{1,2}|h{1,2}|a|A|m{1,2}|s{1,2}|Z{1,2}|SSS.
var formatRules = []tokenRule{exact("YYYY"), exact("YY"), upTo('M', 4), upTo('D', 2), upTo('d', 4), upTo('H', 2),
	upTo('h', 2), exact("a"), exact("A"), upTo('m', 2), upTo('s', 2), upTo('Z', 2), exact("SSS")}

const defaultFormat = "YYYY-MM-DDTHH:mm:ssZ"

// format writes date with a dayjs format. It reports false for the week-of-year and time
// zone name tokens, which need dayjs plugins Mermaid does not load and so fail there.
func format(date jsdate.Date, template string) (string, bool) {
	if !date.Valid() {
		return "Invalid Date", true
	}
	str := template
	if str == "" {
		str = defaultFormat
	}
	if str == "YYYY-MM-DD" {
		return isoDate(date), true
	}
	unsupported := false
	advanced := replaceTokens(str, advancedRules, func(match string, _ bool) string {
		switch match {
		case "Q":
			return num(math.Ceil((date.Month() + 1) / 3))
		case "Do":
			return "[" + ordinal(date.Date()) + "]"
		case "GGGG":
			return num(weekThursday(date).FullYear())
		case "W":
			return num(isoWeek(date))
		case "WW":
			return padNum(isoWeek(date), 2)
		case "k":
			return num(hours24(date))
		case "kk":
			return padNum(hours24(date), 2)
		case "X":
			return num(math.Floor(date.Time() / 1000))
		case "x":
			return num(date.Time())
		case "gggg", "wo", "w", "ww", "z", "zzz":
			unsupported = true
		}
		return match
	})
	if unsupported {
		return "", false
	}
	hours := date.Hours()
	return replaceTokens(advanced, formatRules, func(match string, literal bool) string {
		if literal {
			return match[1 : len(match)-1]
		}
		switch match {
		case "YY":
			y := num(date.FullYear())
			return y[max(0, len(y)-2):]
		case "YYYY":
			return padNum(date.FullYear(), 4)
		case "M":
			return num(date.Month() + 1)
		case "MM":
			return padNum(date.Month()+1, 2)
		case "MMM":
			return monthNames[int(date.Month())][:3]
		case "MMMM":
			return monthNames[int(date.Month())]
		case "D":
			return num(date.Date())
		case "DD":
			return padNum(date.Date(), 2)
		case "d":
			return num(date.Day())
		case "dd":
			return weekdayNames[int(date.Day())][:2]
		case "ddd":
			return weekdayNames[int(date.Day())][:3]
		case "dddd":
			return weekdayNames[int(date.Day())]
		case "H":
			return num(hours)
		case "HH":
			return padNum(hours, 2)
		case "h":
			return num(hours12(hours))
		case "hh":
			return padNum(hours12(hours), 2)
		case "a":
			if hours < 12 {
				return "am"
			}
			return "pm"
		case "A":
			if hours < 12 {
				return "AM"
			}
			return "PM"
		case "m":
			return num(date.Minutes())
		case "mm":
			return padNum(date.Minutes(), 2)
		case "s":
			return num(date.Seconds())
		case "ss":
			return padNum(date.Seconds(), 2)
		case "SSS":
			return padNum(date.Milliseconds(), 3)
		case "Z":
			return zone(date)
		default:
			return strings.Replace(zone(date), ":", "", 1)
		}
	}), true
}

// hours24 is getHours() || 24.
func hours24(date jsdate.Date) float64 {
	if h := date.Hours(); h != 0 {
		return h
	}
	return 24
}

// hours12 is hours % 12 || 12.
func hours12(hours float64) float64 {
	if h := math.Mod(hours, 12); h != 0 {
		return h
	}
	return 12
}

func isoDate(date jsdate.Date) string {
	year, month, day := date.FullYear(), date.Month()+1, date.Date()
	s := num(year)
	if !(year >= 1000 && year <= 9999) {
		s = padNum(year, 4)
	}
	if month < 10 {
		s += "-0"
	} else {
		s += "-"
	}
	s += num(month)
	if day < 10 {
		s += "-0"
	} else {
		s += "-"
	}
	return s + num(day)
}

// opt is a number that may be undefined.
type opt struct {
	v   float64
	set bool
}

func some(v float64) opt { return opt{v: v, set: true} }

// truthy is the value as a JS condition sees it.
func (o opt) truthy() bool { return o.set && o.v != 0 && !math.IsNaN(o.v) }

// or is o || def.
func (o opt) or(def float64) float64 {
	if o.truthy() {
		return o.v
	}
	return def
}

type fields struct {
	year, month, day, hours, minutes, seconds, milliseconds, week, offset opt
	dayGiven                                                              bool
	afternoon                                                             *bool
}

// match finds a reader's pattern in a string, unanchored, and returns its span or -1.
type match func(u []uint16) (int, int)

type readerStep struct {
	match match
	read  func(f *fields, input string) bool
}

func isDigitUnit(u []uint16, i int) bool { return i < len(u) && u[i] >= '0' && u[i] <= '9' }

// digitsN is /\d{n}/.
func digitsN(n int) match {
	return func(u []uint16) (int, int) {
		run := 0
		for i := range u {
			if isDigitUnit(u, i) {
				run++
				if run == n {
					return i + 1 - n, i + 1
				}
			} else {
				run = 0
			}
		}
		return -1, -1
	}
}

// oneOrTwoDigits is /\d\d?/.
func oneOrTwoDigits(u []uint16) (int, int) {
	for i := range u {
		if isDigitUnit(u, i) {
			if isDigitUnit(u, i+1) {
				return i, i + 2
			}
			return i, i + 1
		}
	}
	return -1, -1
}

// signedDigits is /[+-]?\d+/.
func signedDigits(u []uint16) (int, int) {
	for i := range u {
		start := i
		j := i
		if (u[j] == '+' || u[j] == '-') && isDigitUnit(u, j+1) {
			j++
		}
		if isDigitUnit(u, j) {
			for isDigitUnit(u, j) {
				j++
			}
			return start, j
		}
	}
	return -1, -1
}

// offsetPattern is /[+-]\d\d:?(\d\d)?|Z/.
func offsetPattern(u []uint16) (int, int) {
	for i := range u {
		if (u[i] == '+' || u[i] == '-') && isDigitUnit(u, i+1) && isDigitUnit(u, i+2) {
			e := i + 3
			if e < len(u) && u[e] == ':' {
				e++
			}
			if isDigitUnit(u, e) && isDigitUnit(u, e+1) {
				e += 2
			}
			return i, e
		}
		if u[i] == 'Z' {
			return i, i + 1
		}
	}
	return -1, -1
}

// wordPattern is /\d*[^-_:/,()\s\d]+/.
func wordPattern(u []uint16) (int, int) {
	inWord := func(c uint16) bool {
		return !strings.ContainsRune("-_:/,()", rune(c)) && !util.IsSpace(int(c)) && !(c >= '0' && c <= '9')
	}
	for i := 0; i < len(u); {
		j := i
		for isDigitUnit(u, j) {
			j++
		}
		e := j
		for e < len(u) && inWord(u[e]) {
			e++
		}
		if e > j {
			return i, e
		}
		// No start within the digits can match either.
		i = j + 1
	}
	return -1, -1
}

func setField(name func(f *fields) *opt) func(f *fields, input string) bool {
	return func(f *fields, input string) bool {
		*name(f) = some(jsnum.Parse(input))
		return true
	}
}

func monthName(names []string) func(f *fields, input string) bool {
	return func(f *fields, input string) bool {
		index := 0
		for i, name := range names {
			if name == input {
				index = i + 1
				break
			}
		}
		if index < 1 {
			return false
		}
		m := index % 12
		if m == 0 {
			m = index
		}
		f.month = some(float64(m))
		return true
	}
}

var shortMonthNames = func() []string {
	out := make([]string, len(monthNames))
	for i, m := range monthNames {
		out[i] = m[:3]
	}
	return out
}()

func secondsField(f *fields) *opt { return &f.seconds }
func minutesField(f *fields) *opt { return &f.minutes }
func hoursField(f *fields) *opt   { return &f.hours }
func dayField(f *fields) *opt     { return &f.day }
func weekField(f *fields) *opt    { return &f.week }
func monthField(f *fields) *opt   { return &f.month }
func yearField(f *fields) *opt    { return &f.year }

func readOffset(f *fields, input string) bool {
	if input == "Z" {
		f.offset = some(0)
		return true
	}
	// input.match(/([+-]|\d\d)/g): the sign, then pairs of digits.
	sign := input[0]
	var pairs []float64
	for i := 1; i+1 < len(input); {
		if input[i] >= '0' && input[i] <= '9' && input[i+1] >= '0' && input[i+1] <= '9' {
			pairs = append(pairs, jsnum.Parse(input[i:i+2]))
			i += 2
		} else {
			i++
		}
	}
	minutes := float64(pairs[0] * 60)
	if len(pairs) > 1 && pairs[1] != 0 {
		minutes += pairs[1]
	}
	switch {
	case minutes == 0:
		f.offset = some(0)
	case sign == '+':
		f.offset = some(-minutes)
	default:
		f.offset = some(minutes)
	}
	return true
}

var readers = map[string]readerStep{
	"A": {wordPattern, func(f *fields, input string) bool { pm := input == "PM"; f.afternoon = &pm; return true }},
	"a": {wordPattern, func(f *fields, input string) bool { pm := input == "pm"; f.afternoon = &pm; return true }},
	"Q": {digitsN(1), func(f *fields, input string) bool {
		f.month = some(float64((jsnum.Parse(input)-1)*3) + 1)
		return true
	}},
	"S":   {digitsN(1), func(f *fields, input string) bool { f.milliseconds = some(jsnum.Parse(input) * 100); return true }},
	"SS":  {digitsN(2), func(f *fields, input string) bool { f.milliseconds = some(jsnum.Parse(input) * 10); return true }},
	"SSS": {digitsN(3), func(f *fields, input string) bool { f.milliseconds = some(jsnum.Parse(input)); return true }},
	"s":   {oneOrTwoDigits, setField(secondsField)},
	"ss":  {oneOrTwoDigits, setField(secondsField)},
	"m":   {oneOrTwoDigits, setField(minutesField)},
	"mm":  {oneOrTwoDigits, setField(minutesField)},
	"H":   {oneOrTwoDigits, setField(hoursField)},
	"h":   {oneOrTwoDigits, setField(hoursField)},
	"HH":  {oneOrTwoDigits, setField(hoursField)},
	"hh":  {oneOrTwoDigits, setField(hoursField)},
	"D":   {oneOrTwoDigits, setField(dayField)},
	"DD":  {digitsN(2), setField(dayField)},
	"Do": {wordPattern, func(f *fields, input string) bool {
		i := 0
		for i < len(input) && input[i] >= '0' && input[i] <= '9' {
			i++
		}
		if i == 0 {
			return false
		}
		// dayjs keeps the digits as a string here, so even "0" counts as a day having been given.
		f.day = some(jsnum.Parse(input[:i]))
		f.dayGiven = true
		for d := 1.0; d <= 31; d++ {
			if ordinal(d) == input {
				f.day = some(d)
			}
		}
		return true
	}},
	"w":    {oneOrTwoDigits, setField(weekField)},
	"ww":   {digitsN(2), setField(weekField)},
	"M":    {oneOrTwoDigits, setField(monthField)},
	"MM":   {digitsN(2), setField(monthField)},
	"MMM":  {wordPattern, monthName(shortMonthNames)},
	"MMMM": {wordPattern, monthName(monthNames[:])},
	"Y":    {signedDigits, setField(yearField)},
	"YY": {digitsN(2), func(f *fields, input string) bool {
		v := jsnum.Parse(input)
		if v > 68 {
			f.year = some(v + 1900)
		} else {
			f.year = some(v + 2000)
		}
		return true
	}},
	"YYYY": {digitsN(4), setField(yearField)},
	"Z":    {offsetPattern, readOffset},
	"ZZ":   {offsetPattern, readOffset},
}

// step is a compiled format token: a reader, or literal text the parser steps over.
type step struct {
	reader  *readerStep
	literal int
}

// compile turns a format into steps, or reports false when dayjs cannot parse with it.
func compile(template string) ([]step, bool) {
	u := jsstr.UTF16(template)
	// dayjs expands localized tokens through a locale table the English locale does not
	// have, and fails.
	for i := 0; i < len(u); i++ {
		if u[i] == '[' {
			if j := indexUnit(u, ']', i+1); j > i+1 {
				i = j
				continue
			}
		}
		if u[i] == 'L' || u[i] == 'l' {
			return nil, false
		}
	}
	tokens := matchTokens(u)
	if len(tokens) == 0 {
		return nil, false
	}
	steps := make([]step, len(tokens))
	for i, token := range tokens {
		s := jsstr.FromUTF16(token)
		if r, ok := readers[s]; ok {
			steps[i] = step{reader: &r}
			continue
		}
		n := len(token)
		if n > 0 && token[0] == '[' {
			n--
		}
		if n > 0 && token[len(token)-1] == ']' && len(token) > 1 {
			n--
		}
		steps[i] = step{literal: n}
	}
	return steps, true
}

func indexUnit(u []uint16, c uint16, from int) int {
	for i := from; i < len(u); i++ {
		if u[i] == c {
			return i
		}
	}
	return -1
}

func isSeparator(c uint16) bool {
	return strings.ContainsRune("-_:/.,()", rune(c)) || util.IsSpace(int(c))
}

// tokenAlternatives is the last group of RE_TOKENS, in order:
// A|a|Q|YYYY|YY?|ww?|MM?M?M?|Do|DD?|hh?|HH?|mm?|ss?|S{1,3}|z|ZZ?.
var tokenAlternatives = []tokenRule{exact("A"), exact("a"), exact("Q"), exact("YYYY"), upTo('Y', 2), upTo('w', 2),
	upTo('M', 4), exact("Do"), upTo('D', 2), upTo('h', 2), upTo('H', 2), upTo('m', 2), upTo('s', 2), upTo('S', 3),
	exact("z"), upTo('Z', 2)}

// matchTokens is template.match(RE_TOKENS), with RE_TOKENS
// /(\[[^[]*\])|([-_:/.,()\s]+)|(alternatives)/g.
func matchTokens(u []uint16) [][]uint16 {
	var out [][]uint16
	ascii := func(i int) string {
		// The alternatives are ASCII; anything else stands in as a byte none of them has.
		var b strings.Builder
		for k := i; k < len(u) && k < i+4; k++ {
			if u[k] < 0x80 {
				b.WriteByte(byte(u[k]))
			} else {
				b.WriteByte(0)
			}
		}
		return b.String()
	}
	for i := 0; i < len(u); {
		if u[i] == '[' {
			// [^[]* runs to the next [, then gives back characters until a ] ends the match.
			end := len(u)
			if j := indexUnit(u, '[', i+1); j >= 0 {
				end = j
			}
			last := -1
			for k := end - 1; k > i; k-- {
				if u[k] == ']' {
					last = k
					break
				}
			}
			if last > 0 {
				out = append(out, u[i:last+1])
				i = last + 1
				continue
			}
		}
		if isSeparator(u[i]) {
			j := i
			for j < len(u) && isSeparator(u[j]) {
				j++
			}
			out = append(out, u[i:j])
			i = j
			continue
		}
		head := ascii(i)
		matched := false
		for _, r := range tokenAlternatives {
			if n := r.match(head, 0); n > 0 {
				out = append(out, u[i:i+n])
				i += n
				matched = true
				break
			}
		}
		if !matched {
			i++
		}
	}
	return out
}

func loose(input, template string, now jsdate.Date) (jsdate.Date, bool) {
	loc := now.Location()
	if template == "x" || template == "X" {
		scale := 1.0
		if template == "X" {
			scale = 1000
		}
		return jsdate.New(scale*jsnum.Parse(input), loc), true
	}
	steps, ok := compile(template)
	if !ok {
		return jsdate.Date{}, false
	}
	var f fields
	rest := jsstr.UTF16(input)
	start := 0
	for _, s := range steps {
		if s.reader == nil {
			start += s.literal
			continue
		}
		sub := rest[min(start, len(rest)):]
		ms, me := s.reader.match(sub)
		if ms < 0 {
			return jsdate.Date{}, false
		}
		text := sub[ms:me]
		if !s.reader.read(&f, jsstr.FromUTF16(text)) {
			return jsdate.Date{}, false
		}
		// dayjs drops the first occurrence of the matched text, wherever it is, and does not
		// move start.
		rest = removeFirst(rest, text)
	}
	if f.afternoon != nil && f.hours.set {
		if *f.afternoon {
			if f.hours.v < 12 {
				f.hours.v += 12
			}
		} else if f.hours.v == 12 {
			f.hours.v = 0
		}
	}
	var day float64
	switch {
	case f.dayGiven || f.day.truthy():
		day = f.day.v
	case !f.year.truthy() && !f.month.truthy():
		day = now.Date()
	default:
		day = 1
	}
	year := f.year.or(now.FullYear())
	month := 0.0
	if !(f.year.truthy() && !f.month.truthy()) {
		if f.month.set && f.month.v > 0 {
			month = f.month.v - 1
		} else {
			month = now.Month()
		}
	}
	hours := f.hours.or(0)
	minutes := f.minutes.or(0)
	seconds := f.seconds.or(0)
	ms := f.milliseconds.or(0)
	if f.offset.set {
		return jsdate.New(jsdate.UTC(year, month, day, hours, minutes, seconds, ms+float64(f.offset.v*60000)), loc), true
	}
	// Applying a week number needs a dayjs plugin Mermaid does not load.
	if f.week.truthy() {
		return jsdate.Date{}, false
	}
	return jsdate.Local(loc, year, month, day, hours, minutes, seconds, ms), true
}

func removeFirst(u, part []uint16) []uint16 {
	for i := 0; i+len(part) <= len(u); i++ {
		if equalUnits(u[i:i+len(part)], part) {
			out := make([]uint16, 0, len(u)-len(part))
			out = append(out, u[:i]...)
			return append(out, u[i+len(part):]...)
		}
	}
	return u
}

func equalUnits(a, b []uint16) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// parseStrict parses the way dayjs(input, format, true) does. now supplies the fields the
// format leaves out, and the time zone.
func parseStrict(input, template string, now jsdate.Date) (jsdate.Date, bool) {
	date, ok := loose(input, template, now)
	if !ok || !date.Valid() {
		return jsdate.Date{}, false
	}
	if formatted, ok := format(date, template); !ok || formatted != input {
		return jsdate.Date{}, false
	}
	return date, true
}
