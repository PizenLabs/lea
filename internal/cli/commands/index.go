package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	graph "github.com/PizenLabs/lea/internal/graph/contracts"
	"github.com/PizenLabs/lea/internal/parser/golang"
	"github.com/PizenLabs/lea/internal/parser/treesitter"
	"github.com/PizenLabs/lea/internal/storage/contracts"
	"github.com/PizenLabs/lea/internal/storage/sqlite"
	"github.com/PizenLabs/lea/internal/workspace/ignore"
	"github.com/spf13/cobra"
)

// indexCmd represents the CLI subcommand to analyze a repository and dump structural metadata.
var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index a repository and generate unified agent metadata architecture",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		// Ensure the strict agent metadata state directory (.lea) exists at target root
		leaDir := filepath.Join(path, ".lea")
		if _, err := os.Stat(leaDir); os.IsNotExist(err) {
			if err := os.Mkdir(leaDir, 0755); err != nil {
				return fmt.Errorf("failed creating .lea configuration container: %w", err)
			}
		}

		// Initialize authoritative structural SQLite storage layer
		dbPath := filepath.Join(leaDir, "graph.db")
		store, err := sqlite.NewStore(dbPath)
		if err != nil {
			return fmt.Errorf("failed initializing graph store engine: %w", err)
		}
		defer func() { _ = store.Close() }()

		// Initialize multi-language parser architectures
		goParser := golang.NewParser()
		modName, goRequires := parseGoMod(path)
		if modName != "" {
			goParser.SetModuleName(modName)
		}
		tsParser := treesitter.NewParser()
		ctx := context.Background()
		matcher := ignore.NewMatcher(path)

		// Traverse the file system tree sequentially to build the static graph components
		err = filepath.WalkDir(path, func(filePath string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if matcher.ShouldSkipDir(filePath, entry) {
					return filepath.SkipDir
				}
				return nil
			}
			if matcher.ShouldSkipFile(filePath, entry) {
				return nil
			}

			var nodes []*graph.Node
			var edges []*graph.Edge

			ext := strings.ToLower(filepath.Ext(filePath))
			switch ext {
			case ".go":
				var parseErr error
				nodes, edges, parseErr = goParser.ParseFile(ctx, filePath)
				if parseErr != nil {
					return fmt.Errorf("structural syntax parsing failure on %s: %w", filePath, parseErr)
				}

				// Extract call graph links and internal procedural execution flow edges
				callEdges, callErr := goParser.ExtractCalls(ctx, filePath)
				if callErr != nil {
					return fmt.Errorf("call-graph edge extraction error on %s: %w", filePath, callErr)
				}
				flowEdges, flowErr := goParser.ExtractControlFlow(ctx, filePath)
				if flowErr != nil {
					return fmt.Errorf("control-flow trace edge extraction error on %s: %w", filePath, flowErr)
				}
				edges = append(edges, callEdges...)
				edges = append(edges, flowEdges...)

			case ".py", ".rs", ".ts":
				var parseErr error
				nodes, edges, parseErr = tsParser.ParseFile(ctx, filePath)
				if parseErr != nil {
					return fmt.Errorf("treesitter abstract syntax tree parsing failure on %s: %w", filePath, parseErr)
				}

			default:
				return nil
			}

			// Bypass persistent storage allocations if the parsing unit yields no entities
			if len(nodes) == 0 && len(edges) == 0 {
				return nil
			}

			fmt.Printf("Indexing %s...\n", filePath)
			if err := store.SaveGraph(ctx, nodes, edges); err != nil {
				return fmt.Errorf("transaction write failure pushing graph elements for %s: %w", filePath, err)
			}

			return nil
		})

		if err != nil {
			return err
		}

		fmt.Println("Indexing complete.")

		// Query compiled database metrics to construct the immutable telemetry payload
		stats, err := store.GetStats(ctx)
		if err != nil {
			return fmt.Errorf("failed compiling static repository graph stats: %w", err)
		}

		repoRoot, err := filepath.Abs(path)
		if err != nil {
			repoRoot = path
		}

		// Trigger generation of standardized operational JSON metadata files
		if err := generateMetadata(leaDir, repoRoot, modName, goRequires, stats); err != nil {
			return fmt.Errorf("failed building downstream metadata architecture contracts: %w", err)
		}

		return nil
	},
}

const schemaVersion = "1.0"

