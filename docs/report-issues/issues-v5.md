# Technical Investigation Report: AST Type Resolution and Graph Edge Disconnection in Lea

## 1. Executive Summary
During validation testing with a multi-layered reference codebase, the core symbol indexing functionality of Lea operated successfully. However, the graph engine failed to establish critical structural relationships, specifically `CALLS` dependencies within deep expressions and implicit `IMPLEMENTS` interface boundaries. This report outlines the root causes found within the SQLite schema state and proposes deterministic software engineering solutions to fix the graph connectivity.

---

## 2. Identified Issues & Root Cause Analysis

### Issue 1: Broken Call Graph Due to Unresolved Identifiers (`unknown` prefix)
* **Symptom:** In the `edges` table, certain function invocation targets are stored with an `unknown:` prefix (e.g., `unknown:variableName.MethodName`) instead of mapping directly to the actual method declaration.
* **Root Cause:** When the Abstract Syntax Tree (AST) parser processes the application entry point, it hits external module invocations. Because the local scoping context does not retain the type metadata evaluated from the assignment operator (e.g., `variableName := package.NewConstructor()`), the parser falls back to a literal string extraction, leading to a disconnected graph node.

### Issue 2: Failure to Capture Deep Selector Expressions (Nested Calls)
* **Symptom:** Critical internal boundary invocations made through struct fields or interface properties are missing from the graph entirely, while standard standard library calls (like `fmt.Println`) are recorded.
* **Root Cause:** The AST traversal layer lacks recursive handling for nested pointer/interface selectors (`*ast.SelectorExpr`). When evaluating a chained statement like `s.repositoryField.Update()`, the parser inspects the expression as a single unit but fails to track down the underlying data type of the struct field, thus dropping the method execution node.

### Issue 3: Missing Interface Implementation Mapping (`IMPLEMENTS` Edge)
* **Symptom:** No `IMPLEMENTS` edge types exist within the SQLite graph database dump, resulting in empty outputs when querying for structural neighbors or dependents of a concrete structure.
* **Root Cause:** Go models interfaces implicitly. The current pipeline lacks a deferred structural sub-typing resolver. There is no background matching engine that verifies whether the method set declared on a concrete structure fully satisfies the method set of an indexed interface.

---

## 3. Proposed Technical Solutions

### Solution 1: Implement Local Scope Type Inference
Enhance the parser to build a local object-type symbol table during a single-file walk.
* When encountering an assignment `obj := package.NewConstructor()`, resolve the constructor's return type identifier.
* Store the map of `obj -> full_package_path:StructName` in memory.
* When parsing an `*ast.CallExpr`, check the local table to substitute the `unknown:obj.Method` format with the deterministic URI schema: `method:full_package_path:StructName.Method`.

### Solution 2: Recursive Selector Resolution for Chained Struct Fields
Refactor the AST visitor layer to flatten deep selector expressions recursively. Use the following algorithm pattern to peel layers off complex selectors:

```go
func ResolveSelectorType(expr ast.Expr, structRegistry TypeRegistry) string {
    switch v := expr.(type) {
    case *ast.SelectorExpr:
        // Recursively evaluate the base (X), e.g., 's.repo'
        baseType := ResolveSelectorType(v.X, structRegistry)
        // Look up the field 'Sel.Name' (e.g., 'repo') within 'baseType' structure
        return structRegistry.LookupFieldType(baseType, v.Sel.Name)
    case *ast.Ident:
        return v.Name // Base variable identifier, e.g., 's'
    }
    return "unknown"
}

```

### Solution 3: Deferred Interface Resolution Engine (Post-Index Phase)

Introduce a two-pass indexing architecture:

1. **Pass 1 (Extraction):** Scan all files and populate raw `nodes` (structs, interfaces, methods) independently into SQLite.
2. **Pass 2 (Linking):** Execute a deferred global query matching structural signatures.
* Fetch all interfaces and their defined method signatures.
* Fetch all structs and aggregate their associated pointer/value receiver methods.
* If `Set(Struct.Methods) ⊇ Set(Interface.Methods)`, execute an atomic transaction to inject the missing relationship:
```sql
INSERT INTO edges (source, target, type) VALUES ('struct_id', 'interface_id', 'IMPLEMENTS');

```
### Solution 4: Normalize Command-Line Identifier Inputs

Enforce strict URI schema parsing at the CLI handler level. When a user runs a graph query (such as `trace` or `impact`), the application must cross-check the user input against both the `func:` and `method:` prefixes, or dynamically query the database using `LIKE` wildcards to safely map natural language strings to the correct database key.

NOTE: path project testing ./testdata/ 
