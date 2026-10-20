package agentflow

// diagnosticIDs is Mermaid's diagnostic vocabulary for agentflow. Only the ids in emitted are
// raised by a parser; the rest are reserved for tools that check a diagram's meaning.
var diagnosticIDs = []string{
	"SHAPE_UNSUPPORTED",
	"SHAPE_REMOVED",
	"EDGE_OPERATOR_UNSUPPORTED",
	"REFERENCE_EDGE_LABEL_REJECTED",
	"CONNECTOR_REF_UNRESOLVED",
	"CONNECTOR_REF_NOT_A_CONNECTOR",
	"METADATA_KEY_MISAPPLIED",
	"DUPLICATE_ID_NODE",
	"RESERVED_SYNTHETIC_ID",
	"CONTAINMENT_VIOLATION",
	"EDGE_SEMANTIC_CONTRADICTION",
	"FLOW_NO_INPUT",
}

var emitted = setOf("SHAPE_UNSUPPORTED", "SHAPE_REMOVED", "CONTAINMENT_VIOLATION")
