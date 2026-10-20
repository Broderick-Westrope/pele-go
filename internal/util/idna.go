package util

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// domainToASCII is domain to ASCII as Node's URL parser, ada, runs it: UTS #46 ToASCII with
// CheckHyphens, UseSTD3ASCIIRules and VerifyDnsLength off, CheckBidi and CheckJoiners on, and
// nontransitional processing, over ada's own Unicode data (idna_tables.go, IDNA 15.0.0, which
// is older than the Unicode version Node reports for V8 and ICU).
//
// ada's checks are not quite UTS #46's, and the port keeps them: an ASCII label is never
// checked, an xn-- label decoding to ASCII is accepted, and labelValid has the rest.
func domainToASCII(domain []rune) (string, bool) {
	mapped, ok := idnaMap(domain)
	if !ok {
		return "", false
	}
	mapped = nfc(mapped)
	var out strings.Builder
	for i, label := range splitRunes(mapped, '.') {
		if i > 0 {
			out.WriteByte('.')
		}
		switch {
		case len(label) == 0:
		case len(label) >= 4 && string(label[:4]) == "xn--":
			for _, r := range label {
				if r >= 0x80 {
					return "", false
				}
				out.WriteRune(r)
			}
			decoded, ok := punycodeDecode(string(label[4:]))
			if !ok {
				return "", false
			}
			if remapped, ok := idnaMap(decoded); !ok || !slices.Equal(remapped, decoded) {
				return "", false
			}
			if !slices.Equal(nfc(decoded), decoded) || len(decoded) == 0 || !labelValid(decoded) {
				return "", false
			}
		case !slices.ContainsFunc(label, func(r rune) bool { return r >= 0x80 }):
			out.WriteString(string(label))
		default:
			if !labelValid(label) {
				return "", false
			}
			encoded, ok := punycodeEncode(label)
			if !ok {
				return "", false
			}
			out.WriteString("xn--" + encoded)
		}
	}
	result := out.String()
	return result, result != ""
}

func splitRunes(rs []rune, sep rune) [][]rune {
	var out [][]rune
	for {
		i := slices.Index(rs, sep)
		if i < 0 {
			return append(out, rs)
		}
		out = append(out, rs[:i])
		rs = rs[i+1:]
	}
}

// unpack reads one of the tables of idna_tables.go.
func unpack(s string) []rune {
	fields := strings.Fields(s)
	out := make([]rune, len(fields))
	for i, f := range fields {
		n, err := strconv.ParseInt(f, 36, 32)
		if err != nil {
			panic("idna table: " + err.Error())
		}
		out[i] = rune(n)
	}
	return out
}

// runeRanges reads a table of gaps and lengths less one, with extra values after each pair.
func runeRanges(s string, extra int) (ranges [][2]rune, values []rune) {
	nums := unpack(s)
	end := rune(-1)
	for i := 0; i < len(nums); i += 2 + extra {
		start := end + 1 + nums[i]
		end = start + nums[i+1]
		ranges = append(ranges, [2]rune{start, end})
		values = append(values, nums[i+2:i+2+extra]...)
	}
	return ranges, values
}

// rangeIndex is the index of the range holding r, or -1.
func rangeIndex(ranges [][2]rune, r rune) int {
	i := sort.Search(len(ranges), func(i int) bool { return ranges[i][1] >= r })
	if i < len(ranges) && r >= ranges[i][0] {
		return i
	}
	return -1
}

func rangeSet(s string) func(rune) bool {
	table := sync.OnceValue(func() [][2]rune {
		ranges, _ := runeRanges(s, 0)
		return ranges
	})
	return func(r rune) bool { return rangeIndex(table(), r) >= 0 }
}

var (
	isLeadingMark = rangeSet(idnaMarks)
	isVirama      = rangeSet(idnaViramas)
	isJoiningR    = rangeSet(joiningR)
	isJoiningL    = rangeSet(joiningL)
	isJoiningD    = rangeSet(joiningD)
)

