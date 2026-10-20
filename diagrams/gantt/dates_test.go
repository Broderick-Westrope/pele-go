package gantt

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/Broderick-Westrope/pele-go/internal/jsdate"
)

// Ported from upstream's tests/gantt/dates.test.ts. Dates are built in local time
// throughout, so each case holds in every zone it runs in.

var testZones = []string{"UTC", "Australia/Sydney", "America/New_York", "Asia/Kolkata"}

func inZones(t *testing.T, fn func(t *testing.T, loc *time.Location)) {
	for _, zone := range testZones {
		t.Run(zone, func(t *testing.T) {
			loc, err := time.LoadLocation(zone)
			if err != nil {
				t.Fatal(err)
			}
			fn(t, loc)
		})
	}
}

// at is new Date(y, m, d, h, min, s, ms) in loc.
func at(loc *time.Location, fields ...float64) jsdate.Date {
	f := [7]float64{0, 0, 1}
	copy(f[:], fields)
	return jsdate.Local(loc, f[0], f[1], f[2], f[3], f[4], f[5], f[6])
}

func TestParseStrict(t *testing.T) {
	inZones(t, func(t *testing.T, loc *time.Location) {
		now := at(loc, 2031, 4, 6, 9, 30, 15, 250)
		accepted := map[string]struct {
			input, template string
			want            jsdate.Date
		}{
			"iso date":         {"2014-01-06", "YYYY-MM-DD", at(loc, 2014, 0, 6)},
			"day first":        {"06-01-2014", "DD-MM-YYYY", at(loc, 2014, 0, 6)},
			"short year":       {"14/1/6", "YY/M/D", at(loc, 2014, 0, 6)},
			"short year 1900s": {"70/12/31", "YY/M/D", at(loc, 1970, 11, 31)},
			"time":             {"2014-01-06 17:49", "YYYY-MM-DD HH:mm", at(loc, 2014, 0, 6, 17, 49)},
			"unpadded time":    {"2014-01-06 7:5:9", "YYYY-MM-DD H:m:s", at(loc, 2014, 0, 6, 7, 5, 9)},
			"milliseconds":     {"2014-01-06T07:05:09.123", "YYYY-MM-DDTHH:mm:ss.SSS", at(loc, 2014, 0, 6, 7, 5, 9, 123)},
			"pm":               {"9:30 PM", "h:mm A", at(loc, 2031, 4, 6, 21, 30)},
			"midnight am":      {"12:05 am", "hh:mm a", at(loc, 2031, 4, 6, 0, 5)},
			"noon pm":          {"12:05 pm", "hh:mm a", at(loc, 2031, 4, 6, 12, 5)},
			"short month":      {"Jan 5, 2024", "MMM D, YYYY", at(loc, 2024, 0, 5)},
			"ordinal":          {"5th January 2024", "Do MMMM YYYY", at(loc, 2024, 0, 5)},
			"ordinal nd":       {"22nd December 2024", "Do MMMM YYYY", at(loc, 2024, 11, 22)},
			"literal":          {"on 2024-03-01", "[on] YYYY-MM-DD", at(loc, 2024, 2, 1)},
			"time only":        {"17:49", "HH:mm", at(loc, 2031, 4, 6, 17, 49)},
			"year only":        {"2024", "YYYY", at(loc, 2024, 0, 1)},
			"month only":       {"03", "MM", at(loc, 2031, 2, 1)},
			"day only":         {"7", "D", at(loc, 2031, 4, 7)},
			"seconds only":     {"20", "ss", at(loc, 2031, 4, 6, 0, 0, 20)},
			"quarter":          {"2 2024", "Q YYYY", at(loc, 2024, 3, 1)},
			"weekday name":     {"Monday 2024", "dddd YYYY", at(loc, 2024, 0, 1)},
			"seconds since":    {"1410715640", "X", jsdate.New(1410715640000, loc)},
			"ms since":         {"1410715640579", "x", jsdate.New(1410715640579, loc)},
			"negative ms":      {"-5", "x", jsdate.New(-5, loc)},
		}
		for name, tc := range accepted {
			got, ok := parseStrict(tc.input, tc.template, now)
			if !ok || got.Time() != tc.want.Time() {
				t.Errorf("%s: parseStrict(%q, %q) = %v, %v; want %v", name, tc.input, tc.template, got.Time(), ok, tc.want.Time())
			}
		}
		rejected := map[string]struct{ input, template string }{
			"unpadded":        {"2014-1-6", "YYYY-MM-DD"},
			"padded time":     {"2014-01-06 7:05:09", "YYYY-MM-DD H:m:s"},
			"month 13":        {"2014-13-01", "YYYY-MM-DD"},
			"february 30":     {"2014-02-30", "YYYY-MM-DD"},
			"short year":      {"202-12-01", "YYYY-MM-DD"},
			"too short":       {"202304", "YYYYMMDD"},
			"trailing space":  {"2014-01-06 ", "YYYY-MM-DD"},
			"leading space":   {" 2014-01-06", "YYYY-MM-DD"},
			"trailing text":   {"2014-01-06x", "YYYY-MM-DD"},
			"zero seconds":    {"0", "ss"},
			"hour 25":         {"25:00", "HH:mm"},
			"no ordinal":      {"5 January 2024", "Do MMMM YYYY"},
			"lower month":     {"5th january 2024", "Do MMMM YYYY"},
			"wrong weekday":   {"Tuesday 2024", "dddd YYYY"},
			"no format":       {"2014-01-06", ""},
			"no tokens":       {"2014-01-06", "q"},
			"localized":       {"12/25/2020", "L"},
			"week":            {"2020-01", "YYYY-ww"},
			"invalid":         {"Invalid Date", ""},
			"empty":           {"", "YYYY-MM-DD"},
			"fractional secs": {"1410715640.579", "X"},
			"exponent":        {"1e3", "x"},
			"empty timestamp": {"", "x"},
		}
		for name, tc := range rejected {
			if got, ok := parseStrict(tc.input, tc.template, now); ok {
				t.Errorf("%s: parseStrict(%q, %q) = %v, want no date", name, tc.input, tc.template, got.Time())
			}
		}
	})
}

