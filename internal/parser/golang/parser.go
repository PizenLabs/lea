// Package golang provides a Go source parser for graph extraction.
package golang

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	graph "github.com/PizenLabs/lea/internal/graph/contracts"
)

// builtinFuncs is the set of Go built-in functions that should be labeled as stdlib:builtin:<name>.
var builtinFuncs = map[string]bool{
	"make": true, "new": true, "panic": true, "append": true,
	"len": true, "cap": true, "delete": true, "close": true,
	"copy": true, "print": true, "println": true, "recover": true,
	"complex": true, "real": true, "imag": true,
}

// isBuiltinFunc returns true if name is a Go built-in function.
func isBuiltinFunc(name string) bool {
	return builtinFuncs[name]
}

// isStdlibImport returns true if importPath belongs to Go's standard library.
// Paths starting with moduleName are internal module paths, not stdlib.
func isStdlibImport(importPath, moduleName string) bool {
	if importPath == "" {
		return false
	}
	if moduleName != "" && strings.HasPrefix(importPath, moduleName) {
		return false
	}
	firstSeg := importPath
	if idx := strings.Index(importPath, "/"); idx >= 0 {
		firstSeg = importPath[:idx]
	}
	return !strings.Contains(firstSeg, ".")
}

// isInternalModulePath checks if the import path belongs to the current module.
func isInternalModulePath(path, moduleName string) bool {
	return moduleName != "" && strings.HasPrefix(path, moduleName)
}

// StructFieldInfo holds the type information for a struct field.
type StructFieldInfo struct {
	FieldName string // The name of the field
	FieldType string // The resolved type name (short name, e.g., "WalletRepository")
}

// StructInfo holds type information for a parsed struct.
type StructInfo struct {
	PkgPath string
	Name    string
	Fields  []StructFieldInfo
}

// Parser parses Go source files into graph nodes and edges.
type Parser struct {
	fset            *token.FileSet
	moduleName      string
	moduleRoot      string                 // Absolute path to module root for computing canonical package paths
	structIndex     map[string]*StructInfo // key: "pkgPath:StructName"
	localVarTypes   map[string]string      // key: varName -> "pkgPath:TypeName" (per-file scope)
	funcReturnTypes map[string]string      // key: "pkgPath:FuncName" -> "TypeName" (constructor return types)
}

// NewParser creates a new Go source parser.
func NewParser() *Parser {
	return &Parser{
		fset:            token.NewFileSet(),
		funcReturnTypes: make(map[string]string),
	}
}

// SetModuleName sets the module name for internal package resolution.
func (p *Parser) SetModuleName(name string) {
	p.moduleName = name
}

// SetRootPath sets the module root directory for computing canonical package paths.
// This ensures nodes are stored with module-relative paths (e.g., "cmd/server")
// instead of absolute filesystem paths.
func (p *Parser) SetRootPath(root string) {
	p.moduleRoot = root
}

// canonicalPkgPath computes the canonical package path from a file path.
// If moduleRoot is set, returns the module-relative directory path.
// Otherwise falls back to the filesystem directory path.
func (p *Parser) canonicalPkgPath(filePath string) string {
	if p.moduleRoot != "" {
		rel, err := filepath.Rel(p.moduleRoot, filePath)
		if err == nil {
			return filepath.Dir(rel)
		}
	}
	return filepath.Dir(filePath)
}

func (p *Parser) extractImports(f *ast.File) map[string]string {
	imports := make(map[string]string)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		} else {
			// Default name is the last part of the path
			parts := strings.Split(path, "/")
			name = parts[len(parts)-1]
		}
		imports[name] = path
	}
	return imports
}

