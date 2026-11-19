// Design: docs/architecture/bgp/structural-forwarding.md -- explicit encoding-test source facts.
package reactor

import (
	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/source"
	"net/netip"
	"testing"
	"time"
)

// fwdPathIDFixtureOrigin supplies an explicit synthetic peer incarnation and
// original connection41/message1 to pure wire-transform tests. These tests
// intentionally have no live Peer/session or receipt and exercise no authority.
// Equal source fixture IDs denote equal logical origins, as their old setup did.
func fwdPathIDFixtureOrigin(wire *wireu.WireUpdate) fwdPathOrigin {
	return fwdPathOrigin{source: fwdPathSource{sourceID: wire.SourceID(), peerOwner: uint64(wire.SourceID()) + 1}, connectionID: 41, messageID: 1}
}

// fwdPathIDFixturePeerOrigin uses the actual fixture's independent stable owner.
// The low-level transformer is not an authority check;41/1 are its declared
// original encoding fixture generation, not an inference from current UP.
func fwdPathIDFixturePeerOrigin(peer *Peer) fwdPathOrigin {
	return fwdPathOrigin{source: fwdPathSource{sourceID: peer.SourceID(), peerOwner: peer.sourceOwner}, connectionID: 41, messageID: 1}
}

// fwdPathIDFixtureReceived issues real immutable receipt facts for old tests
// that model cache eviction or an actual forwarding rail. Source identity,
// receive encoding, message ID and original physical41 are explicit fixture facts.
func fwdPathIDFixtureReceived(t testing.TB, peer *Peer, wire *wireu.WireUpdate) *ReceivedUpdate {
	t.Helper()
	if peer.forwardGeneration.Load() == 0 {
		peer.advanceForwardGeneration()
	}
	if wire.MessageID() == 0 {
		wire.SetMessageID(nextMsgID())
	}
	facts, err := newSourceReceiptFacts(relaySource{addr: peer.Settings().Address, strAdr: peer.addrString,
		info: forwardSourceInfo{resolved: true, peer: peer}, ctxID: wire.SourceCtxID(), srcID: wire.SourceID(), generation: peer.forwardGeneration.Load(), ok: true}, 41)
	require.NoError(t, err)
	live, err := sourceReceipts.issue(facts)
	require.NoError(t, err)
	t.Cleanup(live.close)
	wire.SetSourceReceipt(live.ID())
	wire.SetSourceOwner(peer.sourceOwner)
	return &ReceivedUpdate{ReceivedAt: time.Now(), WireUpdate: wire, SourcePeerIP: peer.Settings().Address, SourcePeerStr: peer.addrString, SourceReceipt: live.ID(), SourceOwner: peer.sourceOwner,
		receivedPeer: peer, receivedGeneration: peer.forwardGeneration.Load(), SourcePolicyEpoch: peer.sourcePolicyEpoch.Load()}
}

func newFwdPathIDFixturePeer(src source.SourceID) *Peer {
	peer := NewPeer(&PeerSettings{Address: netip.MustParseAddr("192.0.2.143"), LocalAS: 65000, PeerAS: 65001})
	// This old encoding/cache fixture already owns its declared synthetic source ID.
	peer.sourceID = src
	peer.state.Store(int32(PeerStateEstablished))
	peer.advanceForwardGeneration()
	return peer
}
