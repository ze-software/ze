package fixture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// VALIDATES: every family-12 observer whose .ci scripts a BGP peer that expects
// ze's wire output waits, by name, on that peer's session before it ends, for
// each scripted peer. A scripted peer is a stdin block a `le test peer` command
// line runs; the ze peer it stands for is the configured peer whose remote
// address is the address that command binds (127.0.0.1 unless --bind names
// one), or whose local address is the one a --dial command connects to. A wait
// names a peer through p12WaitPeersEOR's arguments, or through the selector of
// a p12WaitPeerCounter call ("*" names the peer only when the .ci scripts one).
// String literals and package string constants both count.
// Observe runs the scenario as soon as the plugins are ready, then quiesces and
// asks the daemon to stop. `request quiesce` reports a peer whose TCP connect
// has not completed as settled (docs/architecture/api/commands.md, "What the
// barrier does NOT cover"), and docs/functional-tests.md puts the wait on the
// observer: "Before requesting shutdown, an observer waits for the peer state
// or sent-payload counter its wire assertions require."
// PREVENTS: the family-12 L2TP redistribution observers (announce, withdraw,
// multi-peer next-hop) sleeping a fixed two to three seconds around their emit
// with no reading of the session, and observers that read only L2TP config,
// the RIB or a redistribution consumer ending while the scripted peer may still
// be connecting, so ze sends Cease to a peer still waiting for its messages.
// METHOD: parse the package source, map each family-12 Register call to the
// function that holds its scenario, read the .ci that runs it, and compare the
// peers it scripts with the peers the scenario waits on.
func TestPluginObservers12WaitForTheScriptedPeer(t *testing.T) {
	source := parseFixturePackage12(t)
	if len(source.registrations) == 0 {
		t.Fatal("no family-12 Register call found: the scan no longer reads the registrations")
	}

	checked := 0
	var missing []string
	for fixture, function := range source.registrations {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "test", fixture+".ci"))
		if err != nil {
			continue // a trigger helper or a fixture another directory's .ci runs
		}
		ci := string(data)
		if !strings.Contains(ci, "\nexpect=bgp:") {
			continue
		}
		scripted := scriptedPeers12(t, fixture, ci)
		if len(scripted) == 0 {
			continue
		}
		body := source.body(function)
		if body == nil {
			t.Fatalf("%s registers %s, which the scan did not find", fixture, function.describe())
		}
		checked++
		waited := source.waitedPeers12(body)
		for _, peer := range scripted {
			if peer.waitedOn(waited, len(scripted)) {
				continue
			}
			missing = append(missing, fixture+" ("+function.describe()+") never waits for "+peer.name)
		}
	}
	if checked == 0 {
		t.Fatal("no family-12 observer scripts a peer: the .ci scan no longer finds the tests")
	}
	slices.Sort(missing)
	for _, entry := range missing {
		t.Errorf("%s: its .ci peer expects ze's wire output, so the scenario must wait for that peer by name (p12WaitPeersEOR)", entry)
	}
}

// scenarioRef12 names where a registration's scenario lives: a package
// function, or a function literal written in place.
type scenarioRef12 struct {
	function string
	literal  *ast.FuncLit
}

func (ref scenarioRef12) describe() string {
	if ref.literal != nil {
		return "an inline scenario"
	}
	return ref.function
}

// fixturePackage12 is what the guard reads from the package source.
type fixturePackage12 struct {
	functions     map[string]*ast.FuncDecl
	constants     map[string]string
	registrations map[string]scenarioRef12
}

func (source fixturePackage12) body(ref scenarioRef12) *ast.BlockStmt {
	if ref.literal != nil {
		return ref.literal.Body
	}
	if decl := source.functions[ref.function]; decl != nil {
		return decl.Body
	}
	return nil
}

// parseFixturePackage12 reads every function and string constant of the
// package, and every Register call in a plugin_fixture_12 file.
func parseFixturePackage12(t *testing.T) fixturePackage12 {
	t.Helper()
	source := fixturePackage12{
		functions:     map[string]*ast.FuncDecl{},
		constants:     map[string]string{},
		registrations: map[string]scenarioRef12{},
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var family []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		source.collect(file)
		if strings.HasPrefix(name, "plugin_fixture_12") {
			family = append(family, file)
		}
	}
	for _, file := range family {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			if callee, ok := call.Fun.(*ast.Ident); !ok || callee.Name != "Register" {
				return true
			}
			fixture, ok := source.stringValue(call.Args[0])
			if !ok {
				t.Fatalf("a family-12 Register call names its fixture with a non-constant: %T", call.Args[0])
			}
			if ref, ok := scenarioOf12(t, fixture, call.Args[1]); ok {
				source.registrations[fixture] = ref
			}
			return true
		})
	}
	return source
}

// collect records a file's package functions and string constants.
func (source fixturePackage12) collect(file *ast.File) {
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			if typed.Recv == nil {
				source.functions[typed.Name.Name] = typed
			}
		case *ast.GenDecl:
			if typed.Tok != token.CONST {
				continue
			}
			for _, spec := range typed.Specs {
				values, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, name := range values.Names {
					if index >= len(values.Values) {
						break
					}
					literal, ok := values.Values[index].(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						continue
					}
					if text, err := strconv.Unquote(literal.Value); err == nil {
						source.constants[name.Name] = text
					}
				}
			}
		}
	}
}

// stringValue answers a string literal's text, or a package constant's.
func (source fixturePackage12) stringValue(expr ast.Expr) (string, bool) {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.STRING {
			return "", false
		}
		text, err := strconv.Unquote(typed.Value)
		return text, err == nil
	case *ast.Ident:
		text, ok := source.constants[typed.Name]
		return text, ok
	}
	return "", false
}

