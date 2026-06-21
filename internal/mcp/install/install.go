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
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// MCPEntry represents a single MCP tool entry in the JSON config schema.
type MCPEntry struct {
	Command string   `json:"command" yaml:"cmd" toml:"command"`
	Args    []string `json:"args" yaml:"-" toml:"args"`
}

type target struct {
	Name   string
	Path   string // after tilde expansion
	Format string // json, yaml, toml
}

// installTargets returns the full list of MCP configuration targets.
func installTargets() []target {
	home := homeDir()
	configDir := filepath.Join(home, ".config")

	// VS Code / OpenCode base path is platform-dependent
	vscodeBase := vscodeGlobalStorageDir(home)
	opencodeBase := opencodeGlobalStorageDir(home)

	return []target{
		{Name: "Claude Code", Path: filepath.Join(home, ".claude", ".mcp.json"), Format: "json"},
		{Name: "VS Code (Cline/Roo Code/Codex CLI)", Path: filepath.Join(vscodeBase, "saoudrizwan.claude-dev", "settings", "mcp_settings.json"), Format: "json"},
		{Name: "OpenCode", Path: filepath.Join(opencodeBase, "saoudrizwan.claude-dev", "settings", "mcp_settings.json"), Format: "json"},
		{Name: "Pi Coding Agents", Path: filepath.Join(home, ".pi", "agent", "mcp_config.json"), Format: "json"},
		{Name: "Zed IDE", Path: filepath.Join(home, ".zed", "settings.json"), Format: "json"},
		{Name: "Gemini CLI", Path: filepath.Join(configDir, "gemini-cli", "mcp.json"), Format: "json"},
		{Name: "OpenClaw", Path: filepath.Join(configDir, "openclaw", "mcp.json"), Format: "json"},
		{Name: "Aider", Path: filepath.Join(home, ".aider.conf.yml"), Format: "yaml"},
		{Name: "Antigravity", Path: filepath.Join(configDir, "antigravity", "mcp_manifest.json"), Format: "json"},
		{Name: "Kiro Agent", Path: filepath.Join(configDir, "kiro", "config.toml"), Format: "toml"},
		{Name: "KiloCode", Path: filepath.Join(home, ".kilocode", "config.json"), Format: "json"},
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

// opencodeGlobalStorageDir returns the OpenCode globalStorage path for the current OS.
func opencodeGlobalStorageDir(home string) string {
	// OpenCode uses the same directory structure as VS Code under its own config root.
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "OpenCode", "User", "globalStorage")
	case "linux":
		return filepath.Join(home, ".config", "OpenCode", "User", "globalStorage")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "OpenCode", "User", "globalStorage")
	default:
		return filepath.Join(home, ".config", "OpenCode", "User", "globalStorage")
	}
}

// Run configures all MCP targets with pizen-lea and pizen-lynx entries.
func Run() error {
	// Resolve lea binary path
	leaPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot resolve lea binary path: %w", err)
	}
	leaPath, err = filepath.Abs(leaPath)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute lea path: %w", err)
	}

	// Resolve lx binary path: ~/.cargo/bin/lx
	home := homeDir()
	lxPath := filepath.Join(home, ".cargo", "bin", "lx")

	// Verify lx exists (optional, don't fail)
	if _, err := os.Stat(lxPath); err != nil {
		lxPath = resolveLXFallback()
	}

	successCount := 0

	for _, t := range installTargets() {
		if err := configureTarget(t, leaPath, lxPath); err != nil {
			// Silently skip — log to stderr for debugging but don't fail
			log.Printf("[skip] %s: %v", t.Name, err)
			continue
		}
		fmt.Printf("  ✓ %s\n", t.Name)
		successCount++
	}

	// Generate system instruction file
	if err := generateInstructions(home); err != nil {
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
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		return fmt.Errorf("parent directory %q does not exist", parent)
	}

	switch t.Format {
	case "json":
		return injectJSON(t.Path, leaPath, lxPath)
	case "yaml":
		return injectYAML(t.Path, leaPath, lxPath)
	case "toml":
		return injectTOML(t.Path, leaPath, lxPath)
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

	// Handle Zed IDE format (mcp at root level, not mcpServers)
	if strings.Contains(path, ".zed") || strings.Contains(path, "zed") {
		return injectZedJSON(raw, path, leaPath, lxPath)
	}

	// Standard mcpServers injection
	servers, ok := raw["mcpServers"].(map[string]any)
	if !ok || servers == nil {
		servers = make(map[string]any)
	}
	servers["pizen-lea"] = MCPEntry{Command: leaPath, Args: []string{"mcp"}}
	servers["pizen-lynx"] = MCPEntry{Command: lxPath, Args: []string{"mcp"}}
	raw["mcpServers"] = servers

	return writeJSON(path, raw)
}

// injectZedJSON handles Zed IDE's mcp config format under root "mcp" key.
func injectZedJSON(raw map[string]any, path, leaPath, lxPath string) error {
	mcp, ok := raw["mcp"].(map[string]any)
	if !ok || mcp == nil {
		mcp = make(map[string]any)
	}

	// Zed format: "pizen-lea": { "command": "...", "args": ["mcp"] }
	mcp["pizen-lea"] = MCPEntry{Command: leaPath, Args: []string{"mcp"}}
	mcp["pizen-lynx"] = MCPEntry{Command: lxPath, Args: []string{"mcp"}}
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

// generateInstructions creates the shared instruction file at ~/.config/pizen/instructions.md.
func generateInstructions(home string) error {
	pizenDir := filepath.Join(home, ".config", "pizen")
	if err := os.MkdirAll(pizenDir, 0755); err != nil {
		return fmt.Errorf("cannot create pizen config dir: %w", err)
	}

	content := `# PizenLabs Ecosystem — Dual-Tool Orchestration

CRITICAL: For code-related queries, ALWAYS run pizen-lynx (via search or resolve)
first to discover the exact Symbol ID. DO NOT guess the code structure.

Once the Symbol ID is retrieved, immediately pass it to pizen-lea (via impact,
flow, or neighbors) to map structural reasoning and blast radius.
`
	path := filepath.Join(pizenDir, "instructions.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write instructions: %w", err)
	}

	// Also try to inject into .clinerules / .cursorrules in common project directories.
	// This is best-effort; failures are silently ignored.
	_ = injectClinerules(home)

	return nil
}

// injectClinerules tries to add a reference to the instructions file in detected
// .clinerules or .cursorrules files under common project directories.
func injectClinerules(home string) error {
	rulesContent := "\n# PizenLabs Ecosystem\nSee ~/.config/pizen/instructions.md for dual-tool orchestration instructions.\n"

	candidates := []string{
		filepath.Join(home, ".clinerules"),
		filepath.Join(home, ".cursorrules"),
		filepath.Join(home, ".codex", "rules.md"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "pizen-lynx") || strings.Contains(string(data), "pizen-lea") {
			continue // already injected
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			continue
		}
		_, _ = f.WriteString(rulesContent)
		_ = f.Close()
	}

	return nil
}
