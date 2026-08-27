// Related: gates.go, actions.go -- the table and the dispatch these tests drive
//
// VALIDATES: native lab callbacks and external Go commands share one gate table,
// preserve selector input and exact codes, and refuse an accidental aggregate.
// PREVENTS: a Python or sudo runner returning through a row marked native.

package integration

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/leaction"
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

// TestEveryGateDeclaresOneRunnerAndAReason pins that every row has exactly one
// implementation: either a native callback or an external toolchain command.
func TestEveryGateDeclaresOneRunnerAndAReason(t *testing.T) {
	env.ResetCache()
	for _, gate := range Table() {
		hasNative := gate.Native != nil
		hasCommand := gate.Argv != nil
		if hasCommand {
			hasCommand = len(gate.Argv()) > 0
		}
		if hasNative == hasCommand {
			t.Errorf("gate %s must declare exactly one runner", gate.Name)
		}
		if gate.Why == "" {
			t.Errorf("gate %s states no reason", gate.Name)
		}
		if !strings.HasPrefix(gate.Name, "ze-") {
			t.Errorf("gate %s does not carry the ze- spelling every caller uses", gate.Name)
		}
	}
}

// TestCgoIsDerivedFromTheExternalCommand verifies that only external race
// commands request cgo from the shared command runner.
func TestCgoIsDerivedFromTheExternalCommand(t *testing.T) {
	env.ResetCache()
	raced := 0
	for _, gate := range Table() {
		if gate.Native != nil {
			if gate.NeedsCgo() {
				t.Errorf("native gate %s requested command-runner cgo", gate.Name)
			}
			continue
		}
		want := slices.Contains(gate.Argv(), "-race")
		if gate.NeedsCgo() != want {
			t.Errorf("gate %s: NeedsCgo()=%v while its argv %v", gate.Name, gate.NeedsCgo(), gate.Argv())
		}
		if gate.NeedsCgo() {
			raced++
		}
	}
	if raced == 0 {
		t.Error("no external command in this area is race-instrumented, so the derivation is vacuous")
	}
}

// TestGeneralInteropSelectorReachesTheNativeOptions preserves the Make
// producer's INTEROP_SCENARIO selection without rebuilding a script argv.
func TestGeneralInteropSelectorReachesTheNativeOptions(t *testing.T) {
	env.ResetCache()
	if chosen := generalInteropOptions().Scenario; chosen != "" {
		t.Errorf("an unset INTEROP_SCENARIO selected %q", chosen)
	}

	t.Setenv("INTEROP_SCENARIO", "bgp-ebgp-ipv4-frr")
	env.ResetCache()
	if chosen := generalInteropOptions().Scenario; chosen != "bgp-ebgp-ipv4-frr" {
		t.Errorf("INTEROP_SCENARIO selected %q", chosen)
	}
}

// TestFormerScriptGatesUseNativeCallbacks proves the exact three migrated rows
// publish no Python or sudo command.
func TestFormerScriptGatesUseNativeCallbacks(t *testing.T) {
	native := map[string]bool{}
	for _, gate := range Table() {
		if gate.Native != nil {
			native[gate.Name] = true
		}
	}
	for _, name := range []string{
		"ze-interop-ipsec-test",
		"ze-interop-test",
		"ze-stress-bird-test",
	} {
		gate := gateNamed(t, name)
		if !native[name] {
			t.Errorf("%s has no native callback", name)
		}
		if gate.Argv != nil {
			t.Errorf("%s still publishes an external command: %v", name, gate.Argv())
		}
	}
}

// TestNativeRunnerPreservesRootPayloadAndCode proves the action adapter does
// not flatten a leaf gate's verdict.
func TestNativeRunnerPreservesRootPayloadAndCode(t *testing.T) {
	const root = "/fixture/root"
	called := false
	run := nativeRunner(root, func(_ context.Context, gotRoot string) (any, int) {
		called = true
		if gotRoot != root {
			t.Errorf("native root = %q, want %q", gotRoot, root)
		}
		return "payload", 37
	})
	payload, code := run()
	if !called {
		t.Fatal("native callback was not called")
	}
	if payload != "payload" {
		t.Errorf("payload = %#v", payload)
	}
	if code != 37 {
		t.Errorf("code = %d, want 37", code)
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