// IDNA mapping statuses, as idnaMapping numbers them.
const (
	idnaIgnored = iota
	idnaValid
	idnaDisallowed
	idnaMapped
)

type idnaRange struct {
	start  rune
	status rune
	to     []rune
}

var idnaTable = sync.OnceValue(func() []idnaRange {
	nums := unpack(idnaMapping)
	var out []idnaRange
	start, last := rune(0), rune(0)
	for i := 0; i < len(nums); {
		start += nums[i]
		r := idnaRange{start: start, status: min(nums[i+1], idnaMapped)}
		count := nums[i+1] - idnaMapped
		i += 2
		for ; count > 0; count-- {
			last += nums[i]
			r.to = append(r.to, last)
			i++
		}
		out = append(out, r)
	}
	return out
})

// idnaMap is UTS #46's mapping step, failing on a disallowed code point. A code point past
// the last range's start has its status, as in ada, which is how Punycode decoding to past
// U+10FFFF fails.
func idnaMap(input []rune) ([]rune, bool) {
	table := idnaTable()
	out := make([]rune, 0, len(input))
	for _, r := range input {
		i := sort.Search(len(table), func(i int) bool { return table[i].start > r }) - 1
		switch e := table[max(i, 0)]; e.status {
		case idnaIgnored:
		case idnaValid:
			out = append(out, r)
		case idnaDisallowed:
			return nil, false
		default:
			out = append(out, e.to...)
		}
	}
	return out, true
}

// Hangul syllable composition (Unicode 3.12).
const (
	hangulS = 0xac00
	hangulL = 0x1100
	hangulV = 0x1161
	hangulT = 0x11a7
	countL  = 19
	countV  = 21
	countT  = 28
	countN  = countV * countT
	countS  = countL * countN
)

type nfcData struct {
	decompositions map[rune][]rune
	classes        map[rune]int
	compositions   map[[2]rune]rune
}

var nfcTables = sync.OnceValue(func() *nfcData {
	d := &nfcData{decompositions: map[rune][]rune{}, classes: map[rune]int{}, compositions: map[[2]rune]rune{}}
	nums := unpack(nfcDecompositions)
	c := rune(0)
	for i := 0; i < len(nums); {
		c += nums[i]
		n := int(nums[i+1])
		d.decompositions[c] = nums[i+2 : i+2+n]
		i += 2 + n
	}
	ranges, classes := runeRanges(nfcCombiningClasses, 1)
	for k, r := range ranges {
		for c := r[0]; c <= r[1]; c++ {
			d.classes[c] = int(classes[k])
		}
	}
	nums = unpack(nfcCompositions)
	c = 0
	for i := 0; i < len(nums); i += 3 {
		c += nums[i]
		d.compositions[[2]rune{c, nums[i+1]}] = nums[i+2]
	}
	return d
})

// nfc is Unicode Normalization Form C as ada computes it.
func nfc(input []rune) []rune {
	d := nfcTables()
	out := make([]rune, 0, len(input))
	for _, c := range input {
		switch {
		case c >= hangulS && c < hangulS+countS:
			s := c - hangulS
			out = append(out, hangulL+s/countN, hangulV+s%countN/countT)
			if s%countT != 0 {
				out = append(out, hangulT+s%countT)
			}
		case d.decompositions[c] != nil:
			out = append(out, d.decompositions[c]...)
		default:
			out = append(out, c)
		}
	}
	// Canonical ordering: a stable insertion sort of each mark by its class.
	for i := 1; i < len(out); i++ {
		class := d.classes[out[i]]
		if class == 0 {
			continue
		}
		c, j := out[i], i
		for ; j > 0 && d.classes[out[j-1]] > class; j-- {
			out[j] = out[j-1]
		}
		out[j] = c
	}
	// Composition, in place: w is where the next character is written.
	w := 0
	for i := 0; i < len(out); i, w = i+1, w+1 {
		c := out[i]
		out[w] = c
		switch {
		case c >= hangulL && c < hangulL+countL:
			if i+1 < len(out) && out[i+1] >= hangulV && out[i+1] < hangulV+countV {
				out[w] = hangulS + ((c-hangulL)*countV+out[i+1]-hangulV)*countT
				i++
				if i+1 < len(out) && out[i+1] > hangulT && out[i+1] < hangulT+countT {
					i++
					out[w] += out[i] - hangulT
				}
			}
		case c >= hangulS && c < hangulS+countS:
			if (c-hangulS)%countT != 0 && i+1 < len(out) && out[i+1] > hangulT && out[i+1] < hangulT+countT {
				i++
				out[w] += out[i] - hangulT
			}
		default:
			starter := w
			last := -1
			for ; i+1 < len(out); i++ {
				next := out[i+1]
				class := d.classes[next]
				if last < class {
					if composite, ok := d.compositions[[2]rune{out[starter], next}]; ok {
						out[starter] = composite
						continue
					}
				}
				if class == 0 {
					break
				}
				last = class
				w++
				out[w] = next
			}
		}
	}
	return out[:w]
}

