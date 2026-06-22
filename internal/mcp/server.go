// Package mcp exposes the Model Context Protocol server.
package mcp

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	aictx "github.com/PizenLabs/lea/internal/ai/context"
	"github.com/PizenLabs/lea/internal/architecture"
	graph "github.com/PizenLabs/lea/internal/graph/contracts"
	"github.com/PizenLabs/lea/internal/storage/contracts"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// Server exposes MCP tools backed by the graph store.
type Server struct {
	store    contracts.Store
	compiler *aictx.Compiler
}

// -----------------------------------------------------------------------------
// Tool Arguments
// -----------------------------------------------------------------------------

// GetSymbolContextArgs defines the input for the get_symbol_context tool.
type GetSymbolContextArgs struct {
	SymbolID string `json:"symbol_id" jsonschema:"description=The unique symbol ID (e.g. func:path:name)"`
}

// FindNeighborsArgs defines the input for the find_neighbors tool.
type FindNeighborsArgs struct {
	SymbolID string `json:"symbol_id" jsonschema:"description=The unique symbol ID"`
}

// TraceCallsArgs defines the input for the trace_calls tool.
type TraceCallsArgs struct {
	SymbolID string `json:"symbol_id" jsonschema:"description=The root symbol ID"`
	Depth    int    `json:"depth" jsonschema:"description=Maximum traversal depth,default=3"`
}

// TraceExecutionPathArgs defines the input for the trace_execution_path tool.
type TraceExecutionPathArgs struct {
	SymbolID string `json:"symbol_id" jsonschema:"description=The root symbol ID"`
}

// FindArchitectureViolationsArgs defines the input for the
// find_architecture_violations tool.
type FindArchitectureViolationsArgs struct {
	ConfigPath string `json:"config_path" jsonschema:"description=Path to architecture YAML config"`
}

// -----------------------------------------------------------------------------
// Constructor
// -----------------------------------------------------------------------------

// NewServer creates a new MCP server instance.
func NewServer(store contracts.Store) *Server {
	return &Server{
		store:    store,
		compiler: aictx.NewCompiler(store),
	}
}

// -----------------------------------------------------------------------------
// Server Lifecycle
// -----------------------------------------------------------------------------

// Start registers all MCP tools and starts the stdio transport server.
func (s *Server) Start() error {
	server := mcp_golang.NewServer(
		stdio.NewStdioServerTransport(),
		mcp_golang.WithName("lea"),
	)

	if err := s.registerTools(server); err != nil {
		return err
	}

	if err := server.Serve(); err != nil {
		return err
	}

	// Set up monitored signal channel for graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, os.Interrupt)

	// Launch background goroutine to intercept the signal.
	go func() {
		<-sigChan
		// Cleanly close the store connections.
		if s.store != nil {
			_ = s.store.Close()
		}
		os.Exit(0)
	}()

	// Block until the process receives a termination signal.
	select {}
}

