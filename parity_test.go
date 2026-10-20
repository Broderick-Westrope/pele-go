package pele_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	pele "github.com/Broderick-Westrope/pele-go"
	_ "github.com/Broderick-Westrope/pele-go/all"
	"github.com/Broderick-Westrope/pele-go/internal/core"
	"github.com/Broderick-Westrope/pele-go/internal/jsjson"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/oracle"
)

var fixedNow = time.Date(2026, time.October, 20, 9, 30, 0, 0, time.UTC)

// lengthMeasurer is a measurer unlike Pele's default, so a test that passes it shows the
// measurer reaches layout on both sides.
type lengthMeasurer struct{}

func (lengthMeasurer) Width(text string, size float64, style pele.Style) float64 {
	return float64(jsstr.Len(text))*size*0.55 + float64(style)*0.25
}

type jsMeasurer struct{ m pele.TextMeasurer }

func (j jsMeasurer) Width(text string, size float64, style int) float64 {
	return j.m.Width(text, size, pele.Style(style))
}

func newOracle(t testing.TB) *oracle.Oracle {
	t.Helper()
	o, err := oracle.New()
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func corpus(t testing.TB, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("internal", "oracle", "testdata", "corpus", name))
	if err != nil {
		t.Fatal(err)
	}
	// jsjson keeps lone surrogates, which encoding/json would replace.
	parsed, err := jsjson.Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	var sources []string
	for _, entry := range parsed.([]any) {
		switch entry := entry.(type) {
		case string:
			sources = append(sources, entry)
		case *jsobj.Object:
			src, _ := entry.Get("src")
			sources = append(sources, src.(string))
		}
	}
	return sources
}

var infoInputs = []string{
	"info",
	"info\n",
	"  info showInfo",
	"info\ntitle My title",
	"info\ntitle   spaced    out  ",
	"info\ntitle",
	"info\ntitle x\ttab",
	"info\naccTitle: Hi there\naccDescr: a description",
	"info\naccTitle:",
	"info\naccDescr {\n  multi\n   line\n\n\n  description  \n}",
	"info\naccDescr {\n never closed",
	"info unsupported",
	"info\ntitle inline %% comment",
	"info\n%% comment\ntitle t",
	"%%{init: {\"theme\":\"dark\"}}%%\ninfo",
	"%%{init: {'theme': 'dark'}}%%\ninfo\ntitle quoted",
	"%%{wrap}%%\ninfo",
	"---\ntitle: Front title\n---\ninfo",
	"---\ntitle: Front title\nconfig:\n  info:\n    useMaxWidth: false\n---\ninfo",
	"---\ntitle: [unclosed\n---\ninfo",
	"---\n  bad: indent\n wrong: yes\n---\ninfo",
	"info\ntitle <b>Bold</b> & <i>it</i>",
	"info\ntitle A #35; entity &amp; &lt; #quot;",
	"info\ntitle 中文标题，长一些。",
	"info\ntitle \U0001F600 emoji",
	"info\ntitle fa:fa-car icon",
	"info\ntitle a<br>b<br/>c",
	"info\ntitle " + strings.Repeat("lengthy words ", 400),
	"info\ntitle " + strings.Repeat("长", 600),
	"info\r\ntitle crlf\r\n",
	"info\n\n\ntitle x\n\n",
	"info\ntitle a\x01b\x7f",
	"info\ntitle a\"quote'",
	"info\ntitle x\ntitle y",
	"info\ntitle x\naccTitle: y\nshowInfo",
	"info showInfo extra",
	"",
	"nothing to see",
	"INFO",
	"infox",
	"\u3000info",
	strings.Repeat("x", 50001),
}

func TestInfoParity(t *testing.T) {
	o := newOracle(t)
	inputs := append(corpus(t, "info-docs.json"), infoInputs...)
	for _, src := range inputs {
		for _, set := range optionSets {
			assertParity(t, o, src, set, nil)
			assertParity(t, o, src, set, lengthMeasurer{})
		}
	}
}

