// Package perr is PeleError: the error every failure to parse or render is reported as.
package perr

import (
	"fmt"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// Code classifies an Error.
type Code string

// The codes upstream uses, plus Canceled for a render stopped through its context.
const (
	UnsupportedDiagram Code = "unsupported-diagram"
	Syntax             Code = "syntax"
	Semantic           Code = "semantic"
	Limit              Code = "limit"
	Canceled           Code = "canceled"
)

// Error is a failure to parse or render a diagram. Line and Column are 1-based and count
// UTF-16 code units, as upstream does; they are 0 when the error has no position.
type Error struct {
	Code    Code
	Message string
	// Type and MermaidType are the detected diagram type, or "" when none was detected.
	Type        string
	MermaidType string
	Line        int
	Column      int
	// Snippet is the offending line with a caret under the column.
	Snippet string
	err     error
}

func (e *Error) Error() string { return e.Message }

// Unwrap returns the context error of a Canceled error.
func (e *Error) Unwrap() error { return e.err }

// New returns an Error with no position.
func New(message string, code Code) *Error {
	return &Error{Code: code, Message: message}
}

// Cancel returns the Error for a render stopped by its context.
func Cancel(cause error) *Error {
	return &Error{Code: Canceled, Message: "Render canceled: " + cause.Error(), err: cause}
}

// Throw panics with e. Parsers report errors this way, as upstream throws them; the core
// recovers the panic and returns e.
func Throw(e *Error) {
	panic(e)
}

// Location is a position in a source, as upstream's locate() computes it.
type Location struct {
	Line    int
	Column  int
	Snippet string
}

// Locate finds the line, column and snippet for a UTF-16 offset into src.
func Locate(src []uint16, offset int) Location {
	line := 1
	lineStart := 0
	for i := indexOf(src, '\n', 0); i != -1 && i < offset; i = indexOf(src, '\n', i+1) {
		line++
		lineStart = i + 1
	}
	lineEnd := indexOf(src, '\n', offset)
	if lineEnd == -1 {
		lineEnd = len(src)
	}
	column := offset - lineStart + 1
	text := jsstr.FromUTF16(src[lineStart:max(lineStart, lineEnd)])
	return Location{Line: line, Column: column, Snippet: text + "\n" + strings.Repeat("-", max(0, column-1)) + "^"}
}

func indexOf(src []uint16, c uint16, from int) int {
	for i := max(from, 0); i < len(src); i++ {
		if src[i] == c {
			return i
		}
	}
	return -1
}

// SyntaxError builds upstream's syntaxError(): a lexical error, or a parse error that ends
// with detail.
func SyntaxError(diagramType string, src []uint16, offset int, detail string, lexical bool) *Error {
	loc := Locate(src, offset)
	var message string
	if lexical {
		message = fmt.Sprintf("Lexical error on line %d. Unrecognized text.\n%s", loc.Line, loc.Snippet)
	} else {
		message = fmt.Sprintf("Parse error on line %d:\n%s\n%s", loc.Line, loc.Snippet, detail)
	}
	return &Error{
		Code: Syntax, Message: message, Type: diagramType,
		Line: loc.Line, Column: loc.Column, Snippet: loc.Snippet,
	}
}
