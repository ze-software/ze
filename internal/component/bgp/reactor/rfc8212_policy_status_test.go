// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
package reactor

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// TestRFC8212PolicyInfoReportsDefault checks that peer status reports each
// direction's effective final (spec-bgp-rfc8212-ebgp-default-reject AC-8).
// Method: build one peer per case, read it through reactorAPIAdapter.Peers (the
// snapshot peer detail and `show policy chain` render), and compare the JSON of
// both directions. A known EBGP session with no policy reports reject marked
// rfc8212, distinct from a configured reject. A known IBGP session keeps an empty
// direction omitted and an automatic-only one without a final. A dynamic peer
// whose remote AS is not resolved reports undetermined, never reject or accept.
func TestRFC8212PolicyInfoReportsDefault(t *testing.T) {
	automatic := frefs("rfc:no-self-as")
	cases := []struct {
		name     string
		remoteAS uint32
		dynamic  bool
		policy   filterapi.Policy
		want     string
	}{
		{name: "absent-ebgp", remoteAS: 65001, want: `{"filters":null,"final":"reject","final-source":"rfc8212"}`},
		{name: "automatic-only-ebgp", remoteAS: 65001, policy: filterapi.InternalPolicy(automatic), want: `{"filters":["rfc:no-self-as"],"final":"reject","final-source":"rfc8212"}`},
		{name: "explicit-reject-ebgp", remoteAS: 65001, policy: mustPolicy(t, nil, routepolicy.FinalReject), want: `{"filters":null,"final":"reject"}`},
		{name: "explicit-accept-ebgp", remoteAS: 65001, policy: mustPolicy(t, nil, routepolicy.FinalAccept), want: `{"filters":null,"final":"accept"}`},
		{name: "absent-ibgp", remoteAS: 65000, want: `null`},
		{name: "automatic-only-ibgp", remoteAS: 65000, policy: filterapi.InternalPolicy(automatic), want: `{"filters":["rfc:no-self-as"]}`},
		{name: "absent-unresolved-dynamic", dynamic: true, want: `{"filters":null,"final":"undetermined"}`},
		{name: "automatic-only-unresolved-dynamic", dynamic: true, policy: filterapi.InternalPolicy(automatic), want: `{"filters":["rfc:no-self-as"],"final":"undetermined"}`},
		{name: "explicit-accept-unresolved-dynamic", dynamic: true, policy: mustPolicy(t, nil, routepolicy.FinalAccept), want: `{"filters":null,"final":"accept"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr := netip.MustParseAddr("192.0.2.1")
			peer := NewPeer(&PeerSettings{
				Address: addr, LocalAS: 65000, PeerAS: tc.remoteAS,
				IsDynamic: tc.dynamic, ImportPolicy: tc.policy, ExportPolicy: tc.policy,
			})
			r := &Reactor{peers: map[netip.AddrPort]*Peer{peer.Settings().PeerKey(): peer}}
			peers := (&reactorAPIAdapter{r: r}).Peers()
			if len(peers) != 1 {
				t.Fatalf("Peers() returned %d peers, want 1", len(peers))
			}
			for direction, info := range map[string]any{
				directionImport: peers[0].ImportPolicy,
				directionExport: peers[0].ExportPolicy,
			} {
				got, err := json.Marshal(info)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != tc.want {
					t.Errorf("%s policy = %s, want %s", direction, got, tc.want)
				}
			}
		})
	}
}
