package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	graph "github.com/PizenLabs/lea/internal/graph/contracts"
	"github.com/PizenLabs/lea/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

type flowEntry struct {
	edge *graph.Edge
	node *graph.Node
}

var flowCmd = &cobra.Command{
	Use:   "flow [symbol_id]",
	Short: "Show control flow ordering for a symbol",
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

		nodes, edges, err := store.GetNeighbors(ctx, symbolID)
		if err != nil {
			return err
		}

		var entries []flowEntry
		for i, n := range nodes {
			e := edges[i]
			if e.Type == graph.EdgeFlowsThrough {
				entries = append(entries, flowEntry{edge: e, node: n})
			}
		}

		if len(entries) == 0 {
			fmt.Printf("No control flow entries found for %s\n", symbolID)
			return nil
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].edge.Sequence < entries[j].edge.Sequence
		})

		fmt.Printf("Control flow for %s:\n", symbolID)
		for _, entry := range entries {
			contextLabel := ""
			if entry.edge.Metadata != nil {
				if ctx, ok := entry.edge.Metadata["context"].(string); ok && ctx != "" {
					contextLabel = fmt.Sprintf(" [%s]", ctx)
				}
			}
			fmt.Printf("%d. %s (%s)%s\n", entry.edge.Sequence, entry.node.Name, entry.node.Type, contextLabel)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(flowCmd)
}
