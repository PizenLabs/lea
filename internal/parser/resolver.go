// Package parser provides type resolution and canonical ID construction for Go source analysis.
package parser

import (
	"fmt"
	"go/ast"
	"strings"
)

// StructFieldInfo holds the type information for a struct field.
type StructFieldInfo struct {
	FieldName string
	FieldType string
}

// StructInfo holds type information for a parsed struct.
type StructInfo struct {
	PkgPath string
	Name    string
	Fields  []StructFieldInfo
}

// TypeRegistry tracks canonical type information for local variables and structs.
type TypeRegistry struct {
	// localVarTypes maps variable name -> "fullPkgPath:ExactTypeName"
	// e.g., "svc" -> "internal/service:PaymentService"
	LocalVarTypes map[string]string

	// structIndex maps "fullPkgPath:StructName" -> *StructInfo
	StructIndex map[string]*StructInfo

	// moduleName for trimming internal paths
	ModuleName string
}

// NewTypeRegistry creates a new type registry.
func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		LocalVarTypes: make(map[string]string),
		StructIndex:   make(map[string]*StructInfo),
	}
}

// SetModuleName sets the module prefix for canonical path resolution.
func (tr *TypeRegistry) SetModuleName(name string) {
	tr.ModuleName = name
}

// TrackConstructorCall records the type of a variable initialized via constructor call.
// Example: svc := service.NewPaymentService(repo)
//   - varName = "svc"
//   - pkgAlias = "service"   (import alias)
//   - constructorName = "NewPaymentService"
//   - imports maps "service" -> "github.com/PizenLabs/lea/internal/service"
func (tr *TypeRegistry) TrackConstructorCall(varName, pkgAlias, constructorName string, imports map[string]string) {
	typeName := strings.TrimPrefix(constructorName, "New")
	if typeName == "" || typeName == constructorName {
		return
	}

	// Expand import alias to full package path
	fullPkg := tr.resolveImportPath(pkgAlias, imports)
	key := fmt.Sprintf("%s:%s", fullPkg, typeName)
	tr.LocalVarTypes[varName] = key
}

// TrackCompositeLitType records the type of a variable initialized with composite literal.
// Example: svc := &service.PaymentService{}
func (tr *TypeRegistry) TrackCompositeLitType(varName string, pkgAlias, typeName string, imports map[string]string) {
	fullPkg := tr.resolveImportPath(pkgAlias, imports)
	key := fmt.Sprintf("%s:%s", fullPkg, typeName)
	tr.LocalVarTypes[varName] = key
}

// TrackLocalCompositeLitType records the type of a variable initialized with
// a same-package composite literal.
// Example: svc := &PaymentService{}
func (tr *TypeRegistry) TrackLocalCompositeLitType(varName, typeName, pkgPath string) {
	key := fmt.Sprintf("%s:%s", pkgPath, typeName)
	tr.LocalVarTypes[varName] = key
}

// TrackReceiverType records the type of a method receiver variable.
// Example: func (s *PaymentService) ProcessDeposit()
func (tr *TypeRegistry) TrackReceiverType(recvName, pkgPath, typeName string) {
	key := fmt.Sprintf("%s:%s", pkgPath, typeName)
	tr.LocalVarTypes[recvName] = key
}