func TestFormat(t *testing.T) {
	inZones(t, func(t *testing.T, loc *time.Location) {
		date := at(loc, 2024, 1, 3, 15, 4, 5, 67)
		tests := map[string]struct{ template, want string }{
			"date":         {"YYYY-MM-DD", "2024-02-03"},
			"short":        {"YY M D", "24 2 3"},
			"months":       {"MMM MMMM", "Feb February"},
			"weekdays":     {"d dd ddd dddd", "6 Sa Sat Saturday"},
			"hours":        {"H HH h hh", "15 15 3 03"},
			"minutes":      {"m mm s ss SSS", "4 04 5 05 067"},
			"meridiem":     {"a A", "pm PM"},
			"ordinal":      {"Do [of] MMMM", "3rd of February"},
			"quarter":      {"Q", "1"},
			"k":            {"k kk", "15 15"},
			"iso week":     {"W WW GGGG", "5 05 2024"},
			"literal":      {"[YYYY] YYYY", "YYYY 2024"},
			"partial year": {"Y YYY", "Y 24Y"},
			"fraction":     {"S SS", "S SS"},
			"ms":           {"x", num(date.Time())},
			"seconds":      {"X", num(math.Floor(date.Time() / 1000))},
			"plugin text":  {"[w] YYYY", "w 2024"},
		}
		for name, tc := range tests {
			if got, ok := format(date, tc.template); !ok || got != tc.want {
				t.Errorf("%s: format(%q) = %q, %v; want %q", name, tc.template, got, ok, tc.want)
			}
		}
		for _, template := range []string{"w", "ww", "wo", "gggg", "z", "zzz", "YYYY-ww"} {
			if got, ok := format(date, template); ok {
				t.Errorf("format(%q) = %q, want no output", template, got)
			}
		}
		var ordinals []string
		for _, d := range []float64{1, 2, 3, 4, 11, 12, 13, 21, 22, 23, 31} {
			s, _ := format(at(loc, 2024, 0, d), "Do")
			ordinals = append(ordinals, s)
		}
		if want := []string{"1st", "2nd", "3rd", "4th", "11th", "12th", "13th", "21st", "22nd", "23rd", "31st"}; !slices.Equal(ordinals, want) {
			t.Errorf("ordinals = %v, want %v", ordinals, want)
		}
		edges := map[string]struct {
			date           jsdate.Date
			template, want string
		}{
			"midnight":     {at(loc, 2024, 0, 1, 0, 30), "k h a", "24 12 am"},
			"week of 2020": {at(loc, 2021, 0, 3), "GGGG-[W]WW", "2020-W53"},
			"week of 2025": {at(loc, 2024, 11, 30), "GGGG-[W]WW", "2025-W01"},
			"first week":   {at(loc, 2026, 0, 1), "W", "1"},
			"invalid date": {jsdate.New(math.NaN(), loc), "YYYY", "Invalid Date"},
		}
		for name, tc := range edges {
			if got, _ := format(tc.date, tc.template); got != tc.want {
				t.Errorf("%s: format(%q) = %q, want %q", name, tc.template, got, tc.want)
			}
		}
	})
}

