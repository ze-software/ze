package fixture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tacacsmock "github.com/ze-software/ze/internal/test/mock/tacacs"
)

// VALIDATES: a TACACS+ observer cannot end before the mock has taken the
// records its .ci asserts. Four facts together:
//   - pluginShellExtra4TacacsDriver waits on pluginShellExtra4AwaitTacacs
//     before the scenario's verdict stands;
//   - every observer that driver runs names its records in tacacsExpectations4;
//   - every `tacacs-mock:` line a tacacs .ci expects is the Line of one of its
//     observer's expectations, as ze relays it (inside a quoted slog message);
//   - plugin_shell_extra_4.go calls no time.Sleep.
//
// PREVENTS: the 300 to 800ms sleeps these observers ended on, the longest of
// which (tacacs-acct) was all that kept the asynchronous ACCT STOP ahead of
// ze's shutdown.
// METHOD: parse the fixture source and read the tacacs .ci files.
func TestTacacsObserversWaitForWhatTheirCIExpects(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "plugin_shell_extra_4.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	functions := map[string]*ast.FuncDecl{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			functions[fn.Name.Name] = fn
		}
	}

	driver := functions["pluginShellExtra4TacacsDriver"]
	if driver == nil {
		t.Fatal("pluginShellExtra4TacacsDriver is gone: the scan no longer finds the TACACS+ driver")
	}
	if !calls4(driver.Body, "pluginShellExtra4AwaitTacacs") {
		t.Error("pluginShellExtra4TacacsDriver never calls pluginShellExtra4AwaitTacacs, so an observer can end before the mock took its records")
	}
	if sleeps := sleepCalls4(fset, file); len(sleeps) > 0 {
		t.Errorf("plugin_shell_extra_4.go sleeps at %s: wait on the record or state the assertion needs instead", strings.Join(sleeps, ", "))
	}

	driven := 0
	for _, decl := range functions {
		ast.Inspect(decl.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			if callee, ok := call.Fun.(*ast.Ident); !ok || callee.Name != "pluginShellExtra4TacacsDriver" {
				return true
			}
			literal, ok := call.Args[1].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				t.Fatalf("%s calls pluginShellExtra4TacacsDriver with a non-literal name", decl.Name.Name)
			}
			name, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			driven++
			checkTacacsCI4(t, name)
			return true
		})
	}
	if driven == 0 {
		t.Fatal("no call of pluginShellExtra4TacacsDriver found: the scan no longer finds the observers")
	}
}

// checkTacacsCI4 fails for an observer with no expectations, and for a
// `tacacs-mock:` line its .ci expects that none of them renders.
func checkTacacsCI4(t *testing.T, name string) {
	t.Helper()
	expectations, known := tacacsExpectations4[name]
	if !known {
		t.Errorf("%s is driven by the TACACS+ mock and names no records in tacacsExpectations4", name)
		return
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "plugin", name+".ci"))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	for _, pattern := range mockPatterns4.FindAllStringSubmatch(string(data), -1) {
		expected, err := regexp.Compile(pattern[1])
		if err != nil {
			t.Fatalf("%s: pattern %q: %v", name, pattern[1], err)
		}
		if !renderedByAny4(expected, expectations) {
			t.Errorf("%s.ci expects %q and the observer waits for no record whose line it matches", name, pattern[1])
		}
	}
}

// mockPatterns4 matches an expect line naming a mock stderr line.
var mockPatterns4 = regexp.MustCompile(`(?m)^expect=stderr:pattern=(tacacs-mock: .*)$`)

// renderedByAny4 reports whether pattern matches one expectation's line as ze
// relays it: the plugin's stderr line becomes a quoted slog message, so the
// .ci escapes the line's own quotes.
func renderedByAny4(pattern *regexp.Regexp, expectations []tacacsExpect4) bool {
	for _, expect := range expectations {
		if pattern.MatchString(strconv.Quote(expect.record.Line())) {
			return true
		}
	}
	return false
}

func calls4(body *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if callee, ok := call.Fun.(*ast.Ident); ok && callee.Name == name {
				found = true
			}
		}
		return !found
	})
	return found
}

func sleepCalls4(fset *token.FileSet, file *ast.File) []string {
	var sleeps []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Sleep" {
			return true
		}
		if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "time" {
			sleeps = append(sleeps, fset.Position(call.Pos()).String())
		}
		return true
	})
	return sleeps
}

// VALIDATES: tacacsMissing4 counts only the records an expectation names, so
// the wait cannot be satisfied by a different user's login, a different
// command's authorization, or too few accounting records.
func TestTacacsMissingCountsOnlyMatchingRecords(t *testing.T) {
	expectations := []tacacsExpect4{
		{record: tacacsmock.Record{Kind: tacacsmock.KindAuthen, User: "noc", Pass: true}, count: 1},
		{record: tacacsmock.Record{Kind: tacacsmock.KindAuthor, User: "admin", Command: "show bgp", Pass: true}, count: 1},
		{record: tacacsmock.Record{Kind: tacacsmock.KindAcctStop}, count: 2},
	}
	records := []tacacsmock.Record{
		{Kind: tacacsmock.KindAuthen, User: "admin", Pass: true},
		{Kind: tacacsmock.KindAuthen, User: "noc"},
		{Kind: tacacsmock.KindAuthor, User: "admin", Command: "show bgp"},
		{Kind: tacacsmock.KindAuthor, User: "admin", Command: "clear", Pass: true},
		{Kind: tacacsmock.KindAcctStart},
		{Kind: tacacsmock.KindAcctStop},
	}
	if missing := tacacsMissing4(expectations, records); len(missing) != 3 {
		t.Fatalf("missing = %q, want all three expectations unmet", missing)
	}
	records = append(records,
		tacacsmock.Record{Kind: tacacsmock.KindAuthen, User: "noc", Pass: true},
		tacacsmock.Record{Kind: tacacsmock.KindAuthor, User: "admin", Command: "show bgp", Pass: true},
		tacacsmock.Record{Kind: tacacsmock.KindAcctStop},
	)
	if missing := tacacsMissing4(expectations, records); len(missing) != 0 {
		t.Fatalf("missing = %q, want none once every record arrived", missing)
	}
}
