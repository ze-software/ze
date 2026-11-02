package role

import (
	"slices"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// roleReloadConfig is a bgp root with peer edge (remote 192.0.2.1) importing
// as role, and peer core carrying no role. With added, peer late (remote
// 192.0.2.9) imports as customer too. Role keys its declaration by the remote
// IP (extractPeerRoleConfigs), so a peer needs one to declare at all.
func roleReloadConfig(role string, added bool) []sdk.ConfigSection {
	late := ""
	if added {
		late = `,"late":{"connection":{"remote":{"ip":"192.0.2.9"}},"role":{"import":"customer"}}`
	}
	return []sdk.ConfigSection{{Root: "bgp", Data: `{"bgp":{"peer":{
		"edge":{"connection":{"remote":{"ip":"192.0.2.1"}},"role":{"import":"` + role + `"}},
		"core":{}` + late + `
	}}}`}}
}

// rolePayload answers the payload the candidate declares for selector under
// the role code, and whether it declares one.
func rolePayload(t *testing.T, sections []sdk.ConfigSection, selector string) (string, bool) {
	t.Helper()
	declared, err := sdk.CandidateCapabilities(roleCapabilities, sections)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range declared.Capabilities {
		if decl.Code == roleCapCode && slices.Contains(decl.Peers, selector) {
			return decl.Payload, true
		}
	}
	return "", false
}

// TestRoleCapabilityExtractorRegistered checks that the extractor
// RunRolePlugin registers gives a peer a reload adds its role capability, and
// a live peer whose role a reload changes its new role, in the declaration
// the config-verify answer carries (AC-2).
// VALIDATES: roleCapabilities through sdk.CandidateCapabilities, the
// declaration the composed config-verify answers.
// PREVENTS: a peer added on reload opening with no role capability, or a
// changed role answered with the startup value.
func TestRoleCapabilityExtractorRegistered(t *testing.T) {
	startup := roleReloadConfig("provider", false)
	if _, ok := rolePayload(t, startup, "192.0.2.9"); ok {
		t.Fatal("the startup config declares a role for a peer it does not name")
	}
	before, ok := rolePayload(t, startup, "192.0.2.1")
	if !ok {
		t.Fatal("the startup config declares no role for peer edge")
	}

	added := roleReloadConfig("provider", true)
	got, ok := rolePayload(t, added, "192.0.2.9")
	if !ok {
		t.Fatal("the peer the reload adds carries no role capability")
	}
	// RFC 9234 Section 4.1: Customer is role value 3.
	if got != "03" {
		t.Errorf("added peer's role payload %q, want 03 (customer)", got)
	}
	if kept, _ := rolePayload(t, added, "192.0.2.1"); kept != before {
		t.Errorf("adding a peer changed edge's role payload %q to %q", before, kept)
	}

	changed, ok := rolePayload(t, roleReloadConfig("customer", false), "192.0.2.1")
	if !ok {
		t.Fatal("a changed role declares no role for peer edge")
	}
	if changed == before {
		t.Errorf("a role changed from provider to customer still declares %q", changed)
	}
}
