package doctor

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/claims"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// treeWith builds a config tree with one container per named root, each holding
// one leaf, so the root carries operator data.
func treeWith(roots ...string) *config.Tree {
	t := config.NewTree()
	for _, r := range roots {
		c := t.GetOrCreateContainer(r)
		c.Set("enabled", "true")
	}
	return t
}

// TestConfigClaimsCheckReportsUnclaimedRoot drives the doctor check from its
// entry point with a config root no claim covers.
//
// Server.reloadConfig accepts such a root, logs Info "config reload: no
// affected plugins, updating config" and stores it
// (internal/component/plugin/server/reload.go). This check is what upgrades
// that silence to something an operator can see.
//
// VALIDATES: AC-7 -- `ze doctor` reports an unclaimed config root with a
// registered diagnostic code.
// PREVENTS: config that a running daemon stores and delivers to nobody staying
// invisible until somebody asks why the feature does nothing.
func TestConfigClaimsCheckReportsUnclaimedRoot(t *testing.T) {
	tree := treeWith("nobody-claims-this")
	cs := []claims.Claim{{Path: "bgp", Source: "plugin:bgp"}}

	diags := configClaimDiagnostics(tree, cs, nil)

	if len(diags) != 1 {
		t.Fatalf("want 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Code != diagnosticConfigRootUnclaimed {
		t.Errorf("want code %q, got %q", diagnosticConfigRootUnclaimed, diags[0].Code)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Errorf("want a warning, got %q", diags[0].Severity)
	}
	if !strings.Contains(diags[0].Message, "nobody-claims-this") {
		t.Errorf("diagnostic must name the root: %s", diags[0].Message)
	}
}

// TestConfigClaimsCheckSilentOnClaimedRoot checks the check does not fire on
// config that is delivered.
//
// VALIDATES: AC-7 negative -- a claimed root, and an allowlisted root, are
// silent.
// PREVENTS: a doctor warning on every working configuration.
func TestConfigClaimsCheckSilentOnClaimedRoot(t *testing.T) {
	cs := []claims.Claim{{Path: "bgp", Source: "plugin:bgp"}}
	allow := []claims.Allow{{Path: "system", Reason: "read in-daemon", Owner: "config/system"}}

	if diags := configClaimDiagnostics(treeWith("bgp"), cs, allow); len(diags) != 0 {
		t.Errorf("claimed root must not be reported: %v", diags)
	}
	if diags := configClaimDiagnostics(treeWith("system"), cs, allow); len(diags) != 0 {
		t.Errorf("allowlisted root must not be reported: %v", diags)
	}
}

// TestConfigClaimsCheckFailsClosedWithNoClaims checks the check reports that it
// could not run rather than passing.
//
// With no claim inventory every root looks unclaimed, which says nothing. A
// guard that returns "clean" when it cannot see its subject reads as a pass
// (ai/rules/fail-closed-guards.md).
//
// VALIDATES: fail-closed behavior of the doctor check.
// PREVENTS: a silent doctor on a build whose registry did not populate.
func TestConfigClaimsCheckFailsClosedWithNoClaims(t *testing.T) {
	diags := configClaimDiagnostics(treeWith("bgp"), nil, nil)

	if len(diags) != 1 {
		t.Fatalf("want 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Code != diagnosticConfigClaimsUnavailable {
		t.Errorf("want code %q, got %q", diagnosticConfigClaimsUnavailable, diags[0].Code)
	}
}

// TestConfigClaimsCheckHandlesEmptyTree checks the check says nothing about a
// config that has nothing in it.
//
// VALIDATES: no diagnostic for a nil or empty tree.
// PREVENTS: `ze doctor` on a fresh box reporting a problem that is not there.
func TestConfigClaimsCheckHandlesEmptyTree(t *testing.T) {
	cs := []claims.Claim{{Path: "bgp", Source: "plugin:bgp"}}
	if diags := configClaimDiagnostics(nil, cs, nil); len(diags) != 0 {
		t.Errorf("nil tree must produce no diagnostic: %v", diags)
	}
	if diags := configClaimDiagnostics(config.NewTree(), cs, nil); len(diags) != 0 {
		t.Errorf("empty tree must produce no diagnostic: %v", diags)
	}
}

// TestConfigClaimsCheckCodesRegistered checks both codes reach `ze explain`.
//
// VALIDATES: AC-7 -- the diagnostic codes this check emits are registered, so
// `ze explain <code>` describes them.
// PREVENTS: a diagnostic an operator cannot look up.
func TestConfigClaimsCheckCodesRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	for _, code := range []string{diagnosticConfigRootUnclaimed, diagnosticConfigClaimsUnavailable} {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is emitted but not registered", code)
		}
	}
}

// TestConfigClaimsCheckWiredIntoDoctor proves the check runs.
//
// A check nobody calls is not a check. checkConfigClaims is the entry point
// runChecks calls; this drives it end to end over a live claim inventory.
//
// VALIDATES: AC-7 wiring -- checkConfigClaims resolves its own claim inventory
// and returns a diagnostic for an unclaimed root.
// PREVENTS: the check existing but never running.
func TestConfigClaimsCheckWiredIntoDoctor(t *testing.T) {
	diags := checkConfigClaims(treeWith("definitely-not-a-config-root"))

	found := false
	for _, d := range diags {
		if d.Code == diagnosticConfigRootUnclaimed &&
			strings.Contains(d.Message, "definitely-not-a-config-root") {
			found = true
		}
	}
	if !found {
		t.Errorf("checkConfigClaims did not report the unclaimed root: %v", diags)
	}
}
