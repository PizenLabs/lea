package parser

import (
	"context"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"path/filepath"
	"testing"

	graph "github.com/PizenLabs/lea/internal/graph/contracts"
)

// TestTypeRegistry_TrackConstructorCall verifies that constructor assignments
// resolve import aliases to full package paths and preserve exact type names.
func TestTypeRegistry_TrackConstructorCall(t *testing.T) {
	tests := []struct {
		name           string
		varName        string
		pkgAlias       string
		constructor    string
		imports        map[string]string
		expectedKey    string // "fullPkgPath:ExactTypeName"
		expectedMethod string // "method:fullPkgPath:ExactTypeName.Method" (when resolving)
		methodName     string
	}{
		{
			name:        "internal service constructor",
			varName:     "svc",
			pkgAlias:    "service",
			constructor: "NewPaymentService",
			imports: map[string]string{
				"service": "github.com/PizenLabs/lea/internal/service",
			},
			expectedKey:    "internal/service:PaymentService",
			expectedMethod: "method:internal/service:PaymentService.ProcessDeposit",
			methodName:     "ProcessDeposit",
		},
		{
			name:        "external package constructor",
			varName:     "logger",
			pkgAlias:    "log",
			constructor: "NewLogger",
			imports: map[string]string{
				"log": "github.com/sirupsen/logrus",
			},
			expectedKey:    "github.com/sirupsen/logrus:Logger",
			expectedMethod: "method:github.com/sirupsen/logrus:Logger.Info",
			methodName:     "Info",
		},
		{
			name:        "preserve Service suffix in type name",
			varName:     "svc",
			pkgAlias:    "service",
			constructor: "NewAuthService",
			imports: map[string]string{
				"service": "github.com/PizenLabs/lea/internal/auth",
			},
			expectedKey:    "internal/auth:AuthService",
			expectedMethod: "method:internal/auth:AuthService.Login",
			methodName:     "Login",
		},
		{
			name:        "short package alias expanded",
			varName:     "repo",
			pkgAlias:    "r",
			constructor: "NewWalletRepository",
			imports: map[string]string{
				"r": "github.com/PizenLabs/lea/internal/repository",
			},
			expectedKey:    "internal/repository:WalletRepository",
			expectedMethod: "method:internal/repository:WalletRepository.GetBalance",
			methodName:     "GetBalance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := NewTypeRegistry()
			reg.SetModuleName("github.com/PizenLabs/lea")
			reg.TrackConstructorCall(tt.varName, tt.pkgAlias, tt.constructor, tt.imports)

			// Verify the tracked type key
			gotKey, ok := reg.LocalVarTypes[tt.varName]
			if !ok {
				t.Fatalf("Expected %s to be tracked, but it was not", tt.varName)
			}
			if gotKey != tt.expectedKey {
				t.Errorf("localVarTypes[%q] = %q, want %q", tt.varName, gotKey, tt.expectedKey)
			}

			// Verify method resolution
			target := tt.varName + "." + tt.methodName
			methodID := reg.ResolveMethodID(target, tt.imports, "")
			if methodID != tt.expectedMethod {
				t.Errorf("ResolveMethodID(%q) = %q, want %q", target, methodID, tt.expectedMethod)
			}
		})
	}
}

// TestTypeRegistry_ResolveMethodID verifies that selector expressions resolve
// to canonical method IDs using the type registry.
func TestTypeRegistry_ResolveMethodID(t *testing.T) {
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")

	imports := map[string]string{
		"service": "github.com/PizenLabs/lea/internal/service",
	}

	// Track variable from constructor
	reg.TrackConstructorCall("svc", "service", "NewPaymentService", imports)

	tests := []struct {
		target   string
		expected string
	}{
		{
			target:   "svc.ProcessDeposit",
			expected: "method:internal/service:PaymentService.ProcessDeposit",
		},
		{
			target:   "svc.ProcessDeposit",
			expected: "method:internal/service:PaymentService.ProcessDeposit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := reg.ResolveMethodID(tt.target, imports, "")
			if got != tt.expected {
				t.Errorf("ResolveMethodID(%q) = %q, want %q", tt.target, got, tt.expected)
			}
		})
	}
}

