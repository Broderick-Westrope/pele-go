package jsnum_test

import (
	"math"
	"strings"
	"testing"

	"github.com/dop251/goja"

	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
)

func TestParseIntMatchesJS(t *testing.T) {
	vm := goja.New()
	parseInt, ok := goja.AssertFunction(vm.Get("parseInt"))
	if !ok {
		t.Fatal("parseInt is not a function")
	}
	inputs := []string{
		"", " ", "0", "+0", "12", " 12 ", "\u00a012\u3000", "+12", "-12.5", ".5", "5.", "1e3", "12px",
		"0x1F", "0X1f", "0x", "-0x1F", "+0x10", "0xg", "0x1g", "0o17", "0b101", "Infinity", "-Infinity", "NaN",
		"1_000", "00012", "\t\n42\r", "4 2", "-", "+", "+-1", "١٢", "\u202812", "9007199254740993",
		"0x1fffffffffffff",
	}
	for _, in := range inputs {
		want, err := parseInt(goja.Undefined(), vm.ToValue(in))
		if err != nil {
			t.Fatal(err)
		}
		got := jsnum.ParseInt(in)
		w := want.ToFloat()
		if !(got == w && math.Signbit(got) == math.Signbit(w)) && !(math.IsNaN(got) && math.IsNaN(w)) {
			t.Errorf("ParseInt(%q) = %v, want %v", in, got, w)
		}
	}
}

// goja loses the sign of -0 and rounds long digit runs twice; these values are V8's.
func TestParseIntMatchesV8(t *testing.T) {
	if got := jsnum.ParseInt("-0"); got != 0 || !math.Signbit(got) {
		t.Fatalf("ParseInt(-0) = %v, want -0", got)
	}
	if got, want := jsnum.ParseInt("99999999999999999999999"), 1e23; got != want {
		t.Fatalf("ParseInt = %v, want %v", got, want)
	}
	// goja's answer for this one differs between arm64 and amd64.
	if got, want := jsnum.ParseInt("123456789012345678901234567890"), 1.2345678901234568e+29; got != want {
		t.Fatalf("ParseInt = %v, want %v", got, want)
	}
	if got, want := jsnum.ParseInt("10000000000000000000000000000001"), 1e31; got != want {
		t.Fatalf("ParseInt = %v, want %v", got, want)
	}
	if got := jsnum.ParseInt("1" + strings.Repeat("0", 400)); !math.IsInf(got, 1) {
		t.Fatalf("ParseInt of 401 digits = %v, want +Inf", got)
	}
}
