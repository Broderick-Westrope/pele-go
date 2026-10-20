// Package pele renders Mermaid diagrams to SVG. It is a Go port of Pele
// (https://github.com/obsidianmd/pele) that produces the same SVG as the upstream release it
// follows, byte for byte, given the same options and text measurer.
//
// Diagram types are registered by importing them: import
// github.com/Broderick-Westrope/pele-go/all for every type, or register single types from
// the diagrams packages with Register.
package pele

import (
	"context"

	"github.com/Broderick-Westrope/pele-go/internal/core"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

// UpstreamVersion is the Pele release this package matches.
const UpstreamVersion = "1.7.0"

// Options are the render options. Pointer fields are unset when nil, so that a zero value
// can be told from no value; see the field comments for each default.
type Options = core.Options

// Link is an element of a rendered diagram that links somewhere.
type Link = core.Link

// Result is a rendered diagram.
type Result struct {
	// SVG is the drawing. A lone surrogate the source carried into the drawing is written as
	// U+FFFD, as encoding a JS string as UTF-8 does.
	SVG    string
	Width  float64
	Height float64
	Links  []Link
	Type   DiagramType
	// MermaidType is the name Mermaid reports for the type, such as flowchart-v2 for graph TD.
	MermaidType string
}

// DiagramType names a diagram type, such as "flowchart" or "info".
type DiagramType string

// Error is a failure to parse or render a diagram. Use errors.As to read one.
type Error = perr.Error

// ErrorCode classifies an Error.
type ErrorCode = perr.Code

// The error codes.
const (
	CodeUnsupportedDiagram = perr.UnsupportedDiagram
	CodeSyntax             = perr.Syntax
	CodeSemantic           = perr.Semantic
	CodeLimit              = perr.Limit
	// CodeCanceled reports a render stopped by its context. The Error wraps the context's
	// error.
	CodeCanceled = perr.Canceled
)

// TextMeasurer measures the advance width of text in pixels. Implementations must be safe
// for concurrent use.
type TextMeasurer = text.Measurer

// Style is a set of emphasis bits passed to a TextMeasurer.
type Style = text.Style

// The Style bits.
const (
	Bold   = text.Bold
	Italic = text.Italic
	Mono   = text.Mono
)

// DefaultMeasurer estimates widths from a table of Arial advances. It is used when
// Options.Measurer is nil.
var DefaultMeasurer TextMeasurer = text.Metrics

// Diagram is a diagram type, as the diagrams packages export them.
type Diagram = core.Diagram

// Register makes diagram types available to Render. Call it before the first Render,
// normally from init; it panics afterwards.
func Register(diagrams ...*Diagram) {
	core.Register(diagrams...)
}

// Render renders Mermaid source to SVG. Failures are returned as *Error. Render is safe for
// concurrent use.
func Render(ctx context.Context, source string, opts Options) (Result, error) {
	res, err := core.Render(ctx, source, opts)
	if err != nil {
		return Result{}, err
	}
	return Result{
		SVG:         jsstr.WellFormed(res.SVG),
		Width:       res.Width,
		Height:      res.Height,
		Links:       res.Links,
		Type:        DiagramType(res.Type),
		MermaidType: res.MermaidType,
	}, nil
}

// DetectType returns the diagram type of source, whether or not it is registered. It
// returns false when no type is detected or the front matter cannot be read.
func DetectType(source string) (DiagramType, bool) {
	typ, ok := core.DetectType(source)
	return DiagramType(typ), ok
}

// Supports reports whether source is of a registered diagram type.
func Supports(source string) bool {
	return core.Supports(source)
}
