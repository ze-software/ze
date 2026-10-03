// Design: docs/architecture/wire/attributes.md -- the announce rail builds the UPDATE ze originates
// RFC: rfc/short/rfc4271.md
// Related: reactor_api_batch.go -- AnnounceNLRIBatch and WithdrawNLRIBatch, the entry points under test
// Related: local_as_announce_test.go -- newAnnounceLocalASPeer, the Established peer these tests reuse
//
// RFC 4271 Section 9.2: "Changes to the reachable destinations within its own
// autonomous system SHALL also be advertised in an UPDATE message." A destination
// ze originates is one inside its own AS, and the API's announce and withdraw
// commands are how its reachability changes at runtime. These tests drive both
// commands into Established peers and read the bytes each peer's connection
// received, so the proof is the UPDATE on the wire, not the encoder alone.
package reactor

import (
	"net/netip"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ownASUpdate is one UPDATE body split into its three variable fields.
type ownASUpdate struct {
	withdrawn []byte
	attrs     []byte
	nlri      []byte
}

// splitOwnASUpdate reads the single UPDATE written as wire and answers its
// fields (RFC 4271 Section 4.3: Withdrawn Routes Length, Withdrawn Routes, Total
// Path Attribute Length, Path Attributes, NLRI).
func splitOwnASUpdate(t *testing.T, wire []byte) ownASUpdate {
	t.Helper()
	// RFC 4271 Section 4.1: a 16-octet marker, a 2-octet length and a 1-octet type.
	require.Greater(t, len(wire), 19+4, "an UPDATE must have been written")
	require.Equal(t, byte(2), wire[18], "the message is an UPDATE (type 2)")
	msgLen := int(wire[16])<<8 | int(wire[17])
	require.Len(t, wire, msgLen, "exactly one UPDATE was written for this change")
	body := wire[19:]
	withdrawnLen := int(body[0])<<8 | int(body[1])
	require.GreaterOrEqual(t, len(body), 2+withdrawnLen+2)
	withdrawn := body[2 : 2+withdrawnLen]
	rest := body[2+withdrawnLen:]
	attrLen := int(rest[0])<<8 | int(rest[1])
	require.GreaterOrEqual(t, len(rest), 2+attrLen)
	return ownASUpdate{withdrawn: withdrawn, attrs: rest[2 : 2+attrLen], nlri: rest[2+attrLen:]}
}

// ownASChangeFixture is the reactor, one internal and one external Established
// peer, and the connections recording what each was sent.
type ownASChangeFixture struct {
	adapter *reactorAPIAdapter
	conns   map[string]*recordingConn
}

func newOwnASChangeFixture(t *testing.T) ownASChangeFixture {
	t.Helper()
	dests := []announceLocalASPeer{
		{name: "internal", addr: "10.0.0.2", localAS: localASGlobal, peerAS: localASGlobal},
		{name: "external", addr: "10.0.0.3", localAS: localASGlobal, peerAS: 65099},
	}
	peers := make(map[netip.AddrPort]*Peer, len(dests))
	conns := make(map[string]*recordingConn, len(dests))
	for _, dest := range dests {
		peer, conn := newAnnounceLocalASPeer(t, dest)
		peers[peer.Settings().PeerKey()] = peer
		conns[dest.name] = conn
	}
	r := &Reactor{
		config:          &Config{LocalAS: localASGlobal},
		peers:           peers,
		attrModHandlers: attrModHandlersWithDefaults(),
	}
	return ownASChangeFixture{adapter: &reactorAPIAdapter{r: r}, conns: conns}
}

func ownASBatch() bgptypes.NLRIBatch {
	return bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.20.0.0/24"), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1")),
	}
}

// ownASPrefixWire is 10.20.0.0/24 in the RFC 4271 Section 4.3 prefix form.
var ownASPrefixWire = []byte{24, 10, 20, 0}

// TestRFC4271OwnASDestinationBecomingReachableIsSentToPeers originates a
// destination through the API's announce entry point and reads the peers' wire.
//
// VALIDATES: a destination ze originates that becomes reachable leaves as an
// UPDATE carrying it in the NLRI field, toward an internal and an external peer.
// PREVENTS: a local origination that changes ze's own reachability but never
// reaches a peer.
//
// RFC requirement: RFC4271-9.2-9 positive -- a reachable destination within the
// speaker's own AS that is announced through AnnounceNLRIBatch is written to each
// Established peer (internal and external) as one UPDATE whose NLRI field carries
// the prefix and whose Withdrawn Routes field is empty.
func TestRFC4271OwnASDestinationBecomingReachableIsSentToPeers(t *testing.T) {
	fx := newOwnASChangeFixture(t)

	require.NoError(t, fx.adapter.AnnounceNLRIBatch(t.Context(), selector.All(), ownASBatch(), plugin.OperatorSender()))

	for name, conn := range fx.conns {
		update := splitOwnASUpdate(t, conn.written())
		assert.Empty(t, update.withdrawn, name+": an announcement withdraws nothing")
		assert.NotEmpty(t, update.attrs, name+": a reachable route carries its path attributes")
		assert.Equal(t, ownASPrefixWire, update.nlri, name+": the destination is advertised in the NLRI field")
	}
}

// TestRFC4271OwnASDestinationBecomingUnreachableIsSentToPeers is the other
// polarity: the change is a loss of reachability.
//
// VALIDATES: after the announcement, the API's withdraw entry point writes a
// second UPDATE to each peer carrying the prefix in Withdrawn Routes and nothing
// in the NLRI field.
// PREVENTS: reading "changes are advertised" as "only additions are advertised",
// which would leave the peers holding a destination ze no longer reaches.
//
// RFC requirement: RFC4271-9.2-9 negative -- a destination within the speaker's
// own AS that stops being reachable (WithdrawNLRIBatch after an announcement) is
// not left advertised: each Established peer is sent one more UPDATE whose
// Withdrawn Routes field carries the prefix and whose NLRI field and path
// attributes are empty.
func TestRFC4271OwnASDestinationBecomingUnreachableIsSentToPeers(t *testing.T) {
	fx := newOwnASChangeFixture(t)
	require.NoError(t, fx.adapter.AnnounceNLRIBatch(t.Context(), selector.All(), ownASBatch(), plugin.OperatorSender()))
	announced := make(map[string]int, len(fx.conns))
	for name, conn := range fx.conns {
		announced[name] = len(conn.written())
		require.NotZero(t, announced[name], name+": the announcement must have gone out first")
	}

	require.NoError(t, fx.adapter.WithdrawNLRIBatch(t.Context(), selector.All(), ownASBatch(), plugin.OperatorSender()))

	for name, conn := range fx.conns {
		update := splitOwnASUpdate(t, conn.written()[announced[name]:])
		assert.Equal(t, ownASPrefixWire, update.withdrawn, name+": the lost destination is withdrawn")
		assert.Empty(t, update.attrs, name+": a withdrawal carries no path attributes")
		assert.Empty(t, update.nlri, name+": the lost destination is not advertised as reachable")
	}
}