func TestDetectTypeParity(t *testing.T) {
	o := newOracle(t)
	files, err := filepath.Glob(filepath.Join("internal", "oracle", "testdata", "corpus", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		for _, src := range corpus(t, filepath.Base(file)) {
			want, err := o.DetectType(src)
			if err != nil {
				want = ""
			}
			got, ok := pele.DetectType(src)
			if !ok {
				got = ""
			}
			if string(got) != want {
				t.Errorf("DetectType(%q) = %q, want %q", src, got, want)
			}
		}
	}
	for _, src := range infoInputs {
		want, err := o.DetectType(src)
		if err != nil {
			want = ""
		}
		got, _ := pele.DetectType(src)
		if string(got) != want {
			t.Errorf("DetectType(%q) = %q, want %q", src, got, want)
		}
	}
}

func FuzzInfoParity(f *testing.F) {
	for _, src := range infoInputs {
		if len(src) < 1000 {
			f.Add(src)
		}
	}
	o := newOracle(f)
	f.Fuzz(func(t *testing.T, src string) {
		if typ, err := o.DetectType(src); err == nil && typ != "" && typ != "info" {
			t.Skip("not an info diagram")
		}
		assertParity(t, o, src, optionSets[0], nil)
	})
}

// optionSet is one set of render options, as Go options and as the JS object that means
// the same.
type optionSet struct {
	name  string
	js    map[string]any
	apply func(*pele.Options)
}

func ptr[T any](v T) *T { return &v }

var optionSets = []optionSet{
	{name: "defaults", apply: func(*pele.Options) {}},
	{
		name: "sized",
		js:   map[string]any{"fontSize": 13.5, "padding": 3, "idPrefix": `d1 "x"`},
		apply: func(o *pele.Options) {
			o.FontSize, o.Padding, o.IDPrefix = ptr(13.5), ptr(3.0), ptr(`d1 "x"`)
		},
	},
	{
		name: "fixed fonts",
		js:   map[string]any{"responsive": false, "fontFamily": `Inter, "Helvetica Neue", sans-serif`, "fontFamilyMono": "bad;}"},
		apply: func(o *pele.Options) {
			o.Responsive, o.FontFamily, o.FontFamilyMono = ptr(false), ptr(`Inter, "Helvetica Neue", sans-serif`), ptr("bad;}")
		},
	},
	{
		name:  "zeroes",
		js:    map[string]any{"padding": 0, "fontSize": 0, "idPrefix": ""},
		apply: func(o *pele.Options) { o.Padding, o.FontSize, o.IDPrefix = ptr(0.0), ptr(0.0), ptr("") },
	},
	{
		name:  "limits",
		js:    map[string]any{"limit": 12, "outputLimit": 1600},
		apply: func(o *pele.Options) { o.Limit, o.OutputLimit = ptr(12), ptr(1600) },
	},
}

func (set optionSet) options(m pele.TextMeasurer) pele.Options {
	opts := pele.Options{Measurer: m, Now: fixedNow}
	set.apply(&opts)
	return opts
}

// assertParity compares the port with Pele in goja for one source and option set. goja
// turns lone surrogates into U+FFFD in some string operations where V8 keeps them, so a
// case whose output holds one, from the source or from an escape, is left to the V8 test.
func assertParity(t *testing.T, o *oracle.Oracle, src string, set optionSet, m pele.TextMeasurer) {
	t.Helper()
	if hasLoneSurrogate(src) || outputHasLoneSurrogate(src, set.options(m)) {
		return
	}
	jsOpts := oracle.Options{Now: fixedNow, Extra: set.js}
	if m != nil {
		jsOpts.Measurer = jsMeasurer{m}
	}
	want, wantErr, err := o.Render(src, jsOpts)
	if err != nil {
		t.Fatalf("oracle render %q: %v", src, err)
	}
	compare(t, "goja/"+set.name, src, set.options(m), want, wantErr)
}

// compare renders src with the port and checks both the lossless and the public results
// against what upstream returned.
func compare(t *testing.T, label, src string, opts pele.Options, want *oracle.Result, wantErr *oracle.Error) {
	t.Helper()
	lossless, gotErr := core.Render(context.Background(), src, opts)
	public, publicErr := pele.Render(context.Background(), src, opts)

	if wantErr != nil {
		for name, got := range map[string]error{"lossless": gotErr, "public": publicErr} {
			var pe *pele.Error
			if !errors.As(got, &pe) {
				t.Errorf("%s: render %q (%s): got error %v, want PeleError %+v", label, src, name, got, *wantErr)
				return
			}
			want := *wantErr
			gotFields := oracle.Error{
				Code: string(pe.Code), Message: pe.Message, Type: pe.Type, MermaidType: pe.MermaidType,
				Line: pe.Line, Column: pe.Column, Snippet: pe.Snippet,
			}
			if name == "public" {
				want.Message, want.Snippet = jsstr.WellFormed(want.Message), jsstr.WellFormed(want.Snippet)
			}
			if gotFields != want {
				t.Errorf("%s: render %q (%s) error:\n got %+v\nwant %+v", label, src, name, gotFields, want)
			}
		}
		return
	}
	if gotErr != nil || publicErr != nil {
		t.Errorf("%s: render %q: got errors %v / %v, want success", label, src, gotErr, publicErr)
		return
	}
	if lossless.SVG != want.SVG {
		t.Errorf("%s: render %q SVG (lossless):\n got %q\nwant %q", label, src, lossless.SVG, want.SVG)
	}
	if public.SVG != jsstr.WellFormed(want.SVG) {
		t.Errorf("%s: render %q SVG (public):\n got %s\nwant %s", label, src, public.SVG, jsstr.WellFormed(want.SVG))
	}
	if public.Width != want.Width || public.Height != want.Height {
		t.Errorf("%s: render %q size = %vx%v, want %vx%v", label, src, public.Width, public.Height, want.Width, want.Height)
	}
	if string(public.Type) != want.Type || public.MermaidType != want.MermaidType {
		t.Errorf("%s: render %q type = %s/%s, want %s/%s", label, src, public.Type, public.MermaidType, want.Type, want.MermaidType)
	}
	gotLinks := make([]oracle.Link, len(public.Links))
	for i, l := range public.Links {
		gotLinks[i] = oracle.Link{ID: l.ID, Href: l.Href, Internal: l.Internal}
	}
	if !slices.Equal(gotLinks, want.Links) {
		t.Errorf("%s: render %q links = %+v, want %+v", label, src, gotLinks, want.Links)
	}
}

// loneSurrogateInputs carry lone surrogates into the output, directly or through YAML
// escapes. goja cannot judge them; V8 can.
var loneSurrogateInputs = []string{
	"info\ntitle a" + jsstr.FromUTF16([]uint16{0xd83d}),
	"info\ntitle " + jsstr.FromUTF16([]uint16{0xdf82}) + " low",
	"info\naccTitle: " + jsstr.FromUTF16([]uint16{0xdc00, 'x', 0xd800}),
	"info" + jsstr.FromUTF16([]uint16{0xdb78}),
	"---\ntitle: \"\\uD83D lone\"\n---\ninfo",
	"---\ntitle: \"\\uD83D\\uDE00 pair\"\n---\ninfo",
	"info\ntitle " + strings.Repeat("lone "+jsstr.FromUTF16([]uint16{0xdc01})+" ", 300),
}

// TestInfoParityV8 checks the port against Pele in Node, which keeps lone surrogates where
// goja does not, across the option sets and both measurers the driver knows.
func TestInfoParityV8(t *testing.T) {
	node, err := oracle.NewNode()
	if errors.Is(err, exec.ErrNotFound) {
		t.Skip("node is not installed")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer node.Close()
	inputs := append(append(corpus(t, "info-docs.json"), infoInputs...), loneSurrogateInputs...)
	for _, src := range inputs {
		for _, set := range optionSets {
			for _, measurer := range []string{"", "length"} {
				want, wantErr, err := node.Render(src, oracle.NodeOptions{
					Options: set.js, Measurer: measurer, Now: float64(fixedNow.UnixMilli()),
				})
				if err != nil {
					t.Fatalf("node render %q: %v", src, err)
				}
				var m pele.TextMeasurer
				if measurer == "length" {
					m = lengthMeasurer{}
				}
				compare(t, "v8/"+set.name+"/"+measurer, src, set.options(m), want, wantErr)
			}
		}
	}
}

func outputHasLoneSurrogate(src string, opts pele.Options) bool {
	res, err := core.Render(context.Background(), src, opts)
	if pe := (*pele.Error)(nil); errors.As(err, &pe) {
		return hasLoneSurrogate(pe.Message) || hasLoneSurrogate(pe.Snippet)
	}
	return hasLoneSurrogate(res.SVG)
}

func hasLoneSurrogate(s string) bool {
	return jsstr.WellFormed(jsstr.Normalize(s)) != jsstr.Normalize(s)
}

// FuzzInfoParityV8 fuzzes against Pele in Node, which is slower than goja but judges lone
// surrogates correctly.
func FuzzInfoParityV8(f *testing.F) {
	for _, src := range append(infoInputs, loneSurrogateInputs...) {
		if len(src) < 1000 {
			f.Add(src)
		}
	}
	node, err := oracle.NewNode()
	if errors.Is(err, exec.ErrNotFound) {
		f.Skip("node is not installed")
	}
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() { _ = node.Close() })
	f.Fuzz(func(t *testing.T, src string) {
		if typ, ok := pele.DetectType(src); ok && typ != "info" {
			t.Skip("not an info diagram")
		}
		want, wantErr, err := node.Render(src, oracle.NodeOptions{Now: float64(fixedNow.UnixMilli())})
		if err != nil {
			t.Fatalf("node render %q: %v", src, err)
		}
		compare(t, "v8", src, optionSets[0].options(nil), want, wantErr)
	})
}
