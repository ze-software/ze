package reactor

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// RFC 5575 Section 4: "The (AFI, SAFI) pair carried in the Multiprotocol
// Extension Capability MUST be the same as the one used by the application."
// RFC requirement: RFC5575-4-1 positive -- the configured OPEN advertises code-1 Multiprotocol for IPv4 flow and flow-vpn, and a negotiated rule reaches the peer.
// RFC requirement: RFC5575-4-1 negative -- a peer advertising only the other FlowSpec SAFI receives no rule for the unnegotiated application.
// RFC requirement: RFC5575-4-2 positive -- each IPv4 flow/flow-vpn MP_REACH AFI/SAFI exactly matches its pair in the configured OPEN.
// RFC requirement: RFC5575-4-2 negative -- the shared AFI does not permit a flow rule to be encoded or negotiated as flow-vpn, or vice versa.
func TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily(t *testing.T) {
	caps := openCapabilitiesFromConfig(t, map[string]any{
		"ipv4/flow": map[string]any{}, "ipv4/flow-vpn": map[string]any{},
	}, nil)
	advertised := multiprotocolFamilies(caps)
	for _, safi := range []family.SAFI{family.SAFIFlowSpec, family.SAFIFlowSpecVPN} {
		fam := family.Family{AFI: family.AFIIPv4, SAFI: safi}
		if !slices.Contains(advertised, fam) {
			t.Fatalf("configured OPEN did not advertise %s", fam)
		}
		wire := []byte{5, 1, 24, 10, 1, 0}
		if safi == family.SAFIFlowSpecVPN {
			wire = []byte{13, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 1, 24, 10, 1, 0}
		}
		n, err := nlri.NewWireNLRI(fam, wire, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, matched := range []bool{true, false} {
			peerFamily := fam
			if !matched {
				peerFamily.SAFI = family.SAFIFlowSpec
				if safi == family.SAFIFlowSpec {
					peerFamily.SAFI = family.SAFIFlowSpecVPN
				}
			}
			neg := capability.Negotiate(caps, peerOffering(peerFamily), testPeerIdentity)
			peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
			peer.negotiated.Store(NewNegotiatedCapabilities(neg))
			api := groupUpdatesReactor([]*Peer{peer}, false)
			batch := bgptypes.NLRIBatch{Family: fam, NLRIs: []nlri.NLRI{n}}
			err := api.AnnounceNLRIBatch(t.Context(), selector.All(), batch, plugin.OperatorSender())
			if !matched {
				if !errors.Is(err, route.ErrNoPeersAcceptedFamily) || len(conn.written()) != 0 {
					t.Fatalf("unnegotiated %s: error=%v wire=%x", fam, err, conn.written())
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			bodies := updateBodies(t, conn.written())
			if len(bodies) != 1 {
				t.Fatalf("%s sent %d UPDATEs", fam, len(bodies))
			}
			_, attrs, _ := updateSections(t, bodies[0])
			_, mp, found := findPathAttr(attrs, byte(attribute.AttrMPReachNLRI))
			want := append([]byte{0, 1, byte(safi), 0, 0}, wire...)
			if !found || !bytes.Equal(mp, want) {
				t.Fatalf("%s MP_REACH=%x, want %x matching OPEN", fam, mp, want)
			}
		}
	}
}
