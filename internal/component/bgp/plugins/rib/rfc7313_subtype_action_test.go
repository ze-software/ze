// VALIDATES: RFC 7313 Section 4, the receive side: the action the RIB takes on a
// ROUTE-REFRESH from the peer depends on its Message Subtype. Subtype 0 (a normal
// route refresh request) re-advertises the family's Adj-RIB-Out between BoRR and
// EoRR; subtypes 1 (BoRR) and 2 (EoRR) are the peer's own markers and re-advertise
// nothing.
// PREVENTS: a subtype-blind handler that answers the peer's BoRR or EoRR with a
// re-advertisement of its own, which the reactor units (NoError, Established for
// every subtype) cannot see.

package rib

import (
	"testing"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rfc7313RefreshBody is the ROUTE-REFRESH body for IPv4 unicast with the given
// Message Subtype: AFI (2 octets), subtype (1), SAFI (1).
func rfc7313RefreshBody(subtype byte) []byte {
	return []byte{0x00, byte(family.AFIIPv4), subtype, byte(family.SAFIUnicast)}
}

// rfc7313RefreshFromPeer delivers one ROUTE-REFRESH body on the DirectBridge rail
// (handleRefreshStructured, which reads the subtype octet itself) and returns what
// the RIB emitted.
func rfc7313RefreshFromPeer(t *testing.T, body []byte) []string {
	t.Helper()
	r, sequence := rfc7313SequencedRIB(t)
	r.handleRefreshStructured(&rpc.StructuredEvent{
		PeerAddress: rfc7313RefreshPeer,
		RawMessage:  &bgptypes.RawMessage{RawBytes: body},
	})
	return *sequence
}

// rfc7313EventFromPeer delivers one refresh-family event on the JSON rail
// (dispatch, where the subtype arrives as the event kind) and returns what the
// RIB emitted.
func rfc7313EventFromPeer(t *testing.T, kind rpc.EventKind) []string {
	t.Helper()
	r, sequence := rfc7313SequencedRIB(t)
	r.dispatch(&Event{
		Message: &MessageInfo{Type: kind},
		Peer: mustMarshal(t, map[string]any{
			"local":  map[string]any{"address": "10.0.0.2", "as": uint32(65002)},
			"remote": map[string]any{"address": rfc7313RefreshPeer, "as": uint32(65001)},
		}),
		AFI:  family.AFIIPv4,
		SAFI: family.SAFIUnicast,
	})
	return *sequence
}

// TestRFC7313SubtypeZeroStartsTheRefresh shows the action for subtype 0.
func TestRFC7313SubtypeZeroStartsTheRefresh(t *testing.T) {
	// RFC requirement: RFC7313-4-3 positive -- a ROUTE-REFRESH from the peer with Message Subtype 0 (body 00 01 00 01) is examined and answered with BoRR, both IPv4 unicast routes of the Adj-RIB-Out, then EoRR, on the DirectBridge rail and on the JSON rail (event kind refresh)
	requireBracketedReadvertisement(t, rfc7313RefreshFromPeer(t, rfc7313RefreshBody(0)))
	requireBracketedReadvertisement(t, rfc7313EventFromPeer(t, rpc.EventKindRefresh))
}

// TestRFC7313PeerMarkersStartNoRefresh shows that the same message with subtype
// 1 or 2, sent by an up peer holding the same Adj-RIB-Out, takes another action.
func TestRFC7313PeerMarkersStartNoRefresh(t *testing.T) {
	// RFC requirement: RFC7313-4-3 negative -- a ROUTE-REFRESH from the peer with Message Subtype 1 (BoRR, body 00 01 01 01) or 2 (EoRR, body 00 01 02 01) is not handled as a refresh request: no BoRR, no route and no EoRR are emitted, on the DirectBridge rail and on the JSON rail (event kinds borr and eorr)
	for _, subtype := range []byte{1, 2} {
		require.Empty(t, rfc7313RefreshFromPeer(t, rfc7313RefreshBody(subtype)), "subtype %d", subtype)
	}
	for _, kind := range []rpc.EventKind{rpc.EventKindBoRR, rpc.EventKindEoRR} {
		require.Empty(t, rfc7313EventFromPeer(t, kind), "event kind %s", kind)
	}
}
