// Design: docs/architecture/update-building.md -- the API rail's withdrawal framing
// Related: reactor_api_batch.go -- queuedWithdrawal, buildBatchWithdrawUpdate
// Related: peer_initial_sync.go -- sendQueuedWithdrawal, the drain that sends it

package reactor

import (
	"bytes"
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// queuedWithdrawalPeer is one Established peer that negotiated ipv4/sr-policy
// with ADD-PATH send/receive, the session test/plugin/update-text-sr-policy-addpath.ci
// opens, backed by a recordingConn.
func queuedWithdrawalPeer(t *testing.T, addr string) (*Peer, *recordingConn) {
	t.Helper()
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	seed, conn := newAnnouncePeer(t, addr)
	session := seed.session
	mp := &capability.Multiprotocol{AFI: fam.AFI, SAFI: fam.SAFI}
	ap := &capability.AddPath{Families: []capability.AddPathFamily{{AFI: fam.AFI, SAFI: fam.SAFI, Mode: capability.AddPathBoth}}}
	local := []capability.Capability{&capability.ASN4{ASN: 65000}, mp, ap}
	remote := []capability.Capability{&capability.ASN4{ASN: 65001}, mp, ap}
	session.localOpen = &message.Open{HoldTime: 90}
	session.peerOpen = &message.Open{ASN4: 65001, HoldTime: 90}
	session.negotiateWith(local, remote)

	peer := NewPeer(session.settings)
	peer.session = session
	peer.state.Store(int32(PeerStateEstablished))
	peer.negotiated.Store(NewNegotiatedCapabilities(session.negotiated))
	peer.setEncodingContexts(session.negotiated)
	t.Cleanup(peer.clearEncodingContexts)
	return peer, conn
}

// queuedWithdrawalNLRI is the .ci's SR Policy NLRI, distinguisher 0, color 300,
// endpoint 10.0.0.3, behind Path Identifier 7 (RFC 7911 Section 3).
func queuedWithdrawalNLRI(t *testing.T) nlri.NLRI {
	t.Helper()
	data, err := hex.DecodeString("0000000760000000000000012C0A000003")
	if err != nil {
		t.Fatal(err)
	}
	n, err := nlri.NewWireNLRI(family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}, data, true)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// lastUpdateBody answers the body of the last UPDATE a connection received.
func lastUpdateBody(t *testing.T, conn *recordingConn) []byte {
	t.Helper()
	bodies := updateBodies(t, conn.written())
	if len(bodies) == 0 {
		t.Fatal("the peer received no UPDATE")
	}
	return bodies[len(bodies)-1]
}

// TestQueuedWithdrawalIsTheDirectWithdrawal proves a withdrawal that waits on the
// opQueue reaches the wire as the UPDATE the direct write sends.
//
// VALIDATES: an API-rail withdrawal of an ipv4/sr-policy route queued during the
// peer's initial sync, then drained, carries the same path attributes and the same
// ADD-PATH framed MP_UNREACH_NLRI as one written to an idle session.
// Method: two peers, one written directly and one held in its initial sync
// (sendingInitialRoutes), each sent the same announce and withdrawal; the last
// UPDATE each received is compared byte for byte, and ORIGIN is asserted present.
// PREVENTS: the drain framing a bare MP_UNREACH_NLRI from the NLRI alone, so the
// same command reached the wire in two shapes chosen by timing
// (test/plugin/update-text-sr-policy-addpath.ci red on a loaded CI host).
func TestQueuedWithdrawalIsTheDirectWithdrawal(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	send := func(t *testing.T, peer *Peer, queued bool) {
		t.Helper()
		adapter := groupUpdatesReactor([]*Peer{peer}, false)
		peer.reactor = adapter.r
		if queued {
			peer.sendingInitialRoutes.Store(1)
		}
		n := queuedWithdrawalNLRI(t)
		announce := bgptypes.NLRIBatch{Family: fam, NLRIs: []nlri.NLRI{n}, NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.1"))}
		if err := adapter.AnnounceNLRIBatch(t.Context(), selector.All(), announce, plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		withdraw := bgptypes.NLRIBatch{Family: fam, NLRIs: []nlri.NLRI{n}}
		if err := adapter.WithdrawNLRIBatch(t.Context(), selector.All(), withdraw, plugin.OperatorSender()); err != nil {
			t.Fatal(err)
		}
		if queued {
			if peer.drainAndCloseQueueGate("192.0.2.12", message.MaxMsgLen) {
				t.Fatal("the drain met a teardown nobody queued")
			}
		}
	}

	direct, directConn := queuedWithdrawalPeer(t, "192.0.2.11")
	send(t, direct, false)
	queued, queuedConn := queuedWithdrawalPeer(t, "192.0.2.12")
	send(t, queued, true)

	want := lastUpdateBody(t, directConn)
	got := lastUpdateBody(t, queuedConn)
	if !bytes.Equal(got, want) {
		t.Fatalf("queued withdrawal = %X, want the direct one %X", got, want)
	}
	sections, err := wire.ParseUpdateSections(got)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, found := attribute.AttrFind(sections.Attrs(got), attribute.AttrOrigin); !found {
		t.Fatalf("queued withdrawal %X carries no ORIGIN", got)
	}
	if _, _, _, found := attribute.AttrFind(sections.Attrs(got), attribute.AttrMPUnreachNLRI); !found {
		t.Fatalf("queued withdrawal %X carries no MP_UNREACH_NLRI", got)
	}
}
