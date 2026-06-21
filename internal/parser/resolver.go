// Package parser provides type resolution and canonical ID construction for Go source analysis.
package parser

import (
	"fmt"
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

// ResolveMethodID resolves a simple selector expression (var.method)
// to a canonical method node ID.
// Only handles 2-part selectors (e.g., "svc.ProcessDeposit").
// Multi-part selectors (e.g., "s.repo.UpdateBalance") require struct field
// chain walk and return "" to fall through to package-level resolution.
// Returns "" when resolution fails.
func (tr *TypeRegistry) ResolveMethodID(target string, imports map[string]string, _ string) string {
	if tr == nil {
		return ""
	}
	if !strings.Contains(target, ".") {
		return ""
	}

	parts := strings.Split(target, ".")
	// Only resolve 2-part selectors (var.method). Multi-part selectors
	// like s.repo.UpdateBalance need struct field chain walk.
	if len(parts) != 2 {
		return ""
	}

	methodName := parts[1]

	// Check if first part is a tracked local variable
	if tr.LocalVarTypes != nil {
		if typeKey, ok := tr.LocalVarTypes[parts[0]]; ok {
			// typeKey is "fullPkgPath:ExactTypeName"
			return tr.resolveFromTypeKey(typeKey, methodName, imports)
		}
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