// RegisterStruct records a struct type and its fields for deep resolution.
func (tr *TypeRegistry) RegisterStruct(pkgPath, typeName string, fields []StructFieldInfo) {
	key := fmt.Sprintf("%s:%s", pkgPath, typeName)
	tr.StructIndex[key] = &StructInfo{
		PkgPath: pkgPath,
		Name:    typeName,
		Fields:  fields,
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

// ResolveMethodID resolves a selector expression (var.method or var.field.sub.method)
// to a canonical method node ID.
// Handles 2-part selectors (e.g., "svc.ProcessDeposit") and multi-part selectors
// (e.g., "s.repo.UpdateBalance") by walking the struct field chain.
// Returns "" when resolution fails.
func (tr *TypeRegistry) ResolveMethodID(target string, imports map[string]string, _ string) string {
	if tr == nil {
		return ""
	}
	if !strings.Contains(target, ".") {
		return ""
	}

	parts := strings.Split(target, ".")
	methodName := parts[len(parts)-1]

	// Check if first part is a tracked local variable
	if tr.LocalVarTypes != nil {
		if typeKey, ok := tr.LocalVarTypes[parts[0]]; ok {
			// For 2-part selectors (var.method), resolve directly
			if len(parts) == 2 {
				return tr.resolveFromTypeKey(typeKey, methodName, imports)
			}

			// For multi-part selectors (var.field.sub.method), walk the struct field chain
			// Resolve the base variable's type key ("fullPkgPath:ExactTypeName") to a struct key
			typeKeyNormalized := tr.normalizeTypeKey(typeKey, imports)
			currentTypeKey := typeKeyNormalized

			// Walk each intermediate field (parts[1]..parts[n-2]) through the struct registry
			for i := 1; i < len(parts)-1; i++ {
				fieldType := tr.ResolveFieldType(currentTypeKey, parts[i])
				if fieldType == "" {
					return "" // Field not found in struct registry
				}
				// The field type might be a simple name ("WalletRepository") or
				// a package-qualified name ("domain.WalletRepository").
				// Build the next struct key from the current struct's package path + field type.
				fieldType = strings.TrimPrefix(fieldType, "*")
				if strings.Contains(fieldType, ".") {
					// Package-qualified: resolve through imports to get canonical key
					if resolved := tr.resolvePackageQualifiedKey(fieldType, imports); resolved != "" {
						currentTypeKey = resolved
					} else {
						return ""
					}
				} else {
					// Same package: use the struct's own package path
					ci := strings.Index(currentTypeKey, ":")
					if ci < 0 {
						return ""
					}
					currentTypeKey = fmt.Sprintf("%s:%s", currentTypeKey[:ci], fieldType)
				}
			}

			// Now currentTypeKey is the key of the terminal struct type holding the method
			ci := strings.Index(currentTypeKey, ":")
			if ci < 0 {
				return ""
			}
			return fmt.Sprintf("method:%s:%s.%s", currentTypeKey[:ci], currentTypeKey[ci+1:], methodName)
		}
	}

	return ""
}

// normalizeTypeKey resolves a type key into a canonical "fullPkgPath:TypeName" form,
// expanding import aliases in the package portion if needed.
func (tr *TypeRegistry) normalizeTypeKey(typeKey string, imports map[string]string) string {
	idx := strings.Index(typeKey, ":")
	if idx < 0 {
		return typeKey
	}
	pkgPart := typeKey[:idx]
	typeName := typeKey[idx+1:]

	// If the package part is an import alias, resolve it to canonical path
	if canonicalPath, ok := imports[pkgPart]; ok {
		if tr.ModuleName != "" && strings.HasPrefix(canonicalPath, tr.ModuleName) {
			rel := strings.TrimPrefix(canonicalPath, tr.ModuleName)
			rel = strings.TrimPrefix(rel, "/")
			pkgPart = rel
		} else {
			pkgPart = canonicalPath
		}
	}
	return fmt.Sprintf("%s:%s", pkgPart, typeName)
}

// resolvePackageQualifiedKey resolves a package-qualified type name (e.g., "domain.WalletRepository")
// to a canonical "fullPkgPath:TypeName" key using the imports map.
func (tr *TypeRegistry) resolvePackageQualifiedKey(typeName string, imports map[string]string) string {
	subParts := strings.SplitN(typeName, ".", 2)
	if len(subParts) != 2 {
		return ""
	}
	if pkgPath2, ok := imports[subParts[0]]; ok {
		if tr.ModuleName != "" && strings.HasPrefix(pkgPath2, tr.ModuleName) {
			rel := strings.TrimPrefix(pkgPath2, tr.ModuleName)
			rel = strings.TrimPrefix(rel, "/")
			return fmt.Sprintf("%s:%s", rel, subParts[1])
		}
		return fmt.Sprintf("%s:%s", pkgPath2, subParts[1])
	}
	return ""
}

// ResolveCallTarget resolves a full call target string to a graph node ID,
// handling local variable method calls, package function calls, and local functions.
func (tr *TypeRegistry) ResolveCallTarget(target string, imports map[string]string, pkgPath string) string {
	if !strings.Contains(target, ".") {
		return fmt.Sprintf("func:%s:%s", pkgPath, target)
	}

	// Try as local variable method call
	if methodID := tr.ResolveMethodID(target, imports, pkgPath); methodID != "" {
		return methodID
	}

	// Try as package function call (pkg.Func)
	parts := strings.SplitN(target, ".", 2)
	prefix := parts[0]
	name := parts[1]

	if tr != nil && tr.ModuleName != "" {
		if path, ok := imports[prefix]; ok {
			relPath := path
			if strings.HasPrefix(path, tr.ModuleName) {
				relPath = strings.TrimPrefix(path, tr.ModuleName)
				relPath = strings.TrimPrefix(relPath, "/")
			}
			return fmt.Sprintf("func:%s:%s", relPath, name)
		}
	}

	// Fallback: try import resolution
	if path, ok := imports[prefix]; ok {
		return fmt.Sprintf("func:%s:%s", path, name)
	}

	return fmt.Sprintf("unknown:%s", target)
}

// ResolveFieldType looks up the type of a struct field by name.
func (tr *TypeRegistry) ResolveFieldType(structKey, fieldName string) string {
	if tr == nil || tr.StructIndex == nil {
		return ""
	}
	si, ok := tr.StructIndex[structKey]
	if !ok {
		return ""
	}
	for _, f := range si.Fields {
		if f.FieldName == fieldName {
			return f.FieldType
		}
	}
	return ""
}

// resolveFromTypeKey converts a type key ("fullPkgPath:ExactTypeName") and
// method name into a canonical method ID ("method:fullPkgPath:ExactTypeName.methodName").
func (tr *TypeRegistry) resolveFromTypeKey(typeKey, methodName string, imports map[string]string) string {
	idx := strings.Index(typeKey, ":")
	if idx < 0 {
		return ""
	}

	pkgPath := typeKey[:idx]
	typeName := typeKey[idx+1:]

	// Handle package-qualified type names (e.g., "domain.WalletRepository")
	// by resolving the prefix through imports
	if strings.Contains(typeName, ".") {
		subParts := strings.SplitN(typeName, ".", 2)
		if len(subParts) == 2 {
			if pkgPath2, ok := imports[subParts[0]]; ok {
				if tr.ModuleName != "" && strings.HasPrefix(pkgPath2, tr.ModuleName) {
					rel := strings.TrimPrefix(pkgPath2, tr.ModuleName)
					rel = strings.TrimPrefix(rel, "/")
					return fmt.Sprintf("method:%s:%s.%s", rel, subParts[1], methodName)
				}
				return fmt.Sprintf("method:%s:%s.%s", pkgPath2, subParts[1], methodName)
			}
		}
	}

	return fmt.Sprintf("method:%s:%s.%s", pkgPath, typeName, methodName)
}

// resolveImportPath expands an import alias to a full package path.
// For internal modules, it strips the module prefix to get the relative path.
func (tr *TypeRegistry) resolveImportPath(alias string, imports map[string]string) string {
	if path, ok := imports[alias]; ok {
		if tr.ModuleName != "" && strings.HasPrefix(path, tr.ModuleName) {
			rel := strings.TrimPrefix(path, tr.ModuleName)
			rel = strings.TrimPrefix(rel, "/")
			return rel
		}
		return path
	}
	return alias
}
