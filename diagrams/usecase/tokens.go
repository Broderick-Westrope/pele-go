package usecase

import "strings"

// Mermaid's token types, in an order that makes its token categories ranges:
// WORD is tUsecase to tIdentifier, and LABEL_TEXT is tUsecase to tLabelSymbol.
const (
	tEOF = iota
	tUsecase
	tActor
	tSystemBoundary
	tEnd
	tDirection
	tTD
	tTB
	tBT
	tLR
	tRL
	tNote
	tFor
	tJSON
	tClassDef
	tClass
	tStyle
	tInclude
	tExtend
	tTrue
	tFalse
	tIdentifier
	tNumber
	tAt
	tComma
	tColon
	tHashColor
	tCSSIdentifier
	tCSSEscapedComma
	tDash
	tDot
	tPercent
	tCSSPunctuation
	tLabelPunctuation
	tLabelSymbol
	tPlainString
	tMarkdownString
	tUnclosedMarkdownString
	tComment
	tNewline
	tAccDescrBlock
	tAccTitleLine
	tAccDescrLine
	tJSONDeclarationStart
	tJSONObjectLiteral
	tUnclosedJSONObjectLiteral
	tGeneralization
	tDependencyArrow
	tStereotypeStart
	tStereotypeEnd
	tStereotypeText
	tUnclosedStereotypeText
	tClassSeparator
	tForwardSolid
	tBackwardSolid
	tForwardCircle
	tBackwardCircle
	tForwardCross
	tBackwardCross
	tMarkerlessSolid
	tMetadataStart
	tLBrace
	tRBrace
	tLBracket
	tRBracket
	tLParen
	tRParen
)

var tokenNames = strings.Split(
	"EOF USECASE ACTOR SYSTEM_BOUNDARY END DIRECTION TD TB BT LR RL NOTE FOR JSON CLASS_DEF CLASS STYLE INCLUDE EXTEND TRUE FALSE IDENTIFIER NUMBER AT COMMA COLON HASH_COLOR CSS_IDENTIFIER CSS_ESCAPED_COMMA DASH DOT PERCENT CSS_PUNCTUATION LABEL_PUNCTUATION LABEL_SYMBOL PLAIN_STRING MARKDOWN_STRING UNCLOSED_MARKDOWN_STRING COMMENT NEWLINE ACC_DESCR_BLOCK ACC_TITLE_LINE ACC_DESCR_LINE JSON_DECLARATION_START JSON_OBJECT_LITERAL UNCLOSED_JSON_OBJECT_LITERAL GENERALIZATION DEPENDENCY_ARROW STEREOTYPE_START STEREOTYPE_END STEREOTYPE_TEXT UNCLOSED_STEREOTYPE_TEXT CLASS_SEPARATOR FORWARD_SOLID BACKWARD_SOLID FORWARD_CIRCLE BACKWARD_CIRCLE FORWARD_CROSS BACKWARD_CROSS MARKERLESS_SOLID METADATA_START LBRACE RBRACE LBRACKET RBRACKET LPAREN RPAREN",
	" ",
)
