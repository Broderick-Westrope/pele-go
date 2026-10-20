package omap_test

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/dop251/goja"

	"github.com/Broderick-Westrope/pele-go/internal/omap"
)

// TestMapMatchesJSMap applies the same random operations to a JS Map and to omap.Map and
// checks they iterate in the same order, including keys re-added after a delete.
func TestMapMatchesJSMap(t *testing.T) {
	vm := goja.New()
	rng := rand.New(rand.NewPCG(13, 14))
	keys := []string{"a", "b", "c", "1", "0", "10", "__proto__", ""}
	for range 300 {
		js, err := vm.RunString(`new Map()`)
		if err != nil {
			t.Fatal(err)
		}
		jsMap := js.ToObject(vm)
		set, _ := goja.AssertFunction(jsMap.Get("set"))
		del, _ := goja.AssertFunction(jsMap.Get("delete"))
		var m omap.Map[string, int]
		for i := range rng.IntN(20) {
			k := keys[rng.IntN(len(keys))]
			if rng.IntN(3) == 0 {
				m.Delete(k)
				_, _ = del(jsMap, vm.ToValue(k))
			} else {
				m.Set(k, i)
				_, _ = set(jsMap, vm.ToValue(k), vm.ToValue(i))
			}
		}
		entries, err := vm.RunString(`(m) => [...m.entries()].map(([k, v]) => k + '=' + v)`)
		if err != nil {
			t.Fatal(err)
		}
		fn, _ := goja.AssertFunction(entries)
		v, err := fn(goja.Undefined(), jsMap)
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		for _, e := range v.Export().([]any) {
			want = append(want, e.(string))
		}
		var got []string
		for k, v := range m.All() {
			got = append(got, k+"="+itoa(v))
		}
		if !slices.Equal(got, want) {
			t.Fatalf("entries %q, want %q", got, want)
		}
		if m.Len() != len(want) {
			t.Fatalf("Len = %d, want %d", m.Len(), len(want))
		}
	}
}

// TestAllVisitsEntriesAddedDuringIteration matches JS, where a Map iterator reaches entries
// set after it started.
func TestAllVisitsEntriesAddedDuringIteration(t *testing.T) {
	var m omap.Map[int, bool]
	m.Set(1, true)
	var seen []int
	for k := range m.All() {
		seen = append(seen, k)
		if k < 3 {
			m.Set(k+1, true)
		}
	}
	if want := []int{1, 2, 3}; !slices.Equal(seen, want) {
		t.Fatalf("visited %v, want %v", seen, want)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