// Bidi classes, as bidiClasses numbers them.
const (
	bidiNone = iota
	bidiBN
	bidiCS
	bidiES
	bidiON
	bidiEN
	bidiL
	bidiR
	bidiNSM
	bidiAL
	bidiAN
	bidiET
)

type bidiData struct {
	ranges  [][2]rune
	classes []rune
}

var bidiTable = sync.OnceValue(func() bidiData {
	ranges, classes := runeRanges(bidiClasses, 1)
	return bidiData{ranges, classes}
})

// bidiClass is a code point's class, or bidiNone where ada's table has none.
func bidiClass(r rune) rune {
	t := bidiTable()
	if i := rangeIndex(t.ranges, r); i >= 0 {
		return t.classes[i]
	}
	return bidiNone
}

// labelValid is ada's is_label_valid for a mapped, normalised label. Where it differs from
// UTS #46 the port follows it: a label passes as soon as a ZWNJ or ZWJ does (no later joiner
// or Bidi check); ZWNJ's context is any L or D joining character before it and any R or D
// after, with no regard for transparent ones in between; the Bidi rule only applies to a
// label that has an R, AL or AN character, whatever the domain's other labels; and an LTR
// label's last character is not checked.
func labelValid(label []rune) bool {
	if len(label) == 0 {
		return true
	}
	if isLeadingMark(label[0]) {
		return false
	}
	for i, c := range label {
		if c != 0x200c && c != 0x200d {
			continue
		}
		if i > 0 && isVirama(label[i-1]) {
			return true
		}
		if c == 0x200d || i+1 >= len(label) {
			return false
		}
		return i > 0 &&
			slices.ContainsFunc(label[:i], func(r rune) bool { return isJoiningL(r) || isJoiningD(r) }) &&
			slices.ContainsFunc(label[i+1:], func(r rune) bool { return isJoiningR(r) || isJoiningD(r) })
	}
	last := len(label) - 1
	for last >= 0 && bidiClass(label[last]) == bidiNSM {
		last--
	}
	if last < 0 {
		return false
	}
	if !slices.ContainsFunc(label, func(r rune) bool {
		d := bidiClass(r)
		return d == bidiR || d == bidiAL || d == bidiAN
	}) {
		return true
	}
	if bidiClass(label[0]) == bidiL {
		for _, r := range label[:last] {
			switch bidiClass(r) {
			case bidiL, bidiEN, bidiES, bidiCS, bidiET, bidiON, bidiBN, bidiNSM:
			default:
				return false
			}
		}
		return true
	}
	hasAN, hasEN := false, false
	for i, r := range label[:last+1] {
		d := bidiClass(r)
		hasAN = hasAN || d == bidiAN
		hasEN = hasEN || d == bidiEN
		if hasAN && hasEN {
			return false
		}
		switch d {
		case bidiR, bidiAL, bidiAN, bidiEN, bidiES, bidiCS, bidiET, bidiON, bidiBN, bidiNSM:
		default:
			return false
		}
		if i == last && d != bidiR && d != bidiAL && d != bidiAN && d != bidiEN {
			return false
		}
	}
	return true
}

