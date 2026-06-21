// Package contracts defines the core data structures for the structural graph.
package contracts

// EdgeType represents the nature of a relationship between two nodes.
type EdgeType string

const (
	// EdgeCalls indicates that one symbol calls another.
	EdgeCalls EdgeType = "CALLS"
	// EdgeImplements indicates that a type implements an interface.
	EdgeImplements EdgeType = "IMPLEMENTS"
	// EdgeUses indicates that one symbol uses or references another.
	EdgeUses EdgeType = "USES"
	// EdgeImports indicates that one file or package imports another.
	EdgeImports EdgeType = "IMPORTS"
	// EdgeBelongsTo indicates a containment relationship (e.g., method belongs to class).
	EdgeBelongsTo EdgeType = "BELONGS_TO"
	// EdgeDependsOn indicates a general dependency.
	EdgeDependsOn EdgeType = "DEPENDS_ON"
	// EdgeFlowsThrough indicates data flow between symbols.
	EdgeFlowsThrough EdgeType = "FLOWS_THROUGH"
	// EdgeImplementsMethod indicates that a concrete method implements an interface method.
	EdgeImplementsMethod EdgeType = "IMPLEMENTS_METHOD"
)

// Edge represents a directed relationship between two nodes in the graph.
type Edge struct {
	FromID   string                 `json:"from_id"`
	ToID     string                 `json:"to_id"`
	Type     EdgeType               `json:"type"`
	Sequence int                    `json:"sequence"`
	Metadata map[string]interface{} `json:"metadata"`
}