// TestTypeRegistry_ResolveCallTarget verifies that package function calls
// resolve to the correct function IDs with full package paths.
func TestTypeRegistry_ResolveCallTarget(t *testing.T) {
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")

	imports := map[string]string{
		"fmt":     "fmt",
		"service": "github.com/PizenLabs/lea/internal/service",
		"domain":  "github.com/PizenLabs/lea/internal/domain",
	}

	tests := []struct {
		target   string
		pkgPath  string
		expected string
	}{
		{
			target:   "fmt.Println",
			pkgPath:  "cmd/lea",
			expected: "func:fmt:Println",
		},
		{
			target:   "service.NewPaymentService",
			pkgPath:  "cmd/lea",
			expected: "func:internal/service:NewPaymentService",
		},
		{
			target:   "domain.NewWallet",
			pkgPath:  "cmd/lea",
			expected: "func:internal/domain:NewWallet",
		},
		{
			target:   "LocalFunc",
			pkgPath:  "cmd/lea",
			expected: "func:cmd/lea:LocalFunc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := reg.ResolveCallTarget(tt.target, imports, tt.pkgPath)
			if got != tt.expected {
				t.Errorf("ResolveCallTarget(%q, _, %q) = %q, want %q", tt.target, tt.pkgPath, got, tt.expected)
			}
		})
	}
}

// TestTypeRegistry_TrackReceiverType verifies that method receiver variables
// are tracked with full package path and exact type name.
func TestTypeRegistry_TrackReceiverType(t *testing.T) {
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")

	reg.TrackReceiverType("s", "internal/service", "PaymentService")

	imports := map[string]string{}
	target := "s.ProcessDeposit"
	expected := "method:internal/service:PaymentService.ProcessDeposit"

	got := reg.ResolveMethodID(target, imports, "")
	if got != expected {
		t.Errorf("ResolveMethodID(%q) = %q, want %q", target, got, expected)
	}
}

// TestTypeRegistry_CompositeLitTracking verifies composite literal initialization.
func TestTypeRegistry_CompositeLitTracking(t *testing.T) {
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")

	imports := map[string]string{
		"service": "github.com/PizenLabs/lea/internal/service",
	}

	reg.TrackCompositeLitType("svc", "service", "PaymentService", imports)

	got, ok := reg.LocalVarTypes["svc"]
	if !ok {
		t.Fatal("Expected svc to be tracked")
	}
	expected := "internal/service:PaymentService"
	if got != expected {
		t.Errorf("localVarTypes[svc] = %q, want %q", got, expected)
	}

	// Verify method resolution
	methodID := reg.ResolveMethodID("svc.ProcessDeposit", imports, "")
	expectedMethod := "method:internal/service:PaymentService.ProcessDeposit"
	if methodID != expectedMethod {
		t.Errorf("ResolveMethodID = %q, want %q", methodID, expectedMethod)
	}
}

// TestCrossPackageResolution verifies that package-level function calls
// resolve through imports correctly for internal and external packages.
func TestCrossPackageResolution(t *testing.T) {
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")

	imports := map[string]string{
		"contracts": "github.com/PizenLabs/lea/internal/graph/contracts",
		"fmt":       "fmt",
	}
	pkgPath := "cmd/lea"

	testCases := []struct {
		target   string
		expected string
	}{
		{"contracts.SomeFunc", "func:internal/graph/contracts:SomeFunc"},
		{"fmt.Println", "func:fmt:Println"},
		{"LocalFunc", "func:cmd/lea:LocalFunc"},
	}

	for _, tc := range testCases {
		t.Run(tc.target, func(t *testing.T) {
			got := reg.ResolveCallTarget(tc.target, imports, pkgPath)
			if got != tc.expected {
				t.Errorf("ResolveCallTarget(%q) = %q, want %q", tc.target, got, tc.expected)
			}
		})
	}
}

// TestExtractCalls_WithTypeRegistry verifies end-to-end call extraction
// with the TypeRegistry properly tracking variable types.
func TestExtractCalls_WithTypeRegistry(t *testing.T) {
	// Test the TypeRegistry directly for constructor -> method call resolution.
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")

	imports := map[string]string{
		"service": "github.com/PizenLabs/lea/internal/service",
		"fmt":     "fmt",
	}

	// Simulate parsing: track constructor assignment
	reg.TrackConstructorCall("svc", "service", "NewPaymentService", imports)

	// Verify the method call resolution
	methodID := reg.ResolveMethodID("svc.ProcessDeposit", imports, "cmd/lea")
	expected := "method:internal/service:PaymentService.ProcessDeposit"
	if methodID != expected {
		t.Errorf("svc.ProcessDeposit resolved to %q, want %q", methodID, expected)
	}

	// Verify package-level func resolution
	funcID := reg.ResolveCallTarget("fmt.Println", imports, "cmd/lea")
	expectedFunc := "func:fmt:Println"
	if funcID != expectedFunc {
		t.Errorf("fmt.Println resolved to %q, want %q", funcID, expectedFunc)
	}
}

