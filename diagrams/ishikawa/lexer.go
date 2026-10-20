package ishikawa

import "github.com/Broderick-Westrope/pele-go/internal/util"

// Token types.
const (
	tEnd = iota
	tEOF
	tSpaceline
	tNL
	tIshikawa
	tSpacelist
	tText
)

var tokenNames = [...]string{"$end", "EOF", "SPACELINE", "NL", "ISHIKAWA", "SPACELIST", "TEXT"}

type tokens struct {
	types, starts, ends []int
}

func at(src []uint16, i int) int {
	if i < 0 || i >= len(src) {
		return -1
	}
	return int(src[i])
}

func keyword(src []uint16, start int) int {
	const word = "ishikawa-beta"
	k := 0
	for k < 13 {
		c := at(src, start+k)
		w := int(word[k])
		if c != w && (c+32 != w || c < 65 || c > 90) {
			break
		}
		k++
	}
	// `ishikawa-beta`, or failing that `ishikawa`, either one ending at a word boundary.
	if k == 13 && !util.IsWord(at(src, start+13)) {
		return 13
	}
	if k >= 8 && !util.IsWord(at(src, start+8)) {
		return 8
	}
	return 0
}

// tokenize produces the tokens of Mermaid's Jison lexer for Ishikawa diagrams, where the
// first rule that matches wins: white space up to a comment or up to the last line break in
// it is a blank line, a single line break before text is a new line, other white space is
// indentation, and the rest of a line is text.
func tokenize(src []uint16) tokens {
	var t tokens
	n := len(src)
	i := 0
	for i < n {
		j := i
		lastBreak := -1
		for j < n && util.IsSpace(int(src[j])) {
			if src[j] == 10 {
				lastBreak = j
			}
			j++
		}
		var typ, end int
		switch {
		case at(src, j) == 37 && at(src, j+1) == 37:
			typ = tSpaceline
			end = j + 2
			for end < n && !util.IsLineEnd(int(src[end])) {
				end++
			}
		case j == i:
			if length := keyword(src, i); length > 0 {
				typ = tIshikawa
				end = i + length
			} else {
				typ = tText
				end = i
				for end < n && src[end] != '\n' {
					end++
				}
			}
		case lastBreak > i:
			typ = tSpaceline
			end = lastBreak + 1
		case lastBreak == i:
			typ = tNL
			end = i + 1
		default:
			typ = tSpacelist
			end = j
		}
		t.types = append(t.types, typ)
		t.starts = append(t.starts, i)
		t.ends = append(t.ends, end)
		i = end
	}
	t.types = append(t.types, tEOF, tEnd)
	t.starts = append(t.starts, n, n)
	t.ends = append(t.ends, n, n)
	return t
}
