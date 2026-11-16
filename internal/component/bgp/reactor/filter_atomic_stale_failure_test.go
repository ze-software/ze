// Design: docs/architecture/bgp/structural-forwarding.md -- source retirement survives stale normalization failure.
// Related: filter_atomic_retirement_pipeline_test.go -- accepted subsets through live writers.
package reactor

import (
	"bufio"
	"bytes"
	"io"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAtomicRetirementStaleFailureKeepsRemovals drives the actual stale sender
// after its accepted subset has excluded A but normalization of kept B fails.
// A's authorized withdrawal must reach the socket, while foreign and newer
// exact receipts survive. No failed survivor may manufacture an EOR, and a
// cleanup write failure must retain both the original and transport causes.
func TestAtomicRetirementStaleFailureKeepsRemovals(t *testing.T) {
	first := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 1}
	second := []byte{48, 0x20, 1, 0x0d, 0xb8, 0, 2}
	global := netip.MustParseAddr("2001:db8::1")
	pair := append(global.AsSlice(), netip.MustParseAddr("fe80::1").AsSlice()...)
	for _, tc := range []struct {
		name        string
		received    bool
		exact       bool
		newer       bool
		writeFails  bool
		wantRemoval bool
	}{
		{name: "local-owner", wantRemoval: true},
		{name: "foreign-unowned", received: true},
		{name: "current-exact-receipt", received: true, exact: true, wantRemoval: true},
		{name: "newer-same-source-receipt", received: true, exact: true, newer: true},
		{name: "cleanup-write-failure", writeFails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := atomicAdmissionMPFixture(t)
			f.destination.session.adjOut = &f.destination.adjOut
			f.destination.reactor = f.r
			batch := staleReadvertiseBatch(t)
			batch.Family = family.IPv6Unicast
			batch.NextHop = bgptypes.NewNextHopExplicit(global)
			batch.NLRIs = nil
			for _, raw := range [][]byte{first, second} {
				prefix, err := nlri.NewWireNLRI(batch.Family, raw, false)
				require.NoError(t, err)
				batch.NLRIs = append(batch.NLRIs, prefix)
			}
			batch.Attrs.SetAtomicAggregate().SetMED(10)
			batch.Replay = true
			adapter := &reactorAPIAdapter{r: f.r}
			facts := announceFactsFor(f.destination, batch.Family, global, false, f.destination.negotiated.Load())
			seed := atomicRetirementMPNextHopBody(t, append(bytes.Clone(first), second...), global.AsSlice(), 10)
			if tc.received {
				require.NoError(t, ownershipWriterForward(t, f.destination, f.source, seed, false))
			} else {
				require.NoError(t, f.destination.session.SendUpdate(ownershipWriterUpdate(t, seed)))
			}
			previous := f.destination.session.sentReceipt.MessageID()
			require.NotZero(t, previous)
			if tc.exact {
				batch.SentOwnerMessage = previous
				batch.SourcePolicyEpoch = f.source.sourcePolicyEpoch.Load()
			}
			if tc.newer {
				replacement := atomicRetirementMPNextHopBody(t, first, global.AsSlice(), 20)
				require.NoError(t, ownershipWriterForward(t, f.destination, f.source, replacement, false))
				require.Greater(t, f.destination.session.sentReceipt.MessageID(), previous)
			}
			before := len(aigpSocketBodies(t, f.conn))
			output := atomicRetirementMPNextHopBody(t, second, pair, 77)
			exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
				return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
			}, f.destination)
			f.destination.raiseExportPolicyFrom(batch.SentOwnerMessage + 1)
			// The off-link pair requires trimming; removing its registered
			// materializer reaches the real normalization failure after policy.
			delete(f.r.attrModHandlers, uint8(attribute.AttrMPReachNLRI))
			if tc.writeFails {
				session := f.destination.session
				session.writeMu.Lock()
				session.conn = &atomicStaleRetirementFailConn{recordingConn: f.conn}
				session.bufWriter = bufio.NewWriterSize(session.conn, message.MaxMsgLen)
				session.writeMu.Unlock()
			}
			sent, err := adapter.sendStaleReadvertiseUnit(t.Context(), announceTarget{peer: f.destination, session: f.destination.session},
				batch, facts, message.MaxMsgLen, make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen))
			require.False(t, sent, "the unusable survivor must still be reported failed")
			require.ErrorIs(t, err, errStaleReadvertiseBuildFailed)
			if tc.writeFails {
				require.ErrorIs(t, err, io.ErrClosedPipe, "cleanup must not hide its own transport failure")
				require.Empty(t, aigpSocketBodies(t, f.conn)[before:])
				return
			}
			written := aigpSocketBodies(t, f.conn)[before:]
			if tc.wantRemoval {
				require.Len(t, written, 1, "the excluded owned source must leave before this failed send returns")
			} else {
				require.Empty(t, written, "an unowned or old exact decision must spare the current received owner")
				withdrawal := ownershipRailBody(family.IPv6Unicast, true, first, 0)
				require.NoError(t, ownershipWriterForward(t, f.destination, f.source, withdrawal, false))
				written = aigpSocketBodies(t, f.conn)[before:]
				require.Len(t, written, 1, "the preserved owner must still be removable by its actual source")
			}
			update := wireu.NewWireUpdate(written[0], f.ctxID)
			withdrawn, err := update.MPUnreach()
			require.NoError(t, err)
			require.NotNil(t, withdrawn)
			require.Equal(t, first, withdrawn.WithdrawnBytes())
			announced, err := update.MPReach()
			require.NoError(t, err)
			require.Nil(t, announced, "failed B must never be advertised")
			decoded, err := message.UnpackUpdate(written[0])
			require.NoError(t, err)
			require.False(t, decoded.IsEndOfRIBAnyFamily(), "cleanup must not invent End-of-RIB")
		})
	}
}

// atomicStaleRetirementFailConn is installed only after the seed announcement.
// Every later write fails, so cleanup cannot accidentally pass through a buffer.
type atomicStaleRetirementFailConn struct{ *recordingConn }

func (*atomicStaleRetirementFailConn) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
