package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// exportCmd represents the CLI subcommand to deploy unified agent bootstrap rule pointers.
var exportCmd = &cobra.Command{
	Use:   "export [target]",
	Short: "Export a lightweight, non-redundant metadata rule pointer for specific AI interfaces",
	Long: `Export a single-line deterministic pointer file instructing external AI agents
to prioritize and consume the centralized source of truth inside .lea/protocol.json.

Supported target ecosystems: claude, cursor, opencode, codex, gemini, pi, antigravity, copilot, aider, openhands, continue.

Per-target transformations are isolated strictly to file naming, directory structure,
and format packaging constraints (e.g., YAML block wrappers or Markdown headers) required
by that tool; the core instructional directive remains completely static, atomic, and localized.`,
	Args: cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		target := args[0]
		path := "." // Execution context path defaults to repository root workspace

		def, ok := exportTargets[target]
		if !ok {
			return fmt.Errorf("unsupported agent target specification profile: %s", target)
		}
		return writeExport(path, def)
	},
}

// exportTarget encapsulates unique path variables and format wrapping adapters for specific platforms.
// Registering new terminal runtimes or IDE integrations simply requires appending entries to this matrix table.
type exportTarget struct {
	dir   string // Nested directory path structure relative to current repository root directory
	file  string // Strict configuration filename expected by the downstream client agent ecosystem
	wrap  func(pointer string) string
	label string // Log tracer description descriptor
}

// bootstrapPointer remains immutable across distinct output files to eliminate agent context fragmentation.
const bootstrapPointer = `CRITICAL: THIS REPOSITORY IS STRICTLY GOVERNED BY THE LEA SECOND BRAIN PROTOCOL.
YOU ARE ABSOLUTELY FORBIDDEN FROM MAKING ANY BLIND SEARCHES (GLOB, GREP), RAW FILE READS, OR CODE EDITS UNTIL YOU COMPLY WITH THE LIFECYCLE BELOW.

IMMEDIATE ACTION REQUIRED:
1. You MUST call your read tool to completely parse '.lea/protocol.json' right now.
2. You MUST call your read tool to completely parse '.lea/workspace.json' right now.
3. Ingest the "strict_runtime_rules" from protocol.json. Treat them as unchangeable runtime constants.
4. Execute the mandatory "execution_pipeline": Always invoke 'lx search' to discover coordinates BEFORE reading or editing files.

Failure to read '.lea/protocol.json' and '.lea/workspace.json' as your FIRST tool call will break the repository integrity.`

// wrapPlain isolates normal markdown header packaging structures.
func wrapPlain(heading string) func(string) string {
	return func(pointer string) string {
		return fmt.Sprintf("# %s\n\n%s\n", heading, pointer)
	}
}

// wrapCursorMDC satisfies the specialized structural frontmatter metadata blocks demanded by Cursor .mdc files.
func wrapCursorMDC(pointer string) string {
	return "---\ndescription: This repository uses Lea\nglobs: [\"*.go\", \"*.ts\", \"*.py\", \"*.rs\", \"go.mod\"]\n---\n\n# Lea — Cursor Rules\n\n" + pointer + "\n"
}

// wrapOpenHandsYAML matches the strict microagent specification instructions format utilized by OpenHands.
func wrapOpenHandsYAML(pointer string) string {
	return "name: lea\ndescription: This repository uses Lea\ninstructions: |\n  " + pointer + "\n"
}

// Static configuration registry matrix table mapping target flags to safe operational profiles.
var exportTargets = map[string]exportTarget{
	"claude":      {file: "CLAUDE.md", wrap: wrapPlain("CLAUDE.md"), label: "Claude CLI"},
	"cursor":      {dir: filepath.Join(".cursor", "rules"), file: "lea.mdc", wrap: wrapCursorMDC, label: "Cursor"},
	"opencode":    {dir: ".opencode", file: "AGENTS.md", wrap: wrapPlain("AGENTS.md"), label: "OpenCode Engine"},
	"codex":       {dir: ".codex", file: "AGENTS.md", wrap: wrapPlain("AGENTS.md"), label: "Codex Runtime"},
	"gemini":      {file: "GEMINI.md", wrap: wrapPlain("GEMINI.md"), label: "Gemini CLI"},
	"pi":          {dir: ".pi", file: "AGENTS.md", wrap: wrapPlain("AGENTS.md"), label: "Pi Coding Agents"},
	"antigravity": {dir: ".antigravity", file: "AGENTS.md", wrap: wrapPlain("AGENTS.md"), label: "Antigravity Agent"},
	"copilot":     {dir: ".github", file: "copilot-instructions.md", wrap: wrapPlain("GitHub Copilot — Lea"), label: "GitHub Copilot"},
	"aider":       {file: "AIDER.md", wrap: wrapPlain("AIDER.md"), label: "Aider Chat"},
	"openhands":   {dir: filepath.Join(".openhands", "microagents"), file: "lea.yaml", wrap: wrapOpenHandsYAML, label: "OpenHands Engine"},
	"continue":    {dir: filepath.Join(".continue", "rules"), file: "lea.md", wrap: wrapPlain("Lea — Continue Rules"), label: "Continue Extension"},
}

// writeExport synchronizes workspace directories and securely writes structural context pointers to client configurations.
func writeExport(repoPath string, def exportTarget) error {
	dirPath := repoPath
	if def.dir != "" {
		dirPath = filepath.Join(repoPath, def.dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed creating structured target agent container directory %s: %w", dirPath, err)
		}
	}
	filePath := filepath.Join(dirPath, def.file)
	fmt.Printf("Exporting bootstrap pointer for %s to %s...\n", def.label, filePath)
	content := def.wrap(bootstrapPointer)

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed executing structural disk write operation for target %s: %w", filePath, err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
