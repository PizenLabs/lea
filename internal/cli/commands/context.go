// Package commands defines CLI subcommands for lea.
package commands

import (
	"context"
	"fmt"
	"path/filepath"

	aictx "github.com/PizenLabs/lea/internal/ai/context"
	"github.com/PizenLabs/lea/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

var (
	contextBudget int
)

var contextCmd = &cobra.Command{
	Use:   "context [symbol_id]",
	Short: "Generate AI-optimized context for a symbol",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		symbolID := args[0]

		dbPath := filepath.Join(".lea", "graph.db")
		store, err := sqlite.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()

		compiler := aictx.NewCompiler(store)
		ctx := context.Background()

		output, err := compiler.CompileWithBudget(ctx, symbolID, contextBudget)
		if err != nil {
			return err
		}

		fmt.Println(output)
		return nil
	},
}

func init() {
	contextCmd.Flags().IntVarP(&contextBudget, "budget", "b", 4000, "Maximum character budget for the context")
	rootCmd.AddCommand(contextCmd)
}