// Punycode (RFC 3492), with ada's overflow checks.
const (
	pcBase        = 36
	pcTMin        = 1
	pcTMax        = 26
	pcSkew        = 38
	pcDamp        = 700
	pcInitialBias = 72
	pcInitialN    = 128
	pcMaxInt      = 0x7fffffff
)

func pcAdapt(delta, numPoints int32, first bool) int32 {
	if first {
		delta /= pcDamp
	} else {
		delta /= 2
	}
	delta += delta / numPoints
	k := int32(0)
	for delta > ((pcBase-pcTMin)*pcTMax)/2 {
		delta /= pcBase - pcTMin
		k += pcBase
	}
	return k + (pcBase-pcTMin+1)*delta/(delta+pcSkew)
}

func pcDigit(d int32) byte {
	if d < 26 {
		return byte('a' + d)
	}
	return byte('0' + d - 26)
}

func pcThreshold(k, bias int32) int32 {
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
		if r > 0x10ffff || r >= 0xd880 && r < 0xe000 {
			return "", false
		}
	}
	basic := len(out)
	handled := basic
	if basic > 0 {
		out = append(out, '-')
	}
	n, delta, bias := uint32(pcInitialN), int32(0), int32(pcInitialBias)
	for handled < len(input) {
		m := uint32(0x10ffff)
		for _, r := range input {
			if uint32(r) >= n && uint32(r) < m {
				m = uint32(r)
			}
		}
		if uint64(m-n) > uint64(pcMaxInt-delta)/uint64(handled+1) {
			return "", false
		}
		delta += int32(uint64(m-n) * uint64(handled+1))
		n = m
		for _, r := range input {
			if uint32(r) < n {
				if delta == pcMaxInt {
					return "", false
				}
				delta++
			}
			if uint32(r) == n {
				q := delta
				for k := int32(pcBase); ; k += pcBase {
					t := pcThreshold(k, bias)
					if q < t {
						break
					}
					out = append(out, pcDigit(t+(q-t)%(pcBase-t)))
					q = (q - t) / (pcBase - t)
				}
				out = append(out, pcDigit(q))
				bias = pcAdapt(delta, int32(handled+1), handled == basic)
				delta = 0
				handled++
			}
		}
		delta++
		n++
	}
	return string(out), true
}

// punycodeDecode accepts lower-case digits only, as ada does (its input is already mapped),
// and code points past U+10FFFF, which the mapping then refuses.
func punycodeDecode(input string) ([]rune, bool) {
	var out []rune
	if b := strings.LastIndexByte(input, '-'); b >= 0 {
		for i := 0; i < b; i++ {
			if input[i] >= 0x80 {
				return nil, false
			}
			out = append(out, rune(input[i]))
		}
		input = input[b+1:]
	}
	n, i, bias := uint32(pcInitialN), int32(0), int32(pcInitialBias)
	for len(input) > 0 {
		oldi, w := i, int32(1)
		for k := int32(pcBase); ; k += pcBase {
			if len(input) == 0 {
				return nil, false
			}
			c := input[0]
			input = input[1:]
			var digit int32
			switch {
			case c >= 'a' && c <= 'z':
				digit = int32(c - 'a')
			case c >= '0' && c <= '9':
				digit = int32(c-'0') + 26
			default:
				return nil, false
			}
			if digit > (pcMaxInt-i)/w {
				return nil, false
			}
			i += digit * w
			t := pcThreshold(k, bias)
			if digit < t {
				break
			}
			if w > pcMaxInt/(pcBase-t) {
				return nil, false
			}
			w *= pcBase - t
		}
		count := int32(len(out)) + 1
		bias = pcAdapt(i-oldi, count, oldi == 0)
		if i/count > int32(pcMaxInt-n) {
			return nil, false
		}
		n += uint32(i / count)
		i %= count
		out = slices.Insert(out, int(i), rune(n))
		i++
	}
	return out, true
}
