
# Lea Agent Integration Review

## Objective

Improve AI-agent compliance, workflow consistency, and structural reasoning effectiveness across all supported agents.

The goal is not to explain Lea to humans.

The goal is to make AI agents reliably follow the intended workflow with minimal prompt overhead and minimal hallucination.

---

# Current Assessment

Current `.lea` metadata is functional but not optimized for agent consumption.

Several files contain duplicated concepts:

* AGENT.md
* MANIFEST.md
* WORKSPACE.md
* protocol.json

This increases token usage while providing little additional execution value.

Most agents do not need philosophy.

Most agents need:

1. Workflow order
2. Tool commands
3. Recovery rules
4. Validation rules

Anything beyond that is often ignored.

---

# Recommended .lea Structure

```text
.lea/
├── AGENT.md
├── WORKSPACE.md
├── protocol.json
└── graph.db
```

Remove:

```text
MANIFEST.md
```

Reason:

The information currently stored inside MANIFEST.md overlaps heavily with AGENT.md and protocol.json.

It adds token cost without improving execution quality.

---

# AGENT.md Responsibilities

AGENT.md should contain only operational instructions.

Purpose:

Tell the agent HOW to work.

It should not explain:

* Lea philosophy
* Internal implementation
* Database details
* Ecosystem history

Recommended contents:

* Authority order
* Workflow
* Tool usage
* Loop recovery
* Verification requirements

Maximum target:

150–250 lines

Never exceed 400 lines.

---

# WORKSPACE.md Responsibilities

WORKSPACE.md should contain repository-specific facts only.

Purpose:

Tell the agent WHAT environment it is operating in.

Examples:

* Languages
* Frameworks
* Build commands
* Test commands
* Repository conventions
* Important architecture boundaries

Avoid:

* Statistics
* Node counts
* Edge counts
* Graph counts
* Internal implementation details

Bad:

```text
Node Count: 188
Edge Count: 1221
```

Good:

```text
Primary Language: Go

Architecture:

cmd/
internal/
pkg/

Testing:

go test ./...
```

Agents rarely use repository statistics when generating code.

---

# protocol.json Responsibilities

protocol.json should be machine-oriented.

Purpose:

Provide deterministic workflow rules.

Recommended example:

{
"workflow": [
"discover",
"analyze",
"plan",
"execute",
"verify"
],
"rules": {
"graph_first": true,
"verify_before_finalize": true,
"stop_on_repeated_failure": true,
"max_context_tokens": 4000
}
}

Keep protocol.json extremely small.

Do not store documentation inside it.

---

# Recommended AGENT Workflow

Agents understand short workflows better than long explanations.

Preferred workflow:

Discover
Analyze
Plan
Execute
Verify

Definitions:

Discover

* Locate symbols
* Locate files
* Resolve entry points

Analyze

* Impact analysis
* Dependency analysis
* Flow analysis

Plan

* Goal
* Constraints
* Acceptance Criteria

Execute

* Smallest safe diff

Verify

* Validation
* Architecture checks

---

# Tool Guidance

Agents should be told exactly when to use tools.

Recommended:

Discovery

```
lx search "<query>"
```

Analysis

```
lea impact <symbol>
```

Context

```
lea context <symbol> --budget 1500
```

Flow

```
lea flow <symbol>
```

Validation

```
lea violations
```

Avoid tool descriptions longer than one sentence.

Agents perform better with concise command mappings.

---

# Runtime Verification

Every exported agent configuration should include:

Verify tool availability before relying on a workflow.

Examples:

```
lx --help
lea --help
```

If unavailable:

* State it explicitly
* Fall back to alternative reasoning
* Never claim tool output

This significantly reduces hallucinated tool usage.

---

# Export Strategy

Current exports are too passive.

Many exports only say:

"Refer to .lea/AGENT.md"

This causes agents to ignore the workflow.

Exports should contain:

1. Workflow summary
2. Tool verification rule
3. Reference to .lea files

Nothing more.

---

# Aider Export

Recommended file:

AIDER.md

Contents:

* Authority
* Workflow
* Tool verification
* Small diff rule
* Recovery rule

Do not include:

* Statistics
* Philosophy
* Internal architecture descriptions

Target size:

100–200 lines

---

# Pi Export

Current Pi export is too weak.

Current:

```text
Refer to .lea/AGENT.md
```

Recommended:

* Verify tools
* Follow workflow
* Use structural analysis before modification
* Reference .lea files

Pi performs better when given direct operational instructions.

---

# Claude Export

CLAUDE.md should include:

* Workflow summary
* Verification rule
* Pointer to .lea files

Claude already performs strong planning.

Avoid over-instruction.

---

# Gemini Export

GEMINI.md should be almost identical to CLAUDE.md.

Gemini benefits from concise workflows.

Avoid long protocol explanations.

---

# Cursor Export

Cursor rules should focus on:

* Discover
* Analyze
* Execute
* Verify

Cursor often skips planning if instructions are too verbose.

Keep rules short.

---

# OpenCode / Codex / Continue

Use the same minimal export template.

Consistency is more important than customization.

---

# Future Improvement

Long-term, Lea should expose workflow commands directly.

Instead of:

```
lx search
lea impact
lea context
lea violations
```

Prefer:

```
lea discover
lea analyze
lea plan
lea verify
```

Agents understand workflow-oriented commands more reliably than implementation-oriented commands.

This reduces prompt complexity and improves compliance across all agent ecosystems.

---

# Final Recommendation

Focus on:

1. Simpler metadata
2. Smaller exports
3. Stronger operational instructions
4. Runtime verification
5. Workflow consistency

Do not optimize for human readability.

Optimize for agent compliance.
