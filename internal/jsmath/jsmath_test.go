package jsmath_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/dop251/goja"

	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
)

func TestRoundMatchesJS(t *testing.T) {
	vm := goja.New()
	round, ok := goja.AssertFunction(vm.Get("Math").ToObject(vm).Get("round"))
	if !ok {
		t.Fatal("Math.round is not a function")
	}
	values := []float64{
		0.5, -0.5, 1.5, -1.5, 2.5, -2.5, 0.49999999999999994, -0.49999999999999994, 4503599627370495.5,
		-4503599627370495.5, 1e300, -1e300, math.NaN(), math.Inf(1), math.Inf(-1), math.Copysign(0, -1), -0.2,
	}
	rng := rand.New(rand.NewPCG(3, 4))
	for range 2000 {
		values = append(values, (rng.Float64()-0.5)*1e6, float64(rng.IntN(20000)-10000)/2)
	}
	for _, v := range values {
		want, err := round(goja.Undefined(), vm.ToValue(v))
		if err != nil {
			t.Fatal(err)
		}
		got := jsmath.Round(v)
		w := want.ToFloat()
		if !(got == w && math.Signbit(got) == math.Signbit(w)) && !(math.IsNaN(got) && math.IsNaN(w)) {
			t.Errorf("Round(%v) = %v, want %v", v, got, w)
		}
	}
}
