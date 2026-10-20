package core

import (
	"context"
	"testing"
)

// The registry's seal has no exported observer short of a panicking Register, and any
// render elsewhere in the test binary seals it, so this test resets it directly.
func TestOnlyRenderingSealsTheRegistry(t *testing.T) {
	was := sealed.Load()
	sealed.Store(false)
	t.Cleanup(func() { sealed.Store(was) })

	Supports("info")
	Registered("info")
	if _, ok := DetectType("info"); !ok {
		t.Fatal("DetectType(info) found no type")
	}
	if sealed.Load() {
		t.Fatal("Supports or Registered sealed the registry")
	}
	_, _ = Parse(context.Background(), "info", Options{})
	if !sealed.Load() {
		t.Fatal("Parse did not seal the registry")
	}
}
