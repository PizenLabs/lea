// Package parser provides call graph extraction from Go source files.
package parser

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	graph "github.com/PizenLabs/lea/internal/graph/contracts"
)

// CallParser extracts call graph edges and control flow edges from Go source files.
type CallParser struct {
	fset    *token.FileSet
	reg     *TypeRegistry
	pkgPath string
	imports map[string]string
	edges   []*graph.Edge
	order   int
}

// NewCallParser creates a new call parser.
func NewCallParser() *CallParser {
	return &CallParser{
		fset: token.NewFileSet(),
	}
}

// SetTypeRegistry attaches a TypeRegistry for canonical type resolution.
func (cp *CallParser) SetTypeRegistry(reg *TypeRegistry) {
	cp.reg = reg
}

// ExtractCalls extracts CALLS edges from a Go source file.
func (cp *CallParser) ExtractCalls(_ context.Context, path string) ([]*graph.Edge, error) {
	f, err := parser.ParseFile(cp.fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	cp.pkgPath = filepath.Dir(path)
	cp.imports = extractImports(f)
	cp.edges = nil

	var currentFunc string

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			currentFunc = cp.funcID(x, cp.pkgPath)
			// Track receiver variable for deep selector resolution
			if x.Recv != nil {
				if recvType := getReceiverType(x.Recv); recvType != "" {
					if len(x.Recv.List) > 0 && len(x.Recv.List[0].Names) > 0 {
						recvName := x.Recv.List[0].Names[0].Name
						if cp.reg != nil {
							cp.reg.TrackReceiverType(recvName, cp.pkgPath, recvType)
						}
					}
				}
			}

		case *ast.GenDecl:
			// Track variable assignments to build local type table
			if cp.reg != nil {
				for _, spec := range x.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok || len(vs.Names) == 0 || vs.Type != nil {
						continue
					}
					for _, val := range vs.Values {
						cp.trackAssignmentVar(val, vs.Names[0].Name)
					}
				}
			}

		case *ast.TypeSpec:
			// Register struct types and their fields for deep selector resolution
			if cp.reg != nil {
				if st, ok := x.Type.(*ast.StructType); ok {
					typeName := x.Name.Name
					var fields []StructFieldInfo
					for _, field := range st.Fields.List {
						if len(field.Names) == 0 {
							continue
						}
						fieldName := field.Names[0].Name
						fieldTypeStr := selectorChainString(field.Type)
						fields = append(fields, StructFieldInfo{
							FieldName: fieldName,
							FieldType: fieldTypeStr,
						})
					}
					cp.reg.RegisterStruct(cp.pkgPath, typeName, fields)
				}
			}

		case *ast.AssignStmt:
			// Track short variable declarations like repo := repository.NewInMem()
			if cp.reg != nil && x.Tok == token.DEFINE && len(x.Lhs) == 1 && len(x.Rhs) == 1 {
				if ident, ok := x.Lhs[0].(*ast.Ident); ok {
					cp.trackAssignmentVar(x.Rhs[0], ident.Name)
				}
			}

		case *ast.CallExpr:
			if currentFunc == "" {
				return true
			}
			if targetID := cp.resolveCallExpr(x); targetID != "" {
				cp.edges = append(cp.edges, &graph.Edge{
					FromID: currentFunc,
					ToID:   targetID,
					Type:   graph.EdgeCalls,
				})
			}
		}
		return true
	})

	return cp.edges, nil
}

