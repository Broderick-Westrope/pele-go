package common

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