// ParseFile extracts symbol nodes and edges from a Go source file.
func (p *Parser) ParseFile(_ context.Context, path string) ([]*graph.Node, []*graph.Edge, error) {
	f, err := parser.ParseFile(p.fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	var nodes []*graph.Node
	var edges []*graph.Edge

	// Reset per-file local type tracking
	p.localVarTypes = make(map[string]string)

	// Extract imports for alias resolution during var tracking
	imports := p.extractImports(f)

	// Use directory as package path for now
	pkgPath := p.canonicalPkgPath(path)
	pkgID := fmt.Sprintf("pkg:%s", pkgPath)

	nodes = append(nodes, &graph.Node{
		ID:   pkgID,
		Type: graph.NodePackage,
		Name: f.Name.Name,
		File: pkgPath,
	})

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			funcName := x.Name.Name
			nodeType := graph.NodeFunction
			id := fmt.Sprintf("func:%s:%s", pkgPath, funcName)

			// Register constructor return type for call-site resolution
			p.registerFuncReturnType(x, pkgPath)

			if x.Recv != nil {
				nodeType = graph.NodeMethod
				recvType := p.getReceiverType(x.Recv)
				if recvType != "" {
					id = fmt.Sprintf("method:%s:%s.%s", pkgPath, recvType, funcName)
					// Add BELONGS_TO edge from method to its receiver struct/interface
					edges = append(edges, &graph.Edge{
						FromID: id,
						ToID:   fmt.Sprintf("type:%s:%s", pkgPath, recvType),
						Type:   graph.EdgeBelongsTo,
					})

					// Track receiver variable name in local type table for deep selector resolution
					// e.g., func (s *PaymentService) ProcessDeposit() -> localVarTypes["s"] = "pkgPath:PaymentService"
					if len(x.Recv.List) > 0 && len(x.Recv.List[0].Names) > 0 {
						recvName := x.Recv.List[0].Names[0].Name
						p.localVarTypes[recvName] = fmt.Sprintf("%s:%s", pkgPath, recvType)
					}
				}
			}

			nodes = append(nodes, &graph.Node{
				ID:   id,
				Type: nodeType,
				Name: funcName,
				File: path,
				Line: p.fset.Position(x.Pos()).Line,
			})

			if x.Recv == nil {
				edges = append(edges, &graph.Edge{
					FromID: id,
					ToID:   pkgID,
					Type:   graph.EdgeBelongsTo,
				})
			}

		case *ast.TypeSpec:
			typeName := x.Name.Name
			var nodeType graph.NodeType
			var it *ast.InterfaceType
			var st *ast.StructType
			switch t := x.Type.(type) {
			case *ast.StructType:
				nodeType = graph.NodeStruct
				st = t
			case *ast.InterfaceType:
				nodeType = graph.NodeInterface
				it = t
			default:
				return true
			}

			id := fmt.Sprintf("type:%s:%s", pkgPath, typeName)
			nodes = append(nodes, &graph.Node{
				ID:   id,
				Type: nodeType,
				Name: typeName,
				File: path,
				Line: p.fset.Position(x.Pos()).Line,
			})

			edges = append(edges, &graph.Edge{
				FromID: id,
				ToID:   pkgID,
				Type:   graph.EdgeBelongsTo,
			})

			// Register struct fields for deep selector resolution (Issue 2 fix)
			if st != nil {
				structKey := fmt.Sprintf("%s:%s", pkgPath, typeName)
				if p.structIndex == nil {
					p.structIndex = make(map[string]*StructInfo)
				}
				si := &StructInfo{PkgPath: pkgPath, Name: typeName}
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 {
						continue
					}
					fieldName := field.Names[0].Name
					fieldTypeStr := p.exprTypeName(field.Type)
					si.Fields = append(si.Fields, StructFieldInfo{
						FieldName: fieldName,
						FieldType: fieldTypeStr,
					})
				}
				p.structIndex[structKey] = si
			}

			if it != nil {
				for _, method := range it.Methods.List {
					if len(method.Names) > 0 {
						methodName := method.Names[0].Name
						methodID := fmt.Sprintf("method:%s:%s.%s", pkgPath, typeName, methodName)
						nodes = append(nodes, &graph.Node{
							ID:   methodID,
							Type: graph.NodeMethod,
							Name: methodName,
							File: path,
							Line: p.fset.Position(method.Pos()).Line,
						})
						edges = append(edges, &graph.Edge{
							FromID: methodID,
							ToID:   id,
							Type:   graph.EdgeBelongsTo,
						})
					}
				}
			}

		case *ast.GenDecl:
			// Track variable assignments to build local type table (Issue 1 fix)
			for _, spec := range x.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Names) == 0 || vs.Type != nil {
					continue
				}
				for _, val := range vs.Values {
					p.trackAssignmentVar(val, vs.Names[0].Name, pkgPath, imports)
				}
			}

		case *ast.AssignStmt:
			// Track short variable declarations like calc := &Calculator{}
			if x.Tok == token.DEFINE && len(x.Lhs) == 1 && len(x.Rhs) == 1 {
				if ident, ok := x.Lhs[0].(*ast.Ident); ok {
					p.trackAssignmentVar(x.Rhs[0], ident.Name, pkgPath, imports)
				}
			}
		}
		return true
	})

	return nodes, edges, nil
}

