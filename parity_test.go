package pele_test

import (
	"context"
	"errors"
	"os"
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
		assertParity(t, o, src, nil)
		assertParity(t, o, src, lengthMeasurer{})
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
		assertParity(t, o, src, nil)
	})
}

func assertParity(t *testing.T, o *oracle.Oracle, src string, m pele.TextMeasurer) {
	t.Helper()
	opts := pele.Options{Measurer: m, Now: fixedNow}
	jsOpts := oracle.Options{Now: fixedNow}
	if m != nil {
		jsOpts.Measurer = jsMeasurer{m}
	}
	want, wantErr, err := o.Render(src, jsOpts)
	if err != nil {
		t.Fatalf("oracle render %q: %v", src, err)
	}
	lossless, gotErr := core.Render(context.Background(), src, opts)
	public, publicErr := pele.Render(context.Background(), src, opts)

	if wantErr != nil {
		for _, got := range []error{gotErr, publicErr} {
			var pe *pele.Error
			if !errors.As(got, &pe) {
				t.Errorf("render %q: got error %v, want PeleError %+v", src, got, *wantErr)
				return
			}
			gotFields := oracle.Error{
				Code: string(pe.Code), Message: pe.Message, Type: pe.Type, MermaidType: pe.MermaidType,
				Line: pe.Line, Column: pe.Column, Snippet: pe.Snippet,
			}
			if gotFields != *wantErr {
				t.Errorf("render %q error:\n got %+v\nwant %+v", src, gotFields, *wantErr)
			}
		}
		return
	}
	if gotErr != nil || publicErr != nil {
		t.Errorf("render %q: got errors %v / %v, want success", src, gotErr, publicErr)
		return
	}
	if lossless.SVG != want.SVG {
		t.Errorf("render %q SVG (lossless):\n got %s\nwant %s", src, lossless.SVG, want.SVG)
	}
	if public.SVG != jsstr.WellFormed(want.SVG) {
		t.Errorf("render %q SVG (public):\n got %s\nwant %s", src, public.SVG, jsstr.WellFormed(want.SVG))
	}
	if public.Width != want.Width || public.Height != want.Height {
		t.Errorf("render %q size = %vx%v, want %vx%v", src, public.Width, public.Height, want.Width, want.Height)
	}
	if string(public.Type) != want.Type || public.MermaidType != want.MermaidType {
		t.Errorf("render %q type = %s/%s, want %s/%s", src, public.Type, public.MermaidType, want.Type, want.MermaidType)
	}
	gotLinks := make([]oracle.Link, len(public.Links))
	for i, l := range public.Links {
		gotLinks[i] = oracle.Link{ID: l.ID, Href: l.Href, Internal: l.Internal}
	}
	if !slices.Equal(gotLinks, want.Links) {
		t.Errorf("render %q links = %+v, want %+v", src, gotLinks, want.Links)
	}
}
