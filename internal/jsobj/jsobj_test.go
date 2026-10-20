package jsobj_test

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/dop251/goja"

	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
)

func TestKeysMatchJS(t *testing.T) {
	vm := goja.New()
	pool := []string{"0", "1", "10", "2", "01", "-1", "4294967294", "4294967295", "a", "b", "__proto__", "1.5", "", " 1", "9"}
	rng := rand.New(rand.NewPCG(9, 10))
	for range 500 {
		obj := jsobj.New()
		js := vm.NewObject()
		for range rng.IntN(8) {
			k := pool[rng.IntN(len(pool))]
			obj.Set(k, nil)
			if err := js.DefineDataProperty(k, goja.Null(), goja.FLAG_TRUE, goja.FLAG_TRUE, goja.FLAG_TRUE); err != nil {
				t.Fatal(err)
			}
		}
		if got, want := obj.Keys(), js.Keys(); !slices.Equal(got, want) {
			t.Fatalf("Keys = %q, want %q", strings.Join(got, ","), strings.Join(want, ","))
		}
	}
}