func (p *Parser) getReceiverType(recv *ast.FieldList) string {
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

// exprTypeName extracts the string representation of a type expression,
// e.g., *ast.Ident -> name, *ast.StarExpr -> *Inner, *ast.SelectorExpr -> pkg.Type
func (p *Parser) exprTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + p.exprTypeName(t.X)
	case *ast.SelectorExpr:
		return p.exprTypeName(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + p.exprTypeName(t.Elt)
	case *ast.MapType:
		return "map[" + p.exprTypeName(t.Key) + "]" + p.exprTypeName(t.Value)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// registerFuncReturnType captures the return type of a function (typically a constructor)
// so that call-site resolution can use the exact concrete type name instead of inferring it
// from the function name (e.g., NewPayment returning *PaymentService rather than "Payment").
func (p *Parser) registerFuncReturnType(fn *ast.FuncDecl, pkgPath string) {
	if fn == nil || fn.Type == nil || fn.Type.Results == nil {
		return
	}
	results := fn.Type.Results.List
	if len(results) != 1 {
		return
	}
	// Unwrap pointer indirection to get the base type name
	typ := results[0].Type
	for {
		star, ok := typ.(*ast.StarExpr)
		if !ok {
			break
		}
		typ = star.X
	}
	if ident, ok := typ.(*ast.Ident); ok {
		if p.funcReturnTypes == nil {
			p.funcReturnTypes = make(map[string]string)
		}
		key := fmt.Sprintf("%s:%s", pkgPath, fn.Name.Name)
		p.funcReturnTypes[key] = ident.Name
	}
}

// trackAssignmentVar records the inferred type of a variable from its right-hand side expression.
// This builds the local type scope table for resolving method calls on variables (Issue 1 fix).
// When imports are available, import aliases are expanded to canonical package paths,
// and registered constructor return types are used instead of name-based inference.
func (p *Parser) trackAssignmentVar(rhs ast.Expr, varName string, pkgPath string, imports map[string]string) {
	if p.localVarTypes == nil {
		p.localVarTypes = make(map[string]string)
	}
	switch val := rhs.(type) {
	case *ast.CallExpr:
		// varName := NewConstructor() or varName := package.NewConstructor()
		switch fun := val.Fun.(type) {
		case *ast.Ident:
			// NewConstructor() - infer type from function name (strip "New" prefix)
			typeName := strings.TrimPrefix(fun.Name, "New")
			if typeName != "" && typeName != fun.Name {
				// Look up registered return type for precise type name
				funcKey := fmt.Sprintf("%s:%s", pkgPath, fun.Name)
				if registeredType, ok := p.funcReturnTypes[funcKey]; ok {
					typeName = registeredType
				}
				p.localVarTypes[varName] = fmt.Sprintf("%s:%s", pkgPath, typeName)
			}
		case *ast.SelectorExpr:
			// package.NewConstructor() - resolve alias, use registered return type
			if id, ok := fun.X.(*ast.Ident); ok {
				pkgAlias := id.Name
				constructorName := fun.Sel.Name

				// First try to look up registered return type before falling back to name inference
				typeName := ""

				// Resolve package alias to canonical path for func key lookup
				canonPkg := pkgAlias
				if path, ok := imports[pkgAlias]; ok {
					if p.moduleName != "" && strings.HasPrefix(path, p.moduleName) {
						canonPkg = strings.TrimPrefix(path, p.moduleName)
						canonPkg = strings.TrimPrefix(canonPkg, "/")
					} else {
						canonPkg = path
					}
				}

				// Look up registered return type for precise type name
				funcKey := fmt.Sprintf("%s:%s", canonPkg, constructorName)
				if registeredType, ok := p.funcReturnTypes[funcKey]; ok {
					typeName = registeredType
				}

				// Fall back to name-based inference if no registered type
				if typeName == "" {
					typeName = strings.TrimPrefix(constructorName, "New")
				}

				if typeName != "" && typeName != constructorName {
					p.localVarTypes[varName] = fmt.Sprintf("%s:%s", canonPkg, typeName)
				}
			}
		}
	case *ast.UnaryExpr:
		if val.Op == token.AND {
			// varName := &Type{}
			if comp, ok := val.X.(*ast.CompositeLit); ok {
				if t, ok := comp.Type.(*ast.Ident); ok {
					p.localVarTypes[varName] = fmt.Sprintf("%s:%s", pkgPath, t.Name)
				}
				if t, ok := comp.Type.(*ast.SelectorExpr); ok {
					if id, ok := t.X.(*ast.Ident); ok {
						pkgAlias := id.Name
						canonPkg := pkgAlias
						if path, ok := imports[pkgAlias]; ok {
							if p.moduleName != "" && strings.HasPrefix(path, p.moduleName) {
								canonPkg = strings.TrimPrefix(path, p.moduleName)
								canonPkg = strings.TrimPrefix(canonPkg, "/")
							} else {
								canonPkg = path
							}
						}
						p.localVarTypes[varName] = fmt.Sprintf("%s:%s", canonPkg, t.Sel.Name)
					}
				}
			}
		}
	case *ast.CompositeLit:
		// varName := Type{}
		if t, ok := val.Type.(*ast.Ident); ok {
			p.localVarTypes[varName] = fmt.Sprintf("%s:%s", pkgPath, t.Name)
		}
	}
}

// resolveID resolves a call target string to a graph node ID.
// Attempts local variable type resolution first (Issue 1 fix), then falls back to
// package import resolution, and finally resorts to "unknown:" prefix.
// When a local variable type key uses an import alias as the package path,
// the alias is resolved through the imports table to produce a canonical path.
func (p *Parser) resolveID(target string, imports map[string]string, pkgPath string) string {
	// Handle built-in functions (no dot, no import resolution needed)
	if !strings.Contains(target, ".") {
		if isBuiltinFunc(target) {
			return fmt.Sprintf("stdlib:builtin:%s", target)
		}
		return fmt.Sprintf("func:%s:%s", pkgPath, target)
	}

	parts := strings.SplitN(target, ".", 2)
	prefix := parts[0]
	name := parts[1]

	// Check local variable type table first (Issue 1 fix)
	if p.localVarTypes != nil {
		if typeKey, ok := p.localVarTypes[prefix]; ok {
			// prefix is a variable name, name is the method
			typeParts := strings.SplitN(typeKey, ":", 2)
			if len(typeParts) == 2 {
				pkgPart := typeParts[0]
				typeNamePart := typeParts[1]

				// If the package part is an import alias, resolve it to canonical path
				if canonicalPath, ok := imports[pkgPart]; ok {
					if p.moduleName != "" && strings.HasPrefix(canonicalPath, p.moduleName) {
						rel := strings.TrimPrefix(canonicalPath, p.moduleName)
						rel = strings.TrimPrefix(rel, "/")
						pkgPart = rel
					} else if isStdlibImport(canonicalPath, p.moduleName) {
						// Keep as-is for stdlib
						pkgPart = canonicalPath
					} else {
						pkgPart = canonicalPath
					}
				}

				// Check if typeNamePart has a package prefix (e.g., "domain.WalletRepository")
				if strings.Contains(typeNamePart, ".") {
					subParts := strings.SplitN(typeNamePart, ".", 2)
					// Try resolving through imports
					if pkgPath2, ok := imports[subParts[0]]; ok {
						if p.moduleName != "" && strings.HasPrefix(pkgPath2, p.moduleName) {
							rel := strings.TrimPrefix(pkgPath2, p.moduleName)
							rel = strings.TrimPrefix(rel, "/")
							return fmt.Sprintf("method:%s:%s.%s", rel, subParts[1], name)
						}
						if isStdlibImport(pkgPath2, p.moduleName) {
							return fmt.Sprintf("method:%s:%s.%s", pkgPath2, subParts[1], name)
						}
						return fmt.Sprintf("method:%s:%s.%s", pkgPath2, subParts[1], name)
					}
					return fmt.Sprintf("method:%s:%s.%s", pkgPart, typeNamePart, name)
				}
				return fmt.Sprintf("method:%s:%s.%s", pkgPart, typeNamePart, name)
			}
			return fmt.Sprintf("unknown:%s.%s", prefix, name)
		}
	}

	if path, ok := imports[prefix]; ok {
		// It's a package call
		if p.moduleName != "" && strings.HasPrefix(path, p.moduleName) {
			// Internal module package
			relPath := strings.TrimPrefix(path, p.moduleName)
			relPath = strings.TrimPrefix(relPath, "/")
			return fmt.Sprintf("func:%s:%s", relPath, name)
		}
		// Go standard library package (fmt, os, context, etc.)
		if isStdlibImport(path, p.moduleName) {
			return fmt.Sprintf("stdlib:%s:%s", path, name)
		}
		// External third-party package
		return fmt.Sprintf("func:%s:%s", path, name)
	}

	// Likely a method call on a variable, still "unknown" but better formatted
	return fmt.Sprintf("unknown:%s", target)
}

// ExtractCalls extracts call graph edges from a Go source file.
func (p *Parser) ExtractCalls(_ context.Context, path string) ([]*graph.Edge, error) {
	f, err := parser.ParseFile(p.fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	// Reset per-file local type tracking
	p.localVarTypes = make(map[string]string)

	var edges []*graph.Edge
	pkgPath := p.canonicalPkgPath(path)
	imports := p.extractImports(f)

	var currentFunc string

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Recv != nil {
				recvType := p.getReceiverType(x.Recv)
				currentFunc = fmt.Sprintf("method:%s:%s.%s", pkgPath, recvType, x.Name.Name)
				// Track receiver variable for deep selector resolution
				if len(x.Recv.List) > 0 && len(x.Recv.List[0].Names) > 0 && recvType != "" {
					recvName := x.Recv.List[0].Names[0].Name
					p.localVarTypes[recvName] = fmt.Sprintf("%s:%s", pkgPath, recvType)
				}
			} else {
				currentFunc = fmt.Sprintf("func:%s:%s", pkgPath, x.Name.Name)
			}

		case *ast.GenDecl:
			// Track variable assignments to build local type table
			for _, spec := range x.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Names) == 0 || vs.Type != nil {
					continue
				}
				for _, val := range vs.Values {
					p.trackAssignmentVar(val, vs.Names[0].Name, pkgPath, imports)
				}
			}

		case *ast.AssignStmt:
			// Track short variable declarations like repo := repository.NewInMem()
			if x.Tok == token.DEFINE && len(x.Lhs) == 1 && len(x.Rhs) == 1 {
				if ident, ok := x.Lhs[0].(*ast.Ident); ok {
					p.trackAssignmentVar(x.Rhs[0], ident.Name, pkgPath, imports)
				}
			}

		case *ast.CallExpr:
			if currentFunc == "" {
				return true
			}
			targetID := p.resolveCallTarget(x, imports, pkgPath)
			if targetID != "" {
				edges = append(edges, &graph.Edge{
					FromID: currentFunc,
					ToID:   targetID,
					Type:   graph.EdgeCalls,
				})
			}
		}
		return true
	})

	return edges, nil
}

