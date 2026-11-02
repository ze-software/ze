package llnh

import (
	"slices"
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// llnhReloadConfig is a bgp root with peer edge, which carries a Link-Local
// address and, when enabled, link-local-nexthop, and peer bare, which carries
// link-local-nexthop only when bareEnabled and never a Link-Local address.
func llnhReloadConfig(edgeEnabled, bareEnabled bool) []sdk.ConfigSection {
	edge := `"edge":{"session":{"link-local":"fe80::1"}}`
	if edgeEnabled {
		edge = `"edge":{"session":{"link-local":"fe80::1","capability":{"link-local-nexthop":"enable"}}}`
	}
	bare := `"bare":{}`
	if bareEnabled {
		bare = `"bare":{"session":{"capability":{"link-local-nexthop":"enable"}}}`
	}
	return []sdk.ConfigSection{{Root: "bgp", Data: `{"bgp":{"peer":{` + edge + `,` + bare + `}}}`}}
}

// llnhDeclared reports whether the candidate declares link-local-nexthop for
// selector.
func llnhDeclared(t *testing.T, sections []sdk.ConfigSection, selector string) bool {
	t.Helper()
	declared, err := sdk.CandidateCapabilities(llnhCapabilities, sections)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range declared.Capabilities {
		if decl.Code == llnhCapCode && slices.Contains(decl.Peers, selector) {
			return true
		}
	}
	return false
}

// TestLLNHCapabilityExtractorRegistered checks that the extractor
// runLLNHPlugin registers declares link-local-nexthop for a peer a reload
// enables it on and drops it for a peer a reload disables it on, in the
// declaration the config-verify answer carries (AC-2), and that llnh's own
// refusal still covers a peer with no Link-Local address: the SDK runs it
// first (TestDeclareCapabilitiesFromOwnVerifyAnswersFirst).
// VALIDATES: llnhCapabilities through sdk.CandidateCapabilities, and
// refuseLinkLocalCapabilityWithoutAddressSections.
// PREVENTS: a reload-enabled link-local-nexthop that never reaches the OPEN,
// a disabled one still sent, or the missing-address refusal lost.
func TestLLNHCapabilityExtractorRegistered(t *testing.T) {
	if llnhDeclared(t, llnhReloadConfig(false, false), "edge") {
		t.Fatal("a disabled link-local-nexthop is declared for peer edge")
	}
	if !llnhDeclared(t, llnhReloadConfig(true, false), "edge") {
		t.Fatal("a reload that enables link-local-nexthop on peer edge declares nothing for it")
	}

	candidate := llnhReloadConfig(true, true)
	own := refuseLinkLocalCapabilityWithoutAddressSections(candidate)
	if own == nil {
		t.Fatal("llnh's own verify accepted a peer with no Link-Local address")
	}
	if !strings.Contains(own.Error(), "bare") {
		t.Errorf("llnh's own refusal %q does not name peer bare", own)
	}
}
