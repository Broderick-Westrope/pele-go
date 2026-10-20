package jsstr_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/dop251/goja"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// randomUnits returns UTF-16 code units mixing ASCII, JS whitespace, BMP letters, surrogate
// pairs and lone surrogates.
func randomUnits(rng *rand.Rand) []uint16 {
	pool := []uint16{
		'a', 'Z', ' ', '\t', '\n', '\r', 0x0b, 0x0c, 0xa0, 0x1680, 0x2000, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f,
		0x3000, 0xfeff, 0x85, 0x180e, 0x200b, 'é', 0x4e2d, 0xd83d, 0xde00, 0xdc00, 0xdbff,
	}
	units := make([]uint16, rng.IntN(12))
	for i := range units {
		units[i] = pool[rng.IntN(len(pool))]
	}
	return units
}

func TestRoundTripKeepsLoneSurrogates(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	for range 5000 {
		units := randomUnits(rng)
		s := jsstr.FromUTF16(units)
		if got := jsstr.UTF16(s); !slices.Equal(got, units) {
			t.Fatalf("UTF16(FromUTF16(%x)) = %x", units, got)
		}
		if got := jsstr.Len(s); got != len(units) {
			t.Fatalf("Len(%x) = %d, want %d", units, got, len(units))
		}
	}
}

func TestInvalidBytesBecomeReplacementCharacters(t *testing.T) {
	got := jsstr.UTF16("a\xffb\xc0")
	want := []uint16{'a', 0xfffd, 'b', 0xfffd}
	if !slices.Equal(got, want) {
		t.Fatalf("UTF16 = %x, want %x", got, want)
	}
}

func TestTrimMatchesJS(t *testing.T) {
	vm := goja.New()
	call := func(method string, units []uint16) []uint16 {
		fn, ok := goja.AssertFunction(vm.Get("String").ToObject(vm).Get("prototype").ToObject(vm).Get(method))
		if !ok {
			t.Fatalf("%s is not a function", method)
		}
		v, err := fn(vm.ToValue(goja.StringFromUTF16(units)))
		if err != nil {
			t.Fatal(err)
		}
		s := v.(goja.String)
		out := make([]uint16, s.Length())
		for i := range out {
			out[i] = s.CharAt(i)
		}
		return out
	}
	rng := rand.New(rand.NewPCG(7, 8))
	for range 3000 {
		units := randomUnits(rng)
		// goja's trim turns lone surrogates into U+FFFD where V8 keeps them, so they are
		// checked against the spec in TestTrimKeepsLoneSurrogates instead.
		if hasLoneSurrogate(units) {
			continue
		}
		s := jsstr.FromUTF16(units)
		for method, got := range map[string]string{
			"trim":      jsstr.Trim(s),
			"trimStart": jsstr.TrimStart(s),
			"trimEnd":   jsstr.TrimEnd(s),
		} {
			if want := call(method, units); !slices.Equal(jsstr.UTF16(got), want) {
				t.Fatalf("%s(%x) = %x, want %x", method, units, jsstr.UTF16(got), want)
			}
		}
	}
}

func hasLoneSurrogate(units []uint16) bool {
	for i := 0; i < len(units); i++ {
		switch u := units[i]; {
		case u >= 0xd800 && u <= 0xdbff && i+1 < len(units) && units[i+1] >= 0xdc00 && units[i+1] <= 0xdfff:
			i++
		case u >= 0xd800 && u <= 0xdfff:
			return true
		}
	}
	return false
}

func TestTrimKeepsLoneSurrogates(t *testing.T) {
	s := jsstr.FromUTF16([]uint16{0x3000, 0xd83d, ' ', 0xdc00, '\n'})
	want := []uint16{0xd83d, ' ', 0xdc00}
	if got := jsstr.UTF16(jsstr.Trim(s)); !slices.Equal(got, want) {
		t.Fatalf("Trim = %x, want %x", got, want)
	}
}

func TestWellFormedReplacesLoneSurrogates(t *testing.T) {
	s := jsstr.FromUTF16([]uint16{'a', 0xd83d, 0xde00, 0xd800, 'b', 0xdc00})
	want := "a\U0001F600\uFFFDb\uFFFD"
	if got := jsstr.WellFormed(s); got != want {
		t.Fatalf("WellFormed = %q, want %q", got, want)
	}
}

func TestSliceCountsCodeUnits(t *testing.T) {
	s := jsstr.FromUTF16([]uint16{0x3000, 'a', 0xd83d, 0xde00, 'b'})
	tests := map[string]struct {
		from, to int
		want     []uint16
	}{
		"from start":         {from: 0, to: 2, want: []uint16{0x3000, 'a'}},
		"whole pair":         {from: 2, to: 4, want: []uint16{0xd83d, 0xde00}},
		"splits a pair":      {from: 3, to: 5, want: []uint16{0xde00, 'b'}},
		"past the end":       {from: 4, to: 99, want: []uint16{'b'}},
		"empty when crossed": {from: 3, to: 1, want: []uint16{}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := jsstr.UTF16(jsstr.Slice(s, tc.from, tc.to)); !slices.Equal(got, tc.want) {
				t.Fatalf("Slice(%d, %d) = %x, want %x", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestCanonicalPairsAdjacentSurrogates(t *testing.T) {
	joined := jsstr.FromUTF16([]uint16{'a', 0xd83d}) + jsstr.FromUTF16([]uint16{0xde00, 'b'})
	if got, want := jsstr.Canonical(joined), "a\U0001F600b"; got != want {
		t.Fatalf("Canonical = %q, want %q", got, want)
	}
	if got, want := jsstr.WellFormed(joined), "a\U0001F600b"; got != want {
		t.Fatalf("WellFormed = %q, want %q", got, want)
	}
}
