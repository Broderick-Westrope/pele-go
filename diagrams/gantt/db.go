package gantt

import (
	"math"
	"strings"
	"time"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsdate"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
	"github.com/Broderick-Westrope/pele-go/internal/util"
)

// Scheduling walks tasks day by day and parses dates with a format the diagram supplies.
// This bounds the total, in rough character operations, so a hostile diagram fails instead
// of hanging.
const (
	workLimit  = 4e8
	dayStep    = 400
	datedStep  = 200
	formatStep = 400
)

// parseDuration reads a duration such as "3d" or "1.5h"; the value is NaN when str is not one.
func parseDuration(str string) (float64, string) {
	s := jsstr.Trim(str)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return math.NaN(), "ms"
	}
	if i+1 < len(s) && s[i] == '.' && s[i+1] >= '0' && s[i+1] <= '9' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	switch unit := s[i:]; unit {
	case "M", "d", "h", "m", "s", "w", "y", "ms":
		return jsnum.Parse(s[:i]), unit
	}
	return math.NaN(), "ms"
}

// mergeTokens adds the tokens of txt, lower-cased and split at whitespace and commas, to
// existing, keeping the first of each.
func mergeTokens(existing []string, txt string) []string {
	merged := existing
	seen := map[string]bool{}
	for _, token := range existing {
		seen[token] = true
	}
	lower := jsstr.ToLowerCase(txt)
	start := 0
	for i := 0; i <= len(lower); {
		r, n := rune(','), 1
		if i < len(lower) {
			r, n = jsstr.Decode(lower, i)
		}
		if r == ',' || jsstr.IsSpace(r) {
			if token := lower[start:i]; token != "" && !seen[token] {
				seen[token] = true
				merged = append(merged, token)
			}
			start = i + n
		}
		i += n
	}
	return merged
}

// ganttDb is upstream's GanttDb as the parser leaves it. Scheduling, which upstream does in
// place on the database, works on a copy of the tasks (see schedule), so a parsed diagram
// can be rendered more than once, and at once.
type ganttDb struct {
	title, accTitle, accDescr string
	dateFormat                string
	axisFormat                string
	tickInterval              *string
	todayMarker               string
	includes, excludes        []string
	inclusiveEndDates         bool
	topAxis                   bool
	displayMode               string
	// weekday is the day week-based ticks start on, when the chart names one.
	weekday  *string
	weekend  string
	sections []string
	links    map[string]string
	tasks    []*task

	byID           map[string]int
	currentSection string
	taskCount      int
	lastOrder      int
	lastTaskID     *string
}

func newGanttDb() *ganttDb {
	return &ganttDb{weekend: "saturday", links: map[string]string{}, byID: map[string]int{}}
}

func (db *ganttDb) setDateFormat(txt string) { db.dateFormat = txt }

func (db *ganttDb) setAccTitle(txt string) { db.accTitle = jsstr.TrimStart(txt) }

func (db *ganttDb) setAccDescription(txt string) { db.accDescr = common.CollapseIndentedBreaks(txt) }

func (db *ganttDb) addSection(txt string) {
	db.currentSection = txt
	db.sections = append(db.sections, txt)
}

func (db *ganttDb) addTask(descr, dataStr string) {
	data := strings.Split(strings.TrimPrefix(dataStr, ":"), ",")
	t := &task{task: descr, section: db.currentSection, order: -1, prevTaskID: db.lastTaskID, raw: rawTask{data: dataStr}}

	for len(data) > 0 {
		tag := jsstr.Trim(data[0])
		switch tag {
		case "active":
			t.active = true
		case "done":
			t.done = true
		case "crit":
			t.crit = true
		case "milestone":
			t.milestone = true
		case "vert":
			t.vert = true
		default:
			goto tagged
		}
		data = data[1:]
	}
tagged:
	// Mermaid fails with a TypeError on both of these.
	if len(data) == 0 {
		semantic(`Task "` + jsstr.Trim(descr) + `" needs a duration or an end date.`)
	}
	if len(data) > 3 {
		semantic(`Task "` + jsstr.Trim(descr) + `" has too many fields. A task takes an id, a start, and an end or duration.`)
	}
	for i := range data {
		data[i] = jsstr.Trim(data[i])
	}
	if len(data) == 3 {
		t.id = data[0]
	} else {
		db.taskCount++
		t.id = "task" + jsnum.Format(float64(db.taskCount))
	}
	if len(data) > 1 {
		t.raw.start = ptr(data[len(data)-2])
	}
	t.raw.end = data[len(data)-1]

	if !t.vert {
		t.order = db.lastOrder
		db.lastOrder++
	}
	db.tasks = append(db.tasks, t)
	db.lastTaskID = ptr(t.id)
	db.byID[t.id] = len(db.tasks) - 1
}

func semantic(message string) {
	e := perr.New(message, perr.Semantic)
	e.Type = "gantt"
	perr.Throw(e)
}

func (db *ganttDb) setLink(ids, link string) {
	href := util.SafeURL(link)
	for _, id := range strings.Split(ids, ",") {
		if _, ok := db.byID[id]; ok {
			db.links[id] = href
		}
	}
	db.setClass(ids, "clickable")
}

