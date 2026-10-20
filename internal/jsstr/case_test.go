package jsstr_test

import (
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// The expected values are Node 22's (V8, Unicode 16) toLowerCase.
func TestToLowerCaseMatchesV8(t *testing.T) {
	tests := map[string]struct {
		in, want string
	}{
		"ascii":                          {in: "HELLO", want: "hello"},
		"sharp s stays":                  {in: "Straße", want: "straße"},
		"title case":                     {in: "ǅ", want: "ǆ"},
		"dotted capital i":               {in: "\u0130x", want: "i\u0307x"},
		"lone sigma":                     {in: "Σ", want: "σ"},
		"final sigma":                    {in: "AΣ", want: "aς"},
		"final sigma before ignorable":   {in: "AΣ.", want: "aς."},
		"medial sigma":                   {in: "AΣb", want: "aσb"},
		"ano teleia is ignorable":        {in: "A\u0387Σ", want: "a\u0387ς"},
		"fullwidth apostrophe ignorable": {in: "AΣ\uff07", want: "aς\uff07"},
		"unicode 16 letters":             {in: "\u1c89\ua7cb\ua7cc\ua7da\ua7dc", want: "\u1c8a\u0264\ua7cd\ua7db\u019b"},
		"unicode 16 astral letters":      {in: "\U00010d50\U00010d65", want: "\U00010d70\U00010d85"},
		"unicode 16 letter is cased":     {in: "\U00010d50Σ", want: "\U00010d70ς"},
		"unicode 16 lower letter after":  {in: "AΣ\U00010d70", want: "aσ\U00010d70"},
		"lone surrogate kept":            {in: "x\xed\xa0\x80Σ", want: "x\xed\xa0\x80σ"},
		"medial sigma before ano teleia": {in: "ΑΣ\u0387", want: "ας\u0387"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := jsstr.ToLowerCase(tc.in); got != tc.want {
				t.Errorf("ToLowerCase(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if got, want := jsstr.ChangesWhenLowered(tc.in), tc.want != tc.in; got != want {
				t.Errorf("ChangesWhenLowered(%q) = %v, want %v", tc.in, got, want)
			}
		})
	}
}
