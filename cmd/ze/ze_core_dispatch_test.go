//go:build ze_core

package main

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// ensureLocalCommandsRegistered registers the cmd/ze-owned root and local
// commands (pipe, start, version, help, --plugins, update serve) exactly once
// into the shared test-binary registry. registerLocalCommands runs from zeSetup
// in production, which `go test` never calls, so these roots are otherwise
// absent from the process-wide registry the cmd/ze tests share (see
// root_dispatch_test.go). The LookupRoot("start") guard keeps the call
// idempotent, so a second invocation (another test, or `go test -count=2`) never
// panics on MustRegisterRootHandler's duplicate check.
func ensureLocalCommandsRegistered() {
	if registry.LookupRoot("start") == nil {
		registerLocalCommands()
	}
}

// TestRootsRegistered proves the root namespace obeys the grammar after the
// rename: `pipe` and `traffic` resolve as roots, and none of the five old
// mis-named / hyphenated-namespace roots survive. The negative half is exactly
// what AC-2 and AC-9 rest on. The isis-decode / ospf-decode roots are gated
// behind ze_isis / ze_ospf, so their positive registration (isis / ospf) is
// asserted in the owning packages' tests (TestISISSubDispatch /
// TestOSPFSubDispatch); listing them here still proves the old names are gone in
// any build that had them.
func TestRootsRegistered(t *testing.T) {
	ensureLocalCommandsRegistered()

	for _, name := range []string{"pipe", "traffic"} {
		if registry.LookupRoot(name) == nil {
			t.Errorf("root %q must be registered", name)
		}
	}
	for _, name := range []string{"format", "traffic-control", "isis-decode", "ospf-decode", "update-serve"} {
		if registry.LookupRoot(name) != nil {
			t.Errorf("old root %q must be gone (no alias)", name)
		}
	}
}

// TestUpdateServeLocalRegistered proves `update serve` moved from the root
// registry to the local-handler registry: registry.LookupLocal resolves the
// two-word path and returns the trailing flags unchanged, so
// `ze update serve --listen <addr>` reaches runUpdateServe with --listen intact
// (R-3). `update` is a YANG verb, so a root handler named `update` would be
// unreachable behind the isYANGVerb branch; the local registry is consulted
// first by RunCommand, which is why this is a local handler, not a root.
func TestUpdateServeLocalRegistered(t *testing.T) {
	ensureLocalCommandsRegistered()

	handler, rest := registry.LookupLocal([]string{"update", "serve", "--listen", ":9999"})
	if handler == nil {
		t.Fatal("`update serve` must resolve as a local handler")
	}
	want := []string{"--listen", ":9999"}
	if len(rest) != len(want) || rest[0] != want[0] || rest[1] != want[1] {
		t.Errorf("local handler remaining args = %v, want %v", rest, want)
	}
}