func (db *ganttDb) setClass(ids, className string) {
	for _, id := range strings.Split(ids, ",") {
		if i, ok := db.byID[id]; ok {
			db.tasks[i].classes = append(db.tasks[i].classes, className)
		}
	}
}

// schedule is the state of one call of upstream's getTasks, which resolves every task's
// start and end from scratch.
type schedule struct {
	db    *ganttDb
	loc   *time.Location
	tasks []*task
	byID  map[string]*task
	now   float64
	work  float64

	includeSet, excludeSet map[string]bool
	excludedDays           [7]bool
	datedTokens            bool
}

// getTasks schedules the tasks against now. A task may refer to tasks defined after it, so,
// like Mermaid, this makes up to eleven passes, each of which schedules every task again.
func (db *ganttDb) getTasks(now float64, loc *time.Location) *schedule {
	s := &schedule{db: db, loc: loc, now: now, byID: map[string]*task{}, includeSet: map[string]bool{}, excludeSet: map[string]bool{}}
	for _, t := range db.tasks {
		c := *t
		c.startTime, c.endTime, c.renderEndTime = nil, nil, nil
		c.processed, c.manualEndTime = false, false
		s.tasks = append(s.tasks, &c)
	}
	for id, i := range db.byID {
		s.byID[id] = s.tasks[i]
	}
	for _, token := range db.includes {
		s.includeSet[token] = true
	}
	for _, token := range db.excludes {
		s.excludeSet[token] = true
	}
	weekendStart := -1
	switch db.weekend {
	case "friday":
		weekendStart = 5
	case "saturday":
		weekendStart = 6
	}
	weekends := s.excludeSet["weekends"]
	for day, name := range weekdayNames {
		iso := day
		if iso == 0 {
			iso = 7
		}
		s.excludedDays[day] = (weekends && weekendStart >= 0 && (iso == weekendStart || iso == weekendStart+1)) ||
			s.excludeSet[strings.ToLower(name)]
	}
	// A date written with only these tokens and separators can never read as a weekday name,
	// so when nothing else is excluded or included there is no need to format each day.
	s.datedTokens = len(db.includes) > 0 || !numericFormat(jsstr.Trim(db.dateFormat))
	for _, token := range db.excludes {
		if token != "weekends" && !isDayName(token) {
			s.datedTokens = true
		}
	}

	allProcessed := s.compileTasks()
	for i := 0; !allProcessed && i < 10; i++ {
		allProcessed = s.compileTasks()
	}
	return s
}

