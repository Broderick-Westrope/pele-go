package jsstr

import (
	"unicode"
	"unicode/utf16"
)

// ToLowerCase is String.prototype.toLowerCase: Unicode's default lower-casing, with İ's
// two-character lower case and Σ's final form, which Go's simple mappings leave out.
func ToLowerCase(s string) string {
	units := UTF16(s)
	var runes []rune
	for i := 0; i < len(units); {
		r := rune(units[i])
		n := 1
		if utf16.IsSurrogate(r) && i+1 < len(units) {
			if pair := utf16.DecodeRune(r, rune(units[i+1])); pair != unicode.ReplacementChar {
				r, n = pair, 2
			}
		}
		runes = append(runes, r)
		i += n
	}
	var out []uint16
	for i, r := range runes {
		switch {
		case r == 0x130:
			out = append(out, 'i', 0x307)
		case r == 0x3a3:
			if finalSigma(runes, i) {
				out = append(out, 0x3c2)
			} else {
				out = append(out, 0x3c3)
			}
		case utf16.IsSurrogate(r):
			out = append(out, uint16(r))
		default:
			out = utf16.AppendRune(out, toLower(r))
		}
	}
	return FromUTF16(out)
}

// ChangesWhenLowered reports whether s.toLowerCase() !== s, without building the lower case.
func ChangesWhenLowered(s string) bool {
	for i := 0; i < len(s); {
		r, n := Decode(s, i)
		if r == 0x130 || toLower(r) != r {
			return true
		}
		i += n
	}
	return false
}

// toLower is r's simple lower case mapping in V8's Unicode 16. Go's tables are Unicode 15,
// which lacks the upper case letters added since.
func toLower(r rune) rune {
	switch r {
	case 0x1c89:
		return 0x1c8a
	case 0xa7cb:
		return 0x264
	case 0xa7cc:
		return 0xa7cd
	case 0xa7da:
		return 0xa7db
	case 0xa7dc:
		return 0x19b
	}
	if r >= 0x10d50 && r <= 0x10d65 {
		return r + 0x20
	}
	return unicode.ToLower(r)
}

// finalSigma is Unicode's Final_Sigma condition: a cased letter before, skipping
// case-ignorable characters, and none after.
func finalSigma(runes []rune, at int) bool {
	before := false
	for i := at - 1; i >= 0; i-- {
		if !caseIgnorable(runes[i]) {
			before = cased(runes[i])
			break
		}
	}
	if !before {
		return false
	}
	for i := at + 1; i < len(runes); i++ {
		if !caseIgnorable(runes[i]) {
			return !cased(runes[i])
		}
	}
	return true
}

// cased is Unicode 16's Cased property.
func cased(r rune) bool {
	switch {
	case r >= 0x1c89 && r <= 0x1c8a, r >= 0xa7cb && r <= 0xa7cd, r >= 0xa7da && r <= 0xa7dc,
		r >= 0x10d50 && r <= 0x10d65, r >= 0x10d70 && r <= 0x10d85:
		return true
	}
	return unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r) ||
		unicode.Is(unicode.Other_Lowercase, r) || unicode.Is(unicode.Other_Uppercase, r)
}

// caseIgnorable is Unicode 16's Case_Ignorable property: the Word_Break MidLetter,
// MidNumLet and Single_Quote characters, and the general categories Mn, Me, Cf, Lm and Sk.
// The listed code points are those Go's Unicode 15 tables put elsewhere.
func caseIgnorable(r rune) bool {
	switch r {
	case '\'', '.', ':', '^', '`', 0xa8, 0xad, 0xaf, 0xb4, 0xb7, 0xb8, 0x387, 0x55f, 0x5f4, 0x897,
		0x2018, 0x2019, 0x2024, 0x2027, 0xfe13, 0xfe52, 0xfe55, 0xff07, 0xff0e, 0xff1a,
		0x10d4e, 0x10d6f, 0x10efc, 0x113ce, 0x113d0, 0x113d2, 0x11f5a:
		return true
	case 0x1171e:
		return false
	}
	switch {
	case r >= 0x10d69 && r <= 0x10d6d, r >= 0x113bb && r <= 0x113c0, r >= 0x113e1 && r <= 0x113e2,
		r >= 0x1611e && r <= 0x16129, r >= 0x1612d && r <= 0x1612f, r >= 0x16d40 && r <= 0x16d42,
		r >= 0x16d6b && r <= 0x16d6c, r >= 0x1e5ee && r <= 0x1e5ef:
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk)
}