// ExtractControlFlow extracts control-flow ordered call edges from a Go source file.
func (p *Parser) ExtractControlFlow(_ context.Context, path string) ([]*graph.Edge, error) {
	f, err := parser.ParseFile(p.fset, path, nil, 0)
	if err != nil {
		return nil, err
	}

	// Reset per-file local type tracking
	p.localVarTypes = make(map[string]string)

	var edges []*graph.Edge
	pkgPath := p.canonicalPkgPath(path)
	imports := p.extractImports(f)

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		currentFunc := ""
		if fn.Recv != nil {
			recvType := p.getReceiverType(fn.Recv)
			currentFunc = fmt.Sprintf("method:%s:%s.%s", pkgPath, recvType, fn.Name.Name)
			// Track receiver variable for deep selector resolution
			if len(fn.Recv.List) > 0 && len(fn.Recv.List[0].Names) > 0 && recvType != "" {
				recvName := fn.Recv.List[0].Names[0].Name
				p.localVarTypes[recvName] = fmt.Sprintf("%s:%s", pkgPath, recvType)
			}
		} else {
			currentFunc = fmt.Sprintf("func:%s:%s", pkgPath, fn.Name.Name)
		}

		order := 0
		p.walkStmtList(fn.Body.List, pkgPath, currentFunc, &order, nil, imports, &edges)
	}

	return edges, nil
}