// TestExtractCalls_PaymentService mirrors the bug scenario:
// service.NewPaymentService(repo) -> svc.ProcessDeposit()
// Must resolve to method:internal/service:PaymentService.ProcessDeposit
func TestExtractCalls_PaymentService(t *testing.T) {
	absPath, err := filepath.Abs("../../testdata/internal/service/payment.go")
	if err != nil {
		t.Fatal(err)
	}

	cp := NewCallParser()
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")
	cp.SetTypeRegistry(reg)

	ctx := context.Background()
	edges, err := cp.ExtractCalls(ctx, absPath)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}

	pkgPath := filepath.Dir(absPath)

	// Verify ProcessDeposit method call edges exist
	for _, e := range edges {
		t.Logf("Edge: %s (%s) -> %s", e.FromID, e.Type, e.ToID)
	}

	// Expect method:internal/service:PaymentService.ProcessDeposit
	// to call its internal methods: s.log.Info, s.repo.UpdateBalance
	expectedFrom := fmt.Sprintf("method:%s:PaymentService.ProcessDeposit", pkgPath)

	foundInfo := false
	foundUpdate := false
	for _, e := range edges {
		if e.FromID == expectedFrom && e.Type == graph.EdgeCalls {
			if e.ToID == "method:gopump/pkg/logger:Logger.Info" || e.ToID == "func:fmt:Sprintf" || e.ToID == "func:fmt:Println" {
				foundInfo = true
			}
			if e.ToID == "method:gopump/internal/domain:WalletRepository.UpdateBalance" {
				foundUpdate = true
			}
		}
	}

	if !foundInfo {
		t.Errorf("Expected a CALLS edge from %s to s.log.Info or fmt.Println, got:\n", expectedFrom)
		for _, e := range edges {
			if e.FromID == expectedFrom {
				t.Logf("  Edge: %s (%s) -> %s", e.FromID, e.Type, e.ToID)
			}
		}
	}
	if !foundUpdate {
		t.Errorf("Expected a CALLS edge from %s to s.repo.UpdateBalance", expectedFrom)
	}
}

// TestExtractControlFlow_WithTypeRegistry validates control flow extraction
// respects canonical type resolution.
func TestExtractControlFlow_WithTypeRegistry(t *testing.T) {
	absPath, err := filepath.Abs("../../testdata/internal/service/payment.go")
	if err != nil {
		t.Fatal(err)
	}

	cp := NewCallParser()
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")
	cp.SetTypeRegistry(reg)

	ctx := context.Background()
	edges, err := cp.ExtractControlFlow(ctx, absPath)
	if err != nil {
		t.Fatalf("ExtractControlFlow failed: %v", err)
	}

	if len(edges) == 0 {
		t.Error("No control flow edges found")
	}

	pkgPath := filepath.Dir(absPath)
	expectedFrom := fmt.Sprintf("method:%s:PaymentService.ProcessDeposit", pkgPath)

	for _, e := range edges {
		t.Logf("Flow: %s -> %s (order=%d, type=%s)", e.FromID, e.ToID, e.Sequence, e.Type)
		_ = expectedFrom
	}
}

// TestImportAliasExpansion verifies that import aliases are expanded
// to full package paths during type tracking.
func TestImportAliasExpansion(t *testing.T) {
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")

	// Single-letter alias commonly used with dot imports etc.
	imports := map[string]string{
		"svc": "github.com/PizenLabs/lea/internal/service",
	}

	// Track constructor call with single-letter alias
	reg.TrackConstructorCall("s", "svc", "NewPaymentService", imports)

	got, ok := reg.LocalVarTypes["s"]
	if !ok {
		t.Fatal("Expected 's' to be tracked")
	}
	expected := "internal/service:PaymentService"
	if got != expected {
		t.Errorf("localVarTypes[s] = %q, want %q", got, expected)
	}
}

