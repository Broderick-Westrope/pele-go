package util_test

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Broderick-Westrope/pele-go/internal/util"
)

type idnaCases struct {
	Node   string       `json:"node"`
	Hosts  [][2]*string `json:"hosts"`
	Sweeps []struct {
		Name    string `json:"name"`
		Pattern string `json:"pattern"`
		Domain  string `json:"domain"`
		Count   int    `json:"count"`
		SHA256  string `json:"sha256"`
	} `json:"sweeps"`
}

func loadIDNACases(t *testing.T) idnaCases {
	t.Helper()
	f, err := os.Open("testdata/idna.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases idnaCases
	if err := json.NewDecoder(z).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func parseHost(host string) string {
	href, ok := util.ParseHTTPURL("http://" + host + "/")
	if !ok {
		return "!"
	}
	return href
}

// TestIDNAHosts checks the host parser against Node's for the hosts of
// testdata/idna.json.gz (scripts/gen-idna-cases.mjs).
func TestIDNAHosts(t *testing.T) {
	cases := loadIDNACases(t)
	failed := 0
	for _, c := range cases.Hosts {
		want := "!"
		if c[1] != nil {
			want = *c[1]
		}
		if got := parseHost(*c[0]); got != want {
			failed++
			if failed <= 30 {
				t.Errorf("host %+q: got %+q, want %+q (Node %s)", *c[0], got, want, cases.Node)
			}
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d hosts differ", failed, len(cases.Hosts))
	}
}

// TestIDNASweeps checks every code point in a few contexts against hashes of Node's results.
func TestIDNASweeps(t *testing.T) {
	if testing.Short() {
		t.Skip("sweeps every code point")
	}
	cases := loadIDNACases(t)
	var all, live []rune
	for c := rune(0); c <= 0x10ffff; c++ {
		if c >= 0xd800 && c <= 0xdfff {
			continue
		}
		all = append(all, c)
		if parseHost(string(c)) != "!" || parseHost("a"+string(c)) != "!" {
			live = append(live, c)
		}
	}
	for _, s := range cases.Sweeps {
		t.Run(s.Name, func(t *testing.T) {
			t.Parallel()
			domain := all
			if s.Domain == "live" {
				domain = live
			}
			if len(domain) != s.Count {
				t.Fatalf("%d code points, want %d", len(domain), s.Count)
			}
			h := sha256.New()
			for _, c := range domain {
				host := strings.ReplaceAll(s.Pattern, "{c}", string(c))
				host = strings.ReplaceAll(host, "{p}", punycode(c))
				h.Write([]byte(parseHost(host) + "\n"))
			}
			if got := hex.EncodeToString(h.Sum(nil)); got != s.SHA256 {
				t.Errorf("pattern %+q: hash %s, want Node's %s", s.Pattern, got, s.SHA256)
			}
		})
	}
}

// punycode is RFC 3492 Punycode of one code point, as the fixture's generator writes it.
func punycode(c rune) string {
	if c < 0x80 {
		return string(c) + "-"
	}
	var out []byte
	for q, k, bias := int(c)-0x80, 36, 72; ; k += 36 {
		t := min(max(k-bias, 1), 26)
		if q < t {
			d := q
			if d < 26 {
				return string(append(out, byte('a'+d)))
			}
			return string(append(out, byte('0'+d-26)))
		}
		d := t + (q-t)%(36-t)
		if d < 26 {
			out = append(out, byte('a'+d))
		} else {
			out = append(out, byte('0'+d-26))
		}
		q = (q - t) / (36 - t)
	}
}
