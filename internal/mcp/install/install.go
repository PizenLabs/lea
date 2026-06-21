// Package install configures MCP server entries for lea and lx across
// supported AI coding agent tools (Claude Code, VS Code, OpenCode, etc.).
package install

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// MCPEntry represents a single MCP tool entry in the JSON config schema.
type MCPEntry struct {
	Command string            `json:"command" yaml:"cmd" toml:"command"`
	Args    []string          `json:"args" yaml:"-" toml:"args"`
	Env     map[string]string `json:"env,omitempty" yaml:"-" toml:"-"`
}

type target struct {
	Name   string
	Path   string // after tilde expansion
	Format string // json, yaml, toml
}

// installTargets returns the full list of MCP configuration targets.
func installTargets(home, projectDir, vscodeUserDir string) []target {
	return []target{
		{Name: "Claude Code", Path: filepath.Join(projectDir, ".claude", ".mcp.json"), Format: "json"},
		{Name: "Codex CLI", Path: filepath.Join(projectDir, ".codex", "config.toml"), Format: "codex_toml"},
		{Name: "Gemini CLI", Path: filepath.Join(projectDir, ".gemini", "settings.json"), Format: "json"},
		{Name: "Zed", Path: filepath.Join(projectDir, "settings.json"), Format: "zed"},
		{Name: "OpenCode", Path: filepath.Join(projectDir, "opencode.json"), Format: "opencode"},
		{Name: "Antigravity", Path: filepath.Join(home, ".gemini", "config", "mcp_config.json"), Format: "json"},
		{Name: "KiloCode", Path: filepath.Join(projectDir, "mcp_settings.json"), Format: "json"},
		{Name: "VS Code", Path: filepath.Join(vscodeUserDir, "mcp.json"), Format: "json"},
		{Name: "OpenClaw", Path: filepath.Join(projectDir, "openclaw.json"), Format: "json"},
		{Name: "Kiro", Path: filepath.Join(projectDir, ".kiro", "settings", "mcp.json"), Format: "json"},
	}
}

// homeDir returns the user's home directory, exiting on failure.
func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot determine home directory: %v", err)
	}
	return h
}

// vscodeGlobalStorageDir returns the VS Code globalStorage path for the current OS.
func vscodeGlobalStorageDir(home string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "globalStorage")
	case "linux":
		return filepath.Join(home, ".config", "Code", "User", "globalStorage")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Code", "User", "globalStorage")
	default:
		return filepath.Join(home, ".config", "Code", "User", "globalStorage")
	}
}

// vscodeUserDir returns the VS Code User directory for the current OS.
func vscodeUserDir(home string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User")
	case "linux":
		return filepath.Join(home, ".config", "Code", "User")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Code", "User")
	default:
		return filepath.Join(home, ".config", "Code", "User")
	}
}

// Run configures all MCP targets with pizen-lea and pizen-lynx entries.
// projectDir is the project root for resolving relative (project-scoped) config paths.
func Run(projectDir string) error {
	// Resolve lea binary path
	leaPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot resolve lea binary path: %w", err)
	}
	leaPath, err = filepath.Abs(leaPath)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute lea path: %w", err)
	}

	home := homeDir()
	lxPath := filepath.Join(home, ".cargo", "bin", "lx")

	if _, err := os.Stat(lxPath); err != nil {
		lxPath = resolveLXFallback()
	}

	vscodeUserDir := vscodeUserDir(home)
	successCount := 0

	for _, t := range installTargets(home, projectDir, vscodeUserDir) {
		if err := configureTarget(t, leaPath, lxPath); err != nil {
			log.Printf("[skip] %s: %v", t.Name, err)
			continue
		}
		fmt.Printf("  ✓ %s\n", t.Name)
		successCount++
	}

	if err := generateInstructions(home, projectDir); err != nil {
		log.Printf("[skip] instructions: %v", err)
	}
	fmt.Printf("  ✓ System Instructions\n")

	fmt.Printf("\nConfigured %d MCP targets successfully.\n", successCount)
	return nil
}

// resolveLXFallback tries to find lx via PATH as a fallback.
func resolveLXFallback() string {
	p, err := exec.LookPath("lx")
	if err != nil {
		return filepath.Join(homeDir(), ".cargo", "bin", "lx") // return default even if missing
	}
	return p
}

