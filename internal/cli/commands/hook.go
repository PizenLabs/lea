package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/PizenLabs/lea/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

// HookInput represents the JSON payload passed to PreToolUse hooks by Claude Code and other agents.
type HookInput struct {
	ToolName       string         `json:"tool_name"`
	ToolNameCamel  string         `json:"toolName"`
	ToolInput      map[string]any `json:"tool_input"`
	ToolInputCamel map[string]any `json:"toolInput"`
}

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Run tool execution hooks for AI coding agents",
	Long:  `The hook command intercepts and validates agent tool calls in the lifecycle.`,
}

var hookPreToolCmd = &cobra.Command{
	Use:   "pre-tool",
	Short: "Execute pre-tool lifecycle checks",
	Run: func(_ *cobra.Command, _ []string) {
		// Read hook input from stdin
		stdinData, err := io.ReadAll(os.Stdin)
		if err != nil {
			// Fail open on error reading stdin to avoid blocking developer flow
			os.Exit(0)
		}

		if len(stdinData) == 0 {
			os.Exit(0)
		}

		var input HookInput
		if err := json.Unmarshal(stdinData, &input); err != nil {
			// Fail open on invalid JSON
			os.Exit(0)
		}

		toolName := input.ToolName
		if toolName == "" {
			toolName = input.ToolNameCamel
		}

		// Clean up toolName
		toolName = strings.TrimSpace(toolName)
		if toolName == "" {
			os.Exit(0)
		}

		// Check if it's a pizen-lea tool.
		// pizen-lea tools typically include: impact, flow, neighbors, violations, symbols
		// They can be named like: "pizen-lea__impact", "mcp__pizen-lea__impact", "pizen-lea/impact", "impact"
		isLeaTool := false
		lowerName := strings.ToLower(toolName)
		if strings.Contains(lowerName, "pizen-lea") {
			isLeaTool = true
		} else {
			// Fallback: check if the tool name matches one of our MCP tools
			leaTools := []string{"impact", "flow", "neighbors", "violations", "symbols"}
			for _, t := range leaTools {
				if lowerName == t || strings.HasSuffix(lowerName, "__"+t) || strings.HasSuffix(lowerName, "/"+t) {
					isLeaTool = true
					break
				}
			}
		}

		if !isLeaTool {
			os.Exit(0)
		}

		// It's a pizen-lea tool. Get tool input.
		toolInput := input.ToolInput
		if len(toolInput) == 0 {
			toolInput = input.ToolInputCamel
		}

		// Try to extract the symbol name.
		// The tool arguments might be: symbol, symbol_id, target, name, qualified_name
		var symbolName string
		symbolKeys := []string{"symbol", "symbol_id", "target", "name", "qualified_name", "symbolName", "symbolId"}
		for _, k := range symbolKeys {
			if val, ok := toolInput[k]; ok {
				if s, ok := val.(string); ok && s != "" {
					symbolName = s
					break
				}
			}
		}

		if symbolName == "" {
			// If no symbol name is passed, let it pass to standard validation/execution
			os.Exit(0)
		}

		// Find .lea directory starting from current working directory
		leaDir, err := findLeaDir(".")
		if err != nil {
			// If .lea dir doesn't exist, we can't validate, so fail open
			os.Exit(0)
		}

		dbPath := filepath.Join(leaDir, "graph.db")
		store, err := sqlite.NewStore(dbPath)
		if err != nil {
			os.Exit(0)
		}
		defer func() { _ = store.Close() }()

		ctx := context.Background()
		_, err = resolveSymbolID(ctx, store, symbolName)
		if err != nil {
			// Symbol resolution failed! Block the tool execution (exit code 2)
			// and print the warning message to stderr.
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintln(os.Stderr, "CRITICAL: You MUST first search for the symbol using pizen-lynx (via search or resolve) to find the correct Symbol ID before calling pizen-lea tools.")
			os.Exit(2)
		}

		// Success, allow tool execution
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(hookCmd)
	hookCmd.AddCommand(hookPreToolCmd)
}
