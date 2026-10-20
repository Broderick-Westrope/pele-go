package class

import (
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

func isVisibility(ch uint16) bool {
	return ch == '#' || ch == '+' || ch == '~' || ch == '-'
}

// classMember is upstream's ClassMember. method is memberType 'method'; visibility and
// classifier are "" or one ASCII character.
type classMember struct {
	id         string
	method     bool
	visibility string
	classifier string
	parameters string
	returnType string
}

func newClassMember(input string, method bool) *classMember {
	m := &classMember{method: method}
	// Mermaid throws a TypeError for a method with no opening parenthesis, such as `run)`.
	// It is kept here as an attribute, so it is shown as written.
	if !method || !m.parseMethod(jsstr.UTF16(input)) {
		m.parseAttribute(input)
	}
	if len(m.id) > 0 && m.id[0] == ' ' {
		m.id = " " + jsstr.Trim(m.id)
	} else {
		m.id = jsstr.Trim(m.id)
	}
	return m
}

// parseMethod is the same split as Mermaid's /([#+~-])?(.+)\((.*)\)([\s$*])?(.*)([$*])?/,
// found in one pass: the first line with an opening parenthesis after its first character
// and a closing one later.
func (m *classMember) parseMethod(input []uint16) bool {
	n := len(input)
	slice := func(from, to int) string { return jsstr.FromUTF16(input[from:to]) }
	for start := 0; start < n; {
		end := start
		for end < n && !util.IsLineEnd(int(input[end])) {
			end++
		}
		close := end - 1
		for close > start && input[close] != ')' {
			close--
		}
		open := close - 1
		for open > start && input[open] != '(' {
			open--
		}
		if open > start {
			first := input[start]
			visible := isVisibility(first) && open > start+1
			from := start
			if visible {
				m.visibility = string(rune(first))
				from = start + 1
			}
			m.id = slice(from, open)
			m.parameters = jsstr.Trim(slice(open+1, close))
			rest := close + 1
			classifier := ""
			if rest < n {
				next := input[rest]
				if next == '$' || next == '*' || util.IsSpace(int(next)) {
					if !util.IsSpace(int(next)) {
						classifier = string(rune(next))
					}
					rest++
				}
			}
			stop := rest
			for stop < n && !util.IsLineEnd(int(input[stop])) {
				stop++
			}
			m.returnType = jsstr.Trim(slice(rest, stop))
			if classifier == "" && m.returnType != "" {
				if last := m.returnType[len(m.returnType)-1]; last == '$' || last == '*' {
					classifier = string(last)
					m.returnType = m.returnType[:len(m.returnType)-1]
				}
			}
			m.classifier = classifier
			return true
		}
		start = end + 1
	}
	m.method = false
	return false
}

func (m *classMember) parseAttribute(input string) {
	if input == "" {
		return
	}
	first, last := input[0], input[len(input)-1]
	if isVisibility(uint16(first)) {
		m.visibility = string(first)
	}
	if last == '$' || last == '*' {
		m.classifier = string(last)
	}
	from, to := 0, len(input)
	if m.visibility != "" {
		from = 1
	}
	if m.classifier != "" {
		to--
	}
	m.id = input[from:to]
}

func (m *classMember) getDisplayDetails() (displayText, cssStyle string) {
	displayText = m.visibility + common.ParseGenericTypes(m.id)
	if m.method {
		displayText += "(" + common.ParseGenericTypes(jsstr.Trim(m.parameters)) + ")"
		if m.returnType != "" {
			displayText += " : " + common.ParseGenericTypes(m.returnType)
		}
	}
	return jsstr.Trim(displayText), m.parseClassifier()
}

func (m *classMember) parseClassifier() string {
	switch m.classifier {
	case "*":
		return "font-style:italic;"
	case "$":
		return "text-decoration:underline;"
	}
	return ""
}
