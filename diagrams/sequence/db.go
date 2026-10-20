package sequence

import (
	"strconv"
	"strings"

	"github.com/Broderick-Westrope/pele-go/internal/jsjson"
	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
	"github.com/Broderick-Westrope/pele-go/internal/omap"
	"github.com/Broderick-Westrope/pele-go/internal/perr"
	"github.com/Broderick-Westrope/pele-go/internal/yaml"
)

type seqDbOptions struct {
	// wrap is the top-level wrap setting, which the %%{wrap}%% directive also sets. Nil is
	// undefined.
	wrap *bool
	// sequenceWrap is sequence.wrap.
	sequenceWrap bool
}

func boolPtr(b bool) *bool { return &b }

func fail(message string) {
	e := perr.New(message, perr.Semantic)
	e.Type = "sequence"
	perr.Throw(e)
}

func extractWrap(raw string) seqText {
	text := jsstr.Trim(raw)
	rest, isWrap := cutWrapPrefix(text, "wrap:")
	if isWrap {
		return seqText{text: jsstr.Trim(rest), wrap: boolPtr(true)}
	}
	rest, isNowrap := cutWrapPrefix(text, "nowrap:")
	if isNowrap {
		return seqText{text: jsstr.Trim(rest), wrap: boolPtr(false)}
	}
	return seqText{text: text}
}

// cutWrapPrefix matches /^:?wrap:/ or /^:?nowrap:/.
func cutWrapPrefix(text, prefix string) (string, bool) {
	rest := strings.TrimPrefix(text, ":")
	if strings.HasPrefix(rest, prefix) {
		return rest[len(prefix):], true
	}
	return "", false
}

// isBoxLineTerminator is /[\r\u2028\u2029]/ for one character.
func isBoxLineTerminator(r rune) bool {
	return r == '\r' || r == 0x2028 || r == 0x2029
}

func hasBoxLineTerminator(s string) bool {
	for i := 0; i < len(s); {
		r, n := jsstr.Decode(s, i)
		if isBoxLineTerminator(r) {
			return true
		}
		i += n
	}
	return false
}

// boxFunction returns the length of the match of /^(?:rgba?|hsla?)\s*\(/ at the start of s,
// or -1.
func boxFunction(s string) int {
	i := 0
	switch {
	case strings.HasPrefix(s, "rgb"), strings.HasPrefix(s, "hsl"):
		i = 3
	default:
		return -1
	}
	if i < len(s) && s[i] == 'a' {
		i++
	}
	for i < len(s) {
		r, n := jsstr.Decode(s, i)
		if !jsstr.IsSpace(r) {
			break
		}
		i += n
	}
	if i < len(s) && s[i] == '(' {
		return i + 1
	}
	return -1
}

// splitBox splits a box line into a leading color and the rest, as Mermaid's
// /^((?:rgba?|hsla?)\s*\(.*\)|\w*)(.*)$/ does. `.` stops at a line terminator, so a line that
// holds one outside the gap before the parenthesis does not match at all.
func splitBox(str string) (color, rest string, ok bool) {
	if fn := boxFunction(str); fn >= 0 {
		if end := strings.LastIndexByte(str, ')'); end >= fn {
			if hasBoxLineTerminator(str[fn:]) {
				return "", "", false
			}
			return str[:end+1], str[end+1:], true
		}
	}
	if hasBoxLineTerminator(str) {
		return "", "", false
	}
	w := 0
	for w < len(str) && isWordByte(str[w]) {
		w++
	}
	return str[:w], str[w:], true
}

func isWordByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

type entry struct {
	key   string
	value any
}

func entries(value any) []entry {
	var out []entry
	switch v := value.(type) {
	case string:
		for i, k := 0, 0; i < len(v); k++ {
			_, n := jsstr.Decode(v, i)
			out = append(out, entry{strconv.Itoa(k), v[i : i+n]})
			i += n
		}
	case []any:
		for k, e := range v {
			out = append(out, entry{strconv.Itoa(k), e})
		}
	case *jsobj.Object:
		for _, k := range v.Keys() {
			e, _ := v.Get(k)
			out = append(out, entry{k, e})
		}
	}
	return out
}

