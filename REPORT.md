
# Lea Improvement Report

## Executive Summary

Lea has already proven valuable as a deterministic structural navigation engine. Early testing shows that AI agents can use Lea to understand execution flow, dependency relationships, and architectural context significantly faster than manually reading source files.

However, current evaluations reveal several structural limitations that reduce its effectiveness as a repository-wide reasoning engine.

This report outlines the highest-priority improvements required to evolve Lea into a production-grade Structural Reasoning Layer for the PizenLabs ecosystem.

---

# Current Strengths

## 1. Execution Flow Analysis

The `lea flow` command provides a deterministic execution path for a function or method.

Example:

```bash
lea flow func:cmd/http:main
```

Benefits:

* Ordered call sequence.
* Preserves logical branches.
* Includes constructs such as:

  * if statements
  * error handling
  * defer blocks
  * nested calls

This allows agents to understand runtime behavior without reading entire files.

---

## 2. Dependency Navigation

Lea currently supports:

```bash
lea trace
```

for outgoing dependencies and:

```bash
lea impact
```

for incoming dependencies.

Benefits:

* Quick caller discovery.
* Fast dependency exploration.
* Structural navigation without grep.

---

## 3. Architectural Context

The `lea context` command provides a compact structural summary for a symbol.

Benefits:

* Fast architectural onboarding.
* Reduced token consumption.
* Efficient navigation to relevant code regions.

---

# Major Limitations

## L1. Cross-Package Resolution

### Current State

Cross-package calls are frequently resolved as:

```text
unknown
```

Example:

```go
main()
 └── userImpl.NewUserUsecases()
```

may appear as:

```text
unknown
```

instead of:

```text
func:internal/user/impl:NewUserUsecases
```

### Impact

This limitation affects:

* impact analysis
* execution flow reconstruction
* dependency tracing
* architectural understanding

Most real-world repositories rely heavily on package boundaries, making this the highest-priority issue.

---

## L2. Interface Method Modeling

### Current State

Interface methods are not represented as first-class graph nodes.

Example:

```go
type UserService interface {
    Login()
    Register()
}
```

Only the interface may be indexed.

Individual methods are not.

### Impact

Lea cannot reason about:

* interface contracts
* implementation mappings
* dependency inversion
* architectural boundaries

---

## L3. Symbol Discovery

### Current State

Users must already know the symbol identifier.

Example:

```bash
lea flow method:internal/auth:Service.Login
```

There is currently no official mechanism for discovering available symbols.

Users often resort to:

```bash
sqlite3 .lea/graph.db
```

or:

```bash
grep
```

### Impact

This creates friction for:

* new contributors
* AI agents
* large repositories

---

# Priority Roadmap

## P0 — Complete Cross-Package Resolution

### Goal

Resolve repository-wide symbol references.

Example:

```go
service.New()
repository.Save()
handler.Login()
```

should all be linked to exact graph nodes.

### Expected Result

```text
func:internal/service:New
method:internal/repository:Repository.Save
method:api/http:AuthHandler.Login
```

### Value

This unlocks:

* accurate impact analysis
* repository-wide flow reconstruction
* true blast-radius calculation

This is the most important improvement for Lea.

---

## P1 — Interface Graph Modeling

### Goal

Treat interface methods as first-class symbols.

Example:

```text
interface:internal/auth:AuthService

method:internal/auth:AuthService.Login
method:internal/auth:AuthService.Register
```

### Additional Capability

Link implementations:

```text
AuthService.Login
 └── Service.Login
```

### Value

Enables contract-aware reasoning.

---

## P2 — Symbol Registry

### Goal

Introduce symbol discovery commands.

Example:

```bash
lea symbols
```

```bash
lea symbols auth
```

```bash
lea symbols --kind method
```

### Expected Output

```text
method:internal/auth:Service.Login
method:internal/auth:Service.Register
interface:internal/auth:AuthService
```

### Value

Removes the need for SQLite queries.

---

## P3 — Blast Radius Engine

### Goal

Transform `lea impact` into a repository-wide risk analysis system.

Example:

```bash
lea impact method:internal/auth:Service.Login
```

Expected output:

```text
Direct Callers:
  - AuthHandler.Login

Indirect Callers:
  - Router.RegisterAuthRoutes

Affected Interfaces:
  - AuthService

Affected Tests:
  - internal/auth/service_test.go
```

### Value

Allows safe refactoring at scale.

---

## P4 — Architectural Rule Engine

### Goal

Support repository guardrails.

Example:

```yaml
rules:
  - from: api
    cannot_depend_on: repository

  - from: handler
    cannot_depend_on: database
```

Validation:

```bash
lea violations --against HEAD
```

### Value

Transforms Lea into an architectural governance tool.

---

## P5 — Dynamic Context Compiler

### Goal

Generate context under a strict token budget.

Example:

```bash
lea context method:internal/auth:Service.Login \
  --budget 2000
```

### Budget-Aware Output

Small budget:

```text
- signatures
- dependencies
- execution path
```

Large budget:

```text
- signatures
- execution path
- selected method bodies
- architectural summaries
```

### Value

Makes Lea LLM-native.

---

## P6 — Agent Integration Layer

### Goal

Automatically generate:

```text
.lea/.agent-manifesto.md
```

during indexing.

Purpose:

* teach agents how to use Lea
* standardize workflows
* reduce token waste

### Workflow

```text
Discover -> Lynx
Reason -> Lea
Modify -> Agent
Validate -> Lea
```

---

# Long-Term Vision

Lea should never become a search engine.

That responsibility belongs to Lynx.

The long-term separation of concerns is:

```text
grep
    ↓
files

Semble
    ↓
chunks

Lynx
    ↓
symbols

Lea
    ↓
structure
```

Lea's responsibility is not discovery.

Lea's responsibility is deterministic structural reasoning.

Every future feature should strengthen one of the following capabilities:

1. Structural Graph Navigation
2. Execution Flow Reconstruction
3. Dependency Analysis
4. Blast Radius Calculation
5. Architectural Governance
6. Token-Efficient Context Assembly

Any feature outside these domains should be considered out-of-scope and delegated to Lynx or other ecosystem components.

---

# Recommended Development Order

```text
P0  Cross-Package Resolution
P1  Interface Graph Modeling
P2  Symbol Registry
P3  Blast Radius Engine
P4  Architectural Rule Engine
P5  Dynamic Context Compiler
P6  Agent Integration Layer
```

Completing P0–P3 alone would significantly increase Lea's value and establish it as a genuine Structural Reasoning Engine rather than a repository navigation utility.
