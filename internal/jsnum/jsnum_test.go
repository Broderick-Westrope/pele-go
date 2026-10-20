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

func TestParseMatchesJS(t *testing.T) {
	vm := goja.New()
	number, ok := goja.AssertFunction(vm.Get("Number"))
	if !ok {
		t.Fatal("Number is not a function")
	}
	inputs := []string{
		"", " ", "0", "-0", "12", " 12 ", "\u00a012\u3000", "+12", "-12.5", ".5", "5.", ".", "1e3", "1E+3", "1e-3",
		"1e", "e3", "0x1F", "0X1f", "0x", "-0x1F", "0o17", "0b101", "0b2", "Infinity", "-Infinity", "+Infinity",
		"infinity", "inf", "NaN", "1_000", "0x1_0", "12px", "1.2.3", "00012", "0012.5", "1e400", "-1e400", "1e-400",
		"123456789012345678901234567890", "0.1e1", "\t\n42\r", "4 2",
	}
	for _, in := range inputs {
		want, err := number(goja.Undefined(), vm.ToValue(in))
		if err != nil {
			t.Fatal(err)
		}
		got := jsnum.Parse(in)
		w := want.ToFloat()
		if !(got == w && math.Signbit(got) == math.Signbit(w)) && !(math.IsNaN(got) && math.IsNaN(w)) {
			t.Errorf("Parse(%q) = %v, want %v", in, got, w)
		}
	}
}

// goja returns NaN for hex integers past 2^64; V8 rounds them, as the spec says.
func TestParseRoundsLargeHexLikeV8(t *testing.T) {
	if got, want := jsnum.Parse("0xFFFFFFFFFFFFFFFFFFFF"), 1.2089258196146292e+24; got != want {
		t.Fatalf("Parse = %v, want %v", got, want)
	}
}

func TestParseFloatMatchesJS(t *testing.T) {
	vm := goja.New()
	parseFloat, ok := goja.AssertFunction(vm.Get("parseFloat"))
	if !ok {
		t.Fatal("parseFloat is not a function")
	}
	inputs := []string{
		"", "1", "-1", "+1", "1.5px", "  \n\t 12", ".5", "5.", ".", "-.5e3x", "1e", "1e+", "1e-2", "1.2.3",
		"Infinity", "-Infinityx", "+Inf", "infinity", "0x10", "1_000", "00012", "-0", "1e400", "1e-400",
		"\u00a012", "\u202812", "12\u00a0", "e5", "..5", "+-1", "١٢", "3.14159265358979323846264338327950288",
		"9007199254740993", "0.1e1.5", " +.e1", "5e-324", "2.4703282292062328e-324",
	}
	for _, in := range inputs {
		want, err := parseFloat(goja.Undefined(), vm.ToValue(in))
		if err != nil {
			t.Fatal(err)
		}
		got := jsnum.ParseFloat(in)
		w := want.ToFloat()
		if !(got == w && math.Signbit(got) == math.Signbit(w)) && !(math.IsNaN(got) && math.IsNaN(w)) {
			t.Errorf("ParseFloat(%q) = %v, want %v", in, got, w)
		}
	}
}
