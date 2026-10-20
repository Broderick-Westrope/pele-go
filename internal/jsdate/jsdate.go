// Package jsdate is the part of JavaScript's Date that gantt charts use, as V8 implements it
// (src/builtins/builtins-date.cc, src/date): the constructor forms, Date.UTC, the string
// parser, local-time getters and setters with their overflow, in a time zone given as a
// time.Location the way V8 reads one from ICU.
package jsdate

import (
	"math"
	"time"
)

const (
	msPerSecond = 1000.0
	msPerMinute = 60000.0
	msPerHour   = 3600000.0
	msPerDay    = 86400000.0

	// maxTime is the largest time value a Date holds.
	maxTime = 8.64e15
	// maxTimeBeforeUTC bounds a local time V8 converts to UTC; past it the result is NaN.
	maxTimeBeforeUTC = maxTime + 30*msPerDay
)

// Date is a JS Date: a time value in milliseconds since the epoch, NaN when the date is
// invalid, read in a time zone. Copying a Date is new Date(date).
type Date struct {
	t   float64
	loc *time.Location
}

// New is new Date(t) in loc.
func New(t float64, loc *time.Location) Date {
	return Date{t: timeClip(t), loc: loc}
}

// Local is new Date(year, month, day, hours, minutes, seconds, ms) in loc: the fields are
// local time, may overflow into the next field, and a year from 0 to 99 means 1900 to 1999.
func Local(loc *time.Location, year, month, day, hours, minutes, seconds, ms float64) Date {
	if !math.IsNaN(year) {
		if y := toInteger(year); y >= 0 && y <= 99 {
			year = 1900 + y
		}
	}
	return Date{t: timeClip(localToUTC(loc, makeDate(makeDay(year, month, day), makeTime(hours, minutes, seconds, ms)))), loc: loc}
}

// UTC is Date.UTC(year, month, day, hours, minutes, seconds, ms).
func UTC(year, month, day, hours, minutes, seconds, ms float64) float64 {
	if !math.IsNaN(year) {
		if y := toInteger(year); y >= 0 && y <= 99 {
			year = 1900 + y
		}
	}
	return timeClip(makeDate(makeDay(year, month, day), makeTime(hours, minutes, seconds, ms)))
}

// Time is getTime(): the time value, NaN for an invalid date.
func (d Date) Time() float64 { return d.t }

// Valid reports whether the date is not an Invalid Date.
func (d Date) Valid() bool { return !math.IsNaN(d.t) }

// fields is the local date and time of d.
type fields struct {
	year, month, day, weekday, hour, minute, second, ms int
	days, timeInDay                                     int64
}

func (d Date) local() fields {
	local := int64(d.t) + offsetFromUTC(d.loc, int64(d.t))
	days := daysFromTime(local)
	timeInDay := local - days*msPerDay
	y, m, dd := yearMonthDay(days)
	wd := (days + 4) % 7
	if wd < 0 {
		wd += 7
	}
	return fields{
		year: y, month: m, day: dd, weekday: int(wd),
		hour: int(timeInDay / msPerHour), minute: int(timeInDay/msPerMinute) % 60,
		second: int(timeInDay/msPerSecond) % 60, ms: int(timeInDay % 1000),
		days: days, timeInDay: timeInDay,
	}
}

func (d Date) get(field func(fields) int) float64 {
	if !d.Valid() {
		return math.NaN()
	}
	return float64(field(d.local()))
}

// FullYear is getFullYear().
func (d Date) FullYear() float64 { return d.get(func(f fields) int { return f.year }) }

// Month is getMonth(), from 0.
func (d Date) Month() float64 { return d.get(func(f fields) int { return f.month }) }

// Date is getDate(), the day of the month.
func (d Date) Date() float64 { return d.get(func(f fields) int { return f.day }) }

// Day is getDay(), the day of the week from Sunday, 0.
func (d Date) Day() float64 { return d.get(func(f fields) int { return f.weekday }) }

// Hours is getHours().
func (d Date) Hours() float64 { return d.get(func(f fields) int { return f.hour }) }

// Minutes is getMinutes().
func (d Date) Minutes() float64 { return d.get(func(f fields) int { return f.minute }) }

// Seconds is getSeconds().
func (d Date) Seconds() float64 { return d.get(func(f fields) int { return f.second }) }

// Milliseconds is getMilliseconds().
func (d Date) Milliseconds() float64 { return d.get(func(f fields) int { return f.ms }) }

// UTCSeconds is getUTCSeconds().
func (d Date) UTCSeconds() float64 {
	if !d.Valid() {
		return math.NaN()
	}
	t := int64(d.t)
	return float64((t - daysFromTime(t)*msPerDay) / msPerSecond % 60)
}

// TimezoneOffset is getTimezoneOffset(): minutes from local time to UTC. V8 truncates it to
// whole minutes, which matters for a zone's local mean time before standard time.
func (d Date) TimezoneOffset() float64 {
	if !d.Valid() {
		return math.NaN()
	}
	return float64(-offsetFromUTC(d.loc, int64(d.t)) / msPerMinute)
}

// SetTime is setTime(t).
func (d *Date) SetTime(t float64) { d.t = timeClip(t) }

// SetDate is setDate(day).
func (d *Date) SetDate(day float64) {
	if !d.Valid() {
		return
	}
	f := d.local()
	d.setLocal(makeDate(makeDay(float64(f.year), float64(f.month), day), float64(f.timeInDay)))
}

