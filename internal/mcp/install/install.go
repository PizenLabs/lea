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

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Options controls automated behavior of the install command.
type Options struct {
	AutoSelectAll bool
}

// MCPEntry represents a single MCP tool entry in the JSON config schema.
type MCPEntry struct {
	Command string            `json:"command" yaml:"cmd" toml:"command"`
	Args    []string          `json:"args" yaml:"-" toml:"args"`
	Env     map[string]string `json:"env,omitempty" yaml:"-" toml:"-"`
}

type target struct {
	Name            string
	Path            string
	Format          string
	ConfigDir       string
	InstructionFile string
}

// installTargets returns the full list of MCP configuration targets using
// global/home configuration directories. No project-scoped directories are used.
func installTargets(home, vscodeUserDir string) []target {
	zedDir := zedConfigDir(home)
	return []target{
		{Name: "Claude Code", Path: filepath.Join(home, ".claude", "settings.json"), Format: "json", ConfigDir: filepath.Join(home, ".claude"), InstructionFile: "CLAUDE.md"},
		{Name: "Codex CLI", Path: filepath.Join(home, ".codex", "config.toml"), Format: "codex_toml", ConfigDir: filepath.Join(home, ".codex"), InstructionFile: "AGENTS.md"},
		{Name: "Pi Coding Agents", Path: filepath.Join(home, ".pi", "agent", "mcp.json"), Format: "json", ConfigDir: filepath.Join(home, ".pi", "agent"), InstructionFile: "AGENTS.md"},
		{Name: "Gemini CLI", Path: filepath.Join(home, ".gemini", "settings.json"), Format: "json", ConfigDir: filepath.Join(home, ".gemini"), InstructionFile: "GEMINI.md"},
		{Name: "Zed", Path: filepath.Join(zedDir, "settings.json"), Format: "zed", ConfigDir: zedDir, InstructionFile: "AGENTS.md"},
		{Name: "OpenCode", Path: filepath.Join(home, ".config", "opencode", "opencode.json"), Format: "opencode", ConfigDir: filepath.Join(home, ".config", "opencode"), InstructionFile: "AGENTS.md"},
		{Name: "Antigravity", Path: filepath.Join(home, ".gemini", "config", "mcp_config.json"), Format: "json", ConfigDir: filepath.Join(home, ".gemini", "config"), InstructionFile: "AGENTS.md"},
		{Name: "Aider", Path: filepath.Join(home, ".aider.conf.yml"), Format: "yaml", ConfigDir: filepath.Join(home, ".aider"), InstructionFile: "AIDER.md"},
		{Name: "KiloCode", Path: filepath.Join(home, ".kilocode", "settings.json"), Format: "json", ConfigDir: filepath.Join(home, ".kilocode"), InstructionFile: "AGENTS.md"},
		{Name: "VS Code", Path: filepath.Join(vscodeUserDir, "globalStorage", "mcp.json"), Format: "json", ConfigDir: filepath.Join(vscodeUserDir, "globalStorage"), InstructionFile: "instructions.md"},
		{Name: "OpenClaw", Path: filepath.Join(home, ".openclaw", "config.json"), Format: "json", ConfigDir: filepath.Join(home, ".openclaw"), InstructionFile: "AGENTS.md"},
		{Name: "Kiro", Path: filepath.Join(home, ".kiro", "settings", "mcp.json"), Format: "json", ConfigDir: filepath.Join(home, ".kiro", "settings"), InstructionFile: "AGENTS.md"},
		{Name: "System Instructions", Path: filepath.Join(home, ".config", "pizen", "instructions.md"), Format: "instructions", ConfigDir: filepath.Join(home, ".config", "pizen")},
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

// zedConfigDir returns the Zed configuration directory for the current OS.
func zedConfigDir(home string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Zed")
	case "linux":
		return filepath.Join(home, ".config", "zed")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Zed", "User")
	default:
		return filepath.Join(home, ".config", "zed")
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

// Run configures MCP targets for detected AI coding agents.
// If opts includes AutoSelectAll=true, all detected targets are configured
// without user interaction. Otherwise an interactive multi-select prompt is shown.
func Run(opts ...Options) error {
	option := Options{}
	if len(opts) > 0 {
		option = opts[0]
	}

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

	vscDir := vscodeUserDir(home)
	allTargets := installTargets(home, vscDir)
	detected := detectTargets(allTargets)

	if len(detected) == 0 {
		fmt.Println("No existing AI agent config directories found; nothing to configure.")
		return nil
	}

	fmt.Printf("Detected %d existing AI agent config directories.\n\n", len(detected))

	var selected []target
	if option.AutoSelectAll {
		selected = detected
		fmt.Println("Configuring all detected targets (--yes/--all).")
	} else {
		selected, err = selectTargets(detected)
		if err != nil {
			return fmt.Errorf("target selection failed: %w", err)
		}
	}

	if len(selected) == 0 {
		fmt.Println("No MCP targets selected.")
		return nil
	}

	successCount := 0
	for _, t := range selected {
		if err := configureTarget(t, leaPath, lxPath); err != nil {
			log.Printf("[skip] %s: %v", t.Name, err)
			continue
		}
		fmt.Printf("  ✓ %s\n", t.Name)
		successCount++
	}

	fmt.Printf("\nConfigured %d MCP targets successfully.\n", successCount)
	return nil
}

// detectTargets returns only targets whose global configuration directory exists.
func detectTargets(targets []target) []target {
	detected := make([]target, 0, len(targets))
	for _, t := range targets {
		info, err := os.Stat(t.ConfigDir)
		if err == nil && info.IsDir() {
			detected = append(detected, t)
		}
	}
	return detected
}

// validateConfigDir returns an error if the given path does not exist or is not a directory.
func validateConfigDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config directory %q does not exist", path)
		}
		return fmt.Errorf("cannot stat config directory %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("config path %q is not a directory", path)
	}
	return nil
}

// selectionItem implements list.Item for the interactive multi-select prompt.
type selectionItem struct {
	target   target
	selected bool
}

func (i selectionItem) Title() string {
	state := "[ ]"
	if i.selected {
		state = "[x]"
	}
	return fmt.Sprintf("%s %s", state, i.target.Name)
}

func (i selectionItem) Description() string {
	return i.target.Path
}

func (i selectionItem) FilterValue() string {
	return i.target.Name
}

type selectionModel struct {
	list list.Model
}

func newSelectionModel(targets []target) selectionModel {
	items := make([]list.Item, len(targets))
	for i, t := range targets {
		items[i] = selectionItem{target: t}
	}
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = ""
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	return selectionModel{list: l}
}

func (m selectionModel) Init() tea.Cmd {
	return nil
}

func (m selectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			return m, tea.Quit
		case " ", "x":
			m.toggle()
			return m, nil
		case "a":
			m.selectAll()
			return m, nil
		case "i":
			m.clearAll()
			return m, nil
		}
	case tea.WindowSizeMsg:
		h := msg.Height - 4
		if h < 0 {
			h = 0
		}
		m.list.SetSize(msg.Width, h)
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *selectionModel) toggle() {
	items := m.list.Items()
	if len(items) == 0 {
		return
	}
	idx := m.list.GlobalIndex()
	if idx < 0 || idx >= len(items) {
		return
	}
	item, ok := items[idx].(selectionItem)
	if !ok {
		return
	}
	item.selected = !item.selected
	m.list.SetItem(idx, item)
}

