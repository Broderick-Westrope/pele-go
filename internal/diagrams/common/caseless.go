package common

// WordAt reports whether word, written in lower case, is at at. Only ASCII letters match in
// either case, as with the i flag on a regular expression without the u flag.
func WordAt(src Src, at int, word string) bool {
	for k := 0; k < len(word); k++ {
		c := src.At(at + k)
		w := int(word[k])
		if c != w && !(w >= 97 && w <= 122 && c == w-32) {
			return false
		}
	}
	return true
}
