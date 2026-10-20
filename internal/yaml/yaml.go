// Package yaml is upstream's small YAML reader (src/util/yaml.ts): the subset Mermaid reads
// from front matter with js-yaml's JSON schema. Values are jsobj values.
package yaml

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

const maxDepth = 64

// failure is thrown, as a panic, for input upstream rejects. Its message is upstream's.
type failure struct{ message string }

// incomplete is thrown when a quoted scalar or flow collection runs past the text read so far.
type incomplete struct{ message string }

func fail(message string) { panic(failure{message}) }

// Parse reads src. The error message is the one upstream throws, without its error name.
func Parse(src string) (value any, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r := r.(type) {
			case failure:
				err = fmt.Errorf("%s", r.message)
			case incomplete:
				err = fmt.Errorf("%s", r.message)
			default:
				panic(r)
			}
		}
	}()
	return parse(src), nil
}

func parse(src string) any {
	trimmed := jsstr.Trim(src)
	if trimmed != "" && (trimmed[0] == '{' || trimmed[0] == '[') {
		if v, end, ok := tryFlow(trimmed); ok && skip(trimmed, end, false) == len(trimmed) {
			return v
		}
	}
	r := newReader(src)
	value := block(r, 0, 0)
	r.skipBlank()
	if r.i < len(r.lines) && jsstr.Trim(r.lines[r.i]) != "..." {
		fail("YAML: end of the stream or a document separator is expected")
	}
	return value
}

// tryFlow reads a whole-document flow value. An incomplete one is an error; any other
// failure is thrown as it is.
func tryFlow(s string) (v any, end int, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if inc, isInc := r.(incomplete); isInc {
				fail(inc.message)
			}
			panic(r)
		}
	}()
	v, end = flow(s, 0, "", 0)
	return v, end, true
}

type reader struct {
	lines []string
	i     int
}

var reNewline = regexp.MustCompile(`\r\n?`)

func newReader(src string) *reader {
	return &reader{lines: strings.Split(reNewline.ReplaceAllString(src, "\n"), "\n")}
}

func (r *reader) skipBlank() {
	for r.i < len(r.lines) {
		t := jsstr.Trim(r.lines[r.i])
		if t != "" && t[0] != '#' && t != "---" {
			break
		}
		r.i++
	}
}

func (r *reader) indent() int {
	line := r.lines[r.i]
	k := 0
	for k < len(line) && line[k] == ' ' {
		k++
	}
	if k < len(line) && line[k] == '\t' {
		fail("YAML: tabs are not allowed for indentation")
	}
	return k
}

// leading returns the number of UTF-16 code units of JS whitespace that start s, as
// s.length - s.trimStart().length does.
func leading(s string) int {
	return jsstr.Len(s) - jsstr.Len(jsstr.TrimStart(s))
}

// byteAt returns s[i], or -1 past the end, for comparisons against ASCII.
func byteAt(s string, i int) int {
	if i < 0 || i >= len(s) {
		return -1
	}
	return int(s[i])
}

var (
	reInt   = regexp.MustCompile(`^[-+]?(?:0|[1-9][0-9]*)$`)
	reHex   = regexp.MustCompile(`^[-+]?0x[0-9a-fA-F]+$`)
	reOct   = regexp.MustCompile(`^[-+]?0o[0-7]+$`)
	reBin   = regexp.MustCompile(`^[-+]?0b[01]+$`)
	reFloat = regexp.MustCompile(`^[-+]?(?:[0-9][0-9]*(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][-+]?[0-9]+)?$`)
	reInf   = regexp.MustCompile(`^[-+]?\.(?:inf|Inf|INF)$`)
	reNaN   = regexp.MustCompile(`^\.(?:nan|NaN|NAN)$`)
)

func isKeyStart(r rune) bool {
	return jsstr.IsSpace(r) || strings.ContainsRune("#:,[]{}&*!|>'\"%@`", r)
}

