package journey

import (
	"slices"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/diagrams/common"
	"github.com/Broderick-Westrope/pele-go/internal/jsnum"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
)

type journeyTask struct {
	task string
	// As written: any number, or NaN when the score is not one.
	score  float64
	people []string
	// Name of the section in force when the task was declared, "" before the first one.
	section string
}

// journeyDb is upstream's JourneyDb and JourneyModel. Empty strings stand for undefined:
// every use treats the two alike.
type journeyDb struct {
	title, accTitle, accDescr string
	sections                  []string
	tasks                     []journeyTask

	currentSection string
}

// journeyActors is everyone named on a task, once each, in code unit order.
func journeyActors(tasks []journeyTask) []string {
	var names omap.Set[string]
	for _, task := range tasks {
		for _, name := range task.people {
			names.Add(name)
		}
	}
	out := slices.Collect(names.All())
	slices.SortStableFunc(out, jsstr.Compare)
	return out
}

func (db *journeyDb) setDiagramTitle(text string) {
	db.title = text
}

func (db *journeyDb) setAccTitle(text string) {
	db.accTitle = jsstr.TrimStart(text)
}

func (db *journeyDb) setAccDescription(text string) {
	db.accDescr = common.CollapseIndentedBreaks(text)
}

func (db *journeyDb) addSection(text string) {
	db.currentSection = text
	db.sections = append(db.sections, text)
}

// addTask adds a task. data is the rest of the line from its first colon: `:score` or
// `:score:actor, actor`.
func (db *journeyDb) addTask(task, data string) {
	pieces := strings.Split(data[1:], ":")
	people := []string{}
	if len(pieces) > 1 {
		for _, name := range strings.Split(pieces[1], ",") {
			people = append(people, jsstr.Trim(name))
		}
	}
	db.tasks = append(db.tasks, journeyTask{task: task, score: jsnum.Parse(pieces[0]), people: people, section: db.currentSection})
}
