package jsjson_test

import (
	"os"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/jsjson"
)

// TestV8MessageMatchesNode replays the messages V8's JSON.parse threw
// (testdata/messages.json, from scripts/gen-jsjson-messages.mjs), and checks Parse fails
// exactly where V8 throws.
func TestV8MessageMatchesNode(t *testing.T) {
	data, err := os.ReadFile("testdata/messages.json")
	if err != nil {
		t.Fatal(err)
	}
	cases, err := jsjson.Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases.([]any) {
		pair := c.([]any)
		text := pair[0].(string)
		want, _ := pair[1].(string)
		got := jsjson.V8Message(text)
		if got != want {
			t.Errorf("V8Message(%q) = %q, want %q", text, got, want)
		}
		if _, err := jsjson.Parse(text); (err != nil) != (want != "") {
			t.Errorf("Parse(%q) error = %v, want an error: %v", text, err, want != "")
		}
	}
}
