// VALIDATES: every construction-registry service -- MCP among them -- is built,
// and so binds its listener, only AFTER apiServer.WaitForStartupComplete
// returns. That wait is released by signalStartupComplete
// (internal/component/plugin/server/startup.go, func signalStartupComplete),
// which calls cr.Freeze() on the dispatcher command registry BEFORE it closes
// startupDone. So the call order asserted here is the whole reason an MCP
// client cannot reach tools/list while a plugin can still register a command.
// PREVENTS: moving the buildServices call back above the wait, and restoring
// the inline startMCPServer call main.go used to carry (service_mcp.go, the
// buildMCPService doc comment). Either one lets MCP serve a command list the
// registry has not frozen, and the ordering arrived incidentally rather than as
// a deliberate fix, so nothing but this test holds it in place.
//
// Why a source-order ratchet rather than a behavioral test: the defect is a
// WINDOW between the bind and the freeze, and a test cannot deterministically
// enter it -- a client that connects during the window sees the full command
// list whenever no plugin happens to register late, which is the normal case.
// test/plugin/mcp-tools-list-deterministic-order.ci is green with the bind on
// either side of the wait for exactly that reason. The property an edit
// actually moves is the call order in run(), so that is what is pinned.

package hub

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestServicesBindAfterCommandRegistryFreeze pins the order of the two calls in
// the startup function: the startup wait, then the service construction that
// binds listeners. The function is found by its buildServices call rather than
// by name, so renaming or splitting it keeps the ratchet pointed at the real
// bind site instead of quietly passing over an empty body.
func TestServicesBindAfterCommandRegistryFreeze(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	name, body := funcCalling(fset, file, "buildServices")
	if body == nil {
		t.Fatal("no function in main.go calls buildServices: the construction registry is driven from somewhere this ratchet cannot see, so nothing orders a service bind after the startup wait")
	}
	t.Logf("service construction runs in %s (main.go)", name)

	waits := directCallLines(fset, body, "WaitForStartupComplete")
	builds := directCallLines(fset, body, "buildServices")

	// Exactly one of each, or the ratchet cannot say which wait gates which
	// build. A second call site is a real change of shape and must be reviewed
	// here rather than silently satisfied by whichever one comes first.
	if len(waits) != 1 {
		t.Fatalf("%s calls WaitForStartupComplete %d times directly (want 1), at lines %v: the wait that gates service construction is no longer identifiable", name, len(waits), waits)
	}
	if len(builds) != 1 {
		t.Fatalf("%s calls buildServices %d times (want 1), at lines %v", name, len(builds), builds)
	}
	if waits[0] > builds[0] {
		t.Fatalf("%s builds services at line %d BEFORE waiting for plugin startup at line %d: every registered service, MCP included, binds its listener while the dispatcher command registry is still writable, so a client can be served a command list that a late plugin registration then changes",
			name, builds[0], waits[0])
	}
}

// TestMCPBindsOnlyThroughTheServiceFactory proves the ordering above actually
// governs MCP: its listener is created by startMCPServer, and buildMCPService
// is the only caller, so the bind cannot happen anywhere but inside
// buildServices.
func TestMCPBindsOnlyThroughTheServiceFactory(t *testing.T) {
	callers := packageCallers(t, "startMCPServer")
	want := "service_mcp.go:buildMCPService"

	if len(callers) != 1 || callers[0] != want {
		t.Fatalf("startMCPServer is called from %v (want exactly [%s]): a bind outside buildMCPService is not driven by buildServices, so nothing orders it after the startup wait and MCP can serve before the command registry freezes",
			callers, want)
	}
}

// TestMCPFactoryIsRegisteredUnderTheServiceName closes the last link of the
// chain: buildServices drives buildMCPService because register_mcp.go hands it
// to the construction registry.
func TestMCPFactoryIsRegisteredUnderTheServiceName(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "register_mcp.go", nil, 0)
	if err != nil {
		t.Fatalf("parse register_mcp.go: %v", err)
	}

	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || calleeName(call.Fun) != "registerService" || len(call.Args) < 2 {
			return true
		}
		name, ok := call.Args[0].(*ast.BasicLit)
		if !ok || name.Value != `"mcp"` {
			return true
		}
		if calleeName(call.Args[1]) == "buildMCPService" {
			found = true
		}
		return true
	})

	if !found {
		t.Fatal(`register_mcp.go no longer calls registerService("mcp", buildMCPService, ...): MCP is built outside the construction registry, so buildServices no longer orders its bind after the startup wait`)
	}
}

// funcCalling returns the name and body of the first top-level function in file
// whose own flow calls callee.
func funcCalling(fset *token.FileSet, file *ast.File, callee string) (string, *ast.BlockStmt) {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if len(directCallLines(fset, fn.Body, callee)) > 0 {
			return fn.Name.Name, fn.Body
		}
	}
	return "", nil
}

// directCallLines reports the lines of calls to name that run in body's own
// flow. Closures are skipped: a call inside a FuncLit runs when that closure is
// invoked, which says nothing about the order of body's statements. main.go's
// reloadAfterCommitContext closure waits for startup too, and counting it would
// make this ratchet green with the real wait moved anywhere at all.
func directCallLines(fset *token.FileSet, body *ast.BlockStmt, name string) []int {
	var lines []int
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if ok && calleeName(call.Fun) == name {
			lines = append(lines, fset.Position(call.Pos()).Line)
		}
		return true
	})
	return lines
}

// packageCallers returns "<file>:<func>" for every non-test file in this
// package that calls name. Build tags do not gate the parse, so the answer is
// the same in every feature-set build.
func packageCallers(t *testing.T, name string) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	var callers []string
	fset := token.NewFileSet()
	for _, entry := range entries {
		fileName := entry.Name()
		if entry.IsDir() || filepath.Ext(fileName) != ".go" || strings.HasSuffix(fileName, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, fileName, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", fileName, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if len(directCallLines(fset, fn.Body, name)) > 0 {
				callers = append(callers, fileName+":"+fn.Name.Name)
			}
		}
	}
	return callers
}

// calleeName returns the identifier a call expression names, without its
// receiver or package qualifier.
func calleeName(e ast.Expr) string {
	switch fn := e.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}
