package gantt

// Time axis support, following d3-time and d3-time-format (English locale, local time),
// which Mermaid uses: the axisFormat specifiers, tick intervals with d3's every rule, and its
// choice of automatic ticks.

import (
	"math"
	"strings"
	"time"

	"github.com/Broderick-Westrope/pele-go/internal/jsdate"
	"github.com/Broderick-Westrope/pele-go/internal/jsmath"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

const (
	second = 1000.0
	minute = 60 * second
	hour   = 60 * minute
	day    = 24 * hour
	week   = 7 * day
	month  = 30 * day
	year   = 365 * day
)

type calendar struct {
	floor  func(d *jsdate.Date)
	offset func(d *jsdate.Date, step float64)
	// field is the value d3 tests against a step: a tick falls where it is a multiple of the
	// step. Nil for units d3 steps through otherwise.
	field func(d jsdate.Date) float64
	// cycle is how many values field cycles through, which bounds how far apart filtered
	// ticks can be.
	cycle float64
	ms    float64
}

func startOfDay(d *jsdate.Date) { d.SetHours(0, 0, 0, 0) }

func weekCalendar(weekStart float64) calendar {
	return calendar{
		floor: func(d *jsdate.Date) {
			d.SetDate(d.Date() - math.Mod(d.Day()+7-weekStart, 7))
			startOfDay(d)
		},
		offset: func(d *jsdate.Date, step float64) { d.SetDate(d.Date() + float64(step*7)) },
		cycle:  math.Inf(1),
		ms:     week,
	}
}

var (
	millisecondCalendar = calendar{
		floor:  func(*jsdate.Date) {},
		offset: func(d *jsdate.Date, step float64) { d.SetTime(d.Time() + step) },
		cycle:  math.Inf(1),
		ms:     1,
	}
	secondCalendar = calendar{
		floor:  func(d *jsdate.Date) { d.SetTime(d.Time() - d.Milliseconds()) },
		offset: func(d *jsdate.Date, step float64) { d.SetTime(d.Time() + float64(step*second)) },
		field:  func(d jsdate.Date) float64 { return d.UTCSeconds() },
		cycle:  60,
		ms:     second,
	}
	minuteCalendar = calendar{
		floor:  func(d *jsdate.Date) { d.SetTime(d.Time() - d.Milliseconds() - float64(d.Seconds()*second)) },
		offset: func(d *jsdate.Date, step float64) { d.SetTime(d.Time() + float64(step*minute)) },
		field:  func(d jsdate.Date) float64 { return d.Minutes() },
		cycle:  60,
		ms:     minute,
	}
	hourCalendar = calendar{
		floor: func(d *jsdate.Date) {
			d.SetTime(d.Time() - d.Milliseconds() - float64(d.Seconds()*second) - float64(d.Minutes()*minute))
		},
		offset: func(d *jsdate.Date, step float64) { d.SetTime(d.Time() + float64(step*hour)) },
		field:  func(d jsdate.Date) float64 { return d.Hours() },
		cycle:  24,
		ms:     hour,
	}
	dayCalendar = calendar{
		floor:  startOfDay,
		offset: func(d *jsdate.Date, step float64) { d.SetDate(d.Date() + step) },
		field:  func(d jsdate.Date) float64 { return d.Date() - 1 },
		cycle:  31,
		ms:     day,
	}
	monthCalendar = calendar{
		floor: func(d *jsdate.Date) {
			d.SetDate(1)
			startOfDay(d)
		},
		offset: func(d *jsdate.Date, step float64) { d.SetMonth(d.Month() + step) },
		field:  func(d jsdate.Date) float64 { return d.Month() },
		cycle:  12,
		ms:     month,
	}
	yearCalendar = calendar{
		floor: func(d *jsdate.Date) {
			d.SetMonthDate(0, 1)
			startOfDay(d)
		},
		offset: func(d *jsdate.Date, step float64) { d.SetFullYear(d.FullYear() + step) },
		cycle:  math.Inf(1),
		ms:     year,
	}
)

var calendars = map[string]calendar{
	"millisecond": millisecondCalendar,
	"second":      secondCalendar,
	"minute":      minuteCalendar,
	"hour":        hourCalendar,
	"day":         dayCalendar,
	"month":       monthCalendar,
	"year":        yearCalendar,
}

// span is the whole units between two floored dates, discounting a change of UTC offset
// between them.
func span(start, end jsdate.Date, unit float64) float64 {
	return math.Floor((end.Time() - start.Time() - float64((end.TimezoneOffset()-start.TimezoneOffset())*minute)) / unit)
}

func mod(a, b float64) float64 { return math.Mod(math.Mod(a, b)+b, b) }

// every is the interval d3's unit.every(step) gives.
func every(unit string, step, weekStart float64, loc *time.Location) calendar {
	base := calendars[unit]
	if unit == "week" {
		base = weekCalendar(weekStart)
	}
	if step <= 1 {
		return base
	}
	c := base
	switch unit {
	case "millisecond":
		c.floor = func(d *jsdate.Date) { d.SetTime(float64(math.Floor(d.Time()/step) * step)) }
		c.offset = func(d *jsdate.Date, n float64) { d.SetTime(d.Time() + float64(n*step)) }
		return c
	case "year":
		c.floor = func(d *jsdate.Date) {
			d.SetFullYear(float64(math.Floor(d.FullYear()/step) * step))
			base.floor(d)
		}
		c.offset = func(d *jsdate.Date, n float64) { d.SetFullYear(d.FullYear() + float64(n*step)) }
		return c
	case "week":
		epoch := jsdate.New(0, loc)
		base.floor(&epoch)
		c.floor = func(d *jsdate.Date) {
			base.floor(d)
			base.offset(d, -mod(span(epoch, *d, week), step))
		}
		c.offset = func(d *jsdate.Date, n float64) { base.offset(d, float64(n*step)) }
		return c
	}
	test := func(d jsdate.Date) bool { return math.Mod(base.field(d), step) == 0 }
	c.floor = func(d *jsdate.Date) {
		// Date overflow makes field(d) NaN, which can never match the interval.
		for base.floor(d); d.Valid() && !test(*d); base.floor(d) {
			d.SetTime(d.Time() - 1)
		}
	}
	c.offset = func(d *jsdate.Date, n float64) {
		for i := 0.0; i < n && d.Valid(); i++ {
			base.offset(d, 1)
			for d.Valid() && !test(*d) {
				base.offset(d, 1)
			}
		}
	}
	return c
}

// tickRange returns the ticks from start to stop inclusive, or false when there would be more
// than limit.
func tickRange(c calendar, start, stop, limit float64, loc *time.Location) ([]float64, bool) {
	d := jsdate.New(start-1, loc)
	c.floor(&d)
	c.offset(&d, 1)
	c.floor(&d)
	out := []float64{}
	previous := math.Inf(-1)
	for d.Time() <= stop && d.Time() > previous {
		if float64(len(out)) >= limit {
			return nil, false
		}
		previous = d.Time()
		out = append(out, previous)
		c.offset(&d, 1)
		c.floor(&d)
	}
	return out, true
}

// pow10 is Math.pow(10, p) for the powers tickStep reaches: -Infinity, -1, and 0 to 5,
// which V8 and Go both compute exactly or correctly rounded.
func pow10(p float64) float64 {
	if math.IsInf(p, -1) {
		return 0
	}
	return math.Pow(10, p)
}

func tickStep(start, stop, count float64) float64 {
	step := (stop - start) / math.Max(0, count)
	power := math.Floor(jsmath.Log10(step))
	e := step / pow10(power)
	factor := 1.0
	switch {
	case e >= math.Sqrt(50):
		factor = 10
	case e >= math.Sqrt(10):
		factor = 5
	case e >= math.Sqrt2:
		factor = 2
	}
	return pow10(power) * factor
}

var autoIntervals = []struct {
	unit string
	step float64
	ms   float64
}{
	{"second", 1, second},
	{"second", 5, 5 * second},
	{"second", 15, 15 * second},
	{"second", 30, 30 * second},
	{"minute", 1, minute},
	{"minute", 5, 5 * minute},
	{"minute", 15, 15 * minute},
	{"minute", 30, 30 * minute},
	{"hour", 1, hour},
	{"hour", 3, 3 * hour},
	{"hour", 6, 6 * hour},
	{"hour", 12, 12 * hour},
	{"day", 1, day},
	{"day", 2, 2 * day},
	{"week", 1, week},
	{"month", 1, month},
	{"month", 3, 3 * month},
	{"year", 1, year},
}

// autoTicks returns about count ticks at round times, chosen the way d3's time scale
// chooses them.
func autoTicks(start, stop, count float64, loc *time.Location) []float64 {
	target := (stop - start) / count
	i := 0
	for i < len(autoIntervals) && autoIntervals[i].ms <= target {
		i++
	}
	var c calendar
	switch {
	case i == len(autoIntervals):
		c = every("year", math.Max(1, tickStep(start/year, stop/year, count)), 0, loc)
	case i == 0:
		c = every("millisecond", math.Max(tickStep(start, stop, count), 1), 0, loc)
	default:
		k := i
		if target/autoIntervals[i-1].ms < autoIntervals[i].ms/target {
			k = i - 1
		}
		c = every(autoIntervals[k].unit, autoIntervals[k].step, 0, loc)
	}
	out, ok := tickRange(c, start, stop, float64(4*count)+8, loc)
	if !ok {
		return []float64{}
	}
	return out
}

var weekStarts = [...]string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

// intervalTicks returns the ticks for a tickInterval such as 1week, or false when it is not
// one or asks for more than limit.
func intervalTicks(interval, weekday string, start, stop, limit float64, loc *time.Location) ([]float64, bool) {
	// /^([1-9]\d*)(millisecond|second|minute|hour|day|week|month)$/
	i := 0
	for i < len(interval) && interval[i] >= '0' && interval[i] <= '9' {
		i++
	}
	if i == 0 || interval[0] == '0' {
		return nil, false
	}
	unit := interval[i:]
	switch unit {
	case "millisecond", "second", "minute", "hour", "day", "week", "month":
	default:
		return nil, false
	}
	step := jsnum.Parse(interval[:i])
	weekStart := 0.0
	for k, name := range weekStarts {
		if name == weekday {
			weekStart = float64(k)
			break
		}
	}
	c := every(unit, step, weekStart, loc)
	if (stop-start)/float64(math.Min(step, c.cycle)*c.ms) > limit {
		return nil, false
	}
	return tickRange(c, start, stop, limit, loc)
}

func pad(value float64, fill string, width int) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	digits := jsnum.Format(value)
	if len(digits) < width {
		return sign + strings.Repeat(fill, width-len(digits)) + digits
	}
	return sign + digits
}

