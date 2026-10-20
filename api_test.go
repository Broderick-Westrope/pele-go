package pele_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	pele "github.com/Broderick-Westrope/pele-go"
	"github.com/Broderick-Westrope/pele-go/internal/core"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

func TestPublicErrorsAreValidUTF8(t *testing.T) {
	src := "info" + jsstr.FromUTF16([]uint16{0xdb78})
	_, err := pele.Render(context.Background(), src, pele.Options{})
	var pe *pele.Error
	if !errors.As(err, &pe) {
		t.Fatalf("Render error = %v, want *pele.Error", err)
	}
	if !utf8.ValidString(pe.Message) || !utf8.ValidString(pe.Snippet) {
		t.Fatalf("error has invalid UTF-8: %q / %q", pe.Message, pe.Snippet)
	}
}

func TestRenderStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := pele.Render(ctx, "info", pele.Options{})
	var pe *pele.Error
	if !errors.As(err, &pe) || pe.Code != pele.CodeCanceled {
		t.Fatalf("Render error = %v, want a %s *pele.Error", err, pele.CodeCanceled)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Render error = %v, want it to wrap context.Canceled", err)
	}
}

func TestEscapedSurrogatePairsBecomeOneCharacter(t *testing.T) {
	res, err := pele.Render(context.Background(), "---\ntitle: \"\\uD83D\\uDE00 smile\"\n---\ninfo", pele.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.SVG, "\U0001F600 smile") {
		t.Fatalf("SVG does not contain the emoji: %s", res.SVG)
	}
}

func TestLosslessOutputPairsEscapedSurrogates(t *testing.T) {
	res, err := core.Render(context.Background(), "---\ntitle: \"\\uD83D\\uDE00 smile\"\n---\ninfo", pele.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.SVG, "\U0001F600 smile") {
		t.Fatalf("SVG does not contain the emoji: %q", res.SVG)
	}
}

// cancelAfter is a context that is canceled from its nth call to Err on, so a test can
// cancel a render once it is past the checks the core makes before calling the renderer.
type cancelAfter struct {
	context.Context
	calls, n int
}

func (c *cancelAfter) Err() error {
	c.calls++
	if c.calls >= c.n {
		return context.Canceled
	}
	return nil
}

func TestVennRenderStopsOnCanceledContext(t *testing.T) {
	src := "venn-beta\nset A\nset B\nset C\n"
	ctx := &cancelAfter{Context: context.Background(), n: 4}
	_, err := pele.Render(ctx, src, pele.Options{})
	var pe *pele.Error
	if !errors.As(err, &pe) || pe.Code != pele.CodeCanceled {
		t.Fatalf("Render error = %v, want a %s *pele.Error", err, pele.CodeCanceled)
	}
	if ctx.calls != 4 {
		t.Fatalf("context checked %d times, want the renderer's first check to stop it", ctx.calls)
	}
}

func TestPtrPointsToACopy(t *testing.T) {
	v := 12.5
	p := pele.Ptr(v)
	*p = 1
	if v != 12.5 {
		t.Fatalf("writing through Ptr(v) changed v to %v", v)
	}
}

func TestDefaultMeasurerIsWhatANilMeasurerUses(t *testing.T) {
	src := "flowchart LR\n  A[Mermaid text] --> B[Some wider SVG label]"
	want, err := pele.Render(context.Background(), src, pele.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := pele.Render(context.Background(), src, pele.Options{Measurer: pele.DefaultMeasurer()})
	if err != nil {
		t.Fatal(err)
	}
	if got.SVG != want.SVG {
		t.Fatalf("SVG with DefaultMeasurer() differs from SVG with a nil Measurer:\n got %s\nwant %s", got.SVG, want.SVG)
	}
	if w := pele.DefaultMeasurer().Width("Mermaid", 16, 0); w <= 0 {
		t.Fatalf("DefaultMeasurer().Width = %v, want a positive width", w)
	}
}

type panickingMeasurer struct{}

func (panickingMeasurer) Width(string, float64, pele.Style) float64 { panic("measurer exploded") }

func TestPanicsInHostCodeBecomeInternalErrors(t *testing.T) {
	tests := map[string]struct {
		src  string
		opts pele.Options
		want string
	}{
		"measurer": {
			src:  "flowchart LR\n  A --> B",
			opts: pele.Options{Measurer: panickingMeasurer{}},
			want: "measurer exploded",
		},
		"icons": {
			src:  "architecture-beta\n  service db(database)[Database]",
			opts: pele.Options{Icons: func(string) string { panic(errors.New("icons exploded")) }},
			want: "icons exploded",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := pele.Render(context.Background(), tc.src, tc.opts)
			var pe *pele.Error
			if !errors.As(err, &pe) || pe.Code != pele.CodeInternal {
				t.Fatalf("Render error = %v, want a %s *pele.Error", err, pele.CodeInternal)
			}
			if !strings.Contains(pe.Message, tc.want) {
				t.Errorf("message %q does not name the panic value %q", pe.Message, tc.want)
			}
			var panicErr *pele.PanicError
			if !errors.As(err, &panicErr) {
				t.Fatalf("Render error = %v, want it to wrap a *pele.PanicError", err)
			}
			if !strings.Contains(string(panicErr.Stack), "api_test.go") {
				t.Errorf("stack does not reach the panic site in this file:\n%s", panicErr.Stack)
			}
		})
	}
}

func TestPanicErrorUnwrapsAPanickedError(t *testing.T) {
	sentinel := errors.New("icons exploded")
	_, err := pele.Render(context.Background(), "architecture-beta\n  service db(database)[Database]",
		pele.Options{Icons: func(string) string { panic(sentinel) }})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Render error = %v, want it to wrap the panicked error", err)
	}
}

func TestRenderTreatsANilContextAsBackground(t *testing.T) {
	var ctx context.Context
	res, err := pele.Render(ctx, "info", pele.Options{})
	if err != nil {
		t.Fatalf("Render with a nil context: %v", err)
	}
	if res.Type != "info" {
		t.Fatalf("Render with a nil context: type %q, want info", res.Type)
	}
}
