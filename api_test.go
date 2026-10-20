package pele_test

import (
	"context"
	"errors"
	"testing"
	"unicode/utf8"

	pele "github.com/Broderick-Westrope/pele-go"
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

func TestRenderIsSafeForConcurrentUse(t *testing.T) {
	sources := []string{"info", "info\ntitle One", "info\ntitle Two\naccTitle: t", "nope"}
	want := make([]pele.Result, len(sources))
	for i, src := range sources {
		want[i], _ = pele.Render(context.Background(), src, pele.Options{})
	}
	done := make(chan struct{})
	for range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := range 50 {
				k := i % len(sources)
				got, _ := pele.Render(context.Background(), sources[k], pele.Options{})
				if got.SVG != want[k].SVG {
					t.Errorf("concurrent render of %q differs", sources[k])
					return
				}
			}
		}()
	}
	for range 8 {
		<-done
	}
}
