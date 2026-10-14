package role

import (
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// roleReloadConfig is a bgp root with peer edge (remote 192.0.2.1) importing
// as role, and peer core carrying no role. Role keys its declaration by the
// remote IP (extractPeerRoleConfigs), so the peer needs one to declare at all.
func roleReloadConfig(role string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: "bgp", Data: `{"bgp":{"peer":{
		"edge":{"connection":{"remote":{"ip":"192.0.2.1"}},"role":{"import":"` + role + `"}},
		"core":{}
	}}}`}}
}

// TestRoleCapabilityExtractorRegistered checks that the extractor and the
// leaves RunRolePlugin registers make the SDK refuse a reload that changes a
// live peer's role and accept one that keeps it (AC-2d, AC-2e).
// VALIDATES: roleCapabilities and roleCapabilityLeaves through
// sdk.CheckCapabilityReload, the comparison the composed config-verify runs.
// PREVENTS: a role declaration the SDK cannot compare, or a refusal that names
// a code number instead of the role leaf.
func TestRoleCapabilityExtractorRegistered(t *testing.T) {
	startup := roleReloadConfig("provider")

	if err := sdk.CheckCapabilityReload("bgp-role", roleCapabilities, roleCapabilityLeaves,
		startup, roleReloadConfig("provider")); err != nil {
		t.Fatalf("an unchanged role was refused: %v", err)
	}

	err := sdk.CheckCapabilityReload("bgp-role", roleCapabilities, roleCapabilityLeaves,
		startup, roleReloadConfig("customer"))
	if err == nil {
		t.Fatal("a changed role was accepted")
	}
	want := "peer 192.0.2.1: capability role cannot change while ze runs (plugin bgp-role)."
	if first, _, _ := strings.Cut(err.Error(), "\n"); first != want {
		t.Errorf("refusal opens with %q, want %q", first, want)
	}
}
