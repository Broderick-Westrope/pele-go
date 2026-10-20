package util

import (
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// The part of the WHATWG URL parser (as Node's ada implements it) that sanitizeUrl's
// new URL() reaches: an http or https URL with no base. upstream's url.ts has no such code;
// it calls the platform's URL.

type encodeSet func(r rune) bool

func c0Control(r rune) bool { return r < 0x20 || r > 0x7e }

func fragmentSet(r rune) bool {
	return c0Control(r) || r == ' ' || r == '"' || r == '<' || r == '>' || r == '`'
}

func specialQuerySet(r rune) bool {
	return c0Control(r) || r == ' ' || r == '"' || r == '#' || r == '<' || r == '>' || r == '\''
}

// pathSet is the path percent-encode set as Node 22 has it, without ^.
func pathSet(r rune) bool {
	return c0Control(r) || r == ' ' || r == '"' || r == '#' || r == '<' || r == '>' || r == '?' || r == '`' || r == '{' || r == '}'
}

func userinfoSet(r rune) bool {
	return pathSet(r) || r == '/' || r == ':' || r == ';' || r == '=' || r == '@' || r == '[' || r == '\\' || r == ']' || r == '^' || r == '|'
}

func percentEncode(b *strings.Builder, r rune, set encodeSet) {
	if !set(r) {
		b.WriteRune(r)
		return
	}
	var buf [4]byte
	n := utf8.EncodeRune(buf[:], r)
	for _, c := range buf[:n] {
		b.WriteString("%" + strings.ToUpper(strconv.FormatUint(uint64(c)>>4, 16)) + strings.ToUpper(strconv.FormatUint(uint64(c)&15, 16)))
	}
}

// scalars is the input as Unicode scalar values: a lone surrogate becomes U+FFFD.
func scalars(s string) []rune {
	var out []rune
	for i := 0; i < len(s); {
		r, n := jsstr.Decode(s, i)
		if r >= 0xd800 && r <= 0xdfff {
			r = utf8.RuneError
		}
		out = append(out, r)
		i += n
	}
	return out
}

func isASCIIAlpha(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }

// parseURL parses an absolute http or https URL and serialises it, or reports false where
// new URL() throws.
func parseURL(input string) (string, bool) {
	in := scalars(input)
	// Leading and trailing C0 controls and spaces go, and every tab and newline.
	for len(in) > 0 && in[0] <= 0x20 {
		in = in[1:]
	}
	for len(in) > 0 && in[len(in)-1] <= 0x20 {
		in = in[:len(in)-1]
	}
	in = slicesDeleteFunc(in, func(r rune) bool { return r == '\t' || r == '\n' || r == '\r' })

	// Scheme.
	if len(in) == 0 || !isASCIIAlpha(in[0]) {
		return "", false
	}
	i := 1
	for i < len(in) && (isASCIIAlpha(in[i]) || in[i] >= '0' && in[i] <= '9' || in[i] == '+' || in[i] == '-' || in[i] == '.') {
		i++
	}
	if i >= len(in) || in[i] != ':' {
		return "", false
	}
	scheme := strings.ToLower(string(in[:i]))
	defaultPort := ""
	switch scheme {
	case "http":
		defaultPort = "80"
	case "https":
		defaultPort = "443"
	default:
		return "", false
	}
	i++
	// Special authority slashes, then any more slashes.
	for i < len(in) && (in[i] == '/' || in[i] == '\\') {
		i++
	}

	// Authority: the last @ ends the credentials.
	end := i
	for end < len(in) && in[end] != '/' && in[end] != '\\' && in[end] != '?' && in[end] != '#' {
		end++
	}
	authority := in[i:end]
	var username, password strings.Builder
	if at := lastIndexRune(authority, '@'); at != -1 {
		passwordSeen := false
		for _, r := range authority[:at] {
			if r == ':' && !passwordSeen {
				passwordSeen = true
				continue
			}
			target := &username
			if passwordSeen {
				target = &password
			}
			// An earlier @ is written as %40.
			if r == '@' {
				target.WriteString("%40")
				continue
			}
			percentEncode(target, r, userinfoSet)
		}
		if at == len(authority)-1 {
			return "", false
		}
		authority = authority[at+1:]
	}

	// Host, then port.
	hostEnd := len(authority)
	inBrackets := false
	for k, r := range authority {
		if r == '[' {
			inBrackets = true
		} else if r == ']' {
			inBrackets = false
		} else if r == ':' && !inBrackets {
			hostEnd = k
			break
		}
	}
	if hostEnd == 0 {
		return "", false
	}
	host, ok := parseHost(string(authority[:hostEnd]))
	if !ok {
		return "", false
	}
	port := ""
	if hostEnd < len(authority) {
		digits := authority[hostEnd+1:]
		for _, d := range digits {
			if d < '0' || d > '9' {
				return "", false
			}
		}
		if len(digits) > 0 {
			n, _ := new(big.Int).SetString(string(digits), 10)
			if n.Cmp(big.NewInt(65535)) > 0 {
				return "", false
			}
			if port = n.String(); port == defaultPort {
				port = ""
			}
		}
	}

	var out strings.Builder
	out.WriteString(scheme + "://")
	if username.Len() > 0 || password.Len() > 0 {
		out.WriteString(username.String())
		if password.Len() > 0 {
			out.WriteString(":" + password.String())
		}
		out.WriteString("@")
	}
	out.WriteString(host)
	if port != "" {
		out.WriteString(":" + port)
	}

	// Path.
	rest := in[end:]
	var path []string
	var segment strings.Builder
	k := 0
	if k < len(rest) && (rest[k] == '/' || rest[k] == '\\') {
		k++
	}
	for {
		var c rune = -1
		if k < len(rest) {
			c = rest[k]
		}
		if c == -1 || c == '/' || c == '\\' || c == '?' || c == '#' {
			seg := segment.String()
			slash := c == '/' || c == '\\'
			switch {
			case isDoubleDot(seg):
				if len(path) > 0 {
					path = path[:len(path)-1]
				}
				if !slash {
					path = append(path, "")
				}
			case isSingleDot(seg):
				if !slash {
					path = append(path, "")
				}
			default:
				path = append(path, seg)
			}
			segment.Reset()
			if !slash {
				break
			}
		} else {
			percentEncode(&segment, c, pathSet)
		}
		k++
	}
	out.WriteString("/" + strings.Join(path, "/"))

	// Query and fragment.
	if k < len(rest) && rest[k] == '?' {
		k++
		out.WriteByte('?')
		for k < len(rest) && rest[k] != '#' {
			percentEncode(&out, rest[k], specialQuerySet)
			k++
		}
	}
	if k < len(rest) && rest[k] == '#' {
		k++
		out.WriteByte('#')
		for ; k < len(rest); k++ {
			percentEncode(&out, rest[k], fragmentSet)
		}
	}
	return out.String(), true
}

func lastIndexRune(rs []rune, r rune) int {
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i] == r {
			return i
		}
	}
	return -1
}