func TestAdd(t *testing.T) {
	inZones(t, func(t *testing.T, loc *time.Location) {
		tests := map[string]struct {
			from   jsdate.Date
			amount float64
			unit   string
			want   jsdate.Date
		}{
			"a day":            {at(loc, 2020, 10, 1), 1, "d", at(loc, 2020, 10, 2)},
			"days over dst":    {at(loc, 2024, 2, 9), 2, "d", at(loc, 2024, 2, 11)},
			"weeks":            {at(loc, 2013, 0, 1), 2, "w", at(loc, 2013, 0, 15)},
			"rounded up":       {at(loc, 2013, 0, 1), 1.5, "d", at(loc, 2013, 0, 3)},
			"rounded down":     {at(loc, 2013, 0, 1), 0.4, "d", at(loc, 2013, 0, 1)},
			"half a week":      {at(loc, 2013, 0, 1), 0.5, "w", at(loc, 2013, 0, 5)},
			"short month":      {at(loc, 2024, 0, 31), 1, "M", at(loc, 2024, 1, 29)},
			"into leap month":  {at(loc, 2023, 11, 31), 2, "M", at(loc, 2024, 1, 29)},
			"eleven months":    {at(loc, 2024, 2, 30), 11, "M", at(loc, 2025, 1, 28)},
			"leap day":         {at(loc, 2024, 1, 29), 1, "y", at(loc, 2025, 1, 28)},
			"fractional month": {at(loc, 2024, 0, 15), 1.5, "M", at(loc, 2024, 1, 15)},
		}
		for name, tc := range tests {
			if got := add(tc.from, tc.amount, tc.unit); got.Time() != tc.want.Time() {
				t.Errorf("%s: add = %v, want %v", name, got.Time(), tc.want.Time())
			}
		}
		start := at(loc, 2013, 0, 1)
		exact := map[string]struct {
			amount float64
			unit   string
			want   float64
		}{
			"hours":   {2, "h", 7200000},
			"minutes": {2, "m", 120000},
			"seconds": {0.005, "s", 5},
			"ms":      {20, "ms", 20},
		}
		for name, tc := range exact {
			if got := add(start, tc.amount, tc.unit).Time() - start.Time(); got != tc.want {
				t.Errorf("%s: add moved %v, want %v", name, got, tc.want)
			}
		}
		if got := add(at(loc, 2024, 0, 1), 99999999, "y"); got.Valid() {
			t.Errorf("add past the range of dates = %v, want an invalid date", got.Time())
		}
	})
}

