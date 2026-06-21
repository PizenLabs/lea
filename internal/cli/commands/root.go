package commands

import (
	"context"
	"fmt"
	"os"
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

// normalizeSymbolInput strips common prefixes and path artifacts from user input
// to produce a clean candidate for symbol lookup.
func normalizeSymbolInput(input string) string {
	s := input
	// Strip surrounding whitespace
	s = strings.TrimSpace(s)

	// Strip common runtime prefixes (e.g., absolute file paths, $PWD, etc.)
	if strings.HasPrefix(s, "/") {
		// Absolute path prefix: strip down to last meaningful component
		if strings.Contains(s, ":") {
			parts := strings.SplitN(s, ":", 2)
			if len(parts) == 2 && strings.HasPrefix(parts[0], "/") {
				s = parts[1]
			} else {
				s = parts[1]
			}
		}
	}

	// Strip graph node type prefixes (func:, method:, type:, pkg:) if they appear mid-input
	s = strings.TrimPrefix(s, "func:")
	s = strings.TrimPrefix(s, "method:")
	s = strings.TrimPrefix(s, "type:")
	s = strings.TrimPrefix(s, "pkg:")

	// Strip module path prefix up to the first meaningful path component.
	// e.g., "github.com/PizenLabs/lea/internal/domain:WalletRepository" becomes "internal/domain:WalletRepository"
	if strings.Contains(s, ":") && !strings.HasPrefix(s, "internal/") {
		parts := strings.SplitN(s, ":", 2)
		// Check if the left side contains a full module path
		if strings.Count(parts[0], "/") > 1 {
			// Extract last meaningful path segment after the module
			pathParts := strings.Split(parts[0], "/")
			// Find the "internal" or first meaningful segment
			idx := -1
			for i, p := range pathParts {
				if p == "internal" || p == "cmd" || p == "pkg" || p == "testdata" {
					idx = i
					break
				}
			}
			if idx > 0 {
				s = strings.Join(pathParts[idx:], "/") + ":" + parts[1]
			}
		}
	}

	// If the input is a simple identifier (e.g., "UpdateBalance"), leave as-is
	return s
}

// extractBaseName extracts the last meaningful name component from an input string.
// For "func:cmd/server:main" it returns "main".
// For "method:internal/wallet:UpdateBalance" it returns "UpdateBalance".
// For "main" it returns "main" unchanged.
func extractBaseName(input string) string {
	// Find the last colon or dot separator
	lastColon := strings.LastIndex(input, ":")
	lastDot := strings.LastIndex(input, ".")
	lastSep := lastColon
	if lastDot > lastSep {
		lastSep = lastDot
	}
	if lastSep >= 0 && lastSep < len(input)-1 {
		return input[lastSep+1:]
	}
	return input
}

// resolveSymbolID normalizes user input for symbol lookups (Issue 4 fix).
// It tries multiple strategies:
// 1. Exact match as-is
// 2. Try with func:/method:/type: prefixes
// 3. Fuzzy suffix/contains match against all node IDs
// 4. Name-based fallback: extract the base name and search by symbol name
func resolveSymbolID(ctx context.Context, store contracts.Store, input string) (string, error) {
	// Normalize input first (strip absolute paths, runtime prefixes, module prefixes)
	normalized := normalizeSymbolInput(input)

	// Strategy 1: Exact match on normalized input
	node, err := store.GetNode(ctx, normalized)
	if err != nil {
		return "", fmt.Errorf("error looking up symbol: %w", err)
	}
	if node != nil {
		return node.ID, nil
	}

	// Strategy 2: Try with graph prefixes
	prefixes := []string{"func:", "method:", "type:", "pkg:"}
	for _, prefix := range prefixes {
		node, err = store.GetNode(ctx, prefix+normalized)
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

	// Try suffix match first: normalized input matches the end of ID
	var candidates []string
	for _, n := range allNodes {
		if strings.HasSuffix(n.ID, ":"+normalized) || strings.HasSuffix(n.ID, "."+normalized) {
			candidates = append(candidates, n.ID)
		}
	}

	// Try name match: normalized input matches the Name field
	if len(candidates) == 0 {
		for _, n := range allNodes {
			if strings.EqualFold(n.Name, normalized) || strings.Contains(strings.ToLower(n.Name), strings.ToLower(normalized)) {
				candidates = append(candidates, n.ID)
			}
		}
	}

	// Try contains match in ID
	if len(candidates) == 0 {
		lowerInput := strings.ToLower(normalized)
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

	// Strategy 4: Name-based fallback — extract the base name and search by symbol Name
	baseName := extractBaseName(normalized)
	if baseName != "" && baseName != normalized {
		var nameMatches []string
		for _, n := range allNodes {
			if strings.EqualFold(n.Name, baseName) {
				nameMatches = append(nameMatches, n.ID)
			}
		}
		if len(nameMatches) == 1 {
			fmt.Fprintf(os.Stderr, "Did you mean %q? (auto-resolved)\n", nameMatches[0])
			return nameMatches[0], nil
		}
		if len(nameMatches) > 1 {
			return "", fmt.Errorf("symbol %q not found. Did you mean one of these?\n  %s",
				input, strings.Join(nameMatches, "\n  "))
		}
	}

	// Strategy 5: Last resort — find any node whose name contains the base name
	if baseName != "" {
		var fuzzyNames []string
		lowerBase := strings.ToLower(baseName)
		for _, n := range allNodes {
			if strings.Contains(strings.ToLower(n.Name), lowerBase) {
				fuzzyNames = append(fuzzyNames, n.ID)
			}
		}
		if len(fuzzyNames) > 0 {
			return "", fmt.Errorf("symbol %q not found. Did you mean one of these?\n  %s",
				input, strings.Join(fuzzyNames, "\n  "))
		}
	}

	return "", fmt.Errorf("symbol %q not found in the graph. Use 'lea symbols' to list available symbols", input)
}


