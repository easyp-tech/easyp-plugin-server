package registry

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Validates: Requirements 1.1, 1.2, 1.3, 1.4, 1.5
// Property 1: Fault Condition — Package registry uses sqlx directly, bypassing database.SQL
//
// This exploration test encodes the EXPECTED behavior after the fix.
// On UNFIXED code it MUST FAIL — failure confirms the defect exists.

// registrySourcePath returns the absolute path to registry.go.
func registrySourcePath() string {
	_, filename, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(filename), "registry.go")
}

// parseRegistryAST parses registry.go and returns the AST file.
func parseRegistryAST(t *testing.T) *ast.File {
	t.Helper()
	src, err := os.ReadFile(registrySourcePath())
	if err != nil {
		t.Fatalf("failed to read registry.go: %v", err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "registry.go", src, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse registry.go: %v", err)
	}

	return f
}

// TestFaultCondition_RegistryStoresDatabaseSQL checks that the Registry struct
// stores *database.SQL (not *sqlx.DB). On unfixed code the field is `sql *sqlx.DB`.
func TestFaultCondition_RegistryStoresDatabaseSQL(t *testing.T) {
	rt := reflect.TypeFor[Registry]()

	// After the fix the field should be named "db" with type *database.SQL.
	field, ok := rt.FieldByName("db")
	if !ok {
		t.Fatal("Registry struct must have a field named 'db' (found none — defect: field is named 'sql' and stores *sqlx.DB)")
	}

	typeName := field.Type.String()
	if !strings.Contains(typeName, "database.SQL") {
		t.Fatalf("Registry.db must be *database.SQL, got %s", typeName)
	}
}

// TestFaultCondition_DBReturnsDatabaseSQL checks that DB() returns *database.SQL.
// On unfixed code DB() returns *sqlx.DB.
func TestFaultCondition_DBReturnsDatabaseSQL(t *testing.T) {
	rt := reflect.TypeFor[*Registry]()

	method, ok := rt.MethodByName("DB")
	if !ok {
		t.Fatal("Registry must have a DB() method")
	}

	// DB() should return exactly one value of type *database.SQL.
	mt := method.Type
	if mt.NumOut() != 1 {
		t.Fatalf("DB() must return 1 value, returns %d", mt.NumOut())
	}

	retType := mt.Out(0).String()
	if !strings.Contains(retType, "database.SQL") {
		t.Fatalf("DB() must return *database.SQL, got %s (defect: returns *sqlx.DB)", retType)
	}
}

// TestFaultCondition_RunMigrationsRemoved checks that the custom runMigrations
// function is removed from registry.go. On unfixed code it exists.
func TestFaultCondition_RunMigrationsRemoved(t *testing.T) {
	f := parseRegistryAST(t)

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == "runMigrations" {
			t.Fatal("function 'runMigrations' must be removed (defect: custom migration implementation exists)")
		}
	}
}

// TestFaultCondition_ExtractUpSectionRemoved checks that the custom extractUpSection
// function is removed from registry.go. On unfixed code it exists.
func TestFaultCondition_ExtractUpSectionRemoved(t *testing.T) {
	f := parseRegistryAST(t)

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == "extractUpSection" {
			t.Fatal("function 'extractUpSection' must be removed (defect: custom migration helper exists)")
		}
	}
}

// TestFaultCondition_ConfigStructRemoved checks that the custom Config struct
// is removed from registry.go. On unfixed code it exists.
func TestFaultCondition_ConfigStructRemoved(t *testing.T) {
	f := parseRegistryAST(t)

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if ts.Name.Name == "Config" {
				t.Fatal("type 'Config' must be removed (defect: custom config struct bypasses database.Connector)")
			}
		}
	}
}

// TestFaultCondition_GetUsesNoTxContext checks that the Get method body contains
// a call to NoTxContext (wrapping SQL queries for metrics/tracing).
// On unfixed code, Get calls r.sql.GetContext directly without NoTxContext.
func TestFaultCondition_GetUsesNoTxContext(t *testing.T) {
	f := parseRegistryAST(t)

	found := methodBodyContains(f, "Get", "NoTxContext")
	if !found {
		t.Fatal("Registry.Get must use NoTxContext wrapper (defect: calls sqlx.DB.GetContext directly)")
	}
}

// TestFaultCondition_ListUsesNoTxContext checks that the List method body contains
// a call to NoTxContext (wrapping SQL queries for metrics/tracing).
// On unfixed code, List calls r.sql.SelectContext directly without NoTxContext.
func TestFaultCondition_ListUsesNoTxContext(t *testing.T) {
	f := parseRegistryAST(t)

	found := methodBodyContains(f, "List", "NoTxContext")
	if !found {
		t.Fatal("Registry.List must use NoTxContext wrapper (defect: calls sqlx.DB.SelectContext directly)")
	}
}

// methodBodyContains checks if a method with the given name on any receiver
// contains a call or selector matching the target string in its body source.
func methodBodyContains(f *ast.File, methodName, target string) bool {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != methodName {
			continue
		}
		// Walk the AST body looking for the target identifier.
		return astContainsIdent(fn.Body, target)
	}

	return false
}

// astContainsIdent walks an AST node tree and returns true if any *ast.Ident
// matches the given name.
func astContainsIdent(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found {
			return false
		}
		ident, ok := n.(*ast.Ident)
		if ok && ident.Name == name {
			found = true

			return false
		}

		return true
	})

	return found
}
