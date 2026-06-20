package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	graph "github.com/PizenLabs/lea/internal/graph/contracts"
	"github.com/PizenLabs/lea/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

var impactCmd = &cobra.Command{
	Use:   "impact [symbol_id]",
	Short: "Find symbols that depend on this symbol",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		input := args[0]

		dbPath := filepath.Join(".lea", "graph.db")
		store, err := sqlite.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()

		ctx := context.Background()

		// Normalize user input to a proper symbol ID (Issue 4 fix)
		symbolID, err := resolveSymbolID(ctx, store, input)
		if err != nil {
			return err
		}

		// If resolved symbol is a concrete method, also resolve the interface method
		// counterpart by following IMPLEMENTS_METHOD edges
		additionalIDs := []string{}
		node, err := store.GetNode(ctx, symbolID)
		if err == nil && node != nil && strings.HasPrefix(symbolID, "method:") {
			edgeList, err := store.GetEdgesByType(ctx, graph.EdgeImplementsMethod)
			if err == nil {
				for _, e := range edgeList {
					if e.FromID == symbolID {
						additionalIDs = append(additionalIDs, e.ToID)
					}
				}
			}
		}

		// Query impact for primary symbol
		nodes, edges, err := store.GetImpactRecursive(ctx, symbolID)
		if err != nil {
			return err
		}

		// Query impact for interface counterpart(s) and union results
		seenNodes := make(map[string]bool)
		for _, n := range nodes {
			seenNodes[n.ID] = true
		}

		for _, aid := range additionalIDs {
			moreNodes, moreEdges, err := store.GetImpactRecursive(ctx, aid)
			if err != nil {
				continue
			}
			for _, n := range moreNodes {
				if !seenNodes[n.ID] {
					nodes = append(nodes, n)
					seenNodes[n.ID] = true
				}
			}
			edges = append(edges, moreEdges...)

			// Also add the interface method node itself to the result
			if !seenNodes[aid] {
				ifaceNode, err := store.GetNode(ctx, aid)
				if err == nil && ifaceNode != nil {
					nodes = append(nodes, ifaceNode)
					seenNodes[aid] = true
				}
			}
		}

		if len(nodes) == 0 {
			fmt.Printf("No impact found for %s\n", symbolID)
			return nil
		}

		fmt.Printf("Blast Radius Analysis for %s\n", symbolID)
		fmt.Println("==================================================")

		var direct []*graph.Node
		var indirect []*graph.Node
		var interfaces []*graph.Node
		var tests []*graph.Node

		seen := make(map[string]bool)

		// Direct callers
		for _, e := range edges {
			if e.ToID == symbolID {
				n := findNode(nodes, e.FromID)
				if n != nil && !seen[n.ID] {
					if isTest(n.File) {
						tests = append(tests, n)
					} else if n.Type == graph.NodeInterface {
						interfaces = append(interfaces, n)
					} else {
						direct = append(direct, n)
					}
					seen[n.ID] = true
				}
			}
		}

		// Indirect callers
		for _, n := range nodes {
			if !seen[n.ID] {
				if isTest(n.File) {
					tests = append(tests, n)
				} else if n.Type == graph.NodeInterface {
					interfaces = append(interfaces, n)
				} else {
					indirect = append(indirect, n)
				}
				seen[n.ID] = true
			}
		}

		printSection("Direct Callers", direct)
		printSection("Indirect Callers", indirect)
		printSection("Affected Interfaces", interfaces)
		printSection("Affected Tests", tests)

		return nil
	},
}

func findNode(nodes []*graph.Node, id string) *graph.Node {
	for _, n := range nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

func isTest(file string) bool {
	base := filepath.Base(file)
	return base == "test.go" || base == "test.ts" || strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, "_test.ts") || strings.HasSuffix(base, ".test.go") || strings.HasSuffix(base, ".test.ts")
}

func printSection(title string, nodes []*graph.Node) {
	if len(nodes) == 0 {
		return
	}
	fmt.Printf("\n%s:\n", title)
	for _, n := range nodes {
		fmt.Printf("  - %s (%s) at %s:%d\n", n.Name, n.Type, n.File, n.Line)
	}
}

func init() {
	rootCmd.AddCommand(impactCmd)
}
