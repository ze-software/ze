//go:build ze_core

// Design: docs/architecture/plugin/plugin-system.md -- the registry seals after start-up

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// registryImportPath is the package whose Seal main MUST call.
const registryImportPath = "github.com/ze-software/ze/internal/component/plugin/registry"

// TestMainSealsBeforeDispatch proves that main seals the plugin registry
// before it dispatches any command. Method: parse main.go, resolve the local
// name of the registry import, and find, among the top-level statements of
// main, the one that calls <registry>.Seal() and the one that calls
// dispatchMain. A seal inside a branch, after dispatch, or missing leaves a
// window in which run-time code could still register, so each is a failure.
// It reads the source rather than running main, because main ends the process.
func TestMainSealsBeforeDispatch(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	registryName := ""
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("main.go import %s: %v", spec.Path.Value, err)
		}
		if path != registryImportPath {
			continue
		}
		registryName = "registry"
		if spec.Name != nil {
			registryName = spec.Name.Name
		}
	}
	if registryName == "" {
		t.Fatalf("main.go does not import %s, so main cannot seal the registry", registryImportPath)
	}

	var body *ast.BlockStmt
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == "main" {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("main.go declares no func main")
	}

	sealAt, dispatchAt := -1, -1
	for index, statement := range body.List {
		if sealAt < 0 && callsSeal(statement, registryName) {
			sealAt = index
		}
		if dispatchAt < 0 && containsCall(statement, "dispatchMain") {
			dispatchAt = index
		}
	}
	if sealAt < 0 {
		t.Fatalf("main has no top-level statement %s.Seal(): the registry stays open after start-up", registryName)
	}
	if dispatchAt < 0 {
		t.Fatal("main has no top-level statement calling dispatchMain: this test no longer reads the dispatch it guards")
	}
	if sealAt > dispatchAt {
		t.Errorf("main calls %s.Seal() at statement %d, after dispatchMain at statement %d", registryName, sealAt, dispatchAt)
	}
}

// callsSeal reports whether statement is exactly the call <registryName>.Seal().
func callsSeal(statement ast.Stmt, registryName string) bool {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := expression.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	return receiver.Name == registryName && selector.Sel.Name == "Seal" && len(call.Args) == 0
}

// containsCall reports whether statement calls the package function name anywhere.
func containsCall(statement ast.Stmt, name string) bool {
	found := false
	ast.Inspect(statement, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if ok && ident.Name == name {
			found = true
		}
		return !found
	})
	return found
}
