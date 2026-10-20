// Package core is upstream's src/core.ts: the registry of diagram types and the render
// pipeline from source text to SVG.
package core

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Broderick-Westrope/pele-go/internal/detect"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
	"github.com/Broderick-Westrope/pele-go/internal/preprocess"
	"github.com/Broderick-Westrope/pele-go/internal/text"
)

// Options are the render options. Pointer fields are unset when nil, so a zero value can be
// told from no value, as upstream tells undefined from 0 or "".
type Options struct {
	// Measurer measures label text. Nil uses the built-in Arial table.
	Measurer       text.Measurer
	FontFamily     *string
	FontFamilyMono *string
	FontSize       *float64
	IDPrefix       *string
	// Responsive makes the SVG scale to its container. Nil leaves it on unless the diagram's
	// config sets useMaxWidth: false.
	Responsive          *bool
	MaxWidth            *float64
	AutoDirection       *bool
	DirectionBreakpoint *float64
	Padding             *float64
	// Limit is the longest source accepted, in UTF-16 code units. Nil means 50,000.
	Limit *int
	// OutputLimit is the longest SVG returned, in UTF-16 code units. Nil means 4,000,000.
	OutputLimit *int
	// MaxEdges is the most edges a flowchart may have. Nil means 5,000.
	MaxEdges     *int
	LinkSchemes  []string
	ImageSchemes []string
	Links        *bool
	Images       *bool
	LinkRel      *string
	// Icons returns SVG markup for an icon name, or "" when the host has none.
	Icons func(name string) string
	// Now is the time gantt charts treat as now. The zero time means the current time.
	Now time.Time
	// Location is the time zone gantt charts read dates in. Nil means time.Local.
	Location *time.Location
}

// Link is an element of a rendered diagram that links somewhere.
type Link struct {
	ID       string
	Href     string
	Internal bool
}

// Rendered is what a diagram type's renderer returns.
type Rendered struct {
	SVG    string
	Width  float64
	Height float64
	Links  []Link
}

// Result is a rendered diagram. SVG is WTF-8: lone surrogates upstream would put in the
// output are kept.
type Result struct {
	Rendered
	Type        string
	MermaidType string
}

// Limits are set by the host. A diagram's own configuration cannot change them.
type Limits struct {
	MaxEdges int
}

// Context is what a diagram type's renderer gets besides its model: the options as the core
// resolved them, and per-render state upstream keeps in module variables.
type Context struct {
	Ctx     context.Context
	Options Options
	// Responsive is options.responsive after the core defaulted it.
	Responsive bool
	Icons      *text.Icons
}

// Measurer returns the measurer to lay labels out with.
func (c *Context) Measurer() text.Measurer {
	if c.Options.Measurer != nil {
		return c.Options.Measurer
	}
	return text.Metrics
}

// Diagram is what a diagram type provides. Parse is given text that has been through
// preprocessing and entity encoding and ends with a newline, as Mermaid's parsers expect.
// Both report failures by panicking with a *perr.Error, as upstream throws them.
type Diagram struct {
	Type string
	// Section is the type's section in Mermaid's config, when it is not the type's name.
	Section string
	// KeepComments asks for the text with its %% comment lines kept, for a type that
	// reports source positions.
	KeepComments bool
	Parse        func(source string, config *jsobj.Object, title *string, lineOffset int, limits Limits) any
	Render       func(model any, config *jsobj.Object, rc *Context) Rendered
}

var (
	registryMu sync.Mutex
	registry   = map[string]*Diagram{}
	// sealed is set by the first render. Registering after it is a programming error.
	sealed atomic.Bool
)

// Register makes diagram types available to Render. It must be called before the first
// render, normally from init.
func Register(diagrams ...*Diagram) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if sealed.Load() {
		panic("pele: Register called after the first Render")
	}
	for _, d := range diagrams {
		registry[d.Type] = d
	}
}

func lookup(typ string) (*Diagram, bool) {
	if !sealed.Load() {
		registryMu.Lock()
		sealed.Store(true)
		registryMu.Unlock()
	}
	d, ok := registry[typ]
	return d, ok
}

// Registered reports whether a diagram type is available.
func Registered(typ string) bool {
	_, ok := lookup(typ)
	return ok
}

// Upstream's defaults.
const (
	// DefaultLimit is Mermaid's default maxTextSize.
	DefaultLimit = 50000
	// DefaultOutputLimit bounds the SVG: a source within the limit can still describe a
	// drawing of many megabytes.
	DefaultOutputLimit = 4_000_000
	// DefaultMaxEdges bounds flowchart edges, which `A & B --> C & D` multiplies.
	DefaultMaxEdges = 5000
)

