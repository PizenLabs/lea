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
// 3. LIKE-based fuzzy match for plain text input
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

	// If the user passed a prefixed URI (contains ":"), only use exact match
	if strings.Contains(input, ":") {
		return "", fmt.Errorf("symbol %q not found in the graph. Use 'lea symbols' to list available symbols", input)
	}

	// Strategy 3: LIKE-based fuzzy match for plain text input
	matches, err := store.SearchNodes(ctx, "%"+normalized+"%")
	if err != nil {
		return "", fmt.Errorf("error searching for symbol: %w", err)
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	if len(matches) > 1 {
		// Try suffix-priority match: normalized matches after ":" or "."
		var suffixCandidates []string
		for _, n := range matches {
			if strings.HasSuffix(n.ID, ":"+normalized) || strings.HasSuffix(n.ID, "."+normalized) {
				suffixCandidates = append(suffixCandidates, n.ID)
			}
		}
		if len(suffixCandidates) == 1 {
			return suffixCandidates[0], nil
		}
		if len(suffixCandidates) > 1 {
			return "", fmt.Errorf("ambiguous symbol %q, multiple matches:\n  %s",
				input, strings.Join(suffixCandidates, "\n  "))
		}
		var ids []string
		for _, n := range matches {
			ids = append(ids, n.ID)
		}
		return "", fmt.Errorf("ambiguous symbol %q, multiple matches:\n  %s",
			input, strings.Join(ids, "\n  "))
	}

	// Strategy 4: Name-based fallback — search by symbol Name
	if err == nil {
		var byName []string
		for _, n := range matches {
			if strings.EqualFold(n.Name, normalized) || strings.Contains(strings.ToLower(n.Name), strings.ToLower(normalized)) {
				byName = append(byName, n.ID)
			}
		}
		if len(byName) == 1 {
			fmt.Fprintf(os.Stderr, "Did you mean %q? (auto-resolved)\n", byName[0])
			return byName[0], nil
		}
		if len(byName) > 1 {
			return "", fmt.Errorf("symbol %q not found. Did you mean one of these?\n  %s",
				input, strings.Join(byName, "\n  "))
		}
	}

	return "", fmt.Errorf("symbol %q not found in the graph. Use 'lea symbols' to list available symbols", input)
}
