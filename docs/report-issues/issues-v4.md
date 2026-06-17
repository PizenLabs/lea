# Lea Architecture Review (AI-First Edition)

## Executive Summary

Lea should be optimized for AI reasoning, not human readability.

The primary objective is:

> Provide the highest-quality repository understanding with the lowest possible ambiguity, token cost, and cognitive overhead.

Every file inside `.lea/` must directly improve an AI agent's ability to reason about the repository.

Any file that exists only for aesthetics, documentation, duplication, compatibility layering, or human convenience should be removed from the core architecture.

---

# Design Principles

## 1. Single Source of Truth

Every fact must exist exactly once.

Bad:

* workspace.json
* WORKSPACE.md

containing the same information.

Good:

* workspace.json (authoritative)
* WORKSPACE.md (optional generated projection)

The AI should never need to decide which copy is correct.

---

## 2. Structured Data Over Natural Language

AI agents consume structured data more reliably than prose.

Prefer:

```json
{
  "language": "Go"
}
```

over:

```markdown
Primary language: Go
```

The first requires zero interpretation.

The second requires parsing and inference.

---

## 3. Minimize File Count

Every additional file creates:

* more I/O
* more token consumption
* more navigation overhead
* more chances for inconsistency

A file must justify its existence.

If two files are always read together, they should probably be merged.

---

## 4. Separate Facts From Intent

Repository facts:

* languages
* frameworks
* graph statistics
* dependencies

belong in workspace.json.

Human goals:

* business objectives
* architectural goals
* ownership
* constraints

belong in intent.json.

These are fundamentally different categories of information.

---

## 5. Graph Is The Brain

The graph is the primary intelligence source.

Everything else exists only to help the AI interpret and use the graph.

Priority:

1. graph.db
2. protocol.json
3. workspace.json
4. memory.json
5. intent.json
6. limitations.json

---

# Recommended .lea Layout

```text
.lea/
├── graph.db
├── protocol.json
├── workspace.json
├── memory.json
├── intent.json
└── limitations.json
```

Nothing else is required.

---

# File Definitions

## graph.db

Purpose:

Structural memory.

Contains:

* symbols
* calls
* dependencies
* relationships
* architecture graph

This is the authoritative representation of repository structure.

---

## protocol.json

Purpose:

AI operating protocol.

Contains:

* workflow
* authority order
* commands
* capabilities
* execution rules

Example responsibilities:

* discover before reasoning
* graph-first workflow
* command definitions
* validation rules

This is the first file every AI should read.

---

## workspace.json

Purpose:

Repository facts.

Contains:

* module name
* languages
* frameworks
* graph statistics
* generated metadata

Contains facts only.

No instructions.

No prompts.

No policy.

---

## memory.json

Purpose:

Accumulated operational knowledge.

Contains:

* hotspots
* frequently changed files
* historical failures
* successful patterns

Unlike workspace.json, this file evolves over time.

It represents experience rather than facts.

---

## intent.json

Purpose:

Human intent.

Contains:

* product goals
* architecture goals
* ownership
* constraints
* forbidden changes

This information cannot be inferred from the graph.

Therefore it deserves a dedicated file.

---

## limitations.json

Purpose:

Self-awareness.

Contains:

* unsupported languages
* missing graph dimensions
* heuristic detections
* confidence limitations

AI agents should understand what Lea does not know.

This improves planning quality and reduces false confidence.

---

# Files To Remove

## runtime.json

Reason:

Its contents belong inside protocol.json.

A separate file creates navigation overhead without adding information value.

---

## capabilities.json

Reason:

Capabilities are protocol information.

Merge into protocol.json.

---

## commands.json

Reason:

Commands are protocol information.

Merge into protocol.json.

---

## bootstrap.md

Reason:

Adds an unnecessary indirection layer.

AI should read protocol.json directly.

---

## AGENT.md

Reason:

Internal AI state should not depend on markdown pointers.

Keep only for external tool compatibility.

Never use as a knowledge source.

---

## WORKSPACE.md

Reason:

Human projection only.

Not part of the AI architecture.

---

## mental_model.json

Reason:

Current content is heuristic and low-confidence.

The value gained is smaller than the ambiguity introduced.

Remove until meaningful signals exist.

---

# Export Architecture

Export files exist only for compatibility.

They are not knowledge sources.

Examples:

* CLAUDE.md
* GEMINI.md
* AIDER.md
* .codex/AGENTS.md
* .pi/AGENTS.md
* .continue/rules/lea.md

Every export should contain only:

```text
This repository uses Lea.
Read .lea/protocol.json.
```

No duplicated instructions.

No duplicated metadata.

No duplicated prompts.

The export layer is an adapter, not a storage layer.

---

# Final Architecture Score

Current architecture:
7.5–8.0 / 10

Reasons:

* Too many metadata files
* Multiple indirection layers
* Repeated concepts split across files

Recommended architecture:
9.5 / 10

Reasons:

* Minimal surface area
* Clear ownership of information
* Low ambiguity
* Low token cost
* AI-first design
* Single-source-of-truth everywhere

---

# Long-Term Vision

Lea should evolve toward:

> A repository cognition layer for AI agents.

Not documentation.

Not prompts.

Not markdown knowledge bases.

The graph is the memory.

Protocol is the operating system.

Workspace is reality.

Memory is experience.

Intent is purpose.

Limitations are self-awareness.

Everything else is optional.
