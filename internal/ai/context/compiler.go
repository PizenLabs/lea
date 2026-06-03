// Package context provides functionality for compiling AI context from the graph.
package context

import (
	"context"
	"fmt"
	"strings"

	graph "github.com/PizenLabs/lea/internal/graph/contracts"
	"github.com/PizenLabs/lea/internal/storage/contracts"
)

// Compiler handles the compilation of nodes into a textual context for AI consumption.
type Compiler struct {
	store contracts.Store
}

// NewCompiler creates a new context compiler.
func NewCompiler(store contracts.Store) *Compiler {
	return &Compiler{store: store}
}

// Compile generates a markdown representation of a symbol and its relationships.
func (c *Compiler) Compile(ctx context.Context, symbolID string) (string, error) {
	return c.CompileWithBudget(ctx, symbolID, 1000000) // Virtually unlimited
}

// CompileWithBudget generates a markdown representation of a symbol and its relationships,
// attempting to stay within the character budget.
func (c *Compiler) CompileWithBudget(ctx context.Context, symbolID string, budget int) (string, error) {
	node, err := c.store.GetNode(ctx, symbolID)
	if err != nil {
		return "", err
	}
	if node == nil {
		return "", fmt.Errorf("symbol not found: %s", symbolID)
	}

	var sb strings.Builder

	// Header (Highest priority)
	header := fmt.Sprintf("## %s\n\nType: %s\nFile: %s\n\n", node.Name, node.Type, node.File)
	if len(header) > budget {
		return header[:budget], nil
	}
	sb.WriteString(header)

	// Outbound Dependencies (Uses/Calls) - Medium priority
	outNodes, outEdges, err := c.store.GetNeighbors(ctx, symbolID)
	if err == nil && len(outNodes) > 0 {
		var depSection strings.Builder
		depSection.WriteString("### Dependencies\n")
		for i, n := range outNodes {
			e := outEdges[i]
			if e.Type == graph.EdgeCalls || e.Type == graph.EdgeUses || e.Type == graph.EdgeBelongsTo {
				fmt.Fprintf(&depSection, "- [%s] %s (%s)\n", e.Type, n.Name, n.Type)
			}
		}
		depSection.WriteString("\n")
		if sb.Len()+depSection.Len() <= budget {
			sb.WriteString(depSection.String())
		} else {
			return sb.String(), nil
		}
	}

	// Inbound Dependencies (Called by/Used by) - Lower priority
	inNodes, inEdges, err := c.store.GetInboundEdges(ctx, symbolID)
	if err == nil && len(inNodes) > 0 {
		var relSection strings.Builder
		relSection.WriteString("### Relationships\n")
		for i, n := range inNodes {
			e := inEdges[i]
			fmt.Fprintf(&relSection, "- %s (%s) [%s]\n", n.Name, n.Type, e.Type)
		}
		relSection.WriteString("\n")
		if sb.Len()+relSection.Len() <= budget {
			sb.WriteString(relSection.String())
		} else {
			return sb.String(), nil
		}
	}

	return sb.String(), nil
}
