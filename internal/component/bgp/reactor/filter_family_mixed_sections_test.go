package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/filter_family"
	"github.com/ze-software/ze/internal/component/bgp/policychain"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// TestFamilyMixedStripApplied runs the registered family-filter SDK plugin and
// real ingress executor, then reads all four sections of the resulting bytes.
// Every case keeps positive reachability of the family that was not removed.
func TestFamilyMixedStripApplied(t *testing.T) {
	const base = "4001010040020602010000fde940030401010101"
	const reach6 = "800e1c0002011020010db8000000000000000000000001003020010db80001"
	const reach4 = "800e0d000101040101010100180a0001"
	const unreach6 = "800f0a0002013020010db80002"
	const unreach4 = "800f07000101180a0002"
	r := New(&Config{LocalAS: 65000})
	r.configTree = map[string]any{
		"bgp": map[string]any{},
		"policy": map[string]any{"family-filter": map[string]any{
			"ipv4/unicast": map[string]any{"family": "ipv4/unicast", "action": "remove"},
			"ipv6/unicast": map[string]any{"family": "ipv6/unicast", "action": "remove"},
		}},
	}
	policyFinalPluginServer(t, r, "bgp-filter-family")
	for _, tc := range []struct {
		name, strip, attrs, announce, withdrawn string
		kept                                    nlriSection
		want                                    string
	}{
		{"legacy announce", "ipv4/unicast", base + reach6, "180a0001", "", sectionMPReach, "3020010db80001"},
		{"legacy withdraw", "ipv4/unicast", base + reach6, "", "180a0002", sectionMPReach, "3020010db80001"},
		{"both legacy", "ipv4/unicast", base + reach6, "180a0001", "180a0002", sectionMPReach, "3020010db80001"},
		{"target MP withdraw", "ipv6/unicast", base + reach4 + unreach6, "", "", sectionMPReach, "180a0001"},
		{"target MP reach", "ipv6/unicast", base + reach6 + unreach4, "180a0001", "", sectionAnnounce, "180a0001"},
		{"target v4 MP withdraw", "ipv4/unicast", base + reach6 + unreach4, "", "", sectionMPReach, "3020010db80001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := makeUpdateBody(nlriHex(t, tc.withdrawn), nlriHex(t, tc.attrs), nlriHex(t, tc.announce))
			reg, err := routepolicy.NewRegistry()
			if err != nil {
				t.Fatal(err)
			}
			chain, err := policychain.Compile(libraryIntentImport, reg,
				[]policychain.Member{{Name: tc.strip, Plugin: "bgp-filter-family"}},
				routepolicy.FinalAccept, policychain.Bases{LocalPreference: 100})
			if err != nil {
				t.Fatal(err)
			}
			policy := mustPolicy(t, []routepolicy.Ref{{Name: "bgp-filter-family:" + tc.strip}}, routepolicy.FinalAccept).WithLibrary(chain)
			addr := netip.MustParseAddr("10.0.0.4")
			peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: policy})
			got := r.runIngressPolicyChain(peer, addr, nil, contextUpdate(t, body), body)
			if !got.accept || len(got.modifiedPayload) == 0 {
				t.Fatalf("import = accept %v body %x, want retained reachability", got.accept, got.modifiedPayload)
			}
			update := wireu.NewWireUpdate(got.modifiedPayload, 0)
			for section := range sectionCount {
				source, err := nlriSectionOf(update, nil, section)
				if err != nil {
					t.Fatal(err)
				}
				if source.present && source.fam.String() == tc.strip && len(source.raw) != 0 {
					t.Errorf("removed family survives in section %d: %x", section, source.raw)
				}
				if section == tc.kept && !bytes.Equal(source.raw, nlriHex(t, tc.want)) {
					t.Errorf("kept section %d = %x, want %s", section, source.raw, tc.want)
				}
			}
		})
	}
}