// DetectType returns the diagram type of text, or false when none is detected or the front
// matter cannot be read.
func DetectType(text string) (typ string, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, isPeleErr := r.(*perr.Error); !isPeleErr {
				if _, isRange := r.(perr.RangeError); !isRange {
					panic(r)
				}
			}
			typ, ok = "", false
		}
	}()
	d, found := detect.Detect(preprocess.Preprocess(jsstr.Normalize(text)).Text)
	return d.Type, found
}

// Supports reports whether text is of a registered diagram type.
func Supports(text string) bool {
	typ, ok := DetectType(text)
	return ok && Registered(typ)
}

type parsed struct {
	detect.Detected
	diagram *Diagram
	model   any
	config  *jsobj.Object
}

func optInt(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

func parseSource(ctx context.Context, source string, opts Options) parsed {
	limit := optInt(opts.Limit, DefaultLimit)
	if jsstr.Len(source) > limit {
		perr.Throw(perr.New("Diagram source is longer than the limit of "+itoa(limit)+" characters.", perr.Limit))
	}
	pre := preprocess.Preprocess(source)
	found, ok := detect.Detect(pre.Text)
	if !ok {
		perr.Throw(perr.New("No diagram type detected for the given text.", perr.UnsupportedDiagram))
	}
	diagram, ok := lookup(found.Type)
	if !ok {
		e := perr.New(`Diagram type "`+found.Type+`" is not supported yet.`, perr.UnsupportedDiagram)
		e.Type, e.MermaidType = found.Type, found.MermaidType
		perr.Throw(e)
	}
	checkContext(ctx)
	config := pre.Config
	src := pre.Text
	if diagram.KeepComments {
		src = pre.WithComments
	}
	var model any
	tagErrors(found, func() {
		model = diagram.Parse(preprocess.EncodeEntities(src)+"\n", config, pre.Title, pre.LineOffset,
			Limits{MaxEdges: optInt(opts.MaxEdges, DefaultMaxEdges)})
	})
	return parsed{Detected: found, diagram: diagram, model: model, config: config}
}

func checkContext(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		perr.Throw(perr.Cancel(err))
	}
}

// tagErrors runs fn and gives errors it throws the detected type, turning a RangeError into
// the limit error.
func tagErrors(found detect.Detected, fn func()) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		switch e := r.(type) {
		case perr.RangeError:
			err := perr.New("Diagram is nested too deeply.", perr.Limit)
			err.Type, err.MermaidType = found.Type, found.MermaidType
			panic(err)
		case *perr.Error:
			if e.Type == "" {
				e.Type = found.Type
			}
			if e.MermaidType == "" {
				e.MermaidType = found.MermaidType
			}
			panic(e)
		default:
			panic(r)
		}
	}()
	fn()
}

// Render renders Mermaid source to SVG. The SVG keeps lone surrogates; see Result.
func Render(ctx context.Context, source string, opts Options) (res Result, err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		switch e := r.(type) {
		case *perr.Error:
			err = e
		case perr.RangeError:
			err = perr.New("Diagram is nested too deeply.", perr.Limit)
		default:
			panic(r)
		}
	}()
	checkContext(ctx)
	p := parseSource(ctx, jsstr.Normalize(source), opts)

	section := p.diagram.Section
	if section == "" {
		section = p.Type
	}
	fixed := false
	if v, ok := p.config.Get(section); ok {
		if obj, isObj := v.(*jsobj.Object); isObj {
			if useMaxWidth, has := obj.Get("useMaxWidth"); has && useMaxWidth == false {
				fixed = true
			}
		}
	}
	responsive := !fixed
	if opts.Responsive != nil {
		responsive = *opts.Responsive
	}
	resolve := opts.Icons
	var icons func(string) string
	if resolve != nil {
		icons = func(name string) string { return resolve(text.DecodeEntities(name)) }
	}
	rc := &Context{Ctx: ctx, Options: opts, Responsive: responsive, Icons: text.NewIcons(icons)}
	rc.Options.Responsive = &responsive
	checkContext(ctx)
	var rendered Rendered
	tagErrors(p.Detected, func() { rendered = p.diagram.Render(p.model, p.config, rc) })

	outputLimit := optInt(opts.OutputLimit, DefaultOutputLimit)
	if jsstr.Len(rendered.SVG) > outputLimit {
		e := perr.New("Diagram output is longer than the limit of "+itoa(outputLimit)+" characters.", perr.Limit)
		e.Type, e.MermaidType = p.Type, p.MermaidType
		return Result{}, e
	}
	for i := range rendered.Links {
		rendered.Links[i].ID = text.DecodeEntities(rendered.Links[i].ID)
	}
	if rendered.Links == nil {
		rendered.Links = []Link{}
	}
	return Result{Rendered: rendered, Type: p.Type, MermaidType: p.MermaidType}, nil
}

func itoa(n int) string { return strconv.Itoa(n) }
