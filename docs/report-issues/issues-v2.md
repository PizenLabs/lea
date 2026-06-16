
# Lea Repository Intelligence Layer

## Architecture Review & Recommendations

Version: Draft v1
---

# Executive Summary

Current Lea output is already moving in the right direction:

* Structural graph exists.
* Repository metadata exists.
* Agent instructions exist.
* Architectural validation exists.

However, responsibilities between generated files are currently overlapping.

This creates three risks:

1. Instruction duplication.
2. Agent confusion.
3. Future maintenance burden.

The recommendation is to establish a strict separation of responsibilities across all generated artifacts.

---

# Design Principles

The Lea ecosystem should follow:

## Single Responsibility

Every generated file should answer exactly one question.

## Graph First

Structural graph data should always be preferred over file scanning.

## Protocol Over Prompt

Lea should own the workflow protocol.

Agent-specific exports should only adapt the protocol.

## Deterministic Navigation

Discovery and reasoning should be graph-driven whenever possible.

## Minimal Context

Only retrieve context required for the current task.

---

# Recommended .lea Structure

```text
.lea/
├── graph.db
├── protocol.json
├── AGENT.md
├── WORKSPACE.md
└── MANIFEST.md
```

---

# graph.db

## Purpose

Canonical structural knowledge source.

## Audience

Lea runtime only.

## Contents

* Symbols
* Call Graph
* Dependency Graph
* Ownership Relations
* Impact Graph
* Architecture Constraints

## Rules

Must never contain instructions.

Must never contain workflow definitions.

Must never contain human documentation.

## Responsibility

Answer:

"How is the codebase connected?"

---

# MANIFEST.md

## Purpose

Explain what Lea is and how agents should think.

## Audience

Agents.

## Contents

### Capabilities

* Symbol Discovery
* Structural Analysis
* Impact Analysis
* Flow Analysis
* Architecture Validation

### Principles

* Structure First
* Graph Over Files
* Minimal Context
* Deterministic Reasoning

## Rules

No repository-specific information.

No workflow instructions.

No command examples.

## Responsibility

Answer:

"What is Lea?"

---

# WORKSPACE.md

## Purpose

Repository metadata.

## Audience

Humans and agents.

## Contents

### Repository Facts

Languages:

* Go
* Rust
* Python
* TypeScript

### Statistics

* Symbol Count
* Node Count
* Edge Count

### Available Capabilities

* Impact Analysis
* Call Graph Traversal
* Validation

### Generated Metadata

* Lea Version
* Index Timestamp

## Rules

No workflow definitions.

No philosophy.

No operational instructions.

## Responsibility

Answer:

"What repository am I operating in?"

---

# AGENT.md

## Purpose

Repository operational protocol.

## Audience

Agents.

## Importance

Highest priority generated document.

## Contents

### Authority Order

1. User Instructions
2. AGENT.md
3. Lea Analysis
4. Local Assumptions

### Required Workflow

1. Discover
2. Reason
3. Plan
4. Execute
5. Verify

### Discovery Phase

Use structural discovery before reading large file sets.

### Reasoning Phase

Use:

* impact analysis
* dependency analysis
* flow analysis

before modifying code.

### Planning Phase

Define:

* Goal
* Constraints
* Acceptance Criteria

before code generation.

### Execution Phase

Generate the smallest safe diff.

Avoid unrelated refactors.

### Verification Phase

Validate architecture before finalization.

### Loop Recovery

If repeated edits fail:

1. Stop editing.
2. Re-analyze structure.
3. Rebuild plan.
4. Continue only after plan revision.

## Rules

No repository statistics.

No implementation details.

No duplicated metadata.

## Responsibility

Answer:

"How should I operate in this repository?"

---

# protocol.json

## Purpose

Machine-readable protocol specification.

## Audience

Exporters and future agents.

## Example

{
"workflow": [
"discover",
"reason",
"plan",
"execute",
"verify"
],
"rules": {
"graph_first": true,
"allow_blind_search": false,
"max_context_tokens": 4000,
"stop_on_loop": true
}
}

## Benefits

* Agent-independent.
* Export-independent.
* Future-proof.

## Responsibility

Answer:

"What protocol should agents follow?"

---

# Export Architecture

## Current

lea export aider

↓

AIDER.md

## Recommended

protocol.json
+
AGENT.md
+
WORKSPACE.md

↓

Agent Adapter

↓

AIDER.md

---

# AIDER Export Strategy

Generated AIDER.md should contain only:

## Authority

Priority rules.

## Workflow

Discover
Reason
Plan
Execute
Verify

## Rules

Graph First

Minimal Context

Small Diff

## Recovery

Stop
Re-analyze
Re-plan

## Avoid

Repository statistics.

Long philosophical explanations.

Implementation details.

Internal database descriptions.

Large capability lists.

---

# Future Agent Support

The same protocol should be exportable to:

* Aider
* Claude Code
* Codex
* Gemini CLI
* Cursor
* Future Agent Platforms

without changing repository semantics.

Example:

lea export aider

lea export claude

lea export codex

lea export gemini

All exports should be adapters of the same protocol.

---

# Final Recommendation

Lea should become the owner of:

* Structural Knowledge
* Repository Protocol
* Architectural Reasoning

Agent tools should become consumers of that protocol.

Target architecture:

graph.db
→ source of truth

protocol.json
→ workflow source of truth

AGENT.md
→ operational instructions

WORKSPACE.md
→ repository facts

MANIFEST.md
→ Lea philosophy

AIDER.md / CLAUDE.md / CODEX.md
→ generated adapters

This separation maximizes clarity, minimizes token usage, reduces agent confusion, and provides a stable foundation for future AI-native development workflows.