// ExtractControlFlow extracts FLOWS_THROUGH edges with ordering from a Go source file.
func (cp *CallParser) ExtractControlFlow(_ context.Context, path string) ([]*graph.Edge, error) {
	f, err := parser.ParseFile(cp.fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	cp.pkgPath = filepath.Dir(path)
	cp.imports = extractImports(f)
	cp.edges = nil
	cp.order = 0

	// Pre-pass: register struct types and top-level variables for deep selector resolution
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.TypeSpec:
			if cp.reg != nil {
				if st, ok := x.Type.(*ast.StructType); ok {
					typeName := x.Name.Name
					var fields []StructFieldInfo
					for _, field := range st.Fields.List {
						if len(field.Names) == 0 {
							continue
						}
						fieldName := field.Names[0].Name
						fieldTypeStr := selectorChainString(field.Type)
						fields = append(fields, StructFieldInfo{
							FieldName: fieldName,
							FieldType: fieldTypeStr,
						})
					}
					cp.reg.RegisterStruct(cp.pkgPath, typeName, fields)
				}
			}
		case *ast.GenDecl:
			if cp.reg != nil {
				for _, spec := range x.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok || len(vs.Names) == 0 || vs.Type != nil {
						continue
					}
					for _, val := range vs.Values {
						cp.trackAssignmentVar(val, vs.Names[0].Name)
					}
				}
			}
		}
		return true
	})

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		currentFunc := cp.funcID(fn, cp.pkgPath)
		walkStmtList(fn.Body.List, cp.pkgPath, currentFunc, &cp.order, nil, cp.imports, cp.reg, cp.fset, &cp.edges)
	}

	return cp.edges, nil
}

// funcID returns the canonical graph node ID for a function declaration.
func (cp *CallParser) funcID(fn *ast.FuncDecl, pkgPath string) string {
	if fn.Recv != nil {
		recvType := getReceiverType(fn.Recv)
		if recvType != "" {
			return fmt.Sprintf("method:%s:%s.%s", pkgPath, recvType, fn.Name.Name)
		}
	}
	return fmt.Sprintf("func:%s:%s", pkgPath, fn.Name.Name)
}

// resolveCallExpr resolves a call expression to a canonical graph node ID.
func (cp *CallParser) resolveCallExpr(ce *ast.CallExpr) string {
	target := getCallTarget(ce)
	if target == "" {
		return ""
	}

	// Use TypeRegistry for canonical resolution
	if cp.reg != nil {
		if methodID := cp.reg.ResolveMethodID(target, cp.imports, cp.pkgPath); methodID != "" {
			return methodID
		}
		return cp.reg.ResolveCallTarget(target, cp.imports, cp.pkgPath)
	}

	// Fallback: simple resolution without registry
	return resolveCallTarget(target, cp.imports, cp.pkgPath)
}

// extractImports builds an alias -> full path map from import declarations.
func extractImports(f *ast.File) map[string]string {
	imports := make(map[string]string)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		} else {
			parts := strings.Split(path, "/")
			name = parts[len(parts)-1]
		}
		imports[name] = path
	}
	return imports
}

// getReceiverType extracts the receiver type name from a field list.
func getReceiverType(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	typ := recv.List[0].Type
	for {
		if star, ok := typ.(*ast.StarExpr); ok {
			typ = star.X
			continue
		}
		break
	}
	if ident, ok := typ.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// getCallTarget extracts the full dot-separated call target from a CallExpr.
func getCallTarget(ce *ast.CallExpr) string {
	return selectorChainString(ce.Fun)
}

// selectorChainString flattens a selector expression chain into a dot-separated string.
func selectorChainString(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		base := selectorChainString(x.X)
		if base == "" {
			return ""
		}
		return base + "." + x.Sel.Name
	case *ast.ParenExpr:
		return selectorChainString(x.X)
	case *ast.StarExpr:
		return selectorChainString(x.X)
	default:
		return ""
	}
}

