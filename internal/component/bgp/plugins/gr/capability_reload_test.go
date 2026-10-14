package gr

import (
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// grReloadSections delivers the bgp section of grConfig's one peer, peer1.
func grReloadSections(t *testing.T, sessionFamilies []string, gracefulRestartBody string) []sdk.ConfigSection {
	t.Helper()
	return []sdk.ConfigSection{{Root: "bgp", Data: deliverBGPSection(t, grConfig(sessionFamilies, gracefulRestartBody))}}
}

// TestGRCapabilityExtractorRegistered checks that the extractor and the leaves
// runGRPlugin registers make the SDK refuse a session family added on a peer
// whose graceful-restart container has no family list (AC-2d: the payload
// follows the session families), accept the same edit when an explicit family
// list keeps the payload (AC-2g), accept an unchanged config (AC-2e), and that
// gr's own refusal still covers a family the session does not carry: the SDK
// runs it first (TestDeclareCapabilitiesFromOwnVerifyAnswersFirst).
// VALIDATES: grCapabilities and grCapabilityLeaves through
// sdk.CheckCapabilityReload, and refuseUncarriedGRSections.
// PREVENTS: a family edit that reaches no OPEN accepted in silence, or an
// explicit family list refused for a payload that does not change.
func TestGRCapabilityExtractorRegistered(t *testing.T) {
	check := func(startup, candidate []sdk.ConfigSection) error {
		return sdk.CheckCapabilityReload("bgp-gr", grCapabilities, grCapabilityLeaves, startup, candidate)
	}
	oneFamily := []string{"ipv4/unicast"}
	explicit := "\t\t\t\t\tfamily {\n\t\t\t\t\t\tname ipv4/unicast\n\t\t\t\t\t}\n"

	startup := grReloadSections(t, oneFamily, "")
	if err := check(startup, grReloadSections(t, oneFamily, "")); err != nil {
		t.Fatalf("an unchanged graceful-restart was refused: %v", err)
	}

	err := check(startup, grReloadSections(t, twoFamilies, ""))
	if err == nil {
		t.Fatal("a session family added with no graceful-restart family list was accepted")
	}
	want := "peer peer1: capability graceful-restart cannot change while ze runs (plugin bgp-gr)."
	if first, _, _ := strings.Cut(err.Error(), "\n"); first != want {
		t.Errorf("refusal opens with %q, want %q", first, want)
	}

	startupExplicit := grReloadSections(t, oneFamily, explicit)
	if err := check(startupExplicit, grReloadSections(t, twoFamilies, explicit)); err != nil {
		t.Errorf("a session family added beside an explicit family list was refused: %v", err)
	}

	uncarried := grReloadSections(t, oneFamily, "\t\t\t\t\tfamily {\n\t\t\t\t\t\tname ipv6/unicast\n\t\t\t\t\t}\n")
	if refuseUncarriedGRSections(uncarried) == nil {
		t.Error("gr's own verify accepted a graceful-restart family the session does not carry")
	}
}
