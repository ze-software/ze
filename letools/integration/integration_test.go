// Related: gates.go, actions.go -- the table and the dispatch these tests drive
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11.
// The first failed gate keeps its exit code, and each argv matches mk/test-integration.mk.
// The tests also verify the refusal of an accidental aggregate run.
// PREVENTS: race tests without cgo, lost scenario variables, and an unrequested aggregate run.

package integration

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/letools/leaction"
)

// TestTheBareAreaRefusesToRunEverything verifies this area's refusal.
// Other areas run their complete set when no gate is named.
// This set needs hours of Docker, root, namespace, and QEMU work, so bare input must not start it.
func TestTheBareAreaRefusesToRunEverything(t *testing.T) {
	answer, code := Answer(nil)
	if code != 2 {
		t.Errorf("a bare `le integration` answered %d, want the refusal's 2", code)
	}
	listing, ok := answer.(leaction.List)
	if !ok {
		t.Fatalf("the refusal answered %T, want the listing so a reader sees the names", answer)
	}
	if len(listing.Actions) != len(Gates()) {
		t.Errorf("the refusal listed %d gates for %d declared", len(listing.Actions), len(Gates()))
	}
}

// TestEveryGateDeclaresACommandAndAReason pins that no row of the table is half
// written: an empty argv runs nothing, and an empty reason renders the listing
// blank.
func TestEveryGateDeclaresACommandAndAReason(t *testing.T) {
	env.ResetCache()
	for _, gate := range Table() {
		if len(gate.Argv()) == 0 {
			t.Errorf("gate %s declares no command", gate.Name)
		}
		if gate.Why == "" {
			t.Errorf("gate %s states no reason", gate.Name)
		}
		if !strings.HasPrefix(gate.Name, "ze-") {
			t.Errorf("gate %s does not carry the ze- spelling every doc and shim uses", gate.Name)
		}
	}
}

// TestCgoIsDerivedFromTheCommandRatherThanDeclared verifies that the command controls its environment.
// A suite that stops using the race detector stops requesting cgo.
// A race run without cgo cannot start.
func TestCgoIsDerivedFromTheCommandRatherThanDeclared(t *testing.T) {
	env.ResetCache()
	for _, gate := range Table() {
		want := slices.Contains(gate.Argv(), "-race")
		if gate.NeedsCgo() != want {
			t.Errorf("gate %s: NeedsCgo()=%v while its argv %v", gate.Name, gate.NeedsCgo(), gate.Argv())
		}
	}
	raced := 0
	for _, gate := range Table() {
		if gate.NeedsCgo() {
			raced++
		}
	}
	if raced == 0 {
		t.Error("no gate in this area is race-instrumented, so the derivation is vacuous")
	}
}

// TestAScenarioVariableSelectsOneScenario pins the translation the port owes:
// `make ze-interop-test INTEROP_SCENARIO=x` puts the variable in the recipe's
// environment, so the two spellings must build one argv.
func TestAScenarioVariableSelectsOneScenario(t *testing.T) {
	env.ResetCache()
	bare := gateNamed(t, "ze-interop-test").Argv()
	if len(bare) != 2 {
		t.Errorf("an unset INTEROP_SCENARIO built %v, want the bare runner", bare)
	}

	t.Setenv("INTEROP_SCENARIO", "bgp-ebgp-ipv4-frr")
	env.ResetCache()
	chosen := gateNamed(t, "ze-interop-test").Argv()
	if len(chosen) != 3 || chosen[2] != "bgp-ebgp-ipv4-frr" {
		t.Errorf("INTEROP_SCENARIO built %v, want the runner plus the one scenario", chosen)
	}
}

// TestSudoCarriesTheTwoVariablesThroughItself verifies that VERBOSE and SESSION_TIMEOUT are sudo arguments.
// Sudo does not preserve this inherited environment, so exported variables would disappear.
func TestSudoCarriesTheTwoVariablesThroughItself(t *testing.T) {
	t.Setenv("VERBOSE", "1")
	t.Setenv("SESSION_TIMEOUT", "90")
	env.ResetCache()

	argv := gateNamed(t, "ze-stress-bird-test").Argv()
	if argv[0] != "sudo" {
		t.Fatalf("the stress gate runs %q first, want sudo", argv[0])
	}
	if argv[1] != "VERBOSE=1" || argv[2] != "SESSION_TIMEOUT=90" {
		t.Errorf("sudo was handed %v, want both variables as its own arguments", argv[1:3])
	}

	// Spelled even when empty, which is what the Make recipe expanded to.
	t.Setenv("VERBOSE", "")
	t.Setenv("SESSION_TIMEOUT", "")
	env.ResetCache()
	empty := gateNamed(t, "ze-stress-bird-test").Argv()
	if empty[1] != "VERBOSE=" || empty[2] != "SESSION_TIMEOUT=" {
		t.Errorf("an unset pair built %v, want both spelled empty", empty[1:3])
	}
}

// TestEveryVerbIsTypeableAndUnique verifies the dispatch naming rule.
// A verb removes this area's prefix from its gate name.
// A gate without that prefix keeps its complete name instead of gaining an invented short form.
func TestEveryVerbIsTypeableAndUnique(t *testing.T) {
	env.ResetCache()
	seen := map[string]bool{}
	for _, row := range Actions().Actions {
		if seen[row.Verb] {
			t.Errorf("two gates share the verb %q, so one is unreachable", row.Verb)
		}
		seen[row.Verb] = true
	}
	if !seen["iface-test"] {
		t.Error("ze-integration-iface-test did not shorten to iface-test")
	}
	if !seen["ze-interop-test"] {
		t.Error("ze-interop-test lost its full name, which is what every doc and shim spells")
	}
}

// TestAMistypedGateIsRefusedWithTwo keeps a name this area does not hold apart
// from a gate that ran and failed.
func TestAMistypedGateIsRefusedWithTwo(t *testing.T) {
	env.ResetCache()
	if _, code := Answer([]string{"no-such-gate"}); code != 2 {
		t.Errorf("a mistyped gate answered %d, want 2", code)
	}
}

// gateNamed answers one gate of the table, or fails the test.
func gateNamed(t *testing.T, name string) Gate {
	t.Helper()
	for _, gate := range Table() {
		if gate.Name == name {
			return gate
		}
	}
	t.Fatalf("this area declares no gate called %s", name)
	return Gate{}
}