func TestParseDuration(t *testing.T) {
	tests := map[string]struct {
		input string
		value float64
		unit  string
	}{
		"fraction":   {"1.5d", 1.5, "d"},
		"spaced":     {" 3M ", 3, "M"},
		"ms":         {"500ms", 500, "ms"},
		"trailing":   {"3dX", math.NaN(), "ms"},
		"negative":   {"-1d", math.NaN(), "ms"},
		"bare point": {"3.d", math.NaN(), "ms"},
	}
	for name, tc := range tests {
		value, unit := parseDuration(tc.input)
		if unit != tc.unit || !(value == tc.value || math.IsNaN(value) && math.IsNaN(tc.value)) {
			t.Errorf("%s: parseDuration(%q) = %v %q, want %v %q", name, tc.input, value, unit, tc.value, tc.unit)
		}
	}
}

func TestTimeFormat(t *testing.T) {
	inZones(t, func(t *testing.T, loc *time.Location) {
		date := at(loc, 2024, 1, 3, 15, 4, 5, 67)
		tests := map[string]struct{ specifier, want string }{
			"date":        {"%Y-%m-%d", "2024-02-03"},
			"names":       {"%a %A %b %B", "Sat Saturday Feb February"},
			"time":        {"%H:%M:%S.%L", "15:04:05.067"},
			"12 hour":     {"%I %p", "03 PM"},
			"padding":     {"%e|%-d|%_d|%0e", " 3|3| 3|03"},
			"day of year": {"%j", "034"},
			"weeks":       {"%U %W %V", "04 05 05"},
			"weekdays":    {"%u %w", "6 6"},
			"years":       {"%y %G %g", "24 2024 24"},
			"quarter":     {"%q", "1"},
			"locale date": {"%x", "2/3/2024"},
			"locale time": {"%X", "3:04:05 PM"},
			"locale both": {"%c", "2/3/2024, 3:04:05 PM"},
			"micro":       {"%f", "067000"},
			"unknown":     {"%%Y %k %", "%Y k "},
			"seconds":     {"%s", num(math.Floor(date.Time() / 1000))},
			"ms":          {"%Q", num(date.Time())},
		}
		for name, tc := range tests {
			if got := timeFormat(tc.specifier, date); got != tc.want {
				t.Errorf("%s: timeFormat(%q) = %q, want %q", name, tc.specifier, got, tc.want)
			}
		}
		weeks := map[string]struct {
			date            jsdate.Date
			specifier, want string
		}{
			"new year 2023": {at(loc, 2023, 0, 1), "%U %W %V %G", "01 00 52 2022"},
			"end of 2024":   {at(loc, 2024, 11, 30), "%U %W %V %G", "52 53 01 2025"},
			"start of 2024": {at(loc, 2024, 0, 1), "%U %W %V", "00 01 01"},
		}
		for name, tc := range weeks {
			if got := timeFormat(tc.specifier, tc.date); got != tc.want {
				t.Errorf("%s: timeFormat(%q) = %q, want %q", name, tc.specifier, got, tc.want)
			}
		}
	})
}