func isBlank(c int) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// splitKey splits `key: rest` at the start of a line into the raw key and the offset where
// the value starts.
func splitKey(line string) (key string, length int, ok bool) {
	first := byteAt(line, 0)
	var end, colon int
	if first == '"' || first == '\'' {
		i := 1
		for {
			if i >= len(line) {
				return "", 0, false
			}
			c := int(line[i])
			if first == '"' && c == '\\' {
				i += 2
			} else if c == first {
				if first == '"' || byteAt(line, i+1) != '\'' {
					break
				}
				i += 2
			} else {
				i++
			}
		}
		end = i + 1
		colon = end
		for isBlank(byteAt(line, colon)) {
			colon++
		}
		if byteAt(line, colon) != ':' {
			return "", 0, false
		}
	} else {
		if first == -1 {
			return "", 0, false
		}
		if r, _ := jsstr.Decode(line, 0); isKeyStart(r) {
			return "", 0, false
		}
		if first == '-' && (len(line) < 2 || isBlank(byteAt(line, 1)) || line[1] == ':' || line[1] == '#') {
			return "", 0, false
		}
		colon = 1
		for colon < len(line) && line[colon] != ':' && line[colon] != '#' {
			colon++
		}
		if byteAt(line, colon) != ':' {
			return "", 0, false
		}
		end = colon
		for isBlank(byteAt(line, end-1)) {
			end--
		}
	}
	length = colon + 1
	if length < len(line) && !isBlank(byteAt(line, length)) {
		return "", 0, false
	}
	for isBlank(byteAt(line, length)) {
		length++
	}
	return line[:end], length, true
}

func scalar(raw string) any {
	s := jsstr.Trim(raw)
	switch s {
	case "", "~", "null", "Null", "NULL":
		return nil
	case "true", "True", "TRUE":
		return true
	case "false", "False", "FALSE":
		return false
	}
	if reInt.MatchString(s) || reFloat.MatchString(s) {
		return jsnum.Parse(s)
	}
	if reHex.MatchString(s) || reOct.MatchString(s) || reBin.MatchString(s) {
		neg := s[0] == '-'
		body := s
		if neg || s[0] == '+' {
			body = s[1:]
		}
		v := jsnum.Parse(body)
		if neg {
			return -v
		}
		return v
	}
	if reInf.MatchString(s) {
		return jsnum.Parse(map[bool]string{true: "-Infinity", false: "Infinity"}[s[0] == '-'])
	}
	if reNaN.MatchString(s) {
		return jsnum.Parse("NaN")
	}
	return s
}

var escapes = map[uint16]string{
	'0': "\x00", 'a': "\x07", 'b': "\b", 't': "\t", 'n': "\n", 'v': "\v", 'f': "\f", 'r': "\r", 'e': "\x1b",
	' ': " ", '"': "\"", '/': "/", '\\': "\\", 'N': "\u0085", '_': "\u00a0", 'L': "\u2028", 'P': "\u2029",
}

var reBreaks = regexp.MustCompile(`\n+`)

// fold folds the line breaks of a scalar: blanks around a break go, one break becomes a
// space, and each further break stays.
func fold(text string) string {
	if !strings.Contains(text, "\n") {
		return text
	}
	lines := strings.Split(text, "\n")
	last := len(lines) - 1
	for i, line := range lines {
		from, to := 0, len(line)
		if i > 0 {
			for from < to && (line[from] == ' ' || line[from] == '\t') {
				from++
			}
		}
		if i < last {
			for to > from && (line[to-1] == ' ' || line[to-1] == '\t') {
				to--
			}
		}
		lines[i] = line[from:to]
	}
	return reBreaks.ReplaceAllStringFunc(strings.Join(lines, "\n"), func(m string) string {
		if len(m) == 1 {
			return " "
		}
		return strings.Repeat("\n", len(m)-1)
	})
}

var reContinuation = regexp.MustCompile(`\\\n[ \t]*`)

func unquoteDouble(body string) string {
	// Escapes are read by UTF-16 code unit: \x, \u and \U take that many units after them.
	s := jsstr.UTF16(fold(reContinuation.ReplaceAllString(body, "")))
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			start := i
			for i+1 < len(s) && s[i+1] != '\\' {
				i++
			}
			out.WriteString(jsstr.FromUTF16(s[start : i+1]))
			continue
		}
		i++
		var e uint16
		if i < len(s) {
			e = s[i]
		}
		if i < len(s) && (e == 'x' || e == 'u' || e == 'U') {
			n := map[uint16]int{'x': 2, 'u': 4, 'U': 8}[e]
			digits := s[i+1 : min(len(s), i+1+n)]
			out.WriteString(fromCodePoint(parseHexPrefix(digits)))
			i += n
		} else if esc, ok := escapes[e]; ok && i < len(s) {
			out.WriteString(esc)
		} else {
			fail("YAML: unknown escape sequence")
		}
	}
	return out.String()
}

