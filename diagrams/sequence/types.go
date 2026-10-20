package sequence

import "github.com/Broderick-Westrope/pele-go/internal/omap"

// The message kinds, numbered as in Mermaid.
const (
	lineSolid                         = 0
	lineDotted                        = 1
	lineNote                          = 2
	lineSolidCross                    = 3
	lineDottedCross                   = 4
	lineSolidOpen                     = 5
	lineDottedOpen                    = 6
	lineLoopStart                     = 10
	lineLoopEnd                       = 11
	lineAltStart                      = 12
	lineAltElse                       = 13
	lineAltEnd                        = 14
	lineOptStart                      = 15
	lineOptEnd                        = 16
	lineActiveStart                   = 17
	lineActiveEnd                     = 18
	lineParStart                      = 19
	lineParAnd                        = 20
	lineParEnd                        = 21
	lineRectStart                     = 22
	lineRectEnd                       = 23
	lineSolidPoint                    = 24
	lineDottedPoint                   = 25
	lineAutonumber                    = 26
	lineCriticalStart                 = 27
	lineCriticalOption                = 28
	lineCriticalEnd                   = 29
	lineBreakStart                    = 30
	lineBreakEnd                      = 31
	lineParOverStart                  = 32
	lineBidirectionalSolid            = 33
	lineBidirectionalDotted           = 34
	lineSolidTop                      = 41
	lineSolidBottom                   = 42
	lineStickTop                      = 43
	lineStickBottom                   = 44
	lineSolidArrowTopReverse          = 45
	lineSolidArrowBottomReverse       = 46
	lineStickArrowTopReverse          = 47
	lineStickArrowBottomReverse       = 48
	lineSolidTopDotted                = 51
	lineSolidBottomDotted             = 52
	lineStickTopDotted                = 53
	lineStickBottomDotted             = 54
	lineSolidArrowTopReverseDotted    = 55
	lineSolidArrowBottomReverseDotted = 56
	lineStickArrowTopReverseDotted    = 57
	lineStickArrowBottomReverseDotted = 58
	lineCentralConnection             = 59
	lineCentralConnectionReverse      = 60
	lineCentralConnectionDual         = 61
)

const (
	placementLeftOf = iota
	placementRightOf
	placementOver
)

// seqText is upstream's SeqText. A nil wrap is undefined.
type seqText struct {
	text string
	wrap *bool
}

// seqBoxData is upstream's SeqBoxData. An empty text is undefined: upstream never keeps an
// empty one.
type seqBoxData struct {
	text  string
	color string
	wrap  *bool
}

type seqBox struct {
	// name is "" for undefined.
	name      string
	wrap      bool
	fill      string
	actorKeys []string
}

type seqActor struct {
	box         *seqBox
	name        string
	description string
	wrap        bool
	prevActor   string
	nextActor   string
	links       omap.Map[string, any]
	properties  omap.Map[string, any]
	details     string
	typ         string
}

type seqNumbering struct {
	// start and step are 0 for undefined: both are only read with ||.
	start, step float64
	visible     bool
}

// seqMessage is upstream's SeqMessage. from and to are "" for undefined: a participant id
// is never empty. message is a string, or the numbering of an autonumber statement.
type seqMessage struct {
	id                string
	from, to          string
	message           string
	numbering         *seqNumbering
	wrap              bool
	typ               int
	activate          bool
	placement         int
	centralConnection int
}

type seqNote struct {
	actor     []string
	placement int
	message   string
	wrap      bool
}

// seqStatement is one entry of the statement list the parser hands to apply. Upstream's
// several text fields (loopText, optText, ...) are text, as only one is set per statement.
type seqStatement struct {
	typ string
	// actor is the participant, or for a note over two participants both of them.
	actor             string
	actors            []string
	draw              string
	description       *seqText
	config            *string
	signalType        int
	from, to          string
	msg               *seqText
	activate          bool
	centralConnection int
	placement         int
	text              *seqText
	boxData           *seqBoxData
	sequenceIndex     float64
	sequenceIndexStep float64
	sequenceVisible   bool
}