func slicesDeleteFunc(rs []rune, del func(rune) bool) []rune {
	out := rs[:0:0]
	for _, r := range rs {
		if !del(r) {
			out = append(out, r)
		}
	}
	return out
}

func isSingleDot(s string) bool { return s == "." || strings.EqualFold(s, "%2e") }

func isDoubleDot(s string) bool {
	switch strings.ToLower(s) {
	case "..", ".%2e", "%2e.", "%2e%2e":
		return true
	}
	return false
}

// parseHost is the host parser for a special URL's host.
func parseHost(input string) (string, bool) {
	// A valid IPv6 address has a colon, which never reaches here: sanitizeUrl only parses a
	// URL whose first line has no colon after its scheme.
	if strings.HasPrefix(input, "[") {
		return "", false
	}
	domain := utf8DecodeWithoutBOM(percentDecode(input))
	ascii, ok := domainToASCII(domain)
	if !ok {
		return "", false
	}
	for _, r := range ascii {
		if isForbiddenDomain(r) {
			return "", false
		}
	}
	if endsInNumber(ascii) {
		return parseIPv4(ascii)
	}
	return ascii, true
}

func percentDecode(s string) []byte {
	var out []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && IsHex(int(s[i+1])) && IsHex(int(s[i+2])) {
			v, _ := strconv.ParseUint(s[i+1:i+3], 16, 8)
			out = append(out, byte(v))
			i += 2
			continue
		}
		out = append(out, s[i])
	}
	return out
}

