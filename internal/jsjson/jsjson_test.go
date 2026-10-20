package jsjson_test

import (
	"testing"

	"github.com/dop251/goja"

	"github.com/Broderick-Westrope/pele-go/internal/jsjson"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

// describe renders a value the same way for Go and for JS, so the two can be compared.
const describeJS = `(function describe(v) {
	if (v === null) return 'null';
	if (Array.isArray(v)) return '[' + v.map(describe).join(',') + ']';
	if (typeof v === 'object') return '{' + Object.keys(v).map((k) => JSON.stringify(k) + ':' + describe(v[k])).join(',') + '}';
	if (typeof v === 'string') return 's' + JSON.stringify(v);
	return typeof v + ':' + String(v);
})`

func describe(v any) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case []any:
		s := "["
		for i, e := range v {
			if i > 0 {
				s += ","
			}
			s += describe(e)
		}
		return s + "]"
	case *jsobj.Object:
		s := "{"
		for i, k := range v.Keys() {
			if i > 0 {
				s += ","
			}
			e, _ := v.Get(k)
			s += quote(k) + ":" + describe(e)
		}
		return s + "}"
	case string:
		return "s" + quote(v)
	case bool:
		return "boolean:" + jsobj.ToString(v)
	default:
		return "number:" + jsobj.ToString(v)
	}
}

var quoteFn goja.Callable
var quoteVM *goja.Runtime

func quote(s string) string {
	v, err := quoteFn(goja.Undefined(), quoteVM.ToValue(goja.StringFromUTF16(jsstr.UTF16(s))))
	if err != nil {
		panic(err)
	}
	return v.String()
}

func TestParseMatchesJS(t *testing.T) {
	vm := goja.New()
	quoteVM = vm
	quoteFn, _ = goja.AssertFunction(vm.Get("JSON").ToObject(vm).Get("stringify"))
	desc, err := vm.RunString(describeJS)
	if err != nil {
		t.Fatal(err)
	}
	describeFn, _ := goja.AssertFunction(desc)
	parse, _ := goja.AssertFunction(vm.Get("JSON").ToObject(vm).Get("parse"))

	inputs := []string{
		`{}`, `[]`, `null`, `true`, `false`, `0`, `-0`, `1.5e3`, `"x"`, ` { "a" : 1 , "b":[1,2,{"c":null}] } `,
		`{"b":1,"a":2,"1":3,"0":4,"b":5}`, `{"__proto__":{"x":1}}`, `"\u00e9\n\t\"\\\/\b\f\r"`, `"\uD83D\uDE00"`,
		`{"theme":"dark","themeVariables":{"primaryColor":"#ff0000"}}`, `-1E-2`,
		`{`, `{"a":1,}`, `[1,]`, `01`, `1.`, `.5`, `+1`, `"unterminated`, `'single'`, `{"a" 1}`, `nul`, `"\x"`,
		"\"tab\there\"", `[1 2]`, `{"a":1}x`, "\u00a0{}", "\t\n\r {}", `"\u12"`, `{"a":{"b":{"c":[[[]]]}}}`,
		`123456789012345678901234567890`, `0.1`, `-`, `""`, `[true,false,null]`,
	}
	for _, in := range inputs {
		jsVal, jsErr := parse(goja.Undefined(), vm.ToValue(in))
		got, goErr := jsjson.Parse(in)
		if (jsErr != nil) != (goErr != nil) {
			t.Errorf("Parse(%q) error = %v, want error %v", in, goErr, jsErr)
			continue
		}
		if jsErr != nil {
			continue
		}
		want, err := describeFn(goja.Undefined(), jsVal)
		if err != nil {
			t.Fatal(err)
		}
		if g := describe(got); g != want.String() {
			t.Errorf("Parse(%q) = %s, want %s", in, g, want.String())
		}
	}
}

// goja's JSON.parse turns a lone surrogate escape into U+FFFD; V8 keeps it.
func TestParseKeepsLoneSurrogates(t *testing.T) {
	got, err := jsjson.Parse(`"a\uD800b"`)
	if err != nil {
		t.Fatal(err)
	}
	if want := jsstr.FromUTF16([]uint16{'a', 0xd800, 'b'}); got != want {
		t.Fatalf("Parse = %q, want %q", got, want)
	}
}

// goja's JSON.parse rejects numbers past the float64 range; V8 parses them as Infinity.
func TestParseOverflowsToInfinityLikeV8(t *testing.T) {
	got, err := jsjson.Parse(`1e400`)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := got.(float64); !ok || v <= 1e308 {
		t.Fatalf("Parse = %v, want +Inf", got)
	}
}