// parseHexPrefix is parseInt(s, 16): the hex digits at the start of s, or NaN as -1.
func parseHexPrefix(units []uint16) float64 {
	text := jsstr.TrimStart(jsstr.FromUTF16(units))
	sign := 1.0
	if text != "" && (text[0] == '+' || text[0] == '-') {
		if text[0] == '-' {
			sign = -1
		}
		text = text[1:]
	}
	if len(text) >= 2 && text[0] == '0' && (text[1] == 'x' || text[1] == 'X') {
		text = text[2:]
	}
	end := 0
	for end < len(text) && strings.IndexByte("0123456789abcdefABCDEF", text[end]) >= 0 {
		end++
	}
	if end == 0 {
		return jsnum.Parse("NaN")
	}
	return sign * jsnum.Parse("0x"+text[:end])
}

// fromCodePoint is String.fromCodePoint, which throws a RangeError for anything but an
// integer code point.
func fromCodePoint(n float64) string {
	if n != n || n < 0 || n > 0x10ffff || n != float64(int64(n)) {
		fail("Invalid code point " + jsnum.Format(n))
	}
	return jsstr.FromUTF16(utf16Of(rune(n)))
}

func utf16Of(r rune) []uint16 {
	if r >= 0x10000 {
		r -= 0x10000
		return []uint16{uint16(0xd800 + r>>10), uint16(0xdc00 + r&0x3ff)}
	}
	return []uint16{uint16(r)}
}

func prevIsSpace(s string, pos int) bool {
	if pos <= 0 {
		return false
	}
	r, _ := jsstr.DecodeLast(s, pos)
	return jsstr.IsSpace(r)
}

func skip(s string, pos int, commas bool) int {
	for pos < len(s) {
		c := s[pos]
		if c == ' ' || c == '\n' || c == '\t' || c == '\r' || (commas && c == ',') {
			pos++
		} else if c == '#' && (pos == 0 || prevIsSpace(s, pos)) {
			nl := strings.IndexByte(s[pos:], '\n')
			if nl == -1 {
				pos = len(s)
			} else {
				pos += nl
			}
		} else {
			break
		}
	}
	return pos
}

// entryStart checks that flow collection entries are separated by exactly one comma.
func entryStart(c byte, separated bool) {
	if !separated {
		fail("YAML: missed comma between flow collection entries")
	}
	if c == ',' {
		fail("YAML: expected the node content, but found ','")
	}
}

func isFlowBoundary(s string, i int) bool {
	r, _ := jsstr.Decode(s, i)
	return jsstr.IsSpace(r) || strings.ContainsRune(",[]{}", r)
}

// flow parses a flow value starting at s[pos] and returns it with the index after it.
func flow(s string, pos int, stops string, depth int) (any, int) {
	if depth > maxDepth {
		fail("YAML: nesting is too deep")
	}
	pos = skip(s, pos, false)
	switch byteAt(s, pos) {
	case '{':
		out := jsobj.New()
		pos++
		separated := true
		for {
			pos = skip(s, pos, false)
			if pos >= len(s) {
				panic(incomplete{"YAML: unexpected end of the stream within a flow collection"})
			}
			if s[pos] == '}' {
				return out, pos + 1
			}
			entryStart(s[pos], separated)
			var key, value any
			key, pos = flow(s, pos, ":,}", depth+1)
			pos = skip(s, pos, false)
			if byteAt(s, pos) == ':' {
				value, pos = flow(s, pos+1, ",}", depth+1)
			}
			out.Set(jsobj.ToString(key), value)
			pos = skip(s, pos, false)
			separated = byteAt(s, pos) == ','
			if separated {
				pos++
			}
		}
	case '[':
		out := []any{}
		pos++
		separated := true
		for {
			pos = skip(s, pos, false)
			if pos >= len(s) {
				panic(incomplete{"YAML: unexpected end of the stream within a flow collection"})
			}
			if s[pos] == ']' {
				return out, pos + 1
			}
			entryStart(s[pos], separated)
			var value any
			value, pos = flow(s, pos, ",]", depth+1)
			out = append(out, value)
			pos = skip(s, pos, false)
			separated = byteAt(s, pos) == ','
			if separated {
				pos++
			}
		}
	case '"':
		end := pos + 1
		for end < len(s) && s[end] != '"' {
			if s[end] == '\\' {
				end += 2
			} else {
				end++
			}
		}
		if end >= len(s) {
			panic(incomplete{"YAML: unexpected end of the stream within a double quoted scalar"})
		}
		return unquoteDouble(s[pos+1 : end]), end + 1
	case '\'':
		end := pos + 1
		for end < len(s) {
			if s[end] == '\'' {
				if byteAt(s, end+1) != '\'' {
					break
				}
				end++
			}
			end++
		}
		if end >= len(s) {
			panic(incomplete{"YAML: unexpected end of the stream within a single quoted scalar"})
		}
		return strings.ReplaceAll(fold(s[pos+1:end]), "''", "'"), end + 1
	}
	end := pos
	for end < len(s) {
		c := s[end]
		if strings.IndexByte(stops, c) >= 0 && (c != ':' || end+1 >= len(s) || isFlowBoundary(s, end+1)) {
			break
		}
		if c == '#' && end > pos && prevIsSpace(s, end) {
			nl := strings.IndexByte(s[end:], '\n')
			next := len(s)
			if nl != -1 {
				next = end + nl
			}
			return scalar(fold(jsstr.Trim(s[pos:end]))), next
		}
		end++
	}
	return scalar(fold(jsstr.Trim(s[pos:end]))), end
}

