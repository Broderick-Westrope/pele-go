package xychart

// Token kinds, in upstream's order.
const (
	tEnd = iota
	tError
	tNewline
	tTitle
	tAccTitle
	tAccTitleValue
	tAccDescr
	tAccDescrValue
	tAccDescrMultilineValue
	tXYChart
	tChartOrientation
	tXAxis
	tYAxis
	tArrowDelimiter
	tLine
	tBar
	tNumberWithDecimal
	tSquareBracesStart
	tSquareBracesEnd
	tStr
	tAlpha
	tColon
	tPlus
	tComma
	tEquals
	tMult
	tBrkt
	tUnderscore
	tDot
	tAmp
	tMinus
	tNum
	tSemi
	tEOF
)

var tokenNames = [...]string{
	"$end",
	"INVALID",
	"NEWLINE",
	"title",
	"acc_title",
	"acc_title_value",
	"acc_descr",
	"acc_descr_value",
	"acc_descr_multiline_value",
	"XYCHART",
	"CHART_ORIENTATION",
	"X_AXIS",
	"Y_AXIS",
	"ARROW_DELIMITER",
	"LINE",
	"BAR",
	"NUMBER_WITH_DECIMAL",
	"SQUARE_BRACES_START",
	"SQUARE_BRACES_END",
	"STR",
	"ALPHA",
	"COLON",
	"PLUS",
	"COMMA",
	"EQUALS",
	"MULT",
	"BRKT",
	"UNDERSCORE",
	"DOT",
	"AMP",
	"MINUS",
	"NUM",
	"SEMI",
	"EOF",
}