// trackAssignmentVar records the inferred type of a variable from its right-hand side expression.
// This populates the TypeRegistry's LocalVarTypes table for resolving method calls on variables.
func (cp *CallParser) trackAssignmentVar(rhs ast.Expr, varName string) {
	switch val := rhs.(type) {
	case *ast.CallExpr:
		// varName := NewConstructor() or varName := package.NewConstructor()
		switch fun := val.Fun.(type) {
		case *ast.Ident:
			// NewConstructor() - infer type from function name (strip "New" prefix)
			typeName := strings.TrimPrefix(fun.Name, "New")
			if typeName != "" && typeName != fun.Name {
				cp.reg.LocalVarTypes[varName] = fmt.Sprintf("%s:%s", cp.pkgPath, typeName)
			}
		case *ast.SelectorExpr:
			// package.NewConstructor() - resolve alias to canonical path
			if id, ok := fun.X.(*ast.Ident); ok {
				pkgAlias := id.Name
				typeName := strings.TrimPrefix(fun.Sel.Name, "New")
				if typeName != "" && typeName != fun.Sel.Name {
					// Resolve import alias to canonical package path
					canonPkg := pkgAlias
					if path, ok := cp.imports[pkgAlias]; ok {
						if cp.reg.ModuleName != "" && strings.HasPrefix(path, cp.reg.ModuleName) {
							canonPkg = strings.TrimPrefix(path, cp.reg.ModuleName)
							canonPkg = strings.TrimPrefix(canonPkg, "/")
						} else {
							canonPkg = path
						}
					}
					cp.reg.LocalVarTypes[varName] = fmt.Sprintf("%s:%s", canonPkg, typeName)
				}
			}
		}
	case *ast.UnaryExpr:
		if val.Op == token.AND {
			// varName := &Type{}
			if comp, ok := val.X.(*ast.CompositeLit); ok {
				if t, ok := comp.Type.(*ast.Ident); ok {
					cp.reg.LocalVarTypes[varName] = fmt.Sprintf("%s:%s", cp.pkgPath, t.Name)
				}
				if t, ok := comp.Type.(*ast.SelectorExpr); ok {
					if id, ok := t.X.(*ast.Ident); ok {
						pkgAlias := id.Name
						canonPkg := pkgAlias
						if path, ok := cp.imports[pkgAlias]; ok {
							if cp.reg.ModuleName != "" && strings.HasPrefix(path, cp.reg.ModuleName) {
								canonPkg = strings.TrimPrefix(path, cp.reg.ModuleName)
								canonPkg = strings.TrimPrefix(canonPkg, "/")
							} else {
								canonPkg = path
							}
						}
						cp.reg.LocalVarTypes[varName] = fmt.Sprintf("%s:%s", canonPkg, t.Sel.Name)
					}
				}
			}
		}
	case *ast.CompositeLit:
		// varName := Type{}
		if t, ok := val.Type.(*ast.Ident); ok {
			cp.reg.LocalVarTypes[varName] = fmt.Sprintf("%s:%s", cp.pkgPath, t.Name)
		}
	}
}