func floored(date jsdate.Date, c calendar) jsdate.Date {
	d := date
	c.floor(&d)
	return d
}

// weekOfYear is the weeks since the last weekStart day before the year began, which is
// d3's %U and %W.
func weekOfYear(date jsdate.Date, weekStart float64) float64 {
	w := weekCalendar(weekStart)
	yearStart := floored(date, yearCalendar)
	return span(floored(jsdate.New(yearStart.Time()-1, date.Location()), w), floored(date, w), week)
}

// isoThursday is the Thursday that decides which ISO week and ISO year a date belongs to.
func isoThursday(date jsdate.Date) jsdate.Date {
	thursday := weekCalendar(4)
	weekday := date.Day()
	late := weekday >= 4 || weekday == 0
	from := date
	if !late {
		from = jsdate.New(date.Time()-1, date.Location())
	}
	d := floored(from, thursday)
	if !late {
		thursday.offset(&d, 1)
		thursday.floor(&d)
	}
	return d
}

// timeFormat formats a date with a d3 time format specifier such as %Y-%m-%d.
func timeFormat(specifier string, date jsdate.Date) string {
	u := jsstr.UTF16(specifier)
	at := func(i int) []uint16 {
		if i < len(u) {
			return u[i : i+1]
		}
		return nil
	}
	var out []uint16
	from := 0
	for i := 0; i < len(u); i++ {
		if u[i] != '%' {
			continue
		}
		out = append(out, u[from:i]...)
		i++
		c := at(i)
		fill, padded := pads[unitOr(c)]
		if padded {
			i++
			c = at(i)
		} else if unitOr(c) == 'e' {
			fill = " "
		} else {
			fill = "0"
		}
		out = append(out, jsstr.UTF16(directive(c, date, fill))...)
		from = i + 1
	}
	if from < len(u) {
		out = append(out, u[from:]...)
	}
	return jsstr.FromUTF16(out)
}

