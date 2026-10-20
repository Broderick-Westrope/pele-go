package jsdate

import "time"

// offsetFromUTC is the offset of loc from UTC at time t, in milliseconds: V8's
// LocalOffsetInMs(t, true).
func offsetFromUTC(loc *time.Location, t int64) int64 {
	_, offset := time.UnixMilli(t).In(loc).Zone()
	return int64(offset) * 1000
}

// offsetFromLocal is the offset of loc from UTC at local time t: V8's LocalOffsetInMs(t,
// false), which asks ICU with UCAL_TZ_LOCAL_FORMER for both cases a transition makes. A
// local time a transition skips is read with the offset before it (2:30 becomes 3:30 when
// clocks go forward at 2:00), and one a transition repeats is the earlier of the two.
func offsetFromLocal(loc *time.Location, t int64) int64 {
	const day = 86400000
	before := offsetFromUTC(loc, t-day)
	after := offsetFromUTC(loc, t+day)
	if before == after {
		if offsetFromUTC(loc, t-before) == before {
			return before
		}
		// Two transitions within a day of each other; settle on the offset at the guess.
		return offsetFromUTC(loc, t-before)
	}
	if offsetFromUTC(loc, t-before) == before {
		// Valid with the earlier offset; in an overlap that is also the earlier instant.
		return before
	}
	if offsetFromUTC(loc, t-after) == after {
		return after
	}
	return before
}