// seqDb is upstream's SeqDb, the sequence model.
type seqDb struct {
	actors          omap.Map[string, *seqActor]
	createdActors   omap.Map[string, int]
	destroyedActors omap.Map[string, int]
	boxes           []*seqBox
	messages        []seqMessage
	notes           []seqNote
	// title, accTitle and accDescr are "" for undefined: every use treats the two alike.
	title, accTitle, accDescr string

	prevActor     string
	currentBox    *seqBox
	lastCreated   string
	lastDestroyed string
	wrapEnabled   *bool
	sequenceWrap  bool
	active        map[string]int
}

func newSeqDb(options seqDbOptions) *seqDb {
	return &seqDb{wrapEnabled: options.wrap, sequenceWrap: options.sequenceWrap, active: map[string]int{}}
}

func (db *seqDb) autoWrap() bool {
	if db.wrapEnabled != nil {
		return *db.wrapEnabled
	}
	return db.sequenceWrap
}

func (db *seqDb) setAccTitle(text string) {
	db.accTitle = jsstr.TrimStart(text)
}

func (db *seqDb) setAccDescription(text string) {
	db.accDescr = collapseIndentedBreaks(text)
}

// collapseIndentedBreaks is s.replace(/\n\s+/g, '\n').
func collapseIndentedBreaks(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		b.WriteByte(s[i])
		if s[i] != '\n' {
			i++
			continue
		}
		i++
		for i < len(s) {
			r, n := jsstr.Decode(s, i)
			if !jsstr.IsSpace(r) {
				break
			}
			i += n
		}
	}
	return b.String()
}

func (db *seqDb) setDiagramTitle(text string) {
	db.title = text
}

func (db *seqDb) parseMessage(str string) seqText {
	return extractWrap(str)
}

func (db *seqDb) parseBoxData(str string) seqBoxData {
	match0, match1, ok := splitBox(str)
	color := "transparent"
	if ok && match0 != "" {
		color = jsstr.Trim(match0)
	}
	var title string
	hasTitle := ok && match1 != ""
	if hasTitle {
		title = jsstr.Trim(match1)
	}
	if !isCssColor(color) {
		color = "transparent"
		title, hasTitle = jsstr.Trim(str), true
	}
	if !hasTitle {
		return seqBoxData{color: color}
	}
	t := extractWrap(title)
	return seqBoxData{text: t.text, color: color, wrap: t.wrap}
}

func (db *seqDb) addBox(data seqBoxData) {
	wrap := db.autoWrap()
	if data.wrap != nil {
		wrap = *data.wrap
	}
	db.currentBox = &seqBox{name: data.text, wrap: wrap, fill: data.color}
	db.boxes = append(db.boxes, db.currentBox)
}

// boxName is a box's name in a message, where upstream's undefined is spelled out.
func boxName(b *seqBox) string {
	if b.name == "" {
		return "undefined"
	}
	return b.name
}