// trackAssignVarFromExpr records the inferred type of a variable from its right-hand side expression.
// This is a package-level standalone function for use from walkStmt.
func trackAssignVarFromExpr(rhs ast.Expr, varName string, reg *TypeRegistry, imports map[string]string, pkgPath string) {
	if reg == nil {
		return
	}
	switch val := rhs.(type) {
	case *ast.CallExpr:
		switch fun := val.Fun.(type) {
		case *ast.Ident:
			typeName := strings.TrimPrefix(fun.Name, "New")
			if typeName != "" && typeName != fun.Name {
				reg.LocalVarTypes[varName] = fmt.Sprintf("%s:%s", pkgPath, typeName)
			}
		case *ast.SelectorExpr:
			if id, ok := fun.X.(*ast.Ident); ok {
				pkgAlias := id.Name
				typeName := strings.TrimPrefix(fun.Sel.Name, "New")
				if typeName != "" && typeName != fun.Sel.Name {
					canonPkg := pkgAlias
					if path, ok := imports[pkgAlias]; ok {
						if reg.ModuleName != "" && strings.HasPrefix(path, reg.ModuleName) {
							canonPkg = strings.TrimPrefix(path, reg.ModuleName)
							canonPkg = strings.TrimPrefix(canonPkg, "/")
						} else {
							canonPkg = path
						}
					}
					reg.LocalVarTypes[varName] = fmt.Sprintf("%s:%s", canonPkg, typeName)
				}
			}
		}
	case *ast.UnaryExpr:
		if val.Op == token.AND {
			if comp, ok := val.X.(*ast.CompositeLit); ok {
				if t, ok := comp.Type.(*ast.Ident); ok {
					reg.LocalVarTypes[varName] = fmt.Sprintf("%s:%s", pkgPath, t.Name)
				}
				if t, ok := comp.Type.(*ast.SelectorExpr); ok {
					if id, ok := t.X.(*ast.Ident); ok {
						pkgAlias := id.Name
						canonPkg := pkgAlias
						if path, ok := imports[pkgAlias]; ok {
							if reg.ModuleName != "" && strings.HasPrefix(path, reg.ModuleName) {
								canonPkg = strings.TrimPrefix(path, reg.ModuleName)
								canonPkg = strings.TrimPrefix(canonPkg, "/")
							} else {
								canonPkg = path
							}
						}
						reg.LocalVarTypes[varName] = fmt.Sprintf("%s:%s", canonPkg, t.Sel.Name)
					}
				}
			}
		}
	case *ast.CompositeLit:
		if t, ok := val.Type.(*ast.Ident); ok {
			reg.LocalVarTypes[varName] = fmt.Sprintf("%s:%s", pkgPath, t.Name)
		}
	}
}

// resolveCallTarget is a fallback resolution without TypeRegistry.
func resolveCallTarget(target string, imports map[string]string, pkgPath string) string {
	// Handle built-in functions (no dot, no import resolution needed)
	if !strings.Contains(target, ".") {
		if isBuiltinFunc(target) {
			return fmt.Sprintf("stdlib:builtin:%s", target)
		}
		return fmt.Sprintf("func:%s:%s", pkgPath, target)
	}
	parts := strings.SplitN(target, ".", 2)
	if path, ok := imports[parts[0]]; ok {
		// Go standard library package (fmt, os, context, etc.)
		if isStdlibImport(path) {
			return fmt.Sprintf("stdlib:%s:%s", path, parts[1])
		}
		return fmt.Sprintf("func:%s:%s", path, parts[1])
	}
	return fmt.Sprintf("unknown:%s", target)
}

// walkStmtList walks a list of statements to collect control flow edges.
func walkStmtList(stmts []ast.Stmt, pkgPath, currentFunc string, order *int, ctxStack []flowContext, imports map[string]string, reg *TypeRegistry, fset *token.FileSet, edges *[]*graph.Edge) {
	for _, stmt := range stmts {
		walkStmt(stmt, pkgPath, currentFunc, order, ctxStack, imports, reg, fset, edges)
	}
}

type flowContext struct {
	kind      string
	condition string
}

