# lea 

[![Go Version](https://img.shields.io/github/go-mod/go-v/PizenLabs/lea)](https://golang.org/doc/devel/release.html)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://img.shields.io/github/actions/workflow/status/PizenLabs/lea/ci.yml?branch=main)](https://github.com/PizenLabs/lea/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/PizenLabs/lea)](https://goreportcard.com/report/github.com/PizenLabs/lea)


**Structural context operating system for AI-native software engineering.**

`lea` is a terminal-first structural memory system designed to help AI models and developers navigate and understand large codebases with minimal context and maximum precision.

Unlike traditional RAG systems that treat code as flat text, `lea` models repositories as **living structural graphs**, preserving the deterministic relationships (calls, dependencies, implementations) that define software systems.

---

##  The Vision

Modern AI coding systems suffer from context window limitations, token inflation, and "context entropy." Most rely on probabilistic semantic chunking (embeddings), which often loses the architectural "big picture."

**Software is symbolic, not just semantic.** `lea` focuses on:
1.  **Structural Retrieval First**: Symbols, dependencies, call graphs, and architectural boundaries.
2.  **Semantic Retrieval Second**: Natural language understanding on top of structural certainty.

---

##  Key Features

-   **Multi-Language AST Indexing**: Native support for **Go** (`go/ast`) and **Python**, **TypeScript**, and **Rust** via Tree-sitter.
-   **Structural Graph Engine**: Models your codebase as a graph of functions, structs, interfaces, and their relationships (`CALLS`, `IMPLEMENTS`, `USES`).
-   **AI Context Compiler**: Generates high-signal, markdown-optimized context for LLMs (Claude, GPT, Gemini) using deterministic retrieval.
-   **Model Context Protocol (MCP)**: Expose your codebase structure directly to AI agents via a standardized protocol.
-   **Workspace Metadata & Agent Export**: Automatically generate deterministic repository metadata (`WORKSPACE.md`) and reasoning protocols (`AGENT.md`) to teach AI agents how to navigate your codebase structurally.
-   **Interactive TUI**: A rich, terminal-based explorer for fuzzy symbol navigation and dependency browsing.
-   **Control Flow & Architecture**: Trace execution paths and detect boundary violations against architectural constraints.
-   **Incremental & Reactive**: Real-time graph updates using `fsnotify` without re-indexing the entire repository.
-   **Local-First**: Powered by an embedded SQLite database. Works offline and over SSH.

---

##  Architecture

`lea` is built with a modular, performance-oriented architecture designed for local execution:

-   **Parser Layer**: Pluggable parsers using native ASTs and Tree-sitter for high-fidelity symbol extraction.
-   **Graph Engine**: A high-performance relationship model that treats your codebase as a first-class graph.
-   **Storage Layer**: SQLite-backed storage utilizing recursive CTEs for complex graph traversals.
-   **Integration Layer**: Built-in MCP server for AI agents and a Bubble Tea-powered TUI for humans.

```text

       ┌────────────────────────────────────────────────────────┐
       │                 Local Filesystem Event                 │
       └───────────────────────────┬────────────────────────────┘
                                   │ (fsnotify)
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │ Incremental Parser Layer (Native Go AST / Tree-sitter) │
       └───────────────────────────┬────────────────────────────┘
                                   │ (Extracted Symbols)
                                   ▼
       ┌────────────────────────────────────────────────────────┐
       │   Storage Layer: SQLite Graph Engine (Recursive CTEs)  │
       └───────────────────────────┬────────────────────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    ▼                             ▼
       ┌────────────────────────┐    ┌──────────────────────────┐
       │   Integration Layer    │    │     Retrieval Engine     │
       │   (Bubble Tea TUI)     │    │   (MCP Server for AIs)   │
       └────────────────────────┘    └──────────────────────────┘

```

##  Installation

### Go Install

```bash
go install github.com/PizenLabs/lea/cmd/lea@latest
```

### Install Script (curl)

```bash
curl -fsSL https://raw.githubusercontent.com/PizenLabs/lea/main/scripts/install.sh | bash
```

### Homebrew (Tap)

```bash
brew tap PizenLabs/tap
brew install lea
```

### From Source

```bash
# Clone the repository
git clone https://github.com/PizenLabs/lea.git
cd lea

# Build the binary
make build

# Install to your GOPATH/bin
make install
```

### Check Your Version

```bash
lea version
```

---

##  Quick Start

### 1. Index your project
Initialize the structural graph for your repository.
```bash
lea index .
```

### 2. Start the MCP Server
Connect your favorite AI agent (like Claude Code or Aider) directly to your codebase.
```bash
lea mcp
```

### 3. AI-Native Integration
Generate metadata and export instructions for your AI agents (Claude, Cursor, Gemini, etc.).
```bash
# Generate structural metadata and reasoning protocols
lea index .

# Export configurations for specific agents
lea export claude
lea export cursor
lea export gemini
```

### 4. Interactive Exploration

---

## Usage Guide

For a deeper walkthrough (installation, workflows, and diagrams), see [GUIDE.md](GUIDE.md).

## Usage Patterns

### AI-Agent Workflow (MCP)

1. **Find the target symbol** to get exact file and symbol coordinates.
2. **Expand context** to pull immediate neighbors (`CALLS`, `USES`, `IMPLEMENTS`).
3. **Trace execution** to map the ordered call graph for the change.
4. **Check boundaries** against architecture rules before committing updates.

### Developer Workflow (CLI/TUI)

- Use `lea tui` for fuzzy symbol search and graph browsing.
- Use `lea context` to generate prompt-ready context for web LLMs.
- Use `lea flow` and `lea trace` to understand execution order and impact.

### AI Agent Integration (Export)

Lea bridges the gap between raw source code and AI reasoning by providing deterministic integration points:

- **WORKSPACE.md**: Repository statistics and facts generated by `lea index`.
- **AGENT.md**: A recommended structural reasoning protocol for all AI agents.
- **Export Targets**: Use `lea export <target>` to generate `CLAUDE.md`, `.cursorrules`, `GEMINI.md`, and more.

---

##  Command Guide

| Command | Description | Example |
| :--- | :--- | :--- |
| `index` | Build or update the structural graph | `lea index .` |
| `tui` | Open the interactive symbol explorer | `lea tui` |
| `mcp` | Start the Model Context Protocol server | `lea mcp` |
| `export` | Generate AI agent configuration files | `lea export claude` |
| `context` | Generate LLM-optimized context for a symbol | `lea context "type:internal/storage/sqlite:Store"` |
| `trace` | Follow the call graph from a specific function | `lea trace "func:internal/cli/commands:Execute"` |
| `flow` | Inspect ordered call flow within a symbol | `lea flow "func:internal/cli/commands:Execute"` |
| `neighbors` | Find immediate dependencies of a symbol | `lea neighbors AuthService` |
| `impact` | Analyze the impact of changing a symbol | `lea impact TokenService` |
| `violations` | Check for architectural boundary violations | `lea violations --config arch.yaml` |
| `watch` | Watch for file changes and update the graph | `lea watch .` |

---

## Additional Diagrams

### MCP Query Flow

```mermaid
sequenceDiagram
    autonumber
    actor Agent as AI Agent
    participant Lea as lea (MCP Server)
    participant DB as SQLite (Graph Engine)

    Agent->>Lea: find_symbol(name)
    Lea->>DB: Query exact Symbol URI/File
    DB-->>Lea: Return Node
    Lea-->>Agent: Return Symbol Coordinates

    Agent->>Lea: get_neighbors(URI)
    Lea->>DB: Traverse Edges (CALLS, USES, etc.)
    DB-->>Lea: Return Subgraph
    Lea-->>Agent: Return Markdown Context
```

### Incremental Indexing Loop

```text
[Filesystem Event: Modify/Create/Delete]
                   │
                   ▼
       ┌───────────────────────┐
       │   Debounce & Batch    │ (Gathers changes over 100-300ms)
       └───────────┬───────────┘
                   │
                   ▼
       ┌───────────────────────┐
       │   Invalidation Stage  │ (Deletes affected Nodes & Edges in SQLite)
       └───────────┬───────────┘
                   │
                   ▼
       ┌───────────────────────┐
       │  Incremental Parsing  │ (Native go/ast or Tree-sitter AST extraction)
       └───────────┬───────────┘
                   │
                   ▼
       ┌───────────────────────┐
       │     Graph Commit      │ (Atomic SQL Transaction: Inserts new entities)
       └───────────────────────┘
```

---

## Troubleshooting

- If the graph is empty, re-run `lea index .` and confirm your repository path is correct.
- For architecture checks, ensure your rules file (for example `arch.yaml`) is present and valid.
- If symbols are missing, confirm the target language parser is supported and available.

---

##  Roadmap

- [x] **Phase 1: MVP**: Go parser, SQLite storage, basic graph queries.
- [x] **Phase 2: AI Context Layer**: High-signal markdown generation and context compilation.
- [x] **Phase 3: Incremental Updates**: Real-time file watching and partial re-indexing.
- [x] **Phase 4: MCP Integration**: Standardized protocol for AI agent connectivity.
- [x] **Phase 5: Interactive TUI**: Fuzzy navigation and visual dependency exploration.
- [x] **Phase 6: Multi-Language Support**: Tree-sitter integration for Python, Rust, and TypeScript.
- [x] **Phase 7: Advanced Retrieval**: Control flow analysis and architecture guardrails.

---

##  Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on development, testing, and pull requests.

---

##  License

`lea` is licensed under the [MIT License](LICENSE).

---

Built for the future of AI-native engineering. 🦾