// registerTools registers every MCP tool exposed by lea.
func (s *Server) registerTools(server *mcp_golang.Server) error {
	tools := []func(*mcp_golang.Server) error{
		s.registerGetSymbolContextTool,
		s.registerFindNeighborsTool,
		s.registerTraceCallsTool,
		s.registerTraceExecutionPathTool,
		s.registerArchitectureViolationsTool,
	}

	for _, register := range tools {
		if err := register(server); err != nil {
			return err
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// Tool Registration
// -----------------------------------------------------------------------------

// registerGetSymbolContextTool registers the symbol context compiler tool.
func (s *Server) registerGetSymbolContextTool(server *mcp_golang.Server) error {
	return server.RegisterTool(
		"get_symbol_context",
		"Generates AI-optimized markdown context for a symbol",
		func(
			ctx context.Context,
			args GetSymbolContextArgs,
		) (*mcp_golang.ToolResponse, error) {

			content, err := s.compiler.Compile(ctx, args.SymbolID)
			if err != nil {
				return nil, err
			}

			return textResponse(content), nil
		},
	)
}

// registerFindNeighborsTool registers the structural neighbor lookup tool.
func (s *Server) registerFindNeighborsTool(server *mcp_golang.Server) error {
	return server.RegisterTool(
		"find_neighbors",
		"Finds symbols directly connected to a symbol",
		func(
			ctx context.Context,
			args FindNeighborsArgs,
		) (*mcp_golang.ToolResponse, error) {

			nodes, edges, err := s.store.GetNeighbors(ctx, args.SymbolID)
			if err != nil {
				return nil, err
			}

			return textResponse(renderNeighbors(args.SymbolID, nodes, edges)), nil
		},
	)
}

// registerTraceCallsTool registers the recursive call graph tracing tool.
func (s *Server) registerTraceCallsTool(server *mcp_golang.Server) error {
	return server.RegisterTool(
		"trace_calls",
		"Traces the call graph starting from a symbol",
		func(
			ctx context.Context,
			args TraceCallsArgs,
		) (*mcp_golang.ToolResponse, error) {

			if args.Depth <= 0 {
				args.Depth = 3
			}

			var builder strings.Builder

			err := s.traceCalls(
				ctx,
				args.SymbolID,
				0,
				args.Depth,
				make(map[string]bool),
				&builder,
			)
			if err != nil {
				return nil, err
			}

			return textResponse(builder.String()), nil
		},
	)
}

// registerTraceExecutionPathTool registers the ordered execution flow tool.
func (s *Server) registerTraceExecutionPathTool(server *mcp_golang.Server) error {
	return server.RegisterTool(
		"trace_execution_path",
		"Returns ordered control-flow traversal for a symbol",
		func(
			ctx context.Context,
			args TraceExecutionPathArgs,
		) (*mcp_golang.ToolResponse, error) {

			content, err := s.renderExecutionFlow(ctx, args.SymbolID)
			if err != nil {
				return nil, err
			}

			return textResponse(content), nil
		},
	)
}

// registerArchitectureViolationsTool registers the architecture validation tool.
func (s *Server) registerArchitectureViolationsTool(server *mcp_golang.Server) error {
	return server.RegisterTool(
		"find_architecture_violations",
		"Detects architecture boundary violations",
		func(
			ctx context.Context,
			args FindArchitectureViolationsArgs,
		) (*mcp_golang.ToolResponse, error) {

			configPath := args.ConfigPath
			if configPath == "" {
				configPath = filepath.Join(".lea", "architecture.yaml")
			}

			cfg, err := architecture.LoadConfig(configPath)
			if err != nil {
				return nil, err
			}

			violations, err := architecture.FindViolations(
				ctx,
				s.store,
				cfg,
			)
			if err != nil {
				return nil, err
			}

			return textResponse(renderViolations(violations)), nil
		},
	)
}

// -----------------------------------------------------------------------------
// Trace Engine
// -----------------------------------------------------------------------------

// traceCalls recursively traverses CALLS relationships in the graph.
func (s *Server) traceCalls(
	ctx context.Context,
	id string,
	depth int,
	maxDepth int,
	visited map[string]bool,
	builder *strings.Builder,
) error {
	// Stop traversal if maximum depth is reached.
	if depth > maxDepth {
		return nil
	}

	// Prevent infinite cycles in recursive graphs.
	if visited[id] {
		return nil
	}

	visited[id] = true

	nodes, edges, err := s.store.GetNeighbors(ctx, id)
	if err != nil {
		return err
	}

	for i, node := range nodes {
		// Defensive bounds check.
		if i >= len(edges) {
			continue
		}

		edge := edges[i]

		// Only traverse CALLS relationships.
		if edge.Type != graph.EdgeCalls {
			continue
		}

		fmt.Fprintf(
			builder,
			"%s-> [%s] %s (%s)\n",
			strings.Repeat("  ", depth),
			edge.Type,
			node.Name,
			node.Type,
		)

		if err := s.traceCalls(
			ctx,
			node.ID,
			depth+1,
			maxDepth,
			visited,
			builder,
		); err != nil {
			return err
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// Flow Rendering
// -----------------------------------------------------------------------------

// renderExecutionFlow renders ordered control-flow edges for a symbol.
func (s *Server) renderExecutionFlow(
	ctx context.Context,
	id string,
) (string, error) {
	nodes, edges, err := s.store.GetNeighbors(ctx, id)
	if err != nil {
		return "", err
	}

	type flowEntry struct {
		edge *graph.Edge
		node *graph.Node
	}

	var entries []flowEntry

	for i, node := range nodes {
		if i >= len(edges) {
			continue
		}

		edge := edges[i]

		if edge.Type == graph.EdgeFlowsThrough {
			entries = append(entries, flowEntry{
				edge: edge,
				node: node,
			})
		}
	}

	if len(entries) == 0 {
		return fmt.Sprintf(
			"No control-flow entries found for %s",
			id,
		), nil
	}

	// Sort by execution order.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].edge.Sequence < entries[j].edge.Sequence
	})

	var builder strings.Builder

	fmt.Fprintf(&builder, "Execution flow for %s:\n", id)

	for _, entry := range entries {
		contextLabel := ""

		if entry.edge.Metadata != nil {
			if ctxValue, ok := entry.edge.Metadata["context"].(string); ok {
				if ctxValue != "" {
					contextLabel = fmt.Sprintf(" [%s]", ctxValue)
				}
			}
		}

		fmt.Fprintf(
			&builder,
			"%d. %s (%s)%s\n",
			entry.edge.Sequence,
			entry.node.Name,
			entry.node.Type,
			contextLabel,
		)
	}

	return builder.String(), nil
}

// -----------------------------------------------------------------------------
// Render Helpers
// -----------------------------------------------------------------------------

// renderNeighbors renders direct graph neighbors.
func renderNeighbors(
	symbolID string,
	nodes []*graph.Node,
	edges []*graph.Edge,
) string {
	if len(nodes) == 0 {
		return fmt.Sprintf(
			"No neighbors found for %s",
			symbolID,
		)
	}

	var builder strings.Builder

	fmt.Fprintf(&builder, "Neighbors of %s:\n", symbolID)

	for i, node := range nodes {
		if i >= len(edges) {
			continue
		}

		edge := edges[i]

		fmt.Fprintf(
			&builder,
			"- [%s] %s (%s) at %s:%d\n",
			edge.Type,
			node.Name,
			node.Type,
			node.File,
			node.Line,
		)
	}

	return builder.String()
}

// renderViolations renders architecture rule violations.
func renderViolations(
	violations []architecture.Violation,
) string {
	if len(violations) == 0 {
		return "No architecture violations found."
	}

	// Keep output deterministic.
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].FromLayer == violations[j].FromLayer {
			return violations[i].ToLayer < violations[j].ToLayer
		}

		return violations[i].FromLayer < violations[j].FromLayer
	})

	var builder strings.Builder

	fmt.Fprintf(&builder, "Architecture violations (%d):\n", len(violations))

	for _, violation := range violations {
		fmt.Fprintf(
			&builder,
			"- [%s] %s (%s) -> %s (%s)\n",
			violation.EdgeType,
			violation.FromID,
			violation.FromLayer,
			violation.ToID,
			violation.ToLayer,
		)

		fmt.Fprintf(
			&builder,
			"  at %s:%d -> %s:%d\n",
			violation.FromFile,
			violation.FromLine,
			violation.ToFile,
			violation.ToLine,
		)
	}

	return builder.String()
}

// -----------------------------------------------------------------------------
// Response Helpers
// -----------------------------------------------------------------------------

// textResponse creates a standard MCP text response.
func textResponse(content string) *mcp_golang.ToolResponse {
	return mcp_golang.NewToolResponse(
		mcp_golang.NewTextContent(content),
	)
}