func walkStmt(stmt ast.Stmt, pkgPath, currentFunc string, order *int, ctxStack []flowContext, imports map[string]string, reg *TypeRegistry, fset *token.FileSet, edges *[]*graph.Edge) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		// Track short variable declarations like calc := &Calculator{}
		if reg != nil && s.Tok == token.DEFINE && len(s.Lhs) == 1 && len(s.Rhs) == 1 {
			if ident, ok := s.Lhs[0].(*ast.Ident); ok {
				trackAssignVarFromExpr(s.Rhs[0], ident.Name, reg, imports, pkgPath)
			}
		}
		collectCallsFromNode(s, pkgPath, currentFunc, order, ctxStack, imports, reg, fset, edges)
	case *ast.BlockStmt:
		walkStmtList(s.List, pkgPath, currentFunc, order, ctxStack, imports, reg, fset, edges)
	case *ast.IfStmt:
		ctx := append(ctxStack, flowContext{kind: "if", condition: exprString(s.Cond, fset)})
		collectCallsFromNode(s.Init, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		collectCallsFromNode(s.Cond, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		walkStmtList(s.Body.List, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		if s.Else != nil {
			walkStmt(s.Else, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		}
	case *ast.ForStmt:
		ctx := append(ctxStack, flowContext{kind: "for", condition: exprString(s.Cond, fset)})
		collectCallsFromNode(s.Init, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		collectCallsFromNode(s.Cond, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		collectCallsFromNode(s.Post, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		walkStmtList(s.Body.List, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
	case *ast.RangeStmt:
		ctx := append(ctxStack, flowContext{kind: "range", condition: exprString(s.X, fset)})
		collectCallsFromNode(s.X, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		walkStmtList(s.Body.List, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
	case *ast.SwitchStmt:
		ctx := append(ctxStack, flowContext{kind: "switch", condition: exprString(s.Tag, fset)})
		collectCallsFromNode(s.Init, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		collectCallsFromNode(s.Tag, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		for _, stmt := range s.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			walkStmtList(clause.Body, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		}
	case *ast.TypeSwitchStmt:
		ctx := append(ctxStack, flowContext{kind: "type-switch"})
		collectCallsFromNode(s.Init, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		collectCallsFromNode(s.Assign, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		for _, stmt := range s.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			walkStmtList(clause.Body, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		}
	case *ast.SelectStmt:
		ctx := append(ctxStack, flowContext{kind: "select"})
		for _, stmt := range s.Body.List {
			clause, ok := stmt.(*ast.CommClause)
			if !ok {
				continue
			}
			walkStmtList(clause.Body, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
		}
	case *ast.DeferStmt:
		ctx := append(ctxStack, flowContext{kind: "defer"})
		collectCallsFromNode(s.Call, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
	case *ast.GoStmt:
		ctx := append(ctxStack, flowContext{kind: "go"})
		collectCallsFromNode(s.Call, pkgPath, currentFunc, order, ctx, imports, reg, fset, edges)
	default:
		collectCallsFromNode(stmt, pkgPath, currentFunc, order, ctxStack, imports, reg, fset, edges)
	}
}

func collectCallsFromNode(node ast.Node, pkgPath, currentFunc string, order *int, ctxStack []flowContext, imports map[string]string, reg *TypeRegistry, _ *token.FileSet, edges *[]*graph.Edge) {
	if node == nil || currentFunc == "" {
		return
	}
	ast.Inspect(node, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		target := getCallTarget(callExpr)
		if target == "" {
			return true
		}

		var targetID string
		if reg != nil {
			if methodID := reg.ResolveMethodID(target, imports, pkgPath); methodID != "" {
				targetID = methodID
			} else {
				targetID = reg.ResolveCallTarget(target, imports, pkgPath)
			}
		} else {
			targetID = resolveCallTarget(target, imports, pkgPath)
		}
		if targetID == "" {
			return true
		}

		*order++
		seq := *order
		metadata := map[string]interface{}{
			"order":   seq,
			"context": contextString(ctxStack),
			"target":  target,
		}

		*edges = append(*edges, &graph.Edge{
			FromID:   currentFunc,
			ToID:     targetID,
			Type:     graph.EdgeFlowsThrough,
			Sequence: seq,
			Metadata: metadata,
		})
		return true
	})
}

func contextString(ctxStack []flowContext) string {
	if len(ctxStack) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ctxStack))
	for _, ctx := range ctxStack {
		if ctx.condition != "" {
			parts = append(parts, fmt.Sprintf("%s(%s)", ctx.kind, ctx.condition))
		} else {
			parts = append(parts, ctx.kind)
		}
	}
	return strings.Join(parts, " > ")
}

func exprString(expr ast.Expr, fset *token.FileSet) string {
	if expr == nil || fset == nil {
		return ""
	}
	return fmt.Sprintf("%s", expr)
}
