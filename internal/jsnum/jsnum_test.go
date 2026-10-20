package jsnum_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/dop251/goja"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
)

func TestFormatMatchesJS(t *testing.T) {
	vm := goja.New()
	toString, ok := goja.AssertFunction(vm.Get("String"))
	if !ok {
		t.Fatal("String is not a function")
	}
	values := []float64{
		0, math.Copysign(0, -1), 1, -1, 0.1, 0.5, 1.5, 100, 123.45, -123.45, 1e21, 1e20, 999999999999999900000,
		1e-6, 1e-7, 1.5e-7, 0.000001234, 123456789012345680000, 5e-324, math.MaxFloat64, 1 / 3.0, 2 / 3.0,
		math.NaN(), math.Inf(1), math.Inf(-1), 4.35, 0.07, 1e100, -1e-100, 9007199254740993, 1e15 + 0.3,
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		values = append(values, math.Float64frombits(rng.Uint64()))
		values = append(values, float64(rng.IntN(1_000_000))/100)
	}
	for _, v := range values {
		want, err := toString(goja.Undefined(), vm.ToValue(v))
		if err != nil {
			t.Fatal(err)
		}
		if got := jsnum.Format(v); got != want.String() {
			t.Errorf("Format(%v) = %q, want %q", v, got, want.String())
		}
	}
}