// generateMetadata writes the target immutable facts and operational boundaries to the .lea container directory.
func generateMetadata(leaDir, repoRoot, modName string, goRequires []string, stats *contracts.Stats) error {
	primaryLang := "Unknown"
	if len(stats.Languages) > 0 {
		primaryLang = stats.Languages[0]
	}
	frameworks := detectFrameworks(goRequires)
	now := time.Now().UTC().Format(time.RFC3339)

	// protocol.json: Machine-to-Machine (M2M) deterministic execution contract.
	// Explicitly maps abstract stage commands to the available native 'bash' system tool.
	protocolContent := fmt.Sprintf(`{
  "schema_version": %q,
  "source_of_truth": "protocol.json",
  "initialization_lifecycle": {
    "step_1": "Read .lea/workspace.json to ingest immutable repository facts.",
    "step_2": "Read .lea/intent.json to ingest human architectural boundaries.",
    "step_3": "Enforce execution_pipeline constraints. Zero modifications allowed before discovery."
  },
  "tool_execution_adapter": {
    "notice": "You do not have standalone tools named 'lx' or 'lea'. You MUST execute all pipeline commands inside your native 'bash' tool.",
    "binding_format": "bash -c '<command>'"
  },
  "execution_pipeline": {
    "phase_1_discover": {
      "execution_method": "Call native bash tool with local binary execution",
      "mandatory_commands": [
        "bash -c 'lx search <query>'",
        "bash -c 'lx resolve <name>'"
      ],
      "output_requirement": "symbol_or_file_coordinates"
    },
    "phase_2_reason": {
      "execution_method": "Call native bash tool with local binary execution",
      "mandatory_input": "symbol_or_file_coordinates",
      "mandatory_commands": [
        "bash -c 'lea impact <symbol>'",
        "bash -c 'lea context <symbol>'",
        "bash -c 'lea flow <symbol>'"
      ]
    }
  },
  "strict_runtime_rules": {
    "discovery_before_reasoning": true,
    "allow_blind_search": false,
    "allow_raw_file_read_before_discovery": false,
    "max_context_tokens": 4000,
    "stop_after_failed_attempts": 3,
    "smallest_safe_diff_only": true,
    "no_unrelated_refactors": true
  },
  "hard_boundaries": {
    "abort_condition": "3_consecutive_failed_edits",
    "recovery_procedure": [
      "stop_all_file_mutations",
      "bash -c 'lx resolve <failed_symbol>'",
      "bash -c 'lea impact <failed_symbol>'",
      "request_human_intervention"
    ]
  },
  "agent_capabilities_matrix": [
    "symbols",
    "call_graph",
    "control_flow",
    "dependencies",
    "impact_analysis",
    "architecture_validation"
  ],
  "tool_commands_registry": {
    "bash -c 'lx search <query>'": "Discover candidate symbols/files by intent.",
    "bash -c 'lx resolve <name>'": "Resolve a symbol to stable file coordinates.",
    "bash -c 'lx related <file:line>'": "Find related implementations in workspace.",
    "bash -c 'lea impact <symbol>'": "Execute recursive blast-radius impact analysis.",
    "bash -c 'lea context <symbol>'": "Compile token-budgeted context blocks for target symbol.",
    "bash -c 'lea flow <symbol>'": "Generate ordered control-flow trace from target symbol.",
    "bash -c 'lea violations'": "Trigger architectural boundary checking rules."
  }
}
`, schemaVersion)

	if err := os.WriteFile(filepath.Join(leaDir, "protocol.json"), []byte(protocolContent), 0644); err != nil {
		return fmt.Errorf("failed writing machine contract protocol.json: %w", err)
	}

	// SECURITY GUARD: Sanitize raw input strings via json marshaller to prevent raw escape crashes
	safeRepoRoot, _ := json.Marshal(nonEmpty(repoRoot, "Unknown"))
	safeModName, _ := json.Marshal(nonEmpty(modName, "Unknown"))

	// workspace.json: Concrete static repository facts. Absolutely no instructions or dynamic heuristics.
	workspaceJSON := fmt.Sprintf(`{
  "schema_version": %q,
  "repo_root": %s,
  "module": %s,
  "primary_language": %q,
  "languages": %s,
  "frameworks": %s,
  "graph_stats": {"symbols": %d, "nodes": %d, "edges": %d},
  "lea_version": %q,
  "generated": %q
}
`, schemaVersion,
		safeRepoRoot,
		safeModName,
		primaryLang,
		jsonStringArray(stats.Languages),
		jsonStringArray(frameworks),
		stats.NodesCount, stats.NodesCount, stats.EdgesCount,
		Version, now)
	if err := os.WriteFile(filepath.Join(leaDir, "workspace.json"), []byte(workspaceJSON), 0644); err != nil {
		return fmt.Errorf("failed writing workspace.json: %w", err)
	}

	// memory.json: Dynamic operational storage container tracking real-time hotspot evolution.
	memoryContent := fmt.Sprintf(`{
  "schema_version": %q,
  "hotspots": [],
  "frequently_changed_files": [],
  "historical_failures": [],
  "successful_patterns": []
}
`, schemaVersion)
	if err := os.WriteFile(filepath.Join(leaDir, "memory.json"), []byte(memoryContent), 0644); err != nil {
		return fmt.Errorf("failed writing memory.json: %w", err)
	}

	// intent.json: Encapsulates subjective engineering constraints that are impossible to derive from static analysis.
	intentContent := fmt.Sprintf(`{
  "schema_version": %q,
  "product_goals": [],
  "architecture_goals": [],
  "ownership": {},
  "constraints": [],
  "forbidden_changes": []
}
`, schemaVersion)
	if err := os.WriteFile(filepath.Join(leaDir, "intent.json"), []byte(intentContent), 0644); err != nil {
		return fmt.Errorf("failed writing intent.json: %w", err)
	}

	// limitations.json: Prevents dangerous agent hallucination vectors by logging system blind spots.
	limitationsContent := fmt.Sprintf(`{
  "schema_version": %q,
  "unsupported_languages": ["all languages except Go, Python, Rust, TypeScript"],
  "missing_graph_dimensions": ["data_flow", "type_hierarchy", "inheritance_graph"],
  "heuristic_detections": ["go_mod_dependencies", "framework_detection"],
  "confidence_limitations": ["indirect_dependencies", "dynamic_dispatch"]
}
`, schemaVersion)
	if err := os.WriteFile(filepath.Join(leaDir, "limitations.json"), []byte(limitationsContent), 0644); err != nil {
		return fmt.Errorf("failed writing limitations.json: %w", err)
	}

	// Purge stale Markdown files from outdated legacy system iterations
	for _, deprecated := range []string{
		"MANIFEST.md", ".agent-manifesto.md",
		"runtime.json", "capabilities.json", "commands.json",
		"bootstrap.md", "AGENT.md", "WORKSPACE.md",
	} {
		p := filepath.Join(leaDir, deprecated)
		if _, err := os.Stat(p); err == nil {
			_ = os.Remove(p)
		}
	}

	fmt.Printf("Generated operational metadata framework inside %s\n", leaDir)
	return nil
}

