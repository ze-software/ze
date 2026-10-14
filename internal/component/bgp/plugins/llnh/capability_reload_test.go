package llnh

import (
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

// TestLLNHCapabilityExtractorRegistered checks that the extractor and the
// leaves runLLNHPlugin registers make the SDK refuse a reload that removes
// link-local-nexthop from a live peer and accept one that keeps it (AC-2d,
// AC-2e), and that llnh's own refusal still covers a candidate the comparison
// would also refuse: the SDK runs it first (TestDeclareCapabilitiesFromOwnVerifyAnswersFirst).
// VALIDATES: llnhCapabilities and llnhCapabilityLeaves through
// sdk.CheckCapabilityReload, and refuseLinkLocalCapabilityWithoutAddressSections.
// PREVENTS: an llnh declaration the SDK cannot compare, or the missing-address
// refusal replaced by the restart text.
func TestLLNHCapabilityExtractorRegistered(t *testing.T) {
	startup := llnhReloadConfig(true, false)

	if err := sdk.CheckCapabilityReload("bgp-llnh", llnhCapabilities, llnhCapabilityLeaves,
		startup, llnhReloadConfig(true, false)); err != nil {
		t.Fatalf("an unchanged link-local-nexthop was refused: %v", err)
	}

	err := sdk.CheckCapabilityReload("bgp-llnh", llnhCapabilities, llnhCapabilityLeaves,
		startup, llnhReloadConfig(false, false))
	if err == nil {
		t.Fatal("removing link-local-nexthop from a live peer was accepted")
	}
	want := "peer edge: capability link-local-nexthop cannot change while ze runs (plugin bgp-llnh)."
	if first, _, _ := strings.Cut(err.Error(), "\n"); first != want {
		t.Errorf("refusal opens with %q, want %q", first, want)
	}

	// Enabling the capability on a peer with no Link-Local address is a
	// change the comparison refuses too; llnh's own handler answers it first.
	candidate := llnhReloadConfig(true, true)
	if err := sdk.CheckCapabilityReload("bgp-llnh", llnhCapabilities, llnhCapabilityLeaves,
		startup, candidate); err == nil {
		t.Error("enabling link-local-nexthop on peer bare was accepted by the comparison")
	}
	own := refuseLinkLocalCapabilityWithoutAddressSections(candidate)
	if own == nil {
		t.Fatal("llnh's own verify accepted a peer with no Link-Local address")
	}
	if !strings.Contains(own.Error(), "bare") {
		t.Errorf("llnh's own refusal %q does not name peer bare", own)
	}
}
