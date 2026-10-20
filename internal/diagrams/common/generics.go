package common

import "strings"

func tildes(text string) int {
	return strings.Count(text, "~")
}

// pairTildes pairs tildes from the outside in: the first with the last, the second with the
// one before it.
func pairTildes(text string) string {
	count := tildes(text)
	if count <= 1 {
		return text
	}
	lead := count%2 != 0 && strings.HasPrefix(text, "~")
	chars := []byte(text)
	var at []int
	start := 0
	if lead {
		start = 1
	}
	for i := start; i < len(chars); i++ {
		if chars[i] == '~' {
			at = append(at, i)
		}
	}
	for lo, hi := 0, len(at)-1; lo < hi; lo, hi = lo+1, hi-1 {
		chars[at[lo]] = '<'
		chars[at[hi]] = '>'
	}
	return string(chars)
}

// splitCommas is input.split(/(,)/): the parts between commas, with each comma kept.
func splitCommas(input string) []string {
	var sets []string
	for {
		at := strings.IndexByte(input, ',')
		if at == -1 {
			return append(sets, input)
		}
		sets = append(sets, input[:at], ",")
		input = input[at+1:]
	}
}

// ParseGenericTypes turns `List~int~` into `List<int>`, with the quirks of Mermaid's
// parseGenericTypes around commas.
func ParseGenericTypes(input string) string {
	if !strings.Contains(input, "~") {
		return input
	}
	sets := splitCommas(input)
	var output []string
	for i := 0; i < len(sets); i++ {
		set := sets[i]
		if set == "," && i > 0 && i+1 < len(sets) && tildes(sets[i-1]) == 1 && tildes(sets[i+1]) == 1 {
			set = sets[i-1] + "," + sets[i+1]
			i++
			output = output[:len(output)-1]
		}
		output = append(output, pairTildes(set))
	}
	return strings.Join(output, "")
}
