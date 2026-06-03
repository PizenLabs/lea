package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PizenLabs/lea/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

var (
	symbolKind string
	symbolPkg  string
)

var symbolsCmd = &cobra.Command{
	Use:   "symbols [query]",
	Short: "Discover symbols in the repository",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		path := "."
		dbPath := filepath.Join(path, ".lea", "graph.db")
		store, err := sqlite.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()

		ctx := context.Background()
		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return err
		}

		query := ""
		if len(args) > 0 {
			query = strings.ToLower(args[0])
		}

		fmt.Printf("%-10s %-50s %s\n", "KIND", "ID", "FILE")
		fmt.Println(strings.Repeat("-", 100))

		for _, node := range nodes {
			if symbolKind != "" && string(node.Type) != symbolKind {
				continue
			}

			if query != "" && !strings.Contains(strings.ToLower(node.ID), query) && !strings.Contains(strings.ToLower(node.Name), query) {
				continue
			}

			// Extract package from ID if possible
			// func:pkg:name -> pkg
			parts := strings.Split(node.ID, ":")
			if len(parts) > 1 && symbolPkg != "" {
				if !strings.Contains(parts[1], symbolPkg) {
					continue
				}
			}

			fmt.Printf("%-10s %-50s %s:%d\n", node.Type, node.ID, node.File, node.Line)
		}

		return nil
	},
}

func init() {
	symbolsCmd.Flags().StringVarP(&symbolKind, "kind", "k", "", "Filter by symbol kind (function, method, struct, interface, package)")
	symbolsCmd.Flags().StringVarP(&symbolPkg, "pkg", "p", "", "Filter by package name")
	rootCmd.AddCommand(symbolsCmd)
}
