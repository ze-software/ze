// Design: docs/architecture/api/process-protocol.md -- accepted-source withdrawal fences.
// Related: origin_admission_test.go -- original accepted lookup; peer_initial_sync.go -- real queued writer.
package reactor

import (
	"context"
	"net/netip"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The native withdrawal entry must preserve the exact accepted ID before the
// source guard. A replay-marked withdrawal cannot silently become local output.
// MUTATION: omit the context-to-batch ID copy in withdrawNLRIBatch.
func TestOriginAcceptedWithdrawalPreservesContextID(t *testing.T) {
	for _, replay := range []bool{false, true} {
		t.Run(map[bool]string{false: "original-lookup", true: "replay-refused"}[replay], func(t *testing.T) {
			t.Cleanup(redistevents.PreserveForTest())
			f := newSelectedWriterFixture(t, false)
			f.peer.setTestInitialSync(1, false)
			owner, _ := originOwnerFixture(t, selectedWriterProcess)
			var lookups atomic.Uint32
			owner, err := owner.WithOriginAdmissionLookup(func(_ context.Context, in rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
				lookups.Add(1)
				require.Equal(t, uint64(51), in.AdmissionID)
				require.Equal(t, rpc.OriginIntakeWithdraw, in.Operation)
				return originAcceptedAnswer(in, rpc.OriginAdmissionRetiring, rpc.OriginAdmissionSnapshot{
					Protocol: "static", SourceInstance: 7, PeerScope: rpc.OriginAdmissionAllPeers,
				}), nil
			})
			require.NoError(t, err)
			redistevents.RegisterProtocol("static")
			ctx := plugin.WithOriginProducerContext(plugin.WithOriginAdmissionID(t.Context(), 51), owner, nil)
			batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast, Replay: replay,
				NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0, false)}}
			_, err = f.api.WithdrawNLRIBatchResult(ctx, selector.All(), batch, owner.Sender())
			if replay {
				require.ErrorContains(t, err, "accepted redistribution cannot claim")
				require.Zero(t, lookups.Load(), "provenance must be refused before the callback")
				return
			}
			require.NoError(t, err)
			require.Equal(t, uint32(1), lookups.Load(), "withdrawal must reach the original accepted lookup")
		})
	}
}

// A successful accepted withdrawal must fence its actual queued original before
// returning. Drain the real initial-sync queue afterwards and prove unrelated B
// still writes while retired A never installs, even transiently before its del.
// MUTATION: leave accepted identity out of peerOp and its final socket guard.
func TestOriginAcceptedQueuedOriginalCannotInstallAfterWithdrawal(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	redistevents.RegisterProtocol("static")
	f := newSelectedWriterFixture(t, false)
	f.peer.setTestInitialSync(1, false)
	// Publish the recording TCP endpoint required by the genuine Self resolver.
	f.peer.currentSession().transport.Store(connectedTransport(selectedOriginEndpointConn{
		Conn: f.conn, local: netip.MustParseAddrPort("192.0.2.254:179"),
	}))
	owner, _ := originOwnerFixture(t, selectedWriterProcess)
	var retiring atomic.Bool
	owner, err := owner.WithOriginAdmissionLookup(func(_ context.Context, in rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		state := rpc.OriginAdmissionCurrent
		if in.AdmissionID == 51 {
			if retiring.Load() {
				state = rpc.OriginAdmissionRetiring
			}
		}
		return originAcceptedAnswer(in, state, rpc.OriginAdmissionSnapshot{
			Protocol: "static", SourceInstance: 7, PeerScope: rpc.OriginAdmissionAllPeers,
		}), nil
	})
	require.NoError(t, err)
	makeBatch := func(prefix string) bgptypes.NLRIBatch {
		return bgptypes.NLRIBatch{Family: family.IPv4Unicast, NextHop: bgptypes.NewNextHopSelf(),
			Attrs: attribute.NewBuilder().SetOrigin(2),
			NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefix), 0, false)}}
	}
	original := makeBatch("10.0.0.0/24")
	ctx := plugin.WithOriginProducerContext(plugin.WithOriginAdmissionID(t.Context(), 51), owner, nil)
	announced, err := f.api.AnnounceNLRIBatchResult(ctx, selector.All(), original, owner.Sender())
	require.NoError(t, err)
	require.Equal(t, uint32(1), announced.Announced)
	require.Empty(t, framesOf(t, f.conn, false), "the original must still be genuinely queued")
	retiring.Store(true)
	withdrawn, err := f.api.WithdrawNLRIBatchResult(ctx, selector.All(), original, owner.Sender())
	require.NoError(t, err)
	require.NotNil(t, withdrawn)
	independent := makeBatch("10.1.0.0/24")
	independentCtx := plugin.WithOriginProducerContext(plugin.WithOriginAdmissionID(t.Context(), 52), owner, nil)
	_, err = f.api.AnnounceNLRIBatchResult(independentCtx, selector.All(), independent, owner.Sender())
	require.NoError(t, err)
	require.False(t, f.peer.drainAndCloseQueueGate(f.peer.currentSession(), f.peer.addrString, message.MaxMsgLen))
	var installed []netip.Prefix
	for _, frame := range framesOf(t, f.conn, false) {
		installed = append(installed, frame...)
	}
	require.Contains(t, installed, netip.MustParsePrefix("10.1.0.0/24"), "independent accepted work must remain useful")
	require.NotContains(t, installed, netip.MustParsePrefix("10.0.0.0/24"), "a returned retirement cannot leave its original able to install later")
}