// configureTarget injects MCP entries into a single target configuration file.
func configureTarget(t target, leaPath, lxPath string) error {
	parent := filepath.Dir(t.Path)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("cannot create parent directory %q: %w", parent, err)
	}

	switch t.Format {
	case "json":
		return injectJSON(t.Path, leaPath, lxPath)
	case "opencode":
		return injectOpenCodeJSON(t.Path, leaPath, lxPath)
	case "zed":
		return injectZedJSON(t.Path, leaPath, lxPath)
	case "yaml":
		return injectYAML(t.Path, leaPath, lxPath)
	case "toml":
		return injectTOML(t.Path, leaPath, lxPath)
	case "codex_toml":
		return injectCodexTOML(t.Path, leaPath, lxPath)
	default:
		return fmt.Errorf("unsupported format: %s", t.Format)
	}
}

// injectJSON reads or creates a JSON file and injects pizen entries under mcpServers.
func injectJSON(path, leaPath, lxPath string) error {
	data := readOrEmpty(path)

	var raw map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}
	} else {
		raw = make(map[string]any)
	}

	// Standard mcpServers injection
	servers, ok := raw["mcpServers"].(map[string]any)
	if !ok || servers == nil {
		servers = make(map[string]any)
	}
	env := map[string]string{
		"PATH": os.Getenv("PATH"),
		"HOME": os.Getenv("HOME"),
	}
	leaEntry := MCPEntry{Command: leaPath, Args: []string{"mcp"}, Env: env}
	lxEntry := MCPEntry{Command: lxPath, Args: []string{"mcp"}, Env: env}
	servers["pizen-lea"] = leaEntry
	servers["pizen-lynx"] = lxEntry
	raw["mcpServers"] = servers

	return writeJSON(path, raw)
}

// injectZedJSON handles Zed IDE's mcp config format under root "mcp" key.
func injectZedJSON(path, leaPath, lxPath string) error {
	data := readOrEmpty(path)

	var raw map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}
	} else {
		raw = make(map[string]any)
	}

	mcp, ok := raw["mcp"].(map[string]any)
	if !ok || mcp == nil {
		mcp = make(map[string]any)
	}

	env := map[string]string{
		"PATH": os.Getenv("PATH"),
		"HOME": os.Getenv("HOME"),
	}

	mcp["pizen-lea"] = MCPEntry{Command: leaPath, Args: []string{"mcp"}, Env: env}
	mcp["pizen-lynx"] = MCPEntry{Command: lxPath, Args: []string{"mcp"}, Env: env}
	raw["mcp"] = mcp

	return writeJSON(path, raw)
}

// injectOpenCodeJSON handles OpenCode's MCP config format under root "mcp" key.
// OpenCode uses: { "enabled": true, "type": "local", "command": ["path", "mcp"] }
func injectOpenCodeJSON(path, leaPath, lxPath string) error {
	data := readOrEmpty(path)

	var raw map[string]any
	if len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}
	} else {
		raw = make(map[string]any)
	}

	mcp, ok := raw["mcp"].(map[string]any)
	if !ok || mcp == nil {
		mcp = make(map[string]any)
	}

	mcp["pizen-lea"] = map[string]any{
		"enabled": true,
		"type":    "local",
		"command": []any{leaPath, "mcp"},
	}
	mcp["pizen-lynx"] = map[string]any{
		"enabled": true,
		"type":    "local",
		"command": []any{lxPath, "mcp"},
	}
	raw["mcp"] = mcp

	return writeJSON(path, raw)
}

// injectYAML reads or creates a YAML file and injects pizen entries under the mcp list.
func injectYAML(path, leaPath, lxPath string) error {
	data := readOrEmpty(path)

	var raw map[string]any
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}
	} else {
		raw = make(map[string]any)
	}

	// YAML mcp list format for aider:
	// mcp:
	//   - name: pizen-lea
	//     cmd: <path> mcp
	//   - name: pizen-lynx
	//     cmd: <path> mcp
	mcpList, ok := raw["mcp"].([]any)
	if !ok {
		mcpList = []any{}
	}

	mcpList = upsertYAMLList(mcpList, "pizen-lea", leaPath)
	mcpList = upsertYAMLList(mcpList, "pizen-lynx", lxPath)
	raw["mcp"] = mcpList

	return writeYAML(path, raw)
}

// upsertYAMLList adds or updates a named entry in a YAML mcp list.
func upsertYAMLList(list []any, name, cmd string) []any {
	for i, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if n, _ := entry["name"].(string); n == name {
			entry["cmd"] = cmd + " mcp"
			entry["name"] = name
			list[i] = entry
			return list
		}
	}
	list = append(list, map[string]any{
		"name": name,
		"cmd":  cmd + " mcp",
	})
	return list
}