// utf8DecodeWithoutBOM decodes UTF-8, dropping a leading byte order mark and writing
// U+FFFD for what is not UTF-8. (Where Go writes one U+FFFD per bad byte and WHATWG one per
// bad sequence it makes no difference here: a domain with any U+FFFD is refused.)
func utf8DecodeWithoutBOM(b []byte) []rune {
	if len(b) >= 3 && b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf {
		b = b[3:]
	}
	var out []rune
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		out = append(out, r)
		b = b[n:]
	}
	return out
}

// isForbiddenDomain is a forbidden domain code point.
func isForbiddenDomain(r rune) bool {
	switch r {
	case 0, '\t', '\n', '\r', ' ', '#', '/', ':', '<', '>', '?', '@', '[', '\\', ']', '^', '|', '%', 0x7f:
		return true
	}
	return r <= 0x1f
}

// parseIPv4Number reads a part of an IPv4 address: decimal, octal with a leading 0, or hex
// with 0x.
func parseIPv4Number(s string) (*big.Int, bool) {
	if s == "" {
		return nil, false
	}
	radix := 10
	if len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X") {
		radix, s = 16, s[2:]
	} else if len(s) >= 2 && s[0] == '0' {
		radix, s = 8, s[1:]
	}
	if s == "" {
		return big.NewInt(0), true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := false
		switch radix {
		case 10:
			ok = IsDigit(int(c))
		case 8:
			ok = c >= '0' && c <= '7'
		default:
			ok = IsHex(int(c))
		}
		if !ok {
			return nil, false
		}
	}
	n, _ := new(big.Int).SetString(s, radix)
	return n, true
}

func endsInNumber(s string) bool {
	parts := strings.Split(s, ".")
	if parts[len(parts)-1] == "" {
		if len(parts) == 1 {
			return false
		}
		parts = parts[:len(parts)-1]
	}
	last := parts[len(parts)-1]
	if last != "" && strings.Trim(last, "0123456789") == "" {
		return true
	}
	_, ok := parseIPv4Number(last)
	return ok
}

func parseIPv4(s string) (string, bool) {
	parts := strings.Split(s, ".")
	if parts[len(parts)-1] == "" && len(parts) > 1 {
		parts = parts[:len(parts)-1]
	}
	if len(parts) > 4 {
		return "", false
	}
	var numbers []*big.Int
	for _, part := range parts {
		n, ok := parseIPv4Number(part)
		if !ok {
			return "", false
		}
		numbers = append(numbers, n)
	}
	limit := big.NewInt(255)
	for _, n := range numbers[:len(numbers)-1] {
		if n.Cmp(limit) > 0 {
			return "", false
		}
	}
	last := numbers[len(numbers)-1]
	if last.Cmp(new(big.Int).Lsh(big.NewInt(1), uint(8*(5-len(numbers))))) >= 0 {
		return "", false
	}
	ipv4 := last.Uint64()
	for i, n := range numbers[:len(numbers)-1] {
		ipv4 += n.Uint64() << (8 * (3 - i))
	}
	return strconv.FormatUint(ipv4>>24, 10) + "." + strconv.FormatUint(ipv4>>16&255, 10) + "." +
		strconv.FormatUint(ipv4>>8&255, 10) + "." + strconv.FormatUint(ipv4&255, 10), true
}