// addActor is upstream's addActor. typ is "" with hasType false for undefined; metadata is
// nil for undefined.
func (db *seqDb) addActor(id, name string, description *seqText, typ string, hasType bool, metadata *string) {
	assignedBox := db.currentBox
	var doc *jsobj.Object
	if metadata != nil {
		src := "{\n" + *metadata + "\n}"
		if strings.Contains(*metadata, "\n") {
			src = *metadata + "\n"
		}
		parsed, err := yaml.Parse(src)
		if err != nil {
			e := perr.New(`Config of participant "`+id+`" is not valid YAML. `+err.Error(), perr.Syntax)
			e.Type = "sequence"
			perr.Throw(e)
		}
		if o, ok := parsed.(*jsobj.Object); ok {
			doc = o
		}
	}
	docType, hasDocType := doc.Get("type")
	docAlias, _ := doc.Get("alias")
	if hasDocType && docType != nil {
		typ, hasType = jsobj.ToString(docType), true
	}
	if jsobj.Truthy(docAlias) && (description == nil || description.text == name) {
		var wrap *bool
		if description != nil {
			wrap = description.wrap
		}
		description = &seqText{text: jsobj.ToString(docAlias), wrap: wrap}
	}

	if old, ok := db.actors.Get(id); ok {
		if db.currentBox != nil && old.box != nil && db.currentBox != old.box {
			fail("A same participant should only be defined in one Box: " + old.name + " can't be in '" + boxName(old.box) +
				"' and in '" + boxName(db.currentBox) + "' at the same time.")
		}
		if old.box != nil {
			assignedBox = old.box
		} else {
			assignedBox = db.currentBox
		}
		old.box = assignedBox
		if name == old.name && description == nil {
			return
		}
	}
	// A participant that is only mentioned in a message has no kind and takes its name as its label.
	if description == nil || !hasType {
		description = &seqText{text: name}
	}

	wrap := db.autoWrap()
	if description.wrap != nil {
		wrap = *description.wrap
	}
	if !hasType {
		typ = "participant"
	}
	db.actors.Set(id, &seqActor{
		box:         assignedBox,
		name:        name,
		description: description.text,
		wrap:        wrap,
		prevActor:   db.prevActor,
		typ:         typ,
	})
	if db.prevActor != "" {
		if prev, ok := db.actors.Get(db.prevActor); ok {
			prev.nextActor = id
		}
	}
	if db.currentBox != nil {
		db.currentBox.actorKeys = append(db.currentBox.actorKeys, id)
	}
	db.prevActor = id
}

// addSignal is upstream's addSignal. from and to are "" for undefined, as is message nil.
func (db *seqDb) addSignal(from, to string, message *seqText, typ int, activate bool, centralConnection int) {
	// Mermaid recounts every earlier message here. A running count per participant says the same.
	if typ == lineActiveEnd {
		count := db.active[from]
		if count < 1 {
			fail("Trying to inactivate an inactive participant (" + from + ")")
		}
		db.active[from] = count - 1
	} else if typ == lineActiveStart && from != "" {
		db.active[from]++
	}
	text, wrap := "", db.autoWrap()
	if message != nil {
		text = message.text
		if message.wrap != nil {
			wrap = *message.wrap
		}
	}
	db.messages = append(db.messages, seqMessage{
		id:                strconv.Itoa(len(db.messages)),
		from:              from,
		to:                to,
		message:           text,
		wrap:              wrap,
		typ:               typ,
		activate:          activate,
		centralConnection: centralConnection,
	})
}

func (db *seqDb) addNote(actor []string, placement int, message *seqText) {
	wrap := db.autoWrap()
	if message.wrap != nil {
		wrap = *message.wrap
	}
	db.notes = append(db.notes, seqNote{actor: actor, placement: placement, message: message.text, wrap: wrap})
	db.messages = append(db.messages, seqMessage{
		id:        strconv.Itoa(len(db.messages)),
		from:      actor[0],
		to:        actor[1],
		message:   message.text,
		wrap:      wrap,
		typ:       lineNote,
		placement: placement,
	})
}

// addLinks is upstream's addLinks. Mermaid logs and ignores menu text that is not valid JSON.
// It also undoes the `&amp;` and `&equals;` its sanitizer writes; Pele has no sanitizer, and a
// `;` cannot occur in the text.
func (db *seqDb) addLinks(actorID string, text *seqText) {
	actor, ok := db.actors.Get(actorID)
	if !ok {
		return
	}
	parsed, err := jsjson.Parse(text.text)
	if err != nil {
		return
	}
	for _, e := range entries(parsed) {
		actor.links.Set(e.key, e.value)
	}
}