type flowContext struct {
	kind      string
	condition string
}

func (p *Parser) walkStmtList(stmts []ast.Stmt, pkgPath, currentFunc string, order *int, ctxStack []flowContext, imports map[string]string, edges *[]*graph.Edge) {
	for _, stmt := range stmts {
		p.walkStmt(stmt, pkgPath, currentFunc, order, ctxStack, imports, edges)
	}
}

func (p *Parser) walkStmt(stmt ast.Stmt, pkgPath, currentFunc string, order *int, ctxStack []flowContext, imports map[string]string, edges *[]*graph.Edge) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		// Track short variable declarations like repo := repository.NewInMem()
		if s.Tok == token.DEFINE && len(s.Lhs) == 1 && len(s.Rhs) == 1 {
			if ident, ok := s.Lhs[0].(*ast.Ident); ok {
				p.trackAssignmentVar(s.Rhs[0], ident.Name, pkgPath, imports)
			}
		}
		p.collectCallsFromNode(s, pkgPath, currentFunc, order, ctxStack, imports, edges)
	case *ast.BlockStmt:
		p.walkStmtList(s.List, pkgPath, currentFunc, order, ctxStack, imports, edges)
	case *ast.IfStmt:
		ctx := append(ctxStack, flowContext{kind: "if", condition: p.exprString(s.Cond)})
		p.collectCallsFromNode(s.Init, pkgPath, currentFunc, order, ctx, imports, edges)
		p.collectCallsFromNode(s.Cond, pkgPath, currentFunc, order, ctx, imports, edges)
		p.walkStmtList(s.Body.List, pkgPath, currentFunc, order, ctx, imports, edges)
		if s.Else != nil {
			p.walkStmt(s.Else, pkgPath, currentFunc, order, ctx, imports, edges)
		}
	case *ast.ForStmt:
		ctx := append(ctxStack, flowContext{kind: "for", condition: p.exprString(s.Cond)})
		p.collectCallsFromNode(s.Init, pkgPath, currentFunc, order, ctx, imports, edges)
		p.collectCallsFromNode(s.Cond, pkgPath, currentFunc, order, ctx, imports, edges)
		p.collectCallsFromNode(s.Post, pkgPath, currentFunc, order, ctx, imports, edges)
		p.walkStmtList(s.Body.List, pkgPath, currentFunc, order, ctx, imports, edges)
	case *ast.RangeStmt:
		ctx := append(ctxStack, flowContext{kind: "range", condition: p.exprString(s.X)})
		p.collectCallsFromNode(s.X, pkgPath, currentFunc, order, ctx, imports, edges)
		p.walkStmtList(s.Body.List, pkgPath, currentFunc, order, ctx, imports, edges)
	case *ast.SwitchStmt:
		ctx := append(ctxStack, flowContext{kind: "switch", condition: p.exprString(s.Tag)})
		p.collectCallsFromNode(s.Init, pkgPath, currentFunc, order, ctx, imports, edges)
		p.collectCallsFromNode(s.Tag, pkgPath, currentFunc, order, ctx, imports, edges)
		for _, stmt := range s.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			p.walkStmtList(clause.Body, pkgPath, currentFunc, order, ctx, imports, edges)
		}
	case *ast.TypeSwitchStmt:
		ctx := append(ctxStack, flowContext{kind: "type-switch"})
		p.collectCallsFromNode(s.Init, pkgPath, currentFunc, order, ctx, imports, edges)
		p.collectCallsFromNode(s.Assign, pkgPath, currentFunc, order, ctx, imports, edges)
		for _, stmt := range s.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			p.walkStmtList(clause.Body, pkgPath, currentFunc, order, ctx, imports, edges)
		}
	case *ast.SelectStmt:
		ctx := append(ctxStack, flowContext{kind: "select"})
		for _, stmt := range s.Body.List {
			clause, ok := stmt.(*ast.CommClause)
			if !ok {
				continue
			}
			p.walkStmtList(clause.Body, pkgPath, currentFunc, order, ctx, imports, edges)
		}
	case *ast.DeferStmt:
		ctx := append(ctxStack, flowContext{kind: "defer"})
		p.collectCallsFromNode(s.Call, pkgPath, currentFunc, order, ctx, imports, edges)
	case *ast.GoStmt:
		ctx := append(ctxStack, flowContext{kind: "go"})
		p.collectCallsFromNode(s.Call, pkgPath, currentFunc, order, ctx, imports, edges)
	default:
		p.collectCallsFromNode(stmt, pkgPath, currentFunc, order, ctxStack, imports, edges)
	}
}

