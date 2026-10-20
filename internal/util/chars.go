// Package util is upstream's src/util: tests on characters, the Unicode letter table the
// lexers share, and URL sanitising.
package util

// Tests on UTF-16 code units, as the lexers read them with charCodeAt. Past the end of a
// string the lexers use -1 where upstream's code is NaN, which is none of these.

// IsSpace is whitespace as \s sees it.
func IsSpace(c int) bool {
	if c < 128 {
		return c == 32 || (c >= 9 && c <= 13)
	}
	return c == 0xa0 ||
		c == 0x1680 ||
		(c >= 0x2000 && c <= 0x200a) ||
		c == 0x2028 ||
		c == 0x2029 ||
		c == 0x202f ||
		c == 0x205f ||
		c == 0x3000 ||
		c == 0xfeff
}

// IsBlank is a space or a tab.
func IsBlank(c int) bool {
	return c == 32 || c == 9
}

// IsLineEnd is a line terminator: what . does not match.
func IsLineEnd(c int) bool {
	return c == 10 || c == 13 || c == 0x2028 || c == 0x2029
}

// IsDigit is an ASCII digit.
func IsDigit(c int) bool {
	return c >= 48 && c <= 57
}

// IsLetter is an ASCII letter.
func IsLetter(c int) bool {
	return (c >= 65 && c <= 90) || (c >= 97 && c <= 122)
}

// IsHex is an ASCII hex digit.
func IsHex(c int) bool {
	return (c >= 48 && c <= 57) || (c >= 65 && c <= 70) || (c >= 97 && c <= 102)
}

// IsWord is a character of \w.
func IsWord(c int) bool {
	return (c >= 48 && c <= 57) || (c >= 65 && c <= 90) || (c >= 97 && c <= 122) || c == 95
}