func (db *seqDb) addALink(actorID string, text *seqText) {
	actor, ok := db.actors.Get(actorID)
	if !ok {
		return
	}
	s := text.text
	sep := strings.IndexByte(s, '@')
	// slice(0, sep - 1), where a negative end counts from the end.
	end := -2
	if sep >= 0 {
		end = jsstr.Len(s[:sep]) - 1
	}
	if end < 0 {
		end += jsstr.Len(s)
	}
	actor.links.Set(jsstr.Trim(jsstr.Slice(s, 0, end)), jsstr.Trim(s[sep+1:]))
}

func (db *seqDb) addProperties(actorID string, text *seqText) {
	actor, ok := db.actors.Get(actorID)
	if !ok {
		return
	}
	parsed, err := jsjson.Parse(text.text)
	if err != nil {
		return
	}
	for _, e := range entries(parsed) {
		actor.properties.Set(e.key, e.value)
	}
}

// addDetails keeps the id of the page element Mermaid reads properties and links from.
func (db *seqDb) addDetails(actorID string, text *seqText) {
	if actor, ok := db.actors.Get(actorID); ok {
		actor.details = text.text
	}
}

func (db *seqDb) apply(list []*seqStatement) {
	for _, s := range list {
		db.applyOne(s)
	}
}

func (db *seqDb) applyOne(param *seqStatement) {
	actor := param.actor
	switch param.typ {
	case "sequenceIndex":
		db.messages = append(db.messages, seqMessage{
			id:        strconv.Itoa(len(db.messages)),
			numbering: &seqNumbering{start: param.sequenceIndex, step: param.sequenceIndexStep, visible: param.sequenceVisible},
			typ:       param.signalType,
		})
	case "addParticipant":
		db.addActor(actor, actor, param.description, param.draw, param.draw != "", param.config)
	case "createParticipant":
		if db.actors.Has(actor) {
			fail("It is not possible to have actors with the same id, even if one is destroyed before the next is created. Use 'AS' aliases to simulate the behavior")
		}
		db.lastCreated = actor
		db.addActor(actor, actor, param.description, param.draw, param.draw != "", param.config)
		db.createdActors.Set(actor, len(db.messages))
	case "destroyParticipant":
		db.lastDestroyed = actor
		db.destroyedActors.Set(actor, len(db.messages))
	case "activeStart", "activeEnd", "centralConnection", "centralConnectionReverse":
		db.addSignal(actor, "", nil, param.signalType, false, 0)
	case "addNote":
		actors := param.actors
		if actors == nil {
			actors = []string{actor, actor}
		}
		db.addNote(actors, param.placement, param.text)
	case "addLinks":
		db.addLinks(actor, param.text)
	case "addALink":
		db.addALink(actor, param.text)
	case "addProperties":
		db.addProperties(actor, param.text)
	case "addDetails":
		db.addDetails(actor, param.text)
	case "addMessage":
		if db.lastCreated != "" {
			if param.to != db.lastCreated {
				fail("The created participant " + db.lastCreated + " does not have an associated creating message after its declaration. Please check the sequence diagram.")
			}
			db.lastCreated = ""
		} else if db.lastDestroyed != "" {
			if param.to != db.lastDestroyed && param.from != db.lastDestroyed {
				fail("The destroyed participant " + db.lastDestroyed + " does not have an associated destroying message after its declaration. Please check the sequence diagram.")
			}
			db.lastDestroyed = ""
		}
		db.addSignal(param.from, param.to, param.msg, param.signalType, param.activate, param.centralConnection)
	case "boxStart":
		db.addBox(*param.boxData)
	case "boxEnd":
		db.currentBox = nil
	case "loopStart", "rectStart", "optStart", "altStart", "else", "parStart", "and", "criticalStart", "option", "breakStart":
		db.addSignal("", "", param.text, param.signalType, false, 0)
	case "loopEnd", "rectEnd", "optEnd", "altEnd", "parEnd", "criticalEnd", "breakEnd":
		db.addSignal("", "", nil, param.signalType, false, 0)
	}
}