func (p *Parser) collectCallsFromNode(node ast.Node, pkgPath, currentFunc string, order *int, ctxStack []flowContext, imports map[string]string, edges *[]*graph.Edge) {
	if node == nil || currentFunc == "" {
		return
	}
	ast.Inspect(node, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		targetID := p.resolveCallTarget(callExpr, imports, pkgPath)
		if targetID == "" {
			return true
		}

		*order = *order + 1
		sequence := *order
		metadata := map[string]interface{}{
			"order":   sequence,
			"context": p.contextString(ctxStack),
			"target":  p.getCallTarget(callExpr),
		}

		*edges = append(*edges, &graph.Edge{
			FromID:   currentFunc,
			ToID:     targetID,
			Type:     graph.EdgeFlowsThrough,
			Sequence: sequence,
			Metadata: metadata,
		})
		return true
	})
}

func (p *Parser) contextString(ctxStack []flowContext) string {
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

func (p *Parser) exprString(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, p.fset, expr); err != nil {
		return ""
	}
	return buf.String()
}

// getCallTarget extracts the full call target string from a CallExpr,
// recursively resolving nested SelectorExpr chains (Issue 2 fix).
func (p *Parser) getCallTarget(ce *ast.CallExpr) string {
	return p.selectorChainString(ce.Fun)
}