func (m *selectionModel) selectAll() {
	for i, item := range m.list.Items() {
		it, ok := item.(selectionItem)
		if !ok {
			continue
		}
		it.selected = true
		m.list.SetItem(i, it)
	}
}

func (m *selectionModel) clearAll() {
	for i, item := range m.list.Items() {
		it, ok := item.(selectionItem)
		if !ok {
			continue
		}
		it.selected = false
		m.list.SetItem(i, it)
	}
}

func (m selectionModel) selectedTargets() []target {
	var selected []target
	for _, item := range m.list.Items() {
		it, ok := item.(selectionItem)
		if !ok || !it.selected {
			continue
		}
		selected = append(selected, it.target)
	}
	return selected
}

func (m selectionModel) View() string {
	return fmt.Sprintf("Select targets to configure:\n%s\n\n  space/x toggle · a select all · i clear all · enter confirm · ctrl+c cancel", m.list.View())
}

// selectTargets presents an interactive multi-selection prompt and returns chosen targets.
func selectTargets(targets []target) ([]target, error) {
	p := tea.NewProgram(newSelectionModel(targets), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	model, ok := final.(selectionModel)
	if !ok {
		return nil, fmt.Errorf("unexpected prompt result type")
	}
	return model.selectedTargets(), nil
}

// resolveLXFallback tries to find lx via PATH as a fallback.
func resolveLXFallback() string {
	p, err := exec.LookPath("lx")
	if err != nil {
		return filepath.Join(homeDir(), ".cargo", "bin", "lx")
	}
	return p
}

// configureTarget injects MCP entries into a single target configuration file.
func configureTarget(t target, leaPath, lxPath string) error {
	if err := validateConfigDir(t.ConfigDir); err != nil {
		return err
	}

	var err error
	switch t.Format {
	case "json":
		err = injectJSON(t.Path, leaPath, lxPath)
	case "opencode":
		err = injectOpenCodeJSON(t.Path, leaPath, lxPath)
	case "zed":
		err = injectZedJSON(t.Path, leaPath, lxPath)
	case "yaml":
		err = injectYAML(t.Path, leaPath, lxPath)
	case "toml":
		err = injectTOML(t.Path, leaPath, lxPath)
	case "codex_toml":
		err = injectCodexTOML(t.Path, leaPath, lxPath)
	case "instructions":
		return writeInstructions(t.Path)
	default:
		return fmt.Errorf("unsupported format: %s", t.Format)
	}

	if err != nil {
		return err
	}

	if t.InstructionFile != "" {
		if err := writeInstructionsFile(t); err != nil {
			log.Printf("[warning] failed to write instructions for %s: %v", t.Name, err)
		}
	}

	return nil
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

	injectHooksJSON(raw, leaPath)

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

	injectHooksJSON(raw, leaPath)

	return writeJSON(path, raw)
}

// injectOpenCodeJSON handles OpenCode's MCP config format under root "mcp" key.
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

	mcp["lea"] = map[string]any{
		"enabled": true,
		"type":    "local",
		"command": []any{leaPath, "mcp"},
	}
	mcp["lynx"] = map[string]any{
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

	mcpList, ok := raw["mcp"].([]any)
	if !ok {
		mcpList = []any{}
	}

	mcpList = upsertYAMLList(mcpList, "pizen-lea", leaPath)
	mcpList = upsertYAMLList(mcpList, "pizen-lynx", lxPath)
	raw["mcp"] = mcpList

	injectHooksYAML(raw, leaPath)

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

	tools, ok := raw["external_tools"].([]any)
	if !ok {
		tools = []any{}
	}

	tools = upsertTOMLTool(tools, "pizen-lea", leaPath, []string{"mcp"})
	tools = upsertTOMLTool(tools, "pizen-lynx", lxPath, []string{"mcp"})
	raw["external_tools"] = tools

	injectHooksTOML(raw, leaPath)

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

// injectCodexTOML reads or creates a TOML file and injects pizen entries.
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

	injectHooksTOML(raw, leaPath)

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

// writeInstructions writes the global Pizen ecosystem instructions file.
func writeInstructions(path string) error {
	rawContent := `<!-- lea-lx-mcp:start -->
## 1. Tool Priority Queue (Strict Compliance)
1. **FIRST PRIORITY:** Always invoke the custom AST-aware commands directly when searching for code or viewing context.
   * Code Search: Use [BT]lynx_search_graph[BT].
   * Symbol Definition: Use [BT]lynx_resolve_symbol[BT].
   * AST Inspection: Use [BT]lea_view_symbol_ast[BT].

2. **LAST RESORT FALLBACK:** If (and only if) the primary tools above return connection errors, a method-not-found exception, or an empty index result, you are strictly allowed to fallback to grep or glob to locate the plain-text references.

## 2. Strict Parameter Schema Mapping & Identifier Rules
* **[BT]lea_view_symbol_ast[BT]**: Requires [BT]symbol_id[BT] parameter.
  - For Functions: {"symbol_id": "func:package/path:FunctionName"}
  - For Structs/Types: {"symbol_id": "type:package/path:StructName"}
  - For Methods: {"symbol_id": "func:package/path:ReceiverStruct.MethodName"}

* **[BT]lynx_search_graph[BT]**: Requires [BT]query[BT] parameter.
  - Example: {"query": "userRepository"}

* **[BT]lynx_resolve_symbol[BT]**: Requires [BT]name[BT] parameter.
  - Example: {"name": "userRepository"}

## 3. Expected Thinking Pattern & Examples
When tasked to inspect or locate a struct, type, or function without an explicit package path, follow this internal reflection loop to save tokens and avoid failures:
1. **Bad Thought:** "Let's run grep, ripgrep, or glob to find userRepository." -> (STRICT VIOLATION)
2. **Good Thought:** "I must look up the structural definition first. I will call [BT]lynx_lynx_resolve_symbol[BT] with name='userRepository' or [BT]lynx_lynx_search_graph[BT] with query='userRepository' to discover its exact Go package path."
3. **Execution:** Once the package path is resolved (e.g., [BT]internal/user/impl[BT]), I will construct the canonical [BT]symbol_id[BT] (e.g., [BT]type:internal/user/impl:userRepository[BT]) and invoke [BT]lea_lea_view_symbol_ast[BT] directly.

## CRITICAL NOTE ON GO SYMBOLS:
The second component in the [BT]symbol_id[BT] MUST ALWAYS be the exact Go package path (e.g., internal/user/impl), NEVER a standard Go filename or physical file path. Using file paths will result in immediate "symbol not found" errors.
<!-- lea-lx-mcp:end -->
`
	content := strings.ReplaceAll(rawContent, "[BT]", "`")
	return os.WriteFile(path, []byte(content), 0644)
}

func writeInstructionsFile(t target) error {
	if t.InstructionFile == "" {
		return nil
	}
	if err := os.MkdirAll(t.ConfigDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(t.ConfigDir, t.InstructionFile)

	rawContent := `<!-- lea-lx-mcp:start -->
## 1. Tool Priority Queue (Strict Compliance)
1. **FIRST PRIORITY:** Always invoke the custom AST-aware commands directly when searching for code or viewing context.
   * Code Search: Use [BT]lynx_search_graph[BT].
   * Symbol Definition: Use [BT]lynx_resolve_symbol[BT].
   * AST Inspection: Use [BT]lea_view_symbol_ast[BT].

2. **LAST RESORT FALLBACK:** If (and only if) the primary tools above return connection errors, a method-not-found exception, or an empty index result, you are strictly allowed to fallback to grep or glob to locate the plain-text references.

## 2. Strict Parameter Schema Mapping & Identifier Rules
* **[BT]lea_view_symbol_ast[BT]**: Requires [BT]symbol_id[BT] parameter.
  - For Functions: {"symbol_id": "func:package/path:FunctionName"}
  - For Structs/Types: {"symbol_id": "type:package/path:StructName"}
  - For Methods: {"symbol_id": "func:package/path:ReceiverStruct.MethodName"}

* **[BT]lynx_search_graph[BT]**: Requires [BT]query[BT] parameter.
  - Example: {"query": "userRepository"}

* **[BT]lynx_resolve_symbol[BT]**: Requires [BT]name[BT] parameter.
  - Example: {"name": "userRepository"}

## 3. Expected Thinking Pattern & Examples
When tasked to inspect or locate a struct, type, or function without an explicit package path, follow this internal reflection loop to save tokens and avoid failures:
1. **Bad Thought:** "Let's run grep, ripgrep, or glob to find userRepository." -> (STRICT VIOLATION)
2. **Good Thought:** "I must look up the structural definition first. I will call [BT]lynx_lynx_resolve_symbol[BT] with name='userRepository' or [BT]lynx_lynx_search_graph[BT] with query='userRepository' to discover its exact Go package path."
3. **Execution:** Once the package path is resolved (e.g., [BT]internal/user/impl[BT]), I will construct the canonical [BT]symbol_id[BT] (e.g., [BT]type:internal/user/impl:userRepository[BT]) and invoke [BT]lea_lea_view_symbol_ast[BT] directly.

## CRITICAL NOTE ON GO SYMBOLS:
The second component in the [BT]symbol_id[BT] MUST ALWAYS be the exact Go package path (e.g., internal/user/impl), NEVER a standard Go filename or physical file path. Using file paths will result in immediate "symbol not found" errors.
<!-- lea-lx-mcp:end -->
`
	content := strings.ReplaceAll(rawContent, "[BT]", "`")
	return os.WriteFile(path, []byte(content), 0644)
}

func injectHooksJSON(raw map[string]any, leaPath string) {
	hooks, ok := raw["hooks"].(map[string]any)
	if !ok || hooks == nil {
		hooks = make(map[string]any)
	}

	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok {
		preToolUse = []any{}
	}

	hookCmd := leaPath + " hook pre-tool"
	found := false
	for _, item := range preToolUse {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		matcher, _ := entry["matcher"].(string)
		if matcher == "*" {
			subHooks, ok := entry["hooks"].([]any)
			if ok {
				for _, sh := range subHooks {
					shMap, ok := sh.(map[string]any)
					if ok {
						cmd, _ := shMap["command"].(string)
						if strings.Contains(cmd, "lea hook") {
							shMap["command"] = hookCmd
							found = true
							break
						}
					}
				}
			}
		}
	}

	if !found {
		newHook := map[string]any{
			"matcher": "*",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": hookCmd,
				},
			},
		}
		preToolUse = append(preToolUse, newHook)
	}

	hooks["PreToolUse"] = preToolUse
	raw["hooks"] = hooks
}

func injectHooksTOML(raw map[string]any, leaPath string) {
	hookCmd := leaPath + " hook pre-tool"
	hooks, ok := raw["hooks"].(map[string]any)
	if !ok || hooks == nil {
		hooks = make(map[string]any)
	}

	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok {
		preToolUse = []any{}
	}

	found := false
	for i, item := range preToolUse {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		matcher, _ := entry["matcher"].(string)
		if matcher == "*" {
			subHooks, ok := entry["hooks"].([]any)
			if ok {
				for j, sh := range subHooks {
					shMap, ok := sh.(map[string]any)
					if ok {
						cmd, _ := shMap["command"].(string)
						if strings.Contains(cmd, "lea hook") {
							shMap["command"] = hookCmd
							subHooks[j] = shMap
							found = true
							break
						}
					}
				}
				entry["hooks"] = subHooks
				preToolUse[i] = entry
			}
		}
	}

	if !found {
		newHook := map[string]any{
			"matcher": "*",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": hookCmd,
				},
			},
		}
		preToolUse = append(preToolUse, newHook)
	}

	hooks["PreToolUse"] = preToolUse
	raw["hooks"] = hooks
}

func injectHooksYAML(raw map[string]any, leaPath string) {
	hookCmd := leaPath + " hook pre-tool"
	hooks, ok := raw["hooks"].(map[string]any)
	if !ok || hooks == nil {
		hooks = make(map[string]any)
	}

	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok {
		preToolUse = []any{}
	}

	found := false
	for i, item := range preToolUse {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		matcher, _ := entry["matcher"].(string)
		if matcher == "*" {
			subHooks, ok := entry["hooks"].([]any)
			if ok {
				for j, sh := range subHooks {
					shMap, ok := sh.(map[string]any)
					if ok {
						cmd, _ := shMap["command"].(string)
						if strings.Contains(cmd, "lea hook") {
							shMap["command"] = hookCmd
							subHooks[j] = shMap
							found = true
							break
						}
					}
				}
				entry["hooks"] = subHooks
				preToolUse[i] = entry
			}
		}
	}

	if !found {
		newHook := map[string]any{
			"matcher": "*",
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": hookCmd,
				},
			},
		}
		preToolUse = append(preToolUse, newHook)
	}

	hooks["PreToolUse"] = preToolUse
	raw["hooks"] = hooks
}