var pads = map[rune]string{'-': "", '_': " ", '0': "0"}

// unitOr is the code unit of a one-unit slice, or -1 for charAt past the end.
func unitOr(c []uint16) rune {
	if len(c) == 0 {
		return -1
	}
	return rune(c[0])
}

func directive(c []uint16, d jsdate.Date, p string) string {
	switch unitOr(c) {
	case 'a':
		return weekdayNames[int(d.Day())][:3]
	case 'A':
		return weekdayNames[int(d.Day())]
	case 'b':
		return monthNames[int(d.Month())][:3]
	case 'B':
		return monthNames[int(d.Month())]
	case 'c':
		return timeFormat("%x, %X", d)
	case 'd', 'e':
		return pad(d.Date(), p, 2)
	case 'f':
		return pad(d.Milliseconds(), p, 3) + "000"
	case 'g':
		return pad(math.Mod(isoThursday(d).FullYear(), 100), p, 2)
	case 'G':
		return pad(math.Mod(isoThursday(d).FullYear(), 10000), p, 4)
	case 'H':
		return pad(d.Hours(), p, 2)
	case 'I':
		return pad(hours12(d.Hours()), p, 2)
	case 'j':
		return pad(1+span(floored(d, yearCalendar), floored(d, dayCalendar), day), p, 3)
	case 'L':
		return pad(d.Milliseconds(), p, 3)
	case 'm':
		return pad(d.Month()+1, p, 2)
	case 'M':
		return pad(d.Minutes(), p, 2)
	case 'p':
		if d.Hours() >= 12 {
			return "PM"
		}
		return "AM"
	case 'q':
		return num(1 + math.Floor(d.Month()/3))
	case 'Q':
		return num(d.Time())
	case 's':
		return num(math.Floor(d.Time() / 1000))
	case 'S':
		return pad(d.Seconds(), p, 2)
	case 'u':
		return num(isoWeekday(d))
	case 'U':
		return pad(weekOfYear(d, 0), p, 2)
	case 'V':
		thursday := isoThursday(d)
		yearStart := floored(thursday, yearCalendar)
		first := floored(yearStart, weekCalendar(4))
		extra := 0.0
		if yearStart.Day() == 4 {
			extra = 1
		}
		return pad(span(first, thursday, week)+extra, p, 2)
	case 'w':
		return num(d.Day())
	case 'W':
		return pad(weekOfYear(d, 1), p, 2)
	case 'x':
		return timeFormat("%-m/%-d/%Y", d)
	case 'X':
		return timeFormat("%-I:%M:%S %p", d)
	case 'y':
		return pad(math.Mod(d.FullYear(), 100), p, 2)
	case 'Y':
		return pad(math.Mod(d.FullYear(), 10000), p, 4)
	case 'Z':
		z := d.TimezoneOffset()
		abs := math.Abs(z)
		sign := "+"
		if z > 0 {
			sign = "-"
		}
		return sign + pad(math.Floor(abs/60), "0", 2) + pad(math.Mod(abs, 60), "0", 2)
	case '%':
		return "%"
	default:
		return jsstr.FromUTF16(c)
	}
}
