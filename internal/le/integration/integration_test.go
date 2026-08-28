// Related: gates.go, actions.go -- the table and dispatch these tests drive.
//
// VALIDATES: native lab callbacks and external Go commands share one action
// table, preserve selector input and exact codes, and refuse an accidental
// aggregate.
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
// This set needs hours of Docker, root, namespace, and QEMU work, so bare input
// must not start it.
func TestTheBareAreaRefusesToRunEverything(t *testing.T) {
	answer, code := Answer(nil)
	if code != 2 {
		t.Errorf("a bare `le integration` answered %d, want the refusal's 2", code)
	}
	listing, ok := answer.(leaction.List)
	if !ok {
		t.Fatalf("the refusal answered %T, want the listing so a reader sees the names", answer)
	}
	if len(listing.Actions) != len(Table()) {
		t.Errorf("the refusal listed %d actions for %d declared", len(listing.Actions), len(Table()))
	}
}

// TestEveryActionDeclaresOneRunnerAndAReason pins that every row has exactly
// one implementation: either a native callback or an external toolchain command.
func TestEveryActionDeclaresOneRunnerAndAReason(t *testing.T) {
	env.ResetCache()
	for _, gate := range Table() {
		hasNative := gate.Native != nil
		hasCommand := gate.Argv != nil
		if hasCommand {
			hasCommand = len(gate.Argv()) > 0
		}
		if hasNative == hasCommand {
			t.Errorf("action %s must declare exactly one runner", gate.Verb)
		}
		if gate.Why == "" {
			t.Errorf("action %s states no reason", gate.Verb)
		}
		if strings.HasPrefix(gate.Verb, "ze-") {
			t.Errorf("action %s still carries a removed Make target prefix", gate.Verb)
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
			if gate.needsCgo() {
				t.Errorf("native action %s requested command-runner cgo", gate.Verb)
			}
			continue
		}
		want := slices.Contains(gate.Argv(), "-race")
		if gate.needsCgo() != want {
			t.Errorf("action %s: needsCgo()=%v while its argv %v", gate.Verb, gate.needsCgo(), gate.Argv())
		}
		if gate.needsCgo() {
			raced++
		}
	}
	if raced == 0 {
		t.Error("no external command in this area is race-instrumented, so the derivation is vacuous")
	}
}

// TestGeneralInteropSelectorReachesTheNativeOptions preserves direct
// INTEROP_SCENARIO selection without building a script argv.
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

// TestNativeActionsUseNativeCallbacks proves the exact migrated rows publish no
// Python or sudo command.
func TestNativeActionsUseNativeCallbacks(t *testing.T) {
	native := map[string]bool{}
	for _, gate := range Table() {
		if gate.Native != nil {
			native[gate.Verb] = true
		}
	}
	for _, name := range []string{
		"interop-ipsec",
		"interop",
		"stress-bird",
		StressAction,
	} {
		action := actionNamed(t, name)
		if !native[name] {
			t.Errorf("%s has no native callback", name)
		}
		if action.Argv != nil {
			t.Errorf("%s still publishes an external command: %v", name, action.Argv())
		}
	}
}

// TestNativeRunnerPreservesRootPayloadAndCode proves the action adapter does
// not flatten a leaf action's verdict.
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

// TestEveryVerbIsTypeableAndUnique verifies the native dispatch names.
func TestEveryVerbIsTypeableAndUnique(t *testing.T) {
	env.ResetCache()
	seen := map[string]bool{}
	for _, row := range Actions().Actions {
		if seen[row.Verb] {
			t.Errorf("two actions share the verb %q, so one is unreachable", row.Verb)
		}
		seen[row.Verb] = true
	}
	for _, verb := range []string{"iface", "interop", StressAction, "stress-bird"} {
		if !seen[verb] {
			t.Errorf("native action %q is absent", verb)
		}
	}
}

// TestAMistypedActionIsRefusedWithTwo keeps a name this area does not hold apart
// from an action that ran and failed.
func TestAMistypedActionIsRefusedWithTwo(t *testing.T) {
	env.ResetCache()
	if _, code := Answer([]string{"no-such-action"}); code != 2 {
		t.Errorf("a mistyped action answered %d, want 2", code)
	}
}

// actionNamed answers one action of the table, or fails the test.
func actionNamed(t *testing.T, name string) Action {
	t.Helper()
	for _, action := range Table() {
		if action.Verb == name {
			return action
		}
	}
	t.Fatalf("this area declares no action called %s", name)
	return Action{}
}
