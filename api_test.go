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