func stripComment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && quote == '"' {
				i++
			} else if c == quote {
				quote = 0
			}
		} else if c == '"' || c == '\'' {
			quote = c
		} else if c == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return s[:i]
		}
	}
	return s
}

var reIndentDigit = regexp.MustCompile(`[1-9]`)

func startsBlank(s string) bool {
	return s != "" && (s[0] == ' ' || s[0] == '\t')
}

func blockScalar(r *reader, header string, parentIndent int) string {
	folded := header[0] == '>'
	chomp := ""
	if strings.Contains(header, "-") {
		chomp = "-"
	} else if strings.Contains(header, "+") {
		chomp = "+"
	}
	indent := -1
	if d := reIndentDigit.FindString(header); d != "" {
		indent = parentIndent + int(d[0]-'0')
	}
	var out []string
	for r.i < len(r.lines) {
		line := r.lines[r.i]
		if jsstr.Trim(line) == "" {
			out = append(out, "")
			r.i++
			continue
		}
		k := leading(line)
		if indent == -1 {
			if k <= parentIndent {
				break
			}
			indent = k
		}
		if k < indent {
			break
		}
		out = append(out, jsstr.Slice(line, indent, jsstr.Len(line)))
		r.i++
	}
	trailing := 0
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
		trailing++
	}
	var text string
	if folded {
		var b strings.Builder
		for i, cur := range out {
			if i > 0 {
				prev := out[i-1]
				if cur == "" {
					b.WriteByte('\n')
				} else if prev != "" {
					if startsBlank(prev) || startsBlank(cur) {
						b.WriteByte('\n')
					} else {
						b.WriteByte(' ')
					}
				}
			}
			b.WriteString(cur)
		}
		text = b.String()
	} else {
		text = strings.Join(out, "\n")
	}
	if len(out) == 0 {
		if chomp == "+" {
			return strings.Repeat("\n", trailing)
		}
		return ""
	}
	switch chomp {
	case "-":
		return text
	case "+":
		return text + strings.Repeat("\n", trailing+1)
	}
	return text + "\n"
}

// isBlockHeader is /^[|>][-+0-9]*\s*(?:#.*)?$/.
func isBlockHeader(rest string) bool {
	i := 1
	for i < len(rest) && strings.IndexByte("-+0123456789", rest[i]) >= 0 {
		i++
	}
	for i < len(rest) {
		r, n := jsstr.Decode(rest, i)
		if !jsstr.IsSpace(r) {
			break
		}
		i += n
	}
	if i < len(rest) && rest[i] == '#' {
		for i < len(rest) {
			r, n := jsstr.Decode(rest, i)
			if jsstr.IsLineEnd(r) {
				break
			}
			i += n
		}
	}
	return i == len(rest)
}

func inlineValue(r *reader, rest string, indent int) any {
	first := byteAt(rest, 0)
	if (first == '|' || first == '>') && isBlockHeader(rest) {
		return blockScalar(r, rest, indent)
	}
	if first == '{' || first == '[' || first == '"' || first == '\'' {
		return inlineFlow(r, rest)
	}
	text := jsstr.Trim(stripComment(rest))
	for r.i < len(r.lines) {
		line := r.lines[r.i]
		if jsstr.Trim(line) == "" {
			j := r.i
			for j < len(r.lines) && jsstr.Trim(r.lines[j]) == "" {
				j++
			}
			if j >= len(r.lines) || leading(r.lines[j]) <= indent {
				break
			}
			text += strings.Repeat("\n", j-r.i)
			r.i = j
			continue
		}
		if leading(line) <= indent {
			break
		}
		more := jsstr.Trim(stripComment(line))
		if !strings.HasSuffix(text, "\n") {
			text += " "
		}
		text += more
		r.i++
	}
	return scalar(text)
}