// jsonStringArray transforms a primitive string slice slice into a compliant inline JSON array text block.
func jsonStringArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// nonEmpty provides clean fallback boundaries for uninitialized system variables.
func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// parseGoMod evaluates target file bytes to strip out the primary module namespace and direct imports.
func parseGoMod(path string) (module string, requires []string) {
	goModPath := filepath.Join(path, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", nil
	}
	inRequireBlock := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if after, found := strings.CutPrefix(line, "module "); found {
			module = strings.TrimSpace(after)
			continue
		}
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}
		if inRequireBlock {
			fields := strings.Fields(line)
			if len(fields) > 0 && fields[0] != "" {
				requires = append(requires, fields[0])
			}
			continue
		}
		if after, found := strings.CutPrefix(line, "require "); found {
			fields := strings.Fields(after)
			if len(fields) > 0 {
				requires = append(requires, fields[0])
			}
		}
	}
	return module, requires
}

// detectFrameworks registers specific framework footprints to populate the static workspace information blocks.
func detectFrameworks(requires []string) []string {
	known := map[string]string{
		"github.com/spf13/cobra":   "cobra",
		"github.com/spf13/viper":   "viper",
		"github.com/gin-gonic/gin": "gin",
		"github.com/labstack/echo": "echo",
		"google.golang.org/grpc":   "grpc",
		"github.com/gorilla/mux":   "gorilla/mux",
		"gorm.io/gorm":             "gorm",
	}
	var found []string
	for _, req := range requires {
		for prefix, name := range known {
			if strings.HasPrefix(req, prefix) {
				found = append(found, name)
			}
		}
	}
	return found
}

func init() {
	rootCmd.AddCommand(indexCmd)
}
