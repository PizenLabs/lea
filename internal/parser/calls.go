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
			// Track receiver variable
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

// resolveCallTarget is a fallback resolution without TypeRegistry.
func resolveCallTarget(target string, imports map[string]string, pkgPath string) string {
	if !strings.Contains(target, ".") {
		return fmt.Sprintf("func:%s:%s", pkgPath, target)
	}
	parts := strings.SplitN(target, ".", 2)
	if path, ok := imports[parts[0]]; ok {
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

func collectCallsFromNode(node ast.Node, pkgPath, currentFunc string, order *int, ctxStack []flowContext, imports map[string]string, reg *TypeRegistry, fset *token.FileSet, edges *[]*graph.Edge) {
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