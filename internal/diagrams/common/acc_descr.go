package common

import (
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// accDescrHead is /[\t ]*accDescr(?:[\t ]*:(?:[^\n\r]*?(?=%%)|[^\n\r]*)|\s*({))/y. It
// returns the end of the match, whether it is the braced form, or -1.
func accDescrHead(src Src, at int) (int, bool) {
	i := skipBlanks(src, at)
	if !src.HasPrefix(i, "accDescr") {
		return -1, false
	}
	i += len("accDescr")
	if j := skipBlanks(src, i); src.At(j) == ':' {
		return lineRest(src, j+1), false
	}
	for isSpace(src.At(i)) {
		i++
	}
	if src.At(i) != '{' {
		return -1, false
	}
	return i + 1, true
}

// AccDescrScanner is the common accDescr terminal for grammars where lexing goes on after an
// `accDescr {` that never closes, because accDescr is also a name there. Each one would
// otherwise read to the end of the text. The braced form scans for its closing brace; once
// none is found there is none further on, so later braced descriptions fail without
// scanning the rest of the text again. name "" is upstream's default, ACC_DESCR.
func AccDescrScanner(name string) *TokenType {
	if name == "" {
		name = AccDescr.Name
	}
	return &TokenType{Name: name, First: AccDescr.First, NewMatch: func() Matcher {
		unclosed := false
		return func(src Src, at int) int {
			end, braced := accDescrHead(src, at)
			if end == -1 || !braced {
				return end
			}
			if unclosed {
				return -1
			}
			close := src.Index("}", end)
			unclosed = close == -1
			if close == -1 {
				return -1
			}
			return close + 1
		}
	}}
}

// CollapseIndentedBreaks is s.replace(/\n\s+/g, '\n'), which every diagram applies to its
// accessible description.
func CollapseIndentedBreaks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '\n' {
			b.WriteByte(s[i])
			i++
			continue
		}
		b.WriteByte('\n')
		i++
		for i < len(s) {
			r, n := jsstr.Decode(s, i)
			if !jsstr.IsSpace(r) {
				break
			}
			i += n
		}
	}
	return b.String()
}
