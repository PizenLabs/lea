package commands

import (
	"path/filepath"

	"github.com/PizenLabs/lea/internal/mcp"
	"github.com/PizenLabs/lea/internal/mcp/install"
	"github.com/PizenLabs/lea/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server to expose lea to AI agents",
	Long:  `The mcp command starts a Model Context Protocol server over stdio, allowing AI agents like Claude or Pi to query the structural graph.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		dbPath := filepath.Join(".lea", "graph.db")
		store, err := sqlite.NewStore(dbPath)
		if err != nil {
			return err
		}
		defer func() { _ = store.Close() }()

		s := mcp.NewServer(store)
		return s.Start()
	},
}

var mcpInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Configure MCP entries for lea and lx across AI tools",
	Long: `Installs MCP server entries (pizen-lea and pizen-lynx) into the configuration
files of supported AI coding agents: Claude Code, VS Code (Cline/Roo Code/Codex CLI),
OpenCode, Pi, Zed, Gemini CLI, OpenClaw, Aider, Antigravity, Kiro, and KiloCode.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		return install.Run()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpInstallCmd)
}