// selectorChainString recursively flattens a selector expression chain into a dot-separated string.
// e.g., s.repo.UpdateBalance -> "s.repo.UpdateBalance"
func (p *Parser) selectorChainString(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		base := p.selectorChainString(x.X)
		if base == "" {
			return ""
		}
		return base + "." + x.Sel.Name
	case *ast.ParenExpr:
		return p.selectorChainString(x.X)
	case *ast.StarExpr:
		return p.selectorChainString(x.X)
	default:
		return ""
	}
}

// UnwindSelector recursively unwinds a selector expression chain, returning the
// base identifier and the ordered chain of field/method names.
// Example: for a.b.c(), returns (ident("a"), ["b", "c"]).
func UnwindSelector(expr *ast.SelectorExpr) (baseIdent *ast.Ident, chains []string) {
	current := expr
	for {
		chains = append([]string{current.Sel.Name}, chains...)
		if next, ok := current.X.(*ast.SelectorExpr); ok {
			current = next
		} else if ident, ok := current.X.(*ast.Ident); ok {
			baseIdent = ident
			break
		} else {
			break
		}
	}
	return baseIdent, chains
}

// resolveCallTarget resolves a call expression to a graph node ID using
// the local type registry for method calls on variables (Issue 1+2 fix).
// For deeply nested selectors like s.repo.UpdateBalance, it walks the field chain
// through the struct type registry to find the underlying method.
func (p *Parser) resolveCallTarget(ce *ast.CallExpr, imports map[string]string, pkgPath string) string {
	// First, try AST-level resolution using UnwindSelector for SelectorExpr
	if sel, ok := ce.Fun.(*ast.SelectorExpr); ok {
		if targetID := p.resolveSelectorExpr(sel, imports, pkgPath); targetID != "" {
			return targetID
		}
	}

	// Fallback: Get the raw call target string and try standard resolution
	target := p.getCallTarget(ce)
	if target == "" {
		return ""
	}

	// If it's a simple identifier, resolve directly
	if !strings.Contains(target, ".") {
		return p.resolveID(target, imports, pkgPath)
	}

	return p.resolveID(target, imports, pkgPath)
}

// resolveSelectorExpr resolves a SelectorExpr to a graph node ID using AST-level
// resolution with UnwindSelector for accurate base identifier extraction.
func (p *Parser) resolveSelectorExpr(sel *ast.SelectorExpr, imports map[string]string, pkgPath string) string {
	baseIdent, chains := UnwindSelector(sel)
	if baseIdent == nil || len(chains) == 0 {
		return ""
	}

	baseName := baseIdent.Name
	methodName := chains[len(chains)-1]

	// Check if this is a package-level function call (pkg.Func)
	// by checking if baseName resolves as an import with len(chains) >= 1
	if len(chains) == 1 {
		// Two-part: baseName.methodName
		// First check if it's a local variable method call
		if p.localVarTypes != nil {
			if typeKey, ok := p.localVarTypes[baseName]; ok {
				typeParts := strings.SplitN(typeKey, ":", 2)
				if len(typeParts) == 2 {
					pkgPart := p.normalizePkgPart(typeParts[0], imports)
					return fmt.Sprintf("method:%s:%s.%s", pkgPart, typeParts[1], methodName)
				}
			}
		}

		// Check if it's a package function call
		if path, ok := imports[baseName]; ok {
			if p.moduleName != "" && strings.HasPrefix(path, p.moduleName) {
				rel := strings.TrimPrefix(path, p.moduleName)
				rel = strings.TrimPrefix(rel, "/")
				return fmt.Sprintf("func:%s:%s", rel, methodName)
			}
			if isStdlibImport(path, p.moduleName) {
				return fmt.Sprintf("stdlib:%s:%s", path, methodName)
			}
			return fmt.Sprintf("func:%s:%s", path, methodName)
		}

		// Local function
		return fmt.Sprintf("func:%s:%s", pkgPath, baseName)
	}

	// Multi-part selector (baseName.field1.field2...methodName)
	// Walk the struct field chain through the struct registry
	if p.localVarTypes != nil {
		if typeKey, ok := p.localVarTypes[baseName]; ok {
			typeParts := strings.SplitN(typeKey, ":", 2)
			if len(typeParts) == 2 {
				pkgPart := p.normalizePkgPart(typeParts[0], imports)
				currentType := fmt.Sprintf("%s:%s", pkgPart, typeParts[1])

				// Walk through intermediate fields (chains[0..n-2])
				for i := 0; i < len(chains)-1; i++ {
					fieldType := p.resolveFieldType(currentType, chains[i], imports)
					if fieldType == "" {
						return "" // Cannot resolve field
					}
					currentType = fieldType
				}

				// Now currentType is the key of the terminal type holding the method
				if uri := p.typeKeyToMethodURI(currentType, methodName, imports); uri != "" {
					return uri
				}
			}
		}
	}

	return ""
}

