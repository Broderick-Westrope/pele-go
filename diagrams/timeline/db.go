package timeline

import (
	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
)

type timelinePeriod struct {
	text   string
	events []string
	// Name of the section in force when the period was declared, "" before the first one.
	section string
	// Index into sections of that section, -1 before the first one.
	sectionIndex int
}

// timelineDb is upstream's TimelineDb and TimelineModel. Empty strings stand for undefined:
// every use treats the two alike.
type timelineDb struct {
	title, accTitle, accDescr string
	direction                 string
	sections                  []string
	periods                   []*timelinePeriod

	currentSection string
}

func newTimelineDb() *timelineDb {
	return &timelineDb{direction: "LR"}
}

func (db *timelineDb) setDiagramTitle(text string) {
	db.title = text
}

func (db *timelineDb) setAccTitle(text string) {
	db.accTitle = jsstr.TrimStart(text)
}

func (db *timelineDb) setAccDescription(text string) {
	db.accDescr = common.CollapseIndentedBreaks(text)
}

func (db *timelineDb) setDirection(direction string) {
	db.direction = direction
}

func (db *timelineDb) addSection(text string) {
	db.currentSection = text
	db.sections = append(db.sections, text)
}

func (db *timelineDb) addTask(period, event string) {
	events := []string{}
	if event != "" {
		events = append(events, event)
	}
	db.periods = append(db.periods, &timelinePeriod{
		text:         period,
		events:       events,
		section:      db.currentSection,
		sectionIndex: len(db.sections) - 1,
	})
}

func (db *timelineDb) addEvent(event string) {
	if len(db.periods) == 0 {
		// Mermaid fails here too, with a TypeError.
		e := perr.New("An event needs a time period before it.", perr.Semantic)
		e.Type = "timeline"
		perr.Throw(e)
	}
	period := db.periods[len(db.periods)-1]
	period.events = append(period.events, event)
}
