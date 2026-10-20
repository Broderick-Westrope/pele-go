package jsdate_test

import (
	"sync"
	"testing"
	"time"

	"github.com/Broderick-Westrope/pele-go/internal/fixture"
	"github.com/Broderick-Westrope/pele-go/internal/jsdate"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
)

// TestMatchesV8 replays Date calls as Node answered them in several time zones
// (testdata/cases.json, from scripts/gen-jsdate-cases.mjs).
func TestMatchesV8(t *testing.T) {
	var mu sync.Mutex
	zones := map[string]*time.Location{}
	zone := func(a fixture.Args) *time.Location {
		mu.Lock()
		defer mu.Unlock()
		name := a.Str(0)
		if loc, ok := zones[name]; ok {
			return loc
		}
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		zones[name] = loc
		return loc
	}
	num := func(a fixture.Args, i int) float64 { return jsnum.Parse(a.Str(i)) }
	nums := func(a fixture.Args, from int) []float64 {
		out := make([]float64, len(a)-from)
		for i := range out {
			out[i] = num(a, from+i)
		}
		return out
	}
	set := func(apply func(d *jsdate.Date, v []float64)) fixture.Func {
		return func(a fixture.Args) any {
			d := jsdate.New(num(a, 1), zone(a))
			apply(&d, nums(a, 2))
			return d.Time()
		}
	}
	fixture.Run(t, "testdata/cases.json", map[string]fixture.Func{
		"fields": func(a fixture.Args) any {
			d := jsdate.New(num(a, 1), zone(a))
			return []float64{d.FullYear(), d.Month(), d.Date(), d.Day(), d.Hours(), d.Minutes(), d.Seconds(),
				d.Milliseconds(), d.UTCSeconds(), d.TimezoneOffset(), d.Time()}
		},
		"new": func(a fixture.Args) any { return jsdate.New(num(a, 1), zone(a)).Time() },
		"local": func(a fixture.Args) any {
			v := nums(a, 1)
			return jsdate.Local(zone(a), v[0], v[1], v[2], v[3], v[4], v[5], v[6]).Time()
		},
		"utc": func(a fixture.Args) any {
			v := nums(a, 1)
			return jsdate.UTC(v[0], v[1], v[2], v[3], v[4], v[5], v[6])
		},
		"parse":        func(a fixture.Args) any { return jsdate.Parse(a.Str(1), zone(a)).Time() },
		"setDate":      set(func(d *jsdate.Date, v []float64) { d.SetDate(v[0]) }),
		"setMonth":     set(func(d *jsdate.Date, v []float64) { d.SetMonth(v[0]) }),
		"setMonthDate": set(func(d *jsdate.Date, v []float64) { d.SetMonthDate(v[0], v[1]) }),
		"setFullYear":  set(func(d *jsdate.Date, v []float64) { d.SetFullYear(v[0]) }),
		"setHours":     set(func(d *jsdate.Date, v []float64) { d.SetHours(v[0], v[1], v[2], v[3]) }),
		"setTime":      set(func(d *jsdate.Date, v []float64) { d.SetTime(v[0]) }),
	})
}
