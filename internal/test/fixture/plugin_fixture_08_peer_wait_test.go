package fixture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// VALIDATES: every family-08 observer whose .ci scripts a BGP peer that expects
// ze's wire output waits for peer1's initial-sync End-of-RIB before it ends.
// Observe runs the scenario as soon as the plugins are ready, then quiesces and
// asks the daemon to stop. `request quiesce` reports a peer whose TCP connect
// has not completed as settled (docs/architecture/api/commands.md, "What the
// barrier does NOT cover"), and docs/functional-tests.md puts the wait on the
// observer: "Before requesting shutdown, an observer waits for the peer state
// or sent-payload counter its wire assertions require."
// PREVENTS: test/plugin/iface-tunnel-kinds.ci and tunnel-ttl-default.ci failing
// with the scripted peer still waiting for its End-of-RIB: their scenarios read
// only kernel links, finished within ~130ms, and ze sent Cease/Administrative
// Shutdown to a session that had not reached Established.
// METHOD: parse the package source, map each registerPlugin08 name to its
// scenario function, and read the .ci that runs it.
func TestPluginObservers08WaitForTheScriptedPeer(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	functions := map[string]*ast.FuncDecl{}
	scenarios := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				functions[fn.Name.Name] = fn
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 3 {
				return true
			}
			if callee, ok := call.Fun.(*ast.Ident); !ok || callee.Name != "registerPlugin08" {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			scenario, isIdent := call.Args[2].(*ast.Ident)
			if !ok || !isIdent {
				return true // a built scenario (metricsScenario08, forwardLoad08) carries its own wait
			}
			fixture, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			scenarios[fixture] = scenario.Name
			return true
		})
	}
	if len(scenarios) == 0 {
		t.Fatal("no registerPlugin08 call found: the scan no longer reads the registrations")
	}

	checked := 0
	var missing []string
	for fixture, function := range scenarios {
		path := filepath.Join("..", "..", "..", "test", fixture+".ci")
		data, err := os.ReadFile(path)
		if err != nil {
			continue // a fixture another directory's .ci runs is outside this scan
		}
		source := string(data)
		if !strings.Contains(source, "\nstdin=peer:") || !strings.Contains(source, "\nexpect=bgp:") {
			continue
		}
		decl := functions[function]
		if decl == nil {
			t.Fatalf("%s registers %s, which the scan did not find", fixture, function)
		}
		checked++
		waits := false
		ast.Inspect(decl.Body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if callee, ok := call.Fun.(*ast.Ident); ok && callee.Name == "waitPeerEOR08" {
					waits = true
				}
			}
			return !waits
		})
		if !waits {
			missing = append(missing, fixture+" ("+function+")")
		}
	}
	if checked == 0 {
		t.Fatal("no observer scripts a peer: the .ci scan no longer finds the tests")
	}
	slices.Sort(missing)
	for _, entry := range missing {
		t.Errorf("%s ends without waiting for peer1's End-of-RIB its .ci peer expects: call waitPeerEOR08 first", entry)
	}
}
