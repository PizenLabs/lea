# AGENTS.md

> **CRITICAL:** THIS REPOSITORY IS STRICTLY GOVERNED BY THE PIZENLABS DUAL-ENGINE SECOND BRAIN PROTOCOL.
> YOU ARE ABSOLUTELY FORBIDDEN FROM MAKING ANY BLIND SEARCHES (`GLOB`, `GREP`), RAW FILE READS, OR CODE EDITS UNTIL YOU COMPLY WITH THE ECOSYSTEM LIFECYCLE BELOW.

---

## INITIALIZATION LIFECYCLE (MANDATORY FIRST ACTIONS)

Before executing any business logic or making any architectural assumptions, you **MUST** sequentially call your native read tool to parse the following core state configurations inside `.lea/`:

1. **`.lea/protocol.json`**

   * Ingest the `"strict_runtime_rules"` and `"tool_execution_adapter"`.
   * Treat them as unchangeable runtime constants.
   * **CRITICAL BINDING:** You **MUST** execute all ecosystem subcommands through your native `bash` tool using the exact format: `bash -c '<command>'`.

2. **`.lea/workspace.json`**

   * Ingest the immutable repository facts, languages, frameworks, and graph stats to understand the project scale.

3. **`.lea/intent.json` & `.lea/limitations.json`**

   * Ingest current human architectural boundaries, unsupported scopes, and confidence limitations (especially regarding dynamic dispatch).

4. **`.lea/memory.json`**

   * Inherit historical failures, hotspots, and successful patterns to avoid repeating past development bugs.

---

## MANDATORY EXECUTION PIPELINE (NEVER BYPASS)

Every single task requiring file exploration or modification **MUST** navigate through the strict multi-phase pipeline defined in `protocol.json`:

### PHASE 1: DISCOVER (via `pizen-lynx` engine)

You **MUST** invoke discovery commands via bash first to retrieve coordinate states:

* `bash -c 'lx search <query>'` — Discover candidate symbols/files by intent.
* `bash -c 'lx resolve <name>'` — Resolve a symbol to stable file coordinates.
* *Zero modifications or raw file reads are allowed before this phase provides valid coordinates.*

### PHASE 2: REASON (via `pizen-lea` engine)

Once coordinates are resolved, you **MUST** pipe them into the graph reasoning layer via bash to map structural impact and execution flows:

* `bash -c 'lea context <symbol>'` — Compile token-budgeted context blocks for the target symbol.
* `bash -c 'lea flow <symbol>'` — Generate ordered control-flow trace from the target symbol.
* `bash -c 'lea impact <symbol>'` — Execute recursive blast-radius impact analysis.

### PHASE 3: ARCHITECTURE GUARD

Before completing any task or declaring success, you **MUST** validate project boundaries:

* `bash -c 'lea violations'`
* Ensure that all your changes comply strictly with the boundary constraints and return zero violations.

---

## HARD BOUNDARIES & ABORT CONDITIONS

* **Abort Trigger:** If you hit **3 consecutive failed edits**, you **MUST** immediately halt all file mutations.

* **Recovery Procedure:** Run the fallback sequence through your native bash tool:

  1. `bash -c 'lx resolve <failed_symbol>'`
  2. `bash -c 'lea impact <failed_symbol>'`
  3. **Request human intervention** as specified in `protocol.json`.

> **CONSEQUENCE:** Failure to execute this exact setup loop and orchestration pipeline as your **FIRST** tool actions will break repository integrity and result in immediate session rejection.