// inlineFlow reads a flow value that may continue on later lines, in growing batches of
// lines until it closes.
func inlineFlow(r *reader, rest string) any {
	start := r.i
	text := rest
	taken := 0
	batch := 1
	for {
		value, end, inc := flowOrIncomplete(text)
		if inc == nil {
			used := strings.Count(text[:min(end, len(text))], "\n")
			lineEnd := strings.IndexByte(text[min(end, len(text)):], '\n')
			tail := text[min(end, len(text)):]
			if lineEnd != -1 {
				tail = tail[:lineEnd]
			}
			if jsstr.Trim(stripComment(tail)) != "" {
				fail("YAML: unexpected content after a value")
			}
			r.i = start + used
			return value
		}
		if start+taken >= len(r.lines) {
			fail(inc.message)
		}
		more := r.lines[start+taken : min(len(r.lines), start+taken+batch)]
		text += "\n" + strings.Join(more, "\n")
		taken += len(more)
		batch *= 2
	}
}

func flowOrIncomplete(text string) (value any, end int, inc *incomplete) {
	defer func() {
		if rec := recover(); rec != nil {
			if i, ok := rec.(incomplete); ok {
				inc = &i
				return
			}
			panic(rec)
		}
	}()
	value, end = flow(text, 0, "", 0)
	return value, end, nil
}

func isSeqItem(line string) bool {
	return line == "-" || strings.HasPrefix(line, "- ")
}

func block(r *reader, minIndent, depth int) any {
	if depth > maxDepth {
		fail("YAML: nesting is too deep")
	}
	r.skipBlank()
	if r.i >= len(r.lines) {
		return nil
	}
	indent := r.indent()
	if indent < minIndent {
		return nil
	}
	line := r.lines[r.i][indent:]

	if isSeqItem(line) {
		out := []any{}
		for r.i < len(r.lines) {
			r.skipBlank()
			if r.i >= len(r.lines) || r.indent() != indent {
				break
			}
			cur := r.lines[r.i][indent:]
			if !isSeqItem(cur) {
				break
			}
			rest := cur[1:]
			pad := leading(rest) + 1
			body := jsstr.Trim(stripComment(rest))
			switch {
			case body == "":
				r.i++
				out = append(out, block(r, indent+1, depth+1))
			case func() bool { _, _, ok := splitKey(body); return ok }():
				r.lines[r.i] = strings.Repeat(" ", indent+pad) + jsstr.TrimStart(rest)
				out = append(out, block(r, indent+pad, depth+1))
			default:
				r.i++
				out = append(out, inlineValue(r, jsstr.Trim(rest), indent))
			}
		}
		return out
	}

	if _, _, ok := splitKey(line); !ok {
		r.i++
		return inlineValue(r, jsstr.Trim(line), indent-1)
	}

	out := jsobj.New()
	for r.i < len(r.lines) {
		r.skipBlank()
		if r.i >= len(r.lines) {
			break
		}
		k := r.indent()
		if k < indent {
			break
		}
		if k > indent {
			fail("YAML: bad indentation of a mapping entry")
		}
		cur := r.lines[r.i][indent:]
		rawKey, length, ok := splitKey(cur)
		if !ok {
			fail("YAML: can not read a block mapping entry")
		}
		keyValue, _ := flow(rawKey, 0, "", 0)
		key := jsobj.ToString(keyValue)
		rest := cur[length:]
		r.i++
		if jsstr.Trim(stripComment(rest)) != "" {
			out.Set(key, inlineValue(r, jsstr.Trim(rest), indent))
			continue
		}
		r.skipBlank()
		if r.i >= len(r.lines) {
			out.Set(key, nil)
			continue
		}
		next := r.indent()
		seq := isSeqItem(r.lines[r.i][next:])
		if next > indent || (seq && next == indent) {
			minIndent := indent + 1
			if seq {
				minIndent = indent
			}
			out.Set(key, block(r, minIndent, depth+1))
		} else {
			out.Set(key, nil)
		}
	}
	return out
}