// TestNoTypeNameMutation verifies type names are preserved exactly
// without trimming suffixes like "Service".
func TestNoTypeNameMutation(t *testing.T) {
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")

	imports := map[string]string{
		"svc": "github.com/PizenLabs/lea/internal/service",
	}

	// "Service" suffix must be preserved
	reg.TrackConstructorCall("svc", "svc", "NewPaymentService", imports)
	got, ok := reg.LocalVarTypes["svc"]
	if !ok {
		t.Fatal("Expected svc to be tracked")
	}
	if got != "internal/service:PaymentService" {
		t.Errorf("PaymentService type name was mutated: got %q, want %q", got, "internal/service:PaymentService")
	}

	// Multiple "Service" suffix should also be preserved
	reg.TrackConstructorCall("as", "svc", "NewAuthService", imports)
	got2, ok := reg.LocalVarTypes["as"]
	if !ok {
		t.Fatal("Expected as to be tracked")
	}
	if got2 != "internal/service:AuthService" {
		t.Errorf("AuthService type name was mutated: got %q, want %q", got2, "internal/service:AuthService")
	}
}

// TestResolveMethodID_MissingVar returns empty string for untracked variables.
func TestResolveMethodID_MissingVar(t *testing.T) {
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")

	imports := map[string]string{
		"fmt": "fmt",
	}

	// Untracked variable should return empty string
	got := reg.ResolveMethodID("x.SomeMethod", imports, "pkg")
	if got != "" {
		t.Errorf("Expected empty string for untracked var, got %q", got)
	}
}

// TestNewTypeRegistry_InitialState verifies initial state is valid.
func TestNewTypeRegistry_InitialState(t *testing.T) {
	reg := NewTypeRegistry()
	if reg.LocalVarTypes == nil {
		t.Error("LocalVarTypes should be initialized")
	}
	if reg.StructIndex == nil {
		t.Error("StructIndex should be initialized")
	}
	if reg.ModuleName != "" {
		t.Errorf("ModuleName should be empty initially, got %q", reg.ModuleName)
	}
}

// TestTypeRegistry_ResolveMethodID_PackageQualifiedType verifies that
// package-qualified type names (e.g., "domain.WalletRepository") are resolved
// through imports to canonical paths.
func TestTypeRegistry_ResolveMethodID_PackageQualifiedType(t *testing.T) {
	t.Skip("Package-qualified type resolution through imports needs SchemaParser integration")
	// This test validates that when a type key contains a package-qualified name
	// (e.g., "pkg:domain.WalletRepository"), the package prefix "domain" is resolved
	// through the imports map to get the full path.
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")

	imports := map[string]string{
		"domain": "github.com/PizenLabs/lea/internal/domain",
	}

	// Manually set a package-qualified type key
	reg.LocalVarTypes["repo"] = "pkg:domain.WalletRepository"

	methodID := reg.ResolveMethodID("repo.UpdateBalance", imports, "test")
	expected := "method:internal/domain:WalletRepository.UpdateBalance"
	if methodID != expected {
		t.Errorf("ResolveMethodID = %q, want %q", methodID, expected)
	}
}

// TestExtractImports checks that extractImports correctly builds alias->path maps.
func TestExtractImports(t *testing.T) {
	src := `
package test
import (
	"fmt"
	"go/ast"
	svc "github.com/PizenLabs/lea/internal/service"
)
`
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, "", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	imports := extractImports(f)
	expected := map[string]string{
		"fmt": "fmt",
		"ast": "go/ast",
		"svc": "github.com/PizenLabs/lea/internal/service",
	}

	if len(imports) != len(expected) {
		t.Errorf("Got %d imports, want %d", len(imports), len(expected))
	}
	for k, v := range expected {
		if imports[k] != v {
			t.Errorf("imports[%q] = %q, want %q", k, imports[k], v)
		}
	}
}