// SetMonth is setMonth(month).
func (d *Date) SetMonth(month float64) {
	if !d.Valid() {
		return
	}
	f := d.local()
	d.setLocal(makeDate(makeDay(float64(f.year), month, float64(f.day)), float64(f.timeInDay)))
}

// SetMonthDate is setMonth(month, day).
func (d *Date) SetMonthDate(month, day float64) {
	if !d.Valid() {
		return
	}
	f := d.local()
	d.setLocal(makeDate(makeDay(float64(f.year), month, day), float64(f.timeInDay)))
}

// SetFullYear is setFullYear(year). An invalid date becomes January 1 of that year.
func (d *Date) SetFullYear(year float64) {
	month, day, timeInDay := 0.0, 1.0, 0.0
	if d.Valid() {
		f := d.local()
		month, day, timeInDay = float64(f.month), float64(f.day), float64(f.timeInDay)
	}
	d.setLocal(makeDate(makeDay(year, month, day), timeInDay))
}

// SetHours is setHours(hours, minutes, seconds, ms).
func (d *Date) SetHours(hours, minutes, seconds, ms float64) {
	if !d.Valid() {
		return
	}
	d.setLocal(makeDate(float64(d.local().days), makeTime(hours, minutes, seconds, ms)))
}

// setLocal is V8's SetLocalDateValue: t is a local time.
func (d *Date) setLocal(t float64) { d.t = timeClip(localToUTC(d.loc, t)) }

// localToUTC converts a local time value to UTC, NaN when it is out of range.
func localToUTC(loc *time.Location, t float64) float64 {
	if !(t >= -maxTimeBeforeUTC && t <= maxTimeBeforeUTC) {
		return math.NaN()
	}
	local := int64(t)
	return float64(local - offsetFromLocal(loc, local))
}

// toInteger is V8's DoubleToInteger: NaN is 0, and fractions are truncated.
func toInteger(x float64) float64 {
	if math.IsNaN(x) {
		return 0
	}
	return math.Trunc(x)
}

func timeClip(t float64) float64 {
	if -maxTime <= t && t <= maxTime {
		return toInteger(t)
	}
	return math.NaN()
}

// makeDate, makeTime and makeDay are V8's, contracted as Node's arm64 build compiles them
// (see jsmath).
func makeDate(day, t float64) float64 {
	if math.IsInf(day, 0) || math.IsNaN(day) || math.IsInf(t, 0) || math.IsNaN(t) {
		return math.NaN()
	}
	return math.FMA(day, msPerDay, t)
}

func makeTime(h, m, s, ms float64) float64 {
	for _, v := range [...]float64{h, m, s, ms} {
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return math.NaN()
		}
	}
	hm := math.FMA(math.Trunc(h), msPerHour, float64(math.Trunc(m)*msPerMinute))
	return math.FMA(math.Trunc(s), msPerSecond, hm) + math.Trunc(ms)
}

var (
	daysBeforeMonth     = [12]int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}
	daysBeforeMonthLeap = [12]int{0, 31, 60, 91, 121, 152, 182, 213, 244, 274, 305, 335}
)

func makeDay(year, month, date float64) float64 {
	if !(-1e6 <= year && year <= 1e6) || !(-1e7 <= month && month <= 1e7) || math.IsInf(date, 0) || math.IsNaN(date) {
		return math.NaN()
	}
	y, m := int(year), int(month)
	y += m / 12
	m %= 12
	if m < 0 {
		m += 12
		y--
	}
	return float64(daysFromYear(y)+daysBefore(y, m)-1) + math.Trunc(date)
}

// daysFromYear is the days from January 1, 1970 to January 1 of y.
func daysFromYear(y int) int {
	const delta = 399999
	const base = 365*(1970+delta) + (1970+delta)/4 - (1970+delta)/100 + (1970+delta)/400
	y += delta
	return 365*y + y/4 - y/100 + y/400 - base
}

func daysBefore(y, m int) int {
	if y%4 != 0 || (y%100 == 0 && y%400 != 0) {
		return daysBeforeMonth[m]
	}
	return daysBeforeMonthLeap[m]
}

// daysFromTime is floor(t / msPerDay).
func daysFromTime(t int64) int64 {
	if t < 0 {
		t -= msPerDay - 1
	}
	return t / msPerDay
}

// yearMonthDay is V8's DateCache::YearMonthDayFromDays.
func yearMonthDay(days int64) (year, month, day int) {
	const (
		daysIn4Years   = 4*365 + 1
		daysIn100Years = 25*daysIn4Years - 1
		daysIn400Years = 4*daysIn100Years + 1
		yearsOffset    = 400000
		daysOffset     = 1000*daysIn400Years + 5*daysIn400Years - (30*365 + 7)
	)
	d := int(days) + daysOffset
	year = 400*(d/daysIn400Years) - yearsOffset
	d %= daysIn400Years
	d--
	yd1 := d / daysIn100Years
	d %= daysIn100Years
	year += 100 * yd1
	d++
	yd2 := d / daysIn4Years
	d %= daysIn4Years
	year += 4 * yd2
	d--
	yd3 := d / 365
	d %= 365
	year += yd3
	leap := (yd1 == 0 || yd2 != 0) && yd3 == 0
	if leap {
		d++
	}
	feb := 31 + 28
	if leap {
		feb++
	}
	if d >= feb {
		d -= feb
		lengths := [12]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
		for i := 2; i < 12; i++ {
			if d < lengths[i] {
				return year, i, d + 1
			}
			d -= lengths[i]
		}
	}
	if d < 31 {
		return year, 0, d + 1
	}
	return year, 1, d - 31 + 1
}
