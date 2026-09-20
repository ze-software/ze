// Design: docs/architecture/diagnostics/crash-capture.md -- the stderr a panic reaches

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestMainDefersTheCrashlogFlush reads main.go and asserts that main defers
// flushCrashlog.
//
// VALIDATES: a panic that nothing recovers still prints its trace.
// PREVENTS: exit status 2 with no output at all.
//
// crashlog.Init replaces fd 2 with a pipe, and a reader goroutine copies that
// pipe to the real stderr. The runtime writes a panic trace to fd 2 and then
// ends the process, so that goroutine is never scheduled and the trace dies in
// the pipe. Measured on 2026-09-20: the ExaBGP compatibility mock server
// panicked on three cases and the suite reported `exit status 2` with an empty
// stderr for each, which cost a whole bisection to recover
// (plan/journal/failing-gate-prints-no-cause.md).
//
// The check is structural because the behavior needs a process that dies. A
// deferred call runs while the panic unwinds, before the runtime prints, and
// Flush restores fd 2 to the real stderr. The ordinary path cannot rely on the
// defer, because os.Exit runs no deferred function, so main calls Flush twice
// and Flush acts once.
func TestMainDefersTheCrashlogFlush(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	body := mainFunctionBody(t, file)
	for _, statement := range body.List {
		deferred, isDefer := statement.(*ast.DeferStmt)
		if !isDefer {
			continue
		}
		name, isIdentifier := deferred.Call.Fun.(*ast.Ident)
		if !isIdentifier {
			continue
		}
		if name.Name == "flushCrashlog" {
			return
		}
	}
	t.Fatal("main does not defer flushCrashlog, so a panic exits 2 with its trace stuck in the crashlog pipe")
}

// mainFunctionBody returns the body of func main, and fails the test when the
// file holds no such function.
func mainFunctionBody(t *testing.T, file *ast.File) *ast.BlockStmt {
	t.Helper()
	for _, declaration := range file.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction {
			continue
		}
		if function.Recv != nil || function.Name.Name != "main" {
			continue
		}
		if function.Body == nil {
			t.Fatal("func main has no body")
		}
		return function.Body
	}
	t.Fatal("main.go declares no func main")
	return nil
}
