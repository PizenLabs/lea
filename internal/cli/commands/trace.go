package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	graph "github.com/PizenLabs/lea/internal/graph/contracts"
	"github.com/PizenLabs/lea/internal/storage/contracts"
	"github.com/PizenLabs/lea/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

var traceCmd = &cobra.Command{
	Use:   "trace [symbol_id]",
	Short: "Trace the call graph from a symbol",
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

		fmt.Printf("Trace of %s:\n", symbolID)
		return trace(ctx, store, symbolID, 0, make(map[string]bool))
	},
}

func trace(ctx context.Context, store contracts.Store, id string, depth int, visited map[string]bool) error {
	if depth > 5 || visited[id] {
		return nil
	}
	visited[id] = true

	nodes, edges, err := store.GetNeighbors(ctx, id)
	if err != nil {
		return err
	}

	for i, n := range nodes {
		e := edges[i]
		if e.Type == graph.EdgeCalls {
			fmt.Printf("%s-> [%s] %s (%s)\n", strings.Repeat("  ", depth), e.Type, n.Name, n.Type)
			if err := trace(ctx, store, n.ID, depth+1, visited); err != nil {
				return err
			}
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(traceCmd)
}