// With the queueing gate open and the replay fence up, an accepted original is
// written directly, so its withdrawal must be written directly too: behind the
// forwards it would reach the wire after a replacement announced later and
// withdraw that replacement. Method: announce A under ID 51, withdraw it, then
// announce replacement A' under ID 53 for the same prefix, all while the fence
// holds; read the wire's order for the prefix. The fixture's forward worker
// fails the test if anything reaches it.
// MUTATION: let an accepted withdrawal take queueExpectedWrite in withdrawBatchFromPeers.
func TestOriginAcceptedWithdrawalTakesItsOriginalDirectRail(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	redistevents.RegisterProtocol("static")
	f := newSelectedWriterFixture(t, false)
	f.peer.setTestInitialSync(0, false)
	f.peer.initialUpdateOwed.Store(true)
	require.False(t, f.peer.shouldQueue(), "the queueing gate must be open")
	require.True(t, f.peer.withdrawBehindForwards(), "the replay fence must send an ordinary withdrawal behind the forwards")
	// Publish the recording TCP endpoint required by the genuine Self resolver.
	f.peer.currentSession().transport.Store(connectedTransport(selectedOriginEndpointConn{
		Conn: f.conn, local: netip.MustParseAddrPort("192.0.2.254:179"),
	}))
	owner, _ := originOwnerFixture(t, selectedWriterProcess)
	var retiring atomic.Bool
	owner, err := owner.WithOriginAdmissionLookup(func(_ context.Context, in rpc.OriginAdmissionLookupInput) (*rpc.OriginAdmissionLookupOutput, error) {
		state := rpc.OriginAdmissionCurrent
		if in.AdmissionID == 51 {
			if retiring.Load() {
				state = rpc.OriginAdmissionRetiring
			}
		}
		return originAcceptedAnswer(in, state, rpc.OriginAdmissionSnapshot{
			Protocol: "static", SourceInstance: 7, PeerScope: rpc.OriginAdmissionAllPeers,
		}), nil
	})
	require.NoError(t, err)
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	batch := bgptypes.NLRIBatch{Family: family.IPv4Unicast, NextHop: bgptypes.NewNextHopSelf(),
		Attrs: attribute.NewBuilder().SetOrigin(2),
		NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, prefix, 0, false)}}
	original := plugin.WithOriginProducerContext(plugin.WithOriginAdmissionID(t.Context(), 51), owner, nil)
	_, err = f.api.AnnounceNLRIBatchResult(original, selector.All(), batch, owner.Sender())
	require.NoError(t, err)
	retiring.Store(true)
	_, err = f.api.WithdrawNLRIBatchResult(original, selector.All(), batch, owner.Sender())
	require.NoError(t, err)
	replacement := plugin.WithOriginProducerContext(plugin.WithOriginAdmissionID(t.Context(), 53), owner, nil)
	_, err = f.api.AnnounceNLRIBatchResult(replacement, selector.All(), batch, owner.Sender())
	require.NoError(t, err)
	var order []string
	for _, body := range updateBodies(t, f.conn.written()) {
		if slices.Contains(updatePrefixes(t, body, false), prefix) {
			order = append(order, "announce")
		}
		if slices.Contains(updatePrefixes(t, body, true), prefix) {
			order = append(order, "withdraw")
		}
	}
	require.Equal(t, []string{"announce", "withdraw", "announce"}, order, "the withdrawal must leave between its original and the later replacement")
}
