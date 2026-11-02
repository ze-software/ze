package gr

import (
	"slices"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// grReloadSections delivers the bgp section of grConfig's one peer, peer1.
func grReloadSections(t *testing.T, sessionFamilies []string, gracefulRestartBody string) []sdk.ConfigSection {
	t.Helper()
	return []sdk.ConfigSection{{Root: "bgp", Data: deliverBGPSection(t, grConfig(sessionFamilies, gracefulRestartBody))}}
}

// grPayload answers the payload the candidate declares for peer1 under the
// graceful-restart code, and whether it declares one.
func grPayload(t *testing.T, sections []sdk.ConfigSection) (string, bool) {
	t.Helper()
	declared, err := sdk.CandidateCapabilities(grCapabilities, sections)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range declared.Capabilities {
		if decl.Code == grCapCode && slices.Contains(decl.Peers, "peer1") {
			return decl.Payload, true
		}
	}
	return "", false
}

// TestGRCapabilityExtractorRegistered checks that the extractor runGRPlugin
// registers answers, in the declaration the config-verify answer carries, the
// payload a session family edit gives a peer whose graceful-restart container
// has no family list (AC-2: the payload follows the session families), keeps
// the payload when an explicit family list holds it (AC-2g), and that gr's own
// refusal still covers a family the session does not carry: the SDK runs it
// first (TestDeclareCapabilitiesFromOwnVerifyAnswersFirst).
// VALIDATES: grCapabilities through sdk.CandidateCapabilities, and
// refuseUncarriedGRSections.
// PREVENTS: a family edit whose graceful-restart payload never reaches the
// OPEN, or an explicit family list whose payload a session edit rewrites.
func TestGRCapabilityExtractorRegistered(t *testing.T) {
	oneFamily := []string{"ipv4/unicast"}
	explicit := "\t\t\t\t\tfamily {\n\t\t\t\t\t\tname ipv4/unicast\n\t\t\t\t\t}\n"

	before, ok := grPayload(t, grReloadSections(t, oneFamily, ""))
	if !ok {
		t.Fatal("the startup config declares no graceful-restart for peer1")
	}
	after, ok := grPayload(t, grReloadSections(t, twoFamilies, ""))
	if !ok {
		t.Fatal("a session family edit declares no graceful-restart for peer1")
	}
	if after == before {
		t.Errorf("a session family added with no graceful-restart family list still declares %q", after)
	}

	explicitBefore, ok := grPayload(t, grReloadSections(t, oneFamily, explicit))
	if !ok {
		t.Fatal("an explicit family list declares no graceful-restart for peer1")
	}
	if explicitAfter, _ := grPayload(t, grReloadSections(t, twoFamilies, explicit)); explicitAfter != explicitBefore {
		t.Errorf("a session family added beside an explicit family list changed the payload %q to %q", explicitBefore, explicitAfter)
	}

	uncarried := grReloadSections(t, oneFamily, "\t\t\t\t\tfamily {\n\t\t\t\t\t\tname ipv6/unicast\n\t\t\t\t\t}\n")
	if refuseUncarriedGRSections(uncarried) == nil {
		t.Error("gr's own verify accepted a graceful-restart family the session does not carry")
	}
}