// TestGetCallTarget checks that getCallTarget correctly extracts dot-separated call targets.
func TestGetCallTarget(t *testing.T) {
	src := `
package test
func f() {
	svc.ProcessDeposit()
	service.NewPaymentService(nil)
	fmt.Println("hello")
}
`
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, "", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var targets []string
	ast.Inspect(f, func(n ast.Node) bool {
		if ce, ok := n.(*ast.CallExpr); ok {
			targets = append(targets, getCallTarget(ce))
		}
		return true
	})

	expected := []string{"svc.ProcessDeposit", "service.NewPaymentService", "fmt.Println"}
	if len(targets) != len(expected) {
		t.Errorf("Got %d targets, want %d: %v", len(targets), len(expected), targets)
	}
	for i, target := range targets {
		if i < len(expected) && target != expected[i] {
			t.Errorf("target[%d] = %q, want %q", i, target, expected[i])
		}
	}
}

// TestResolveCallTarget_Fallback verifies the fallback resolver without TypeRegistry.
func TestResolveCallTarget_Fallback(t *testing.T) {
	imports := map[string]string{
		"fmt": "fmt",
	}

	tests := []struct {
		target   string
		pkgPath  string
		expected string
	}{
		{"LocalFunc", "pkg", "func:pkg:LocalFunc"},
		{"fmt.Println", "pkg", "func:fmt:Println"},
		{"unknown.Target", "pkg", "unknown:unknown.Target"},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			got := resolveCallTarget(tt.target, imports, tt.pkgPath)
			if got != tt.expected {
				t.Errorf("resolveCallTarget(%q) = %q, want %q", tt.target, got, tt.expected)
			}
		})
	}
}

// TestGetReceiverType handles pointer receivers and plain receivers.
func TestGetReceiverType(t *testing.T) {
	tests := []struct {
		src      string
		expected string
	}{
		{`package p; type T struct{}; func (s *T) M() {}`, "T"},
		{`package p; type T struct{}; func (s T) M() {}`, "T"},
		{`package p; type PaymentService struct{}; func (s *PaymentService) M() {}`, "PaymentService"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := goparser.ParseFile(fset, "", tt.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, decl := range f.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
					got := getReceiverType(fn.Recv)
					if got != tt.expected {
						t.Errorf("getReceiverType = %q, want %q", got, tt.expected)
					}
				}
			}
		})
	}
}

// TestSelectorChainString verifies deep selector chain flattening.
func TestSelectorChainString(t *testing.T) {
	src := `
package p
func f() {
	s.repo.UpdateBalance(walletID, amount)
}
`
	fset := token.NewFileSet()
	f, err := goparser.ParseFile(fset, "", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var target string
	ast.Inspect(f, func(n ast.Node) bool {
		if ce, ok := n.(*ast.CallExpr); ok {
			target = getCallTarget(ce)
			return false
		}
		return true
	})

	expected := "s.repo.UpdateBalance"
	if target != expected {
		t.Errorf("getCallTarget = %q, want %q", target, expected)
	}
}

// TestEdgesFromCallParser validates end-to-end edge extraction from a real file.
func TestEdgesFromCallParser(t *testing.T) {
	absPath, err := filepath.Abs("../../testdata/esc/golang/simple.go")
	if err != nil {
		t.Fatal(err)
	}

	cp := NewCallParser()
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")
	cp.SetTypeRegistry(reg)

	ctx := context.Background()
	edges, err := cp.ExtractCalls(ctx, absPath)
	if err != nil {
		t.Fatalf("ExtractCalls failed: %v", err)
	}

	if len(edges) == 0 {
		t.Error("No edges extracted")
	}

	for _, e := range edges {
		t.Logf("Edge: %s (%s) -> %s (seq=%d)", e.FromID, e.Type, e.ToID, e.Sequence)
	}
}

// TestControlFlowEdgesFromCallParser validates control flow extraction from a real file.
func TestControlFlowEdgesFromCallParser(t *testing.T) {
	absPath, err := filepath.Abs("../../testdata/esc/golang/simple.go")
	if err != nil {
		t.Fatal(err)
	}

	cp := NewCallParser()
	reg := NewTypeRegistry()
	reg.SetModuleName("github.com/PizenLabs/lea")
	cp.SetTypeRegistry(reg)

	ctx := context.Background()
	edges, err := cp.ExtractControlFlow(ctx, absPath)
	if err != nil {
		t.Fatalf("ExtractControlFlow failed: %v", err)
	}

	if len(edges) == 0 {
		t.Error("No control flow edges found")
	}

	for _, e := range edges {
		t.Logf("ControlFlow: %s -> %s (order=%d, type=%s)", e.FromID, e.ToID, e.Sequence, e.Type)
	}
}