func TestTicks(t *testing.T) {
	inZones(t, func(t *testing.T, loc *time.Location) {
		day := func(fields ...float64) float64 { return at(loc, fields...).Time() }
		series := func(n int, fn func(k float64) float64) []float64 {
			out := make([]float64, n)
			for k := range out {
				out[k] = fn(float64(k))
			}
			return out
		}
		auto := map[string]struct {
			start, stop, count float64
			want               []float64
		}{
			"weeks":    {day(2014, 0, 1), day(2014, 1, 20), 10, series(7, func(k float64) float64 { return day(2014, 0, 5+7*k) })},
			"hours":    {day(2024, 0, 10, 9), day(2024, 0, 10, 18), 10, series(10, func(k float64) float64 { return day(2024, 0, 10, 9+k) })},
			"decades":  {day(1900, 0, 1), day(2020, 0, 1), 10, series(13, func(k float64) float64 { return day(1900+10*k, 0, 1) })},
			"months":   {day(2024, 0, 1), day(2024, 6, 31), 7, series(7, func(k float64) float64 { return day(2024, k, 1) })},
			"seconds":  {0, 71000, 10, series(15, func(k float64) float64 { return k * 5000 })},
			"ms":       {0, 30, 10, series(16, func(k float64) float64 { return 2 * k })},
			"overflow": {-8.64e15, -8.64e15 + 120000, 4, []float64{}},
		}
		for name, tc := range auto {
			if got := autoTicks(tc.start, tc.stop, tc.count, loc); !slices.Equal(got, tc.want) {
				t.Errorf("%s: autoTicks = %v, want %v", name, got, tc.want)
			}
		}

		start, stop := day(2024, 0, 1), day(2024, 0, 31)
		days := func(ds ...float64) []float64 {
			out := make([]float64, len(ds))
			for i, d := range ds {
				out[i] = day(2024, 0, d)
			}
			return out
		}
		interval := map[string]struct {
			interval, weekday  string
			start, stop, limit float64
			want               []float64
		}{
			"monday weeks":   {"1week", "monday", start, stop, 100, days(1, 8, 15, 22, 29)},
			"sunday weeks":   {"1week", "sunday", start, stop, 100, days(7, 14, 21, 28)},
			"thursday weeks": {"1week", "thursday", start, stop, 100, days(4, 11, 18, 25)},
			"ten days":       {"10day", "sunday", start, stop, 100, days(1, 11, 21, 31)},
			"months":         {"1month", "sunday", start, day(2024, 3, 2), 100, series(4, func(k float64) float64 { return day(2024, k, 1) })},
			"six hours":      {"6hour", "sunday", start, day(2024, 0, 2), 100, series(5, func(k float64) float64 { return day(2024, 0, 1, 6*k) })},
			"quarter hours": {"15minute", "sunday", day(2024, 0, 1, 9, 5), day(2024, 0, 1, 10), 100,
				series(4, func(k float64) float64 { return day(2024, 0, 1, 9, 15+15*k) })},
			"overflow": {"2second", "sunday", 8.64e15 - 1000, 8.64e15, 10, []float64{8.64e15}},
		}
		for name, tc := range interval {
			if got, ok := intervalTicks(tc.interval, tc.weekday, tc.start, tc.stop, tc.limit, loc); !ok || !slices.Equal(got, tc.want) {
				t.Errorf("%s: intervalTicks = %v, %v; want %v", name, got, ok, tc.want)
			}
		}
		if got, ok := intervalTicks("2week", "monday", start, stop, 100, loc); !ok || len(got) > 3 {
			t.Errorf("2week: intervalTicks = %v, %v; want at most 3 ticks", got, ok)
		}
		if got, _ := intervalTicks("1day", "nonsense", start, stop, 100, loc); len(got) != 31 {
			t.Errorf("unknown weekday: %d ticks, want 31", len(got))
		}
		declined := []string{"", "1decade", "1year", "0day", "01day", "1 day", "1days", "day", "1.5day", "-1day"}
		for _, s := range declined {
			if got, ok := intervalTicks(s, "sunday", start, stop, 100, loc); ok {
				t.Errorf("intervalTicks(%q) = %v, want none", s, got)
			}
		}
		for _, s := range []string{"1second", "99999999second"} {
			if got, ok := intervalTicks(s, "sunday", start, stop, 1000, loc); ok {
				t.Errorf("intervalTicks(%q) = %d ticks, want too many", s, len(got))
			}
		}
	})
}
