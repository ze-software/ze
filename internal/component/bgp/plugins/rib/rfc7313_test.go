package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rfc7313RefreshPeer is the peer whose Adj-RIB-Out is re-advertised below.
const rfc7313RefreshPeer = "10.0.0.1"

// rfc7313ReadyRIB returns a RIB manager holding two IPv4 routes for an up peer,
// with every peer-action dispatch captured instead of sent.
func rfc7313ReadyRIB(t *testing.T) (*RIBManager, *[]string) {
	t.Helper()
	r := newTestRIBManager(t)
	addr := netip.MustParseAddr(rfc7313RefreshPeer)
	r.ribOut[addr] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv4Unicast: {
			"10.0.0.0/24": {MsgID: 1, Family: family.IPv4Unicast, Prefix: "10.0.0.0/24", NextHop: "1.1.1.1"},
			"10.0.1.0/24": {MsgID: 2, Family: family.IPv4Unicast, Prefix: "10.0.1.0/24", NextHop: "1.1.1.1"},
		},
	})
	r.peerUp[addr] = true

	dispatched := &[]string{}
	r.dispatchHook = func(cmd string) { *dispatched = append(*dispatched, cmd) }
	return r, dispatched
}

// TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR drives both route-refresh
// producers and reads the markers each one emits.
//
// RFC 7313 Section 4: "Before the speaker starts a route refresh that is either
// initiated locally, or in response to a 'normal route refresh request' from the
// peer, the speaker MUST send a BoRR message. After the speaker completes the
// re-advertisement of the entire Adj-RIB-Out to the peer, it MUST send an EoRR
// message."
//
// VALIDATES: RIBManager.handleRefresh (rib.go, the JSON rail) and
// RIBManager.handleRefreshStructured (rib_structured.go, the DirectBridge rail) each
// dispatch "request peer <addr> borr <family>" and then
// "request peer <addr> eorr <family>" around the Adj-RIB-Out re-advertisement, for
// the family the request named.
// PREVENTS: a route refresh that re-advertises the Adj-RIB-Out with no BoRR, which
// leaves the receiving speaker unable to mark the family's routes stale and so unable
// to purge what the re-advertisement does not replace. Deleting the BoRR dispatch from
// BOTH producers left the whole rib package green before this test existed: the two
// tests tagged for this requirement lived in other packages and proved only that the
// ROUTE-REFRESH message encodes subtype 1 and that "request peer <addr> borr" reaches
// the reactor when an operator types it. Neither entered either producer.
//
// RFC requirement: RFC7313-4-1 positive -- the speaker sends a BoRR before it starts
// the route refresh, on both the JSON and the structured rail.
// RFC requirement: RFC7313-4-2 positive -- the speaker sends an EoRR after it
// completes the re-advertisement of the Adj-RIB-Out, on both rails.
func TestRFC7313RefreshBracketsReadvertisementWithBoRRAndEoRR(t *testing.T) {
	want := []string{
		"request peer " + rfc7313RefreshPeer + " borr " + family.IPv4Unicast.String(),
		"request peer " + rfc7313RefreshPeer + " eorr " + family.IPv4Unicast.String(),
	}

	t.Run("handleRefresh", func(t *testing.T) {
		r, dispatched := rfc7313ReadyRIB(t)

		r.handleRefresh(&Event{
			Message: &MessageInfo{Type: rpc.EventKindRefresh},
			Peer: mustMarshal(t, map[string]any{
				"local":  map[string]any{"address": "10.0.0.2", "as": uint32(65002)},
				"remote": map[string]any{"address": rfc7313RefreshPeer, "as": uint32(65001)},
			}),
			AFI:  family.AFIIPv4,
			SAFI: family.SAFIUnicast,
		})

		require.Equal(t, want, *dispatched,
			"the re-advertisement must be bracketed by a BoRR and an EoRR for the requested family")
	})

	t.Run("handleRefreshStructured", func(t *testing.T) {
		r, dispatched := rfc7313ReadyRIB(t)

		// Route refresh wire: AFI (2 octets), message subtype (1), SAFI (1).
		// Subtype 0 is the normal refresh request that starts the re-advertisement.
		r.handleRefreshStructured(&rpc.StructuredEvent{
			PeerAddress: rfc7313RefreshPeer,
			RawMessage: &bgptypes.RawMessage{
				RawBytes: []byte{0x00, byte(family.AFIIPv4), 0x00, byte(family.SAFIUnicast)},
			},
		})

		require.Equal(t, want, *dispatched,
			"the structured rail must bracket its re-advertisement the same way the JSON rail does")
	})
}

// TestRFC7313NoMarkersWhenNoRefreshStarts is the control that makes the test above
// discriminate. RFC 7313 Section 4 conditions the BoRR on a route refresh STARTING,
// so a request that starts none must produce neither marker: an implementation that
// emitted the pair unconditionally would satisfy the positive and announce a refresh
// it never performs.
func TestRFC7313NoMarkersWhenNoRefreshStarts(t *testing.T) {
	t.Run("peer is down", func(t *testing.T) {
		r, dispatched := rfc7313ReadyRIB(t)
		delete(r.peerUp, netip.MustParseAddr(rfc7313RefreshPeer))

		r.handleRefresh(&Event{
			Message: &MessageInfo{Type: rpc.EventKindRefresh},
			Peer: mustMarshal(t, map[string]any{
				"local":  map[string]any{"address": "10.0.0.2", "as": uint32(65002)},
				"remote": map[string]any{"address": rfc7313RefreshPeer, "as": uint32(65001)},
			}),
			AFI:  family.AFIIPv4,
			SAFI: family.SAFIUnicast,
		})

		assert.Empty(t, *dispatched, "no refresh starts for a down peer, so no BoRR may be sent")
	})

	t.Run("the request is itself a BoRR", func(t *testing.T) {
		r, dispatched := rfc7313ReadyRIB(t)

		// Subtype 1 is a BoRR from the peer, not a request to re-advertise. Only
		// subtype 0 starts a route refresh (RFC 7313 Section 4).
		r.handleRefreshStructured(&rpc.StructuredEvent{
			PeerAddress: rfc7313RefreshPeer,
			RawMessage: &bgptypes.RawMessage{
				RawBytes: []byte{0x00, byte(family.AFIIPv4), 0x01, byte(family.SAFIUnicast)},
			},
		})

		assert.Empty(t, *dispatched, "a received BoRR starts no re-advertisement, so it emits no markers")
	})
}
