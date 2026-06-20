package commands

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/PizenLabs/lea/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

var neighborsCmd = &cobra.Command{
	Use:   "neighbors [symbol_id]",
	Short: "Find neighbors of a symbol",
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

		if len(nodes) == 0 {
			fmt.Printf("No neighbors found for %s\n", symbolID)
			return nil
		}

		fmt.Printf("Neighbors of %s:\n", symbolID)
		for i, n := range nodes {
			e := edges[i]
			fmt.Printf("- [%s] -> %s (%s) at %s:%d\n", e.Type, n.Name, n.Type, n.File, n.Line)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(neighborsCmd)
}