// numericFormat is /^[YMDHhmsSZXx\-/.:, T]*$/.
func numericFormat(s string) bool {
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune("YMDHhmsSZXx-/.:, T", rune(s[i])) || s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func isDayName(s string) bool {
	for _, name := range weekdayNames {
		if strings.ToLower(name) == s {
			return true
		}
	}
	return false
}

func (s *schedule) date(t float64) jsdate.Date { return jsdate.New(t, s.loc) }

// isInvalidDate reports whether a day is excluded. A day matches a token by its weekday
// name, by its date as YYYY-MM-DD, or by its date written in the chart's own format.
func (s *schedule) isInvalidDate(date jsdate.Date) bool {
	weekday := false
	if day := date.Day(); !math.IsNaN(day) {
		weekday = s.excludedDays[int(day)]
	}
	if !s.datedTokens {
		return weekday
	}
	template := jsstr.Trim(s.db.dateFormat)
	dateOnly := isoDate(date)
	formatted, ok := dateOnly, true
	s.spend(datedStep)
	if template != "YYYY-MM-DD" {
		s.spend(formatStep + float64(jsstr.Len(template)*8))
		formatted, ok = format(date, template)
	}
	listed := func(set map[string]bool) bool { return set[dateOnly] || (ok && set[formatted]) }
	if listed(s.includeSet) {
		return false
	}
	return weekday || listed(s.excludeSet)
}

func (s *schedule) spend(units float64) {
	s.work += units
	if s.work > workLimit {
		e := perr.New("The gantt chart takes too much work to schedule.", perr.Limit)
		e.Type = "gantt"
		perr.Throw(e)
	}
}

func (s *schedule) parse(str, template string) (jsdate.Date, bool) {
	s.spend(float64((jsstr.Len(template) + 1) * (jsstr.Len(str) + 1)))
	return parseStrict(str, template, s.date(s.now))
}

func (s *schedule) today() *jsdate.Date {
	today := s.date(s.now)
	today.SetHours(0, 0, 0, 0)
	return &today
}

func (s *schedule) compileTasks() bool {
	allProcessed := true
	for _, t := range s.tasks {
		s.compileTask(t)
		allProcessed = allProcessed && t.processed
	}
	return allProcessed
}

func (s *schedule) compileTask(t *task) {
	if t.raw.start == nil {
		// Mermaid fails with a TypeError when the first task has no start. It starts today here.
		var previous *task
		if t.prevTaskID != nil {
			previous = s.byID[*t.prevTaskID]
		}
		if previous != nil {
			t.startTime = previous.endTime
		} else {
			t.startTime = s.today()
		}
	} else if start := s.getStartDate(*t.raw.start); start != nil {
		t.startTime = start
	}
	if t.startTime == nil {
		return
	}
	t.endTime = s.getEndDate(*t.startTime, t.raw.end)
	if t.endTime == nil {
		return
	}
	t.processed = true
	_, t.manualEndTime = s.parse(t.raw.end, "YYYY-MM-DD")
	s.checkTaskDates(t)
}

// references returns the tasks a space-separated list of ids names. Upstream warns about
// the ids that name none through a hook the diagram never sets.
func (s *schedule) references(list string) []*task {
	var found []*task
	for _, id := range strings.Split(list, " ") {
		if t, ok := s.byID[id]; ok && id != "" {
			found = append(found, t)
		}
	}
	return found
}

// later and earlier compare dates as JS's > and < do, through their time values: false
// when either is undefined or invalid.
func later(a, b *jsdate.Date) bool   { return a != nil && b != nil && a.Time() > b.Time() }
func earlier(a, b *jsdate.Date) bool { return a != nil && b != nil && a.Time() < b.Time() }

func (s *schedule) getStartDate(raw string) *jsdate.Date {
	str := jsstr.Trim(raw)
	template := jsstr.Trim(s.db.dateFormat)

	if (template == "x" || template == "X") && isDigits(str) {
		d := s.date(jsnum.Parse(str))
		if !d.Valid() {
			semantic("Invalid date:" + str)
		}
		return &d
	}

	if list, ok := referenceList(str, "after"); ok {
		var latest *task
		for _, t := range s.references(list) {
			if latest == nil || later(t.endTime, latest.endTime) {
				latest = t
			}
		}
		if latest != nil {
			return latest.endTime
		}
		return s.today()
	}

	if parsed, ok := s.parse(str, template); ok {
		return &parsed
	}

	// What does not fit the format gets a second chance with the JavaScript date parser, as
	// in Mermaid.
	date := jsdate.Parse(str, s.loc)
	if year := date.FullYear(); !date.Valid() || year < -10000 || year > 10000 {
		semantic("Invalid date:" + str)
	}
	return &date
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// referenceList matches /^keyword\s+([\d\w- ]+)/ and returns the group.
func referenceList(str, keyword string) (string, bool) {
	if !strings.HasPrefix(str, keyword) {
		return "", false
	}
	u := jsstr.UTF16(str[len(keyword):])
	end := 0
	for end < len(u) && util.IsSpace(int(u[end])) {
		end++
	}
	if end == 0 {
		return "", false
	}
	inList := func(c uint16) bool { return c < 128 && (isWordChar(int(c)) || c == '-' || c == ' ') }
	// \s+ gives back whitespace until the list can start, which only a space can.
	for k := end; k > 0; k-- {
		if k < len(u) && inList(u[k]) {
			e := k
			for e < len(u) && inList(u[e]) {
				e++
			}
			return jsstr.FromUTF16(u[k:e]), true
		}
	}
	return "", false
}

func (s *schedule) getEndDate(startTime jsdate.Date, raw string) *jsdate.Date {
	str := jsstr.Trim(raw)

	if list, ok := referenceList(str, "until"); ok {
		var first *task
		for _, t := range s.references(list) {
			if first == nil || earlier(t.startTime, first.startTime) {
				first = t
			}
		}
		if first != nil {
			return first.startTime
		}
		return s.today()
	}

	template := jsstr.Trim(s.db.dateFormat)
	if parsed, ok := s.parse(str, template); ok {
		if s.db.inclusiveEndDates {
			parsed = addDays(parsed, 1)
		}
		return &parsed
	}

	value, unit := parseDuration(str)
	if math.IsNaN(value) {
		return &startTime
	}
	end := add(startTime, value, unit)
	if !end.Valid() {
		return &startTime
	}
	return &end
}

// checkTaskDates pushes the end of a task out by one day for every excluded day it covers.
func (s *schedule) checkTaskDates(t *task) {
	if len(s.db.excludes) == 0 || t.manualEndTime {
		return
	}
	cursor := addDays(*t.startTime, 1)
	endTime := *t.endTime
	maxEndTime := addDays(endTime, 10000).Time()
	invalid := false
	renderEndTime := math.NaN()
	hasRenderEnd := false
	for cursor.Time() <= endTime.Time() {
		s.spend(dayStep)
		if !invalid {
			renderEndTime, hasRenderEnd = endTime.Time(), true
		}
		invalid = s.isInvalidDate(cursor)
		if invalid {
			endTime.SetDate(endTime.Date() + 1)
			if endTime.Time() > maxEndTime {
				semantic("Failed to find a valid date that was not excluded by `excludes` after 10,000 iterations.")
			}
		}
		cursor.SetDate(cursor.Date() + 1)
	}
	t.endTime = &endTime
	if hasRenderEnd {
		r := s.date(renderEndTime)
		t.renderEndTime = &r
	}
}
