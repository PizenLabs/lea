// Package contracts defines the interfaces for graph storage.
package contracts

import (
	"context"
	graph "github.com/PizenLabs/lea/internal/graph/contracts"
)

// Store defines the interface for persisting and querying the structural graph.
type Store interface {
	SaveNode(ctx context.Context, node *graph.Node) error
	SaveEdge(ctx context.Context, edge *graph.Edge) error
	SaveGraph(ctx context.Context, nodes []*graph.Node, edges []*graph.Edge) error
	GetNode(ctx context.Context, id string) (*graph.Node, error)
	// SearchNodes returns nodes whose ID matches the LIKE pattern (use % for wildcards).
	SearchNodes(ctx context.Context, pattern string) ([]*graph.Node, error)
	ListNodes(ctx context.Context) ([]*graph.Node, error)
	GetNeighbors(ctx context.Context, id string) ([]*graph.Node, []*graph.Edge, error)
	GetInboundEdges(ctx context.Context, id string) ([]*graph.Node, []*graph.Edge, error)
	GetImpactRecursive(ctx context.Context, id string) ([]*graph.Node, []*graph.Edge, error)
	ListEdges(ctx context.Context) ([]*graph.Edge, error)
	GetStats(ctx context.Context) (*Stats, error)
	DeleteNode(ctx context.Context, id string) error
	DeleteByFile(ctx context.Context, file string) error
	DeleteEdgesFrom(ctx context.Context, id string) error
	// ListNodesByType returns all nodes of a specific node type.
	ListNodesByType(ctx context.Context, nodeType graph.NodeType) ([]*graph.Node, error)
	// GetEdgesByType returns all edges of a specific edge type.
	GetEdgesByType(ctx context.Context, edgeType graph.EdgeType) ([]*graph.Edge, error)
	Close() error
}

// Stats represents repository-wide structural statistics.
type Stats struct {
	NodesCount int
	EdgesCount int
	Languages  []string
}
