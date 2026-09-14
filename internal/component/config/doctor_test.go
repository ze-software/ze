// Design: docs/features/ai-first.md -- the readiness check this component owns
// Detail: doctor.go -- checkSemantics and its registration

package config

import (
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// mcpOAuthTree builds an enabled environment.mcp block whose auth-mode names
// OAuth without the leaves OAuth needs, which MCPConfig.Validate refuses.
func mcpOAuthTree() *Tree {
	tree := NewTree()
	env := tree.GetOrCreateContainer("environment")
	mcp := env.GetOrCreateContainer("mcp")
	mcp.Set("enabled", "true")
	mcp.Set("auth-mode", "oauth")
	srv := NewTree()
	srv.Set("ip", "127.0.0.1")
	srv.Set("port", "6274")
	mcp.AddListEntry("server", "default", srv)
	return tree
}

// TestCheckSemanticsReportsAnInvalidMCPBlock drives the check over a config
// whose MCP block fails its own consistency check. It carries the assertion
// TestDoctorConfigValidationBridge made in the doctor package before the check
// moved here.
//
// VALIDATES: the diagnostic carries config-mcp-invalid, the code the validator
// emits, with no doctor- alias in between.
// PREVENTS: a config the daemon refuses at boot passing `ze doctor` clean.
func TestCheckSemanticsReportsAnInvalidMCPBlock(t *testing.T) {
	diags := checkSemantics(diagnostic.DoctorCheckContext{Tree: mcpOAuthTree()})

	found := false
	for i := range diags {
		if diags[i].Code == diagnostic.CodeConfigMCPInvalid {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics = %v, want one carrying %q", diags, diagnostic.CodeConfigMCPInvalid)
	}
}

// hasDiagCode reports whether checkSemantics emitted the code for the tree.
func hasDiagCode(t *testing.T, tree *Tree, code string) bool {
	t.Helper()
	diags := checkSemantics(diagnostic.DoctorCheckContext{Tree: tree})
	for i := range diags {
		if diags[i].Code == code {
			return true
		}
	}
	return false
}

// TestCheckSemanticsFlagsGnmiExposure drives the gNMI exposure finding from
// the doctor check's entry point, not from GNMIListenConfig.Validate: the
// defect this closes was a Validate no entry point called
// (ai/rules/evidence.md, test the guard from its entry point).
//
// VALIDATES: AC-6 -- `ze doctor` reports a tokenless non-loopback gNMI
// listener with config-gnmi-invalid.
// PREVENTS: doctor answering "ready" on a config the daemon refuses to boot.
func TestCheckSemanticsFlagsGnmiExposure(t *testing.T) {
	if !hasDiagCode(t, gnmiTree("0.0.0.0", ""), diagnostic.CodeConfigGNMIInvalid) {
		t.Fatal("doctor must flag a tokenless 0.0.0.0 gNMI listener")
	}
}

// TestCheckSemanticsGnmiLoopbackAndTokenAreClean is the negative half: the
// check must not over-report, because loopback needs no token and a token
// authenticates any address.
//
// VALIDATES: no config-gnmi-invalid for a loopback listener or an
// authenticated one.
// PREVENTS: a warning on every authenticated or loopback gNMI deployment.
func TestCheckSemanticsGnmiLoopbackAndTokenAreClean(t *testing.T) {
	if hasDiagCode(t, gnmiTree("127.0.0.1", ""), diagnostic.CodeConfigGNMIInvalid) {
		t.Fatal("a loopback gNMI listener exposes nothing off-box")
	}
	if hasDiagCode(t, gnmiTree("0.0.0.0", "s3cret"), diagnostic.CodeConfigGNMIInvalid) {
		t.Fatal("a token authenticates every gNMI request, so the bind address is free")
	}
}

// TestCheckSemanticsIsSilentWithoutATree is the negative half: the check runs
// in the post-config phase only, and a context with no tree names nothing to
// validate.
//
// VALIDATES: no diagnostic for a nil tree and for a tree with nothing to
// validate.
// PREVENTS: a nil dereference inside a doctor phase the check was never
// written for.
func TestCheckSemanticsIsSilentWithoutATree(t *testing.T) {
	if diags := checkSemantics(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkSemantics(diagnostic.DoctorCheckContext{Tree: NewTree()}); len(diags) != 0 {
		t.Fatalf("empty tree: diagnostics = %d, want 0", len(diags))
	}
}

// TestSemanticsDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this component's check, at the phase and order the
// declaration states, and whether every code it declares resolves for
// `ze explain`.
//
// VALIDATES: the init() in doctor.go installed the check and the registry
// accepted its config-* codes.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestSemanticsDoctorCheckRegistered(t *testing.T) {
	want := semanticsDoctorCheck()
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
	}
	if found.Order != want.Order {
		t.Fatalf("order = %d, want %d", found.Order, want.Order)
	}
	if found.Component != want.Component {
		t.Fatalf("component = %q, want %q", found.Component, want.Component)
	}

	diagnostic.RegisterBuiltinCodes()
	for _, code := range want.Codes {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is not registered", code)
		}
	}
}