// scenarioOf12 answers where a registered driver's scenario lives. The
// observe and subscribed drivers take it as their last argument, a package
// function or the call of one that builds it; any other driver is the scenario
// itself. p12WaitForDaemonDriver is left out: its observer stays alive until
// the daemon says goodbye, so it never ends the run.
func scenarioOf12(t *testing.T, fixture string, driver ast.Expr) (scenarioRef12, bool) {
	t.Helper()
	call, isCall := driver.(*ast.CallExpr)
	if !isCall {
		return refOf12(t, fixture, driver), true
	}
	callee, ok := call.Fun.(*ast.Ident)
	if !ok {
		t.Fatalf("%s registers a driver this scan cannot resolve: %T", fixture, call.Fun)
	}
	switch callee.Name {
	case "p12WaitForDaemonDriver":
		return scenarioRef12{}, false
	case "p12ObserveDriver", "p12SubscribedDriver":
		return refOf12(t, fixture, call.Args[len(call.Args)-1]), true
	}
	return scenarioRef12{function: callee.Name}, true
}

func refOf12(t *testing.T, fixture string, expr ast.Expr) scenarioRef12 {
	t.Helper()
	switch typed := expr.(type) {
	case *ast.Ident:
		return scenarioRef12{function: typed.Name}
	case *ast.CallExpr:
		if callee, ok := typed.Fun.(*ast.Ident); ok {
			return scenarioRef12{function: callee.Name}
		}
	case *ast.FuncLit:
		return scenarioRef12{literal: typed}
	}
	t.Fatalf("%s registers a scenario this scan cannot resolve: %T", fixture, expr)
	return scenarioRef12{}
}

// anyPeer12 is the p12WaitPeerCounter selector that sums every peer.
const anyPeer12 = "*"

// waitedPeers12 answers the peer names and addresses a scenario body waits on.
func (source fixturePackage12) waitedPeers12(body *ast.BlockStmt) []string {
	var waited []string
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		switch callee.Name {
		case "p12WaitPeersEOR":
			for _, arg := range call.Args {
				if name, ok := source.stringValue(arg); ok {
					waited = append(waited, name)
				}
			}
		case "p12WaitPeerCounter":
			if len(call.Args) > 2 {
				if selector, ok := source.stringValue(call.Args[2]); ok {
					waited = append(waited, selector)
				}
			}
		}
		return true
	})
	return waited
}

// scriptedPeer12 is one ze peer a .ci scripts the far side of.
type scriptedPeer12 struct {
	name    string
	address string
}

// waitedOn reports whether waited names this peer, by name or by address. The
// "*" selector names it only when the .ci scripts no other peer, because a sum
// over several peers can reach its minimum from one of them.
func (peer scriptedPeer12) waitedOn(waited []string, scripted int) bool {
	for _, entry := range waited {
		if entry == peer.name {
			return true
		}
		if entry == peer.address {
			return true
		}
		if entry == anyPeer12 && scripted == 1 {
			return true
		}
	}
	return false
}

var (
	// peerCommand12 matches a `le test peer` command line, capturing its
	// arguments and the stdin block it runs.
	peerCommand12 = regexp.MustCompile(`(?m)^cmd=[a-z]+:.*exec=le test peer (.*):stdin=([A-Za-z0-9_-]+)\s*$`)
	bindFlag12    = regexp.MustCompile(`--bind\s+(\S+)`)
	dialFlag12    = regexp.MustCompile(`--dial\s+(\S+):\$PORT`)
	peerBlock12   = regexp.MustCompile(`(?m)^\s*peer\s+([A-Za-z0-9_.-]+)\s*\{`)
	remoteIP12    = regexp.MustCompile(`remote\s*\{\s*ip\s+([^\s;}]+)`)
	localIP12     = regexp.MustCompile(`local\s*\{\s*ip\s+([^\s;}]+)`)
)

// scriptedPeers12 answers the ze peers a .ci scripts, each once.
func scriptedPeers12(t *testing.T, fixture, ci string) []scriptedPeer12 {
	t.Helper()
	var peers []scriptedPeer12
	for _, command := range peerCommand12.FindAllStringSubmatch(ci, -1) {
		arguments := command[1]
		var peer scriptedPeer12
		var found bool
		if dial := dialFlag12.FindStringSubmatch(arguments); dial != nil {
			peer, found = configuredPeer12(ci, localIP12, dial[1])
		} else {
			address := addrLoopback
			if bind := bindFlag12.FindStringSubmatch(arguments); bind != nil {
				address = bind[1]
			}
			peer, found = configuredPeer12(ci, remoteIP12, address)
		}
		if !found {
			t.Fatalf("%s runs `le test peer %s` for stdin=%s, and no configured peer matches its address", fixture, arguments, command[2])
		}
		if !slices.Contains(peers, peer) {
			peers = append(peers, peer)
		}
	}
	return peers
}

// configuredPeer12 answers the first configured peer whose address, as field
// reads it from the peer's block, is address.
func configuredPeer12(ci string, field *regexp.Regexp, address string) (scriptedPeer12, bool) {
	blocks := peerBlock12.FindAllStringSubmatchIndex(ci, -1)
	for index, block := range blocks {
		end := len(ci)
		if index+1 < len(blocks) {
			end = blocks[index+1][0]
		}
		match := field.FindStringSubmatch(ci[block[1]:end])
		if len(match) < 2 {
			continue
		}
		if match[1] != address {
			continue
		}
		remote := remoteIP12.FindStringSubmatch(ci[block[1]:end])
		if remote == nil {
			continue
		}
		return scriptedPeer12{name: ci[block[2]:block[3]], address: remote[1]}, true
	}
	return scriptedPeer12{}, false
}
