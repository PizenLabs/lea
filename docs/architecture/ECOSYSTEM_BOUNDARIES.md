
# PizenLabs Ecosystem Boundaries

## Purpose

This document defines the responsibilities, boundaries, and design principles of the PizenLabs ecosystem.

Its goal is to prevent feature overlap, architectural drift, and duplicated responsibilities between projects.

Every new feature proposal should be evaluated against these rules before implementation.

---

# Core Philosophy

Software understanding consists of three independent stages:

```text
Human Request
      │
      ▼
 Discovery
      │
      ▼
 Structural Reasoning
      │
      ▼
 Execution
```

PizenLabs separates these concerns into dedicated systems.

Each system should remain focused on its domain.

---

# Ecosystem Overview

```text
┌─────────────────────┐
│        Lynx         │
├─────────────────────┤
│ Discovery Engine    │
│ Semantic Retrieval  │
│ Lexical Retrieval   │
└──────────┬──────────┘
           │
           ▼
     Symbol Resolution
           │
           ▼
┌─────────────────────┐
│         Lea         │
├─────────────────────┤
│ Structural Graph    │
│ Dependency Analysis │
│ Impact Analysis     │
│ Architecture Rules  │
└──────────┬──────────┘
           │
           ▼
      AI Agent
```

---

# Lynx Responsibilities

## Mission

Convert human intent into precise repository locations.

## Primary Question

```text
Where should I look?
```

## Responsibilities

* Semantic search
* Lexical search
* Hybrid retrieval
* Natural language understanding
* Symbol discovery
* Code chunk retrieval
* Documentation retrieval
* Config retrieval

## Inputs

Examples:

```text
How is authentication implemented?
```

```text
Where is MFA created?
```

```text
JWT validation flow
```

## Outputs

Examples:

```text
internal/auth/service.go
```

```text
func:internal/auth:Login
```

```text
internal/auth/session.go:128
```

## What Lynx Must Never Do

* Build dependency graphs
* Perform call graph traversal
* Calculate blast radius
* Validate architecture
* Determine ownership layers
* Analyze structural relationships

Those responsibilities belong to Lea.

---

# Lea Responsibilities

## Mission

Convert repository structure into deterministic reasoning.

## Primary Question

```text
What happens if I change this?
```

## Responsibilities

* Structural graph generation
* Symbol relationships
* Call graph traversal
* Dependency analysis
* Impact analysis
* Architecture validation
* Context compilation
* Structural metadata generation

## Inputs

Examples:

```text
func:internal/auth:Login
```

```text
type:internal/auth:AuthService
```

```text
internal/auth/service.go
```

## Outputs

Examples:

```text
Call hierarchy
```

```text
Execution path
```

```text
Architecture violations
```

```text
Impact report
```

## What Lea Must Never Do

* Semantic search
* Embedding generation
* Vector databases
* Natural language retrieval
* BM25 retrieval
* Fuzzy discovery
* Query understanding

Those responsibilities belong to Lynx.

---

# Architectural Rule #1

Lynx discovers.

Lea reasons.

Never reverse this relationship.

---

# Architectural Rule #2

Lynx may feed Lea.

Lea must not depend on Lynx.

Example:

```text
User Query
     │
     ▼
   Lynx
     │
     ▼
 Exact Symbol
     │
     ▼
    Lea
```

Valid.

---

```text
User Query
     │
     ▼
    Lea
     │
     ▼
 Semantic Search
```

Invalid.

---

# Architectural Rule #3

Natural language belongs to Lynx.

Structure belongs to Lea.

Examples:

```text
authentication flow
```

Lynx.

---

```text
func:internal/auth:Login
```

Lea.

---

# Architectural Rule #4

Lynx optimizes discovery.

Lea optimizes certainty.

Lynx answers:

```text
Probably relevant.
```

Lea answers:

```text
Deterministically related.
```

---

# Architectural Rule #5

Never duplicate features.

If a capability already exists in Lynx:

Do not implement it in Lea.

If a capability already exists in Lea:

Do not implement it in Lynx.

Duplication creates ecosystem entropy.

---

# Feature Evaluation Framework

Before implementing a feature, ask:

## Question 1

```text
Is the input natural language?
```

If yes:

→ Lynx

---

## Question 2

```text
Is the input an exact symbol?
```

If yes:

→ Lea

---

## Question 3

```text
Does it require embeddings?
```

If yes:

→ Lynx

---

## Question 4

```text
Does it require graph traversal?
```

If yes:

→ Lea

---

## Question 5

```text
Can this feature exist without repository structure?
```

If yes:

→ Lynx

If no:

→ Lea

---

# Long-Term Vision

The future PizenLabs ecosystem should look like:

```text
Human Intent
      │
      ▼
    Lynx
 Discovery Layer
      │
      ▼
 Exact Symbols
      │
      ▼
     Lea
 Structural Layer
      │
      ▼
 Context Package
      │
      ▼
 AI Agent
      │
      ▼
 Code Changes
```

Every system has a single responsibility.

Every system remains independently evolvable.

Every system remains replaceable.

This separation is a strategic advantage, not a limitation.

---

# Ecosystem Motto

Lynx discovers.

Lea reasons.

Agents execute.
