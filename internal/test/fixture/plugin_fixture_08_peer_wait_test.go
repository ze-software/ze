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

// VALIDATES: every family-08 observer whose .ci scripts a BGP peer that expects
// ze's wire output waits for that peer's initial-sync End-of-RIB before it ends,
// for each scripted peer. A single `stdin=peer:` block is ze's peer1; a
// `stdin=peerN:` block is ze's peerN, which is how the multi-peer .ci files name
// both sides. waitPeerEOR08 waits for peer1, and waitPeersEOR08 for every peer
// its string-literal arguments name.
// Observe runs the scenario as soon as the plugins are ready, then quiesces and
// asks the daemon to stop. `request quiesce` reports a peer whose TCP connect
// has not completed as settled (docs/architecture/api/commands.md, "What the
// barrier does NOT cover"), and docs/functional-tests.md puts the wait on the
// observer: "Before requesting shutdown, an observer waits for the peer state
// or sent-payload counter its wire assertions require."
// PREVENTS: test/plugin/iface-tunnel-kinds.ci and tunnel-ttl-default.ci failing
// with the scripted peer still waiting for its End-of-RIB: their scenarios read
// only kernel links, finished within ~130ms, and ze sent Cease/Administrative
// Shutdown to a session that had not reached Established. Also
// test/plugin/forward-mpreach-nexthop-self-two-peer.ci, whose observer slept a
// fixed two seconds before its emit and three after it, with no reading of
// either session.
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
		scripted := scriptedPeers08(source)
		if len(scripted) == 0 || !strings.Contains(source, "\nexpect=bgp:") {
			continue
		}
		decl := functions[function]
		if decl == nil {
			t.Fatalf("%s registers %s, which the scan did not find", fixture, function)
		}
		checked++
		waited := waitedPeers08(decl.Body)
		for _, peer := range scripted {
			if !slices.Contains(waited, peer) {
				missing = append(missing, fixture+" ("+function+") never waits for "+peer)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no observer scripts a peer: the .ci scan no longer finds the tests")
	}
	slices.Sort(missing)
	for _, entry := range missing {
		t.Errorf("%s: its .ci peer expects ze's wire output, so the scenario must wait for that peer's End-of-RIB (waitPeerEOR08 for peer1, waitPeersEOR08 for several)", entry)
	}
}

// scriptedPeerBlock08 matches a .ci stdin block that scripts a BGP peer:
// `stdin=peer:` or `stdin=peer<N>:`.
var scriptedPeerBlock08 = regexp.MustCompile(`(?m)^stdin=(peer\d*):`)

// scriptedPeers08 answers the ze peer names a .ci scripts a BGP peer for. The
// unnumbered block is ze's peer1, by the convention every single-peer .ci of
// this family follows.
func scriptedPeers08(source string) []string {
	var peers []string
	for _, match := range scriptedPeerBlock08.FindAllStringSubmatch(source, -1) {
		name := match[1]
		if name == "peer" {
			name = "peer1"
		}
		if !slices.Contains(peers, name) {
			peers = append(peers, name)
		}
	}
	return peers
}

// waitedPeers08 answers the peers a scenario body waits for: peer1 for a
// waitPeerEOR08 call, and every string-literal argument of a waitPeersEOR08 call.
func waitedPeers08(body *ast.BlockStmt) []string {
	var peers []string
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
		case "waitPeerEOR08":
			peers = append(peers, "peer1")
		case "waitPeersEOR08":
			for _, arg := range call.Args {
				literal, ok := arg.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				if name, err := strconv.Unquote(literal.Value); err == nil {
					peers = append(peers, name)
				}
			}
		}
		return true
	})
	return peers
}