// normalizePkgPart resolves an import alias to a canonical package path.
func (p *Parser) normalizePkgPart(pkgPart string, imports map[string]string) string {
	if canonicalPath, ok := imports[pkgPart]; ok {
		if p.moduleName != "" && strings.HasPrefix(canonicalPath, p.moduleName) {
			rel := strings.TrimPrefix(canonicalPath, p.moduleName)
			rel = strings.TrimPrefix(rel, "/")
			return rel
		}
		return canonicalPath
	}
	return pkgPart
}

// typeKeyToMethodURI converts a type key and method name to a proper graph node URI.
// currentType can be either "pkgPath:TypeName" or "packageQualifier.TypeName"
func (p *Parser) typeKeyToMethodURI(currentType, methodName string, imports map[string]string) string {
	if strings.Contains(currentType, ":") {
		// Already in "pkgPath:TypeName" format
		ctParts := strings.SplitN(currentType, ":", 2)
		return fmt.Sprintf("method:%s:%s.%s", ctParts[0], ctParts[1], methodName)
	}

	// Package-qualified type like "domain.WalletRepository"
	if strings.Contains(currentType, ".") {
		subParts := strings.SplitN(currentType, ".", 2)
		if len(subParts) == 2 {
			if pkgPath2, ok := imports[subParts[0]]; ok {
				if p.moduleName != "" && strings.HasPrefix(pkgPath2, p.moduleName) {
					rel := strings.TrimPrefix(pkgPath2, p.moduleName)
					rel = strings.TrimPrefix(rel, "/")
					return fmt.Sprintf("method:%s:%s.%s", rel, subParts[1], methodName)
				}
				return fmt.Sprintf("method:%s:%s.%s", pkgPath2, subParts[1], methodName)
			}
			// If import not found, try using first part as another type key recursively
			return fmt.Sprintf("unknown:%s.%s", currentType, methodName)
		}
	}

	return ""
}

// resolveFieldType looks up the type of a field on a struct type from the struct registry.
// For package-qualified field types (e.g., "domain.WalletRepository"), it resolves the
// import to produce a fully qualified "pkgPath:TypeName" key, preventing unknown: fallbacks.
func (p *Parser) resolveFieldType(structKey string, fieldName string, imports map[string]string) string {
	if p.structIndex == nil {
		return ""
	}
	si, ok := p.structIndex[structKey]
	if !ok {
		return ""
	}
	for _, f := range si.Fields {
		if f.FieldName == fieldName {
			// Extract just the type name (handle * prefix, package qualifiers)
			fieldType := f.FieldType
			fieldType = strings.TrimPrefix(fieldType, "*")
			if strings.Contains(fieldType, ".") {
				// Package-qualified type like "domain.WalletRepository"
				// Resolve through imports to produce "pkgPath:TypeName" format
				subParts := strings.SplitN(fieldType, ".", 2)
				if len(subParts) == 2 {
					if pkgPath2, ok := imports[subParts[0]]; ok {
						if p.moduleName != "" && strings.HasPrefix(pkgPath2, p.moduleName) {
							rel := strings.TrimPrefix(pkgPath2, p.moduleName)
							rel = strings.TrimPrefix(rel, "/")
							return fmt.Sprintf("%s:%s", rel, subParts[1])
						}
						return fmt.Sprintf("%s:%s", pkgPath2, subParts[1])
					}
				}
				// Fallback: return unqualified form; typeKeyToMethodURI will also try imports
				return fieldType
			}
			return fmt.Sprintf("%s:%s", si.PkgPath, fieldType)
		}
	}
	return ""
}
