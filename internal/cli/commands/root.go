package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/PizenLabs/lea/internal/storage/contracts"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lea",
	Short: "lea is a structural context engine for AI-native engineering",
	Long:  `lea helps AI models and developers understand large codebases by modeling repositories as living structural graphs.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Root flags if any
}

// resolveSymbolID normalizes user input for symbol lookups (Issue 4 fix).
// It tries multiple strategies:
// 1. Exact match as-is
// 2. Try with func:/method:/type: prefixes
// 3. Fuzzy suffix/contains match against all node IDs
// 4. LIKE wildcard query
func resolveSymbolID(ctx context.Context, store contracts.Store, input string) (string, error) {
	// Strategy 1: Exact match
	node, err := store.GetNode(ctx, input)
	if err != nil {
		return "", fmt.Errorf("error looking up symbol: %w", err)
	}
	if node != nil {
		return node.ID, nil
	}

	// Strategy 2: Try common prefixes
	prefixes := []string{"func:", "method:", "type:"}
	for _, prefix := range prefixes {
		node, err = store.GetNode(ctx, prefix+input)
		if err != nil {
			return "", fmt.Errorf("error looking up symbol: %w", err)
		}
		if node != nil {
			return node.ID, nil
		}
	}

	// Strategy 3: List all nodes and look for suffix/partial matches
	allNodes, err := store.ListNodes(ctx)
	if err != nil {
		return "", fmt.Errorf("error listing nodes for fuzzy match: %w", err)
	}

	// Try suffix match first: input matches the end of ID
	var candidates []string
	for _, n := range allNodes {
		if strings.HasSuffix(n.ID, ":"+input) || strings.HasSuffix(n.ID, "."+input) {
			candidates = append(candidates, n.ID)
		}
	}

	// Try name match: input matches the Name field
	if len(candidates) == 0 {
		for _, n := range allNodes {
			if strings.EqualFold(n.Name, input) || strings.Contains(strings.ToLower(n.Name), strings.ToLower(input)) {
				candidates = append(candidates, n.ID)
			}
		}
	}

	// Try contains match in ID
	if len(candidates) == 0 {
		lowerInput := strings.ToLower(input)
		for _, n := range allNodes {
			if strings.Contains(strings.ToLower(n.ID), lowerInput) {
				candidates = append(candidates, n.ID)
			}
		}
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("ambiguous symbol %q, multiple matches:\n  %s",
			input, strings.Join(candidates, "\n  "))
	}

	return "", fmt.Errorf("symbol %q not found in the graph", input)
}

// openStore opens the .lea/graph.db store from the current or specified root.
func openStore(root string) (contracts.Store, error) {
	dbPath := root
	if dbPath == "" {
		dbPath = ".lea/graph.db"
	}
	return nil, fmt.Errorf("not implemented, use sqlite.NewStore directly")
}
