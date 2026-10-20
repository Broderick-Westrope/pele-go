package util

import (
	"strings"
	"unicode"
)

// domainToASCII is UTS #46 ToASCII as the URL parser runs it (CheckHyphens, UseSTD3ASCIIRules
// and VerifyDnsLength off; CheckBidi, CheckJoiners and nontransitional processing on).
//
// Intentional difference: a domain of ASCII that has no xn-- label is exact (it is only
// lower-cased). A domain with any other character, or with an xn-- label, needs Unicode's
// IDNA mapping, normalisation and validity tables, which the port does not carry: it is
// lower-cased and Punycode-encoded label by label, refusing only the code points below.
// Ordinary lower-case NFC names (münchen.de) come out as Node's do; unusual ones may not.
func domainToASCII(domain []rune) (string, bool) {
	ascii := true
	for _, r := range domain {
		if r >= 0x80 {
			ascii = false
			break
		}
	}
	lowered := strings.ToLower(string(domain))
	if ascii && !hasACELabel(lowered) {
		return lowered, lowered != ""
	}
	var mapped strings.Builder
	for _, r := range domain {
		switch {
		case r == 0x3002 || r == 0xff0e || r == 0xff61:
			mapped.WriteByte('.')
		case r == 0xad || r == 0x200b || r == 0x2060 || r == 0xfeff || r == 0x34f || r >= 0xfe00 && r <= 0xfe0f:
			// Ignored.
		case r == 0xfffd || unicode.IsSpace(r) || unicode.Is(unicode.C, r) && r != 0x200c && r != 0x200d:
			return "", false
		default:
			mapped.WriteRune(unicode.ToLower(r))
		}
	}
	labels := strings.Split(mapped.String(), ".")
	for i, label := range labels {
		if hasACELabel(label) {
			if _, ok := punycodeDecode(label[4:]); !ok {
				return "", false
			}
			continue
		}
		if isASCII(label) {
			continue
		}
		encoded, ok := punycodeEncode([]rune(label))
		if !ok {
			return "", false
		}
		labels[i] = "xn--" + encoded
	}
	result := strings.Join(labels, ".")
	return result, result != ""
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func hasACELabel(domain string) bool {
	for _, label := range strings.Split(domain, ".") {
		if strings.HasPrefix(strings.ToLower(label), "xn--") {
			return true
		}
	}
	return false
}

// Punycode (RFC 3492).
const (
	pcBase        = 36
	pcTMin        = 1
	pcTMax        = 26
	pcSkew        = 38
	pcDamp        = 700
	pcInitialBias = 72
	pcInitialN    = 128
)

func pcAdapt(delta, numPoints int, first bool) int {
	if first {
		delta /= pcDamp
	} else {
		delta /= 2
	}
	delta += delta / numPoints
	k := 0
	for delta > ((pcBase-pcTMin)*pcTMax)/2 {
		delta /= pcBase - pcTMin
		k += pcBase
	}
	return k + (pcBase-pcTMin+1)*delta/(delta+pcSkew)
}

func pcDigit(d int) byte {
	if d < 26 {
		return byte('a' + d)
	}
	return byte('0' + d - 26)
}

func pcThreshold(k, bias int) int {
	switch {
	case k <= bias:
		return pcTMin
	case k >= bias+pcTMax:
		return pcTMax
	}
	return k - bias
}

func punycodeEncode(input []rune) (string, bool) {
	var out []byte
	for _, r := range input {
		if r < 0x80 {
			out = append(out, byte(r))
		}
	}
	basic := len(out)
	handled := basic
	if basic > 0 {
		out = append(out, '-')
	}
	n, delta, bias := pcInitialN, 0, pcInitialBias
	for handled < len(input) {
		m := int(^uint(0) >> 1)
		for _, r := range input {
			if int(r) >= n && int(r) < m {
				m = int(r)
			}
		}
		if (m-n)*(handled+1) < 0 {
			return "", false
		}
		delta += (m - n) * (handled + 1)
		n = m
		for _, r := range input {
			if int(r) < n {
				delta++
			}
			if int(r) == n {
				q := delta
				for k := pcBase; ; k += pcBase {
					t := pcThreshold(k, bias)
					if q < t {
						break
					}
					out = append(out, pcDigit(t+(q-t)%(pcBase-t)))
					q = (q - t) / (pcBase - t)
				}
				out = append(out, pcDigit(q))
				bias = pcAdapt(delta, handled+1, handled == basic)
				delta = 0
				handled++
			}
		}
		delta++
		n++
	}
	return string(out), true
}

func punycodeDecode(input string) ([]rune, bool) {
	var out []rune
	b := strings.LastIndexByte(input, '-')
	if b > 0 {
		for i := 0; i < b; i++ {
			if input[i] >= 0x80 {
				return nil, false
			}
			out = append(out, rune(input[i]))
		}
		input = input[b+1:]
	} else if b == 0 {
		input = input[1:]
	}
	n, i, bias := pcInitialN, 0, pcInitialBias
	for p := 0; p < len(input); {
		oldi, w := i, 1
		for k := pcBase; ; k += pcBase {
			if p >= len(input) {
				return nil, false
			}
			c := input[p]
			p++
			var digit int
			switch {
			case c >= '0' && c <= '9':
				digit = int(c-'0') + 26
			case c >= 'a' && c <= 'z':
				digit = int(c - 'a')
			case c >= 'A' && c <= 'Z':
				digit = int(c - 'A')
			default:
				return nil, false
			}
			i += digit * w
			t := pcThreshold(k, bias)
			if digit < t {
				break
			}
			w *= pcBase - t
			if w > 1<<30 || i > 1<<30 {
				return nil, false
			}
		}
		bias = pcAdapt(i-oldi, len(out)+1, oldi == 0)
		n += i / (len(out) + 1)
		i %= len(out) + 1
		if n > unicode.MaxRune {
			return nil, false
		}
		out = append(out[:i], append([]rune{rune(n)}, out[i:]...)...)
		i++
	}
	return out, len(out) > 0
}
