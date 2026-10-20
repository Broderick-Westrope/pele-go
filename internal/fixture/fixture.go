// Package fixture replays the cases scripts/gen-shared-cases.mjs writes: calls of upstream's
// functions under V8 and what they returned, for differential tests of the ported modules.
package fixture

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsjson"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// Args are a call's arguments as JSON.parse read them.
type Args []any

// Str returns argument i as a string.
func (a Args) Str(i int) string { s, _ := a[i].(string); return s }

// Num returns argument i as a number.
func (a Args) Num(i int) float64 { f, _ := a[i].(float64); return f }

// Bool returns argument i as a boolean.
func (a Args) Bool(i int) bool { b, _ := a[i].(bool); return b }

// Nums returns argument i, an array of numbers.
func (a Args) Nums(i int) []float64 {
	list, _ := a[i].([]any)
	out := make([]float64, len(list))
	for k, v := range list {
		out[k], _ = v.(float64)
	}
	return out
}

// AllNums returns every argument as a number.
func (a Args) AllNums() []float64 {
	out := make([]float64, len(a))
	for k, v := range a {
		out[k], _ = v.(float64)
	}
	return out
}

// Strs returns argument i, an array of strings.
func (a Args) Strs(i int) []string {
	list, _ := a[i].([]any)
	out := make([]string, len(list))
	for k, v := range list {
		out[k], _ = v.(string)
	}
	return out
}

// Obj returns argument i as an object, or nil.
func (a Args) Obj(i int) *jsobj.Object { o, _ := a[i].(*jsobj.Object); return o }

// Undefined reports whether argument i was undefined or null, or not given.
func (a Args) Undefined(i int) bool { return i >= len(a) || a[i] == nil }

// Func is the port's version of an upstream function: it returns a JSON-like value (nil,
// bool, numbers, string, slices, map[string]any) to compare with what upstream returned.
type Func func(a Args) any

// Run replays every case in path, a file the generator wrote, against fns by name. A case
// for a function fns does not have fails the test, so a fixture cannot go unchecked.
func Run(t *testing.T, path string, fns map[string]Func) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := jsjson.Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	failures := 0
	for i, raw := range parsed.([]any) {
		c := raw.(*jsobj.Object)
		name, _ := c.Get("fn")
		fn, ok := fns[name.(string)]
		if !ok {
			t.Fatalf("case %d: no port of %s", i, name)
		}
		argList, _ := c.Get("args")
		args := Args(argList.([]any))
		got, threw, panicked := call(fn, args)
		if message, throws := c.Get("throws"); throws {
			if !panicked || threw != message {
				t.Errorf("case %d: %s(%s) = %s, threw %q; want it to throw %q", i, name, Canon([]any(args)), Canon(got), threw, message)
				failures++
			}
		} else {
			want, _ := c.Get("want")
			if panicked || Canon(got) != Canon(want) {
				t.Errorf("case %d: %s(%s)\n got %s (threw %q)\nwant %s", i, name, Canon([]any(args)), Canon(got), threw, Canon(want))
				failures++
			}
		}
		if failures > 20 {
			t.Fatal("too many failures")
		}
	}
}

// call calls fn, catching what it throws: a Go error, as the port's *perr.Error is, gives
// its message, as upstream's thrown Error does.
func call(fn Func, args Args) (got any, threw string, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			if err, ok := r.(error); ok {
				threw = err.Error()
			} else {
				threw = fmt.Sprint(r)
			}
		}
	}()
	return fn(args), "", false
}

// Canon writes a JSON-like value one way, whichever side it came from: numbers as JS
// writes them (non-finite ones as JSON's null), strings by code unit, object keys sorted.
func Canon(v any) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(v)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "null"
		}
		return jsnum.Format(v)
	case int:
		return strconv.Itoa(v)
	case string:
		return strconv.QuoteToASCII(jsstr.Normalize(v))
	case []any:
		parts := make([]string, len(v))
		for i, e := range v {
			parts[i] = Canon(e)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case Args:
		return Canon([]any(v))
	case []float64:
		parts := make([]string, len(v))
		for i, e := range v {
			parts[i] = Canon(e)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case []string:
		parts := make([]string, len(v))
		for i, e := range v {
			parts[i] = Canon(e)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = Canon(k) + ":" + Canon(v[k])
		}
		return "{" + strings.Join(parts, ",") + "}"
	case *jsobj.Object:
		m := map[string]any{}
		for _, k := range v.Keys() {
			m[k], _ = v.Get(k)
		}
		return Canon(m)
	case error:
		return v.Error()
	default:
		return fmt.Sprintf("%T(%v)", v, v)
	}
}
