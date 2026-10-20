package yaml_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsjson"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/yaml"
)

// units writes a string the way the fixture generator does: its UTF-16 code units as a
// JSON array.
func units(s string) string {
	parts := []string{}
	for _, u := range jsstr.UTF16(s) {
		parts = append(parts, strconv.Itoa(int(u)))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func describe(v any) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case []any:
		parts := make([]string, len(v))
		for i, e := range v {
			parts[i] = describe(e)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case *jsobj.Object:
		parts := []string{}
		for _, k := range v.Keys() {
			e, _ := v.Get(k)
			parts = append(parts, units(k)+":"+describe(e))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case string:
		return "s" + units(v)
	case bool:
		return "boolean:" + jsobj.ToString(v)
	default:
		return "number:" + jsobj.ToString(v)
	}
}

// TestParseMatchesV8 checks every case in testdata/cases.json, which
// scripts/gen-yaml-cases.mjs writes by running upstream's parseYaml under Node.
func TestParseMatchesV8(t *testing.T) {
	data, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	// jsjson keeps lone surrogates that encoding/json would replace.
	parsed, err := jsjson.Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range parsed.([]any) {
		obj := c.(*jsobj.Object)
		input, _ := obj.Get("input")
		wantValue, hasValue := obj.Get("value")
		wantErr, _ := obj.Get("error")

		got, err := yaml.Parse(input.(string))
		if hasValue {
			if err != nil {
				t.Errorf("Parse(%q) error %v, want %s", input, err, wantValue)
			} else if d := describe(got); d != wantValue {
				t.Errorf("Parse(%q) =\n %s\nwant\n %s", input, d, wantValue)
			}
			continue
		}
		_, wantMessage, _ := strings.Cut(wantErr.(string), ": ")
		if err == nil {
			t.Errorf("Parse(%q) = %s, want error %q", input, describe(got), wantMessage)
		} else if err.Error() != wantMessage {
			t.Errorf("Parse(%q) error %q, want %q", input, err.Error(), wantMessage)
		}
	}
}
