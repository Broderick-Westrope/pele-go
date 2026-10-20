package requirement

// Token kinds.
const (
	tEnd = iota
	tError
	tTitle
	tAccTitle
	tAccTitleValue
	tAccDescr
	tAccDescrValue
	tAccDescrMultilineValue
	tDirectionTB
	tDirectionBT
	tDirectionRL
	tDirectionLR
	tNewline
	tEOF
	tRD
	tStructStart
	tStructStop
	tStyleSeparator
	tColonSep
	tID
	tText
	tRisk
	tVerifyMthd
	tRequirement
	tFunctionalRequirement
	tInterfaceRequirement
	tPerformanceRequirement
	tPhysicalRequirement
	tDesignConstraint
	tLowRisk
	tMedRisk
	tHighRisk
	tVerifyAnalysis
	tVerifyDemonstration
	tVerifyInspection
	tVerifyTest
	tElement
	tContains
	tCopies
	tDerives
	tSatisfies
	tVerifies
	tRefines
	tTraces
	tType
	tDocRef
	tStyle
	tClassDef
	tClass
	tAlpha
	tColon
	tSemicolon
	tPercent
	tMinus
	tBrkt
	tEndArrowL
	tEndArrowR
	tLine
	tQString
	tUnqString
	tComma
)

var tokenNames = [...]string{
	"$end",
	"INVALID",
	"title",
	"acc_title",
	"acc_title_value",
	"acc_descr",
	"acc_descr_value",
	"acc_descr_multiline_value",
	"direction_tb",
	"direction_bt",
	"direction_rl",
	"direction_lr",
	"NEWLINE",
	"EOF",
	"RD",
	"STRUCT_START",
	"STRUCT_STOP",
	"STYLE_SEPARATOR",
	"COLONSEP",
	"ID",
	"TEXT",
	"RISK",
	"VERIFYMTHD",
	"REQUIREMENT",
	"FUNCTIONAL_REQUIREMENT",
	"INTERFACE_REQUIREMENT",
	"PERFORMANCE_REQUIREMENT",
	"PHYSICAL_REQUIREMENT",
	"DESIGN_CONSTRAINT",
	"LOW_RISK",
	"MED_RISK",
	"HIGH_RISK",
	"VERIFY_ANALYSIS",
	"VERIFY_DEMONSTRATION",
	"VERIFY_INSPECTION",
	"VERIFY_TEST",
	"ELEMENT",
	"CONTAINS",
	"COPIES",
	"DERIVES",
	"SATISFIES",
	"VERIFIES",
	"REFINES",
	"TRACES",
	"TYPE",
	"DOCREF",
	"STYLE",
	"CLASSDEF",
	"CLASS",
	"ALPHA",
	"COLON",
	"SEMICOLON",
	"PERCENT",
	"MINUS",
	"BRKT",
	"END_ARROW_L",
	"END_ARROW_R",
	"LINE",
	"qString",
	"unqString",
	"COMMA",
}

// keywords: every keyword is matched without regard to case and must end at a word boundary.
var keywords = map[string]int{
	"requirementdiagram":     tRD,
	"id":                     tID,
	"text":                   tText,
	"risk":                   tRisk,
	"verifymethod":           tVerifyMthd,
	"requirement":            tRequirement,
	"functionalrequirement":  tFunctionalRequirement,
	"interfacerequirement":   tInterfaceRequirement,
	"performancerequirement": tPerformanceRequirement,
	"physicalrequirement":    tPhysicalRequirement,
	"designconstraint":       tDesignConstraint,
	"low":                    tLowRisk,
	"medium":                 tMedRisk,
	"high":                   tHighRisk,
	"analysis":               tVerifyAnalysis,
	"demonstration":          tVerifyDemonstration,
	"inspection":             tVerifyInspection,
	"test":                   tVerifyTest,
	"element":                tElement,
	"contains":               tContains,
	"copies":                 tCopies,
	"derives":                tDerives,
	"satisfies":              tSatisfies,
	"verifies":               tVerifies,
	"refines":                tRefines,
	"traces":                 tTraces,
	"type":                   tType,
	"docref":                 tDocRef,
	"style":                  tStyle,
	"classdef":               tClassDef,
	"class":                  tClass,
}
