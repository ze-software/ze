// Design: docs/architecture/core-design.md — the RIB plugin answers a route refresh
// RFC: rfc/short/rfc7313.md — Enhanced Route Refresh, Section 4
// Related: rfc7313_test.go — the marker-only view of the same producers
//
// RFC 7313 Section 4: "Before the speaker starts a route refresh that is either initiated
// locally, or in response to a "normal route refresh request" from the peer, the speaker
// MUST send a BoRR message. After the speaker completes the re-advertisement of the entire
// Adj-RIB-Out to the peer, it MUST send an EoRR message."
//
// The markers leave the RIB on the dispatch rail (dispatchPeerAction) and the routes on the
// update rail (updateRoute), so a test holding one hook sees half the sequence. These tests
// hold both hooks and write into ONE ordered log, so the position of each marker against
// the re-advertised routes is what they assert.

package rib

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rfc7313SequencedRIB returns rfc7313ReadyRIB's manager, holding two IPv4 unicast routes
// and one IPv6 unicast route in the peer's Adj-RIB-Out, with markers and routes captured
// into one log in the order the RIB emitted them.
func rfc7313SequencedRIB(t *testing.T) (*RIBManager, *[]string) {
	t.Helper()
	r, _ := rfc7313ReadyRIB(t)
	addr := netip.MustParseAddr(rfc7313RefreshPeer)
	r.ribOut[addr][family.IPv6Unicast] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		family.IPv6Unicast: {
			"2001:db8::/32": {MsgID: 3, Family: family.IPv6Unicast, Prefix: "2001:db8::/32", NextHop: "::1"},
		},
	})[family.IPv6Unicast]

	sequence := &[]string{}
	r.dispatchHook = func(cmd string) { *sequence = append(*sequence, "marker "+cmd) }
	r.updateHook = func(cmd string, _ map[string]any) { *sequence = append(*sequence, "route "+cmd) }
	return r, sequence
}

// requireBracketedReadvertisement asserts the whole RFC 7313 Section 4 sequence for the
// IPv4 unicast refresh: the BoRR first, then every IPv4 unicast route of the Adj-RIB-Out
// and nothing else, then the EoRR last.
func requireBracketedReadvertisement(t *testing.T, sequence []string) {
	t.Helper()
	require.Len(t, sequence, 4, "BoRR, the two IPv4 unicast routes of the Adj-RIB-Out, EoRR: %q", sequence)

	borr := "marker request peer " + rfc7313RefreshPeer + " borr " + family.IPv4Unicast.String()
	eorr := "marker request peer " + rfc7313RefreshPeer + " eorr " + family.IPv4Unicast.String()
	require.Equal(t, borr, sequence[0], "the BoRR must precede the first re-advertised route")
	require.Equal(t, eorr, sequence[3], "the EoRR must follow the last re-advertised route")

	routes := strings.Join(sequence[1:3], "\n")
	require.True(t, strings.HasPrefix(sequence[1], "route "), "a route sits between the markers: %q", sequence[1])
	require.True(t, strings.HasPrefix(sequence[2], "route "), "a route sits between the markers: %q", sequence[2])
	require.Contains(t, routes, "10.0.0.0/24", "the entire Adj-RIB-Out of the family is re-advertised")
	require.Contains(t, routes, "10.0.1.0/24", "the entire Adj-RIB-Out of the family is re-advertised")
	require.NotContains(t, routes, "2001:db8::/32", "another family's routes are not part of this refresh")
}

// TestRFC7313BoRRRoutesEoRRInOneSequence drives both refresh producers and reads one
// ordered log of what each emitted.
//
// VALIDATES: RIBManager.handleRefresh (the JSON rail) and handleRefreshStructured (the
// DirectBridge rail), answering a normal route refresh request (subtype 0) from the peer,
// emit the BoRR before the first route, then both IPv4 unicast routes of the Adj-RIB-Out,
// then the EoRR after the last route.
// PREVENTS: a producer that re-advertises before its BoRR, or sends its EoRR before the
// re-advertisement completes: the marker-only unit (rfc7313_test.go) stays green on both,
// because the routes never enter its log.
//
// RFC requirement: RFC7313-4-1 positive -- in response to a normal route refresh request,
// the BoRR is sent before the first route of the re-advertisement, on both rails
// (rib.go handleRefresh, rib_structured.go handleRefreshStructured).
// RFC requirement: RFC7313-4-2 positive -- the EoRR is sent after the last route of the
// re-advertisement, which carries every route of the family's Adj-RIB-Out, on both rails.
func TestRFC7313BoRRRoutesEoRRInOneSequence(t *testing.T) {
	t.Run("handleRefresh", func(t *testing.T) {
		r, sequence := rfc7313SequencedRIB(t)

		r.handleRefresh(&Event{
			Message: &MessageInfo{Type: rpc.EventKindRefresh},
			Peer: mustMarshal(t, map[string]any{
				"local":  map[string]any{"address": "10.0.0.2", "as": uint32(65002)},
				"remote": map[string]any{"address": rfc7313RefreshPeer, "as": uint32(65001)},
			}),
			AFI:  family.AFIIPv4,
			SAFI: family.SAFIUnicast,
		})

		requireBracketedReadvertisement(t, *sequence)
	})

	t.Run("handleRefreshStructured", func(t *testing.T) {
		r, sequence := rfc7313SequencedRIB(t)

		// Route refresh wire: AFI (2 octets), message subtype (1), SAFI (1). Subtype 0.
		r.handleRefreshStructured(&rpc.StructuredEvent{
			PeerAddress: rfc7313RefreshPeer,
			RawMessage: &bgptypes.RawMessage{
				RawBytes: []byte{0x00, byte(family.AFIIPv4), 0x00, byte(family.SAFIUnicast)},
			},
		})

		requireBracketedReadvertisement(t, *sequence)
	})
}