// injectTOML reads or creates a TOML file and injects pizen entries.
func injectTOML(path, leaPath, lxPath string) error {
	data := readOrEmpty(path)

	var raw map[string]any
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}
	} else {
		raw = make(map[string]any)
	}

	// Kiro Agent TOML format expects external tools under [[tool]] or similar.
	// Inject under a generic `[[external_tools]]` array or merge into existing.
	tools, ok := raw["external_tools"].([]any)
	if !ok {
		tools = []any{}
	}

	tools = upsertTOMLTool(tools, "pizen-lea", leaPath, []string{"mcp"})
	tools = upsertTOMLTool(tools, "pizen-lynx", lxPath, []string{"mcp"})
	raw["external_tools"] = tools

	return writeTOML(path, raw)
}

// upsertTOMLTool adds or updates a tool entry in a TOML list.
func upsertTOMLTool(list []any, name, command string, args []string) []any {
	for i, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if n, _ := entry["name"].(string); n == name {
			entry["command"] = command
			entry["args"] = args
			list[i] = entry
			return list
		}
	}
	list = append(list, map[string]any{
		"name":    name,
		"command": command,
		"args":    args,
	})
	return list
}

// injectCodexTOML reads or creates a TOML file and injects pizen entries
// using the Codex CLI format: [mcpServers] key with inline table values.
func injectCodexTOML(path, leaPath, lxPath string) error {
	data := readOrEmpty(path)

	var raw map[string]any
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("unmarshal error: %w", err)
		}
	} else {
		raw = make(map[string]any)
	}

	servers, ok := raw["mcpServers"].(map[string]any)
	if !ok || servers == nil {
		servers = make(map[string]any)
	}

	env := map[string]string{
		"PATH": os.Getenv("PATH"),
		"HOME": os.Getenv("HOME"),
	}
	servers["pizen-lea"] = MCPEntry{Command: leaPath, Args: []string{"mcp"}, Env: env}
	servers["pizen-lynx"] = MCPEntry{Command: lxPath, Args: []string{"mcp"}, Env: env}
	raw["mcpServers"] = servers

	return writeTOML(path, raw)
}

// readOrEmpty reads a file, returning nil if it doesn't exist.
func readOrEmpty(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// writeJSON writes a map as formatted JSON.
func writeJSON(path string, v map[string]any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// writeYAML writes a map as YAML.
func writeYAML(path string, v map[string]any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// writeTOML writes a map as TOML.
func writeTOML(path string, v map[string]any) error {
	data, err := toml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// generateInstructions creates per-agent instruction files in the project directory.
func generateInstructions(home, projectDir string) error {
	content := `# PizenLabs Ecosystem — Dual-Tool Orchestration

CRITICAL: For code-related queries, ALWAYS run pizen-lynx (via search or resolve)
first to discover the exact Symbol ID. DO NOT guess the code structure.

Once the Symbol ID is retrieved, immediately pass it to pizen-lea (via impact,
flow, or neighbors) to map structural reasoning and blast radius.
`

	type instructionTarget struct {
		path string
		wrap func(string) string
	}

	targets := []instructionTarget{
		{path: filepath.Join(projectDir, ".codex", "AGENTS.md"), wrap: func(s string) string { return "# Codex CLI — Lea Instructions\n\n" + s + "\n" }},
		{path: filepath.Join(projectDir, ".gemini", "GEMINI.md"), wrap: func(s string) string { return "# Gemini CLI — Lea Instructions\n\n## BeforeTool Hook\nAlways use grep before reading files.\n\n## SessionStart Reminder\n" + s + "\n" }},
		{path: filepath.Join(projectDir, "AGENTS.md"), wrap: func(s string) string { return "# OpenCode — Lea Instructions\n\n" + s + "\n" }},
		{path: filepath.Join(projectDir, "antigravity-cli", "AGENTS.md"), wrap: func(s string) string { return "# Antigravity — Lea Instructions\n\n## SessionStart Reminder\n" + s + "\n" }},
		{path: filepath.Join(projectDir, "AIDER.md"), wrap: func(s string) string { return "# Aider — Lea Instructions\n\n" + s + "\n" }},
		{path: filepath.Join(home, ".kilocode", "rules", "lea.md"), wrap: func(s string) string { return "# KiloCode — Lea Instructions\n\n" + s + "\n" }},
		{path: filepath.Join(projectDir, ".pi", "AGENTS.md"), wrap: func(s string) string { return "# Pi — Lea Instructions\n\n## SessionStart Reminder\n" + s + "\n" }},
	}

	for _, t := range targets {
		dir := filepath.Dir(t.path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create directory %q: %w", dir, err)
		}
		if err := os.WriteFile(t.path, []byte(t.wrap(content)), 0644); err != nil {
			return fmt.Errorf("cannot write %q: %w", t.path, err)
		}
	}

	return nil
}
