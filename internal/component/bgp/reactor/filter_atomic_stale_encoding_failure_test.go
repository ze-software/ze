// Design: docs/architecture/bgp/structural-forwarding.md -- accepted source retirements survive survivor construction failure.
// Related: filter_atomic_stale_failure_test.go -- normalization failure and exact-owner controls.
// Related: filter_atomic_forward_failure_test.go -- valid indivisible extended attributes.
package reactor

import (
	"bufio"
	"bytes"
	"io"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAtomicRetirementStaleEncodingFailure drives the stale sender after an
// accepted marked subset excludes A but valid indivisible attributes make B
// exceed the destination's message size. A must leave independently of B's
// encoding, on both direct and queued rails. Foreign and newer receipts remain
// protected, while fitting B and later independent routes remain useful.
func TestAtomicRetirementStaleEncodingFailure(t *testing.T) {
	first, second := []byte{24, 10, 42, 0}, []byte{24, 10, 43, 0}
	for _, tc := range []struct {
		name        string
		received    bool
		exact       bool
		newer       bool
		queued      bool
		oversized   bool
		writeFails  bool
		wantRemoval bool
	}{
		{name: "direct/local-owner", oversized: true, wantRemoval: true},
		{name: "direct/foreign-unowned", received: true, oversized: true},
		{name: "direct/current-exact-receipt", received: true, exact: true, oversized: true, wantRemoval: true},
		{name: "direct/newer-same-source-receipt", received: true, exact: true, newer: true, oversized: true},
		{name: "queued/current-exact-receipt", received: true, exact: true, queued: true, oversized: true, wantRemoval: true},
		{name: "queued/newer-same-source-receipt", received: true, exact: true, newer: true, queued: true, oversized: true},
		{name: "direct/cleanup-write-failure", oversized: true, writeFails: true},
		{name: "direct/fitting-survivor", received: true, exact: true, wantRemoval: true},
		{name: "queued/fitting-survivor", received: true, exact: true, queued: true, wantRemoval: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			f.destination.session.adjOut = &f.destination.adjOut
			f.destination.reactor = f.r
			batch := staleReadvertiseBatch(t)
			batch.NLRIs = nil
			for _, raw := range [][]byte{first, second} {
				prefix, err := nlri.NewWireNLRI(batch.Family, raw, false)
				require.NoError(t, err)
				batch.NLRIs = append(batch.NLRIs, prefix)
			}
			batch.Attrs.SetAtomicAggregate().SetMED(10)
			batch.Replay = true
			adapter := &reactorAPIAdapter{r: f.r}
			facts := announceFactsFor(f.destination, batch.Family, netip.MustParseAddr("10.0.0.1"), false, f.destination.negotiated.Load())
			seed, err := adapter.buildBatchAnnounceUpdate(make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen), batch, facts)
			require.NoError(t, err)
			input := fwdPackUpdateBody(seed)
			if tc.received {
				require.NoError(t, ownershipWriterForward(t, f.destination, f.source, input, false))
			} else {
				require.NoError(t, f.destination.session.SendUpdate(seed))
			}
			previous := f.destination.session.sentReceipt.MessageID()
			require.NotZero(t, previous)
			if tc.exact {
				batch.SentOwnerMessage = previous
				batch.SourcePolicyEpoch = f.source.sourcePolicyEpoch.Load()
			}
			if tc.newer {
				require.NoError(t, ownershipWriterForward(t, f.destination, f.source, rfc4271ProofRelayBody(first, true, 20), false))
				require.Greater(t, f.destination.session.sentReceipt.MessageID(), previous)
			}
			before := len(aigpSocketBodies(t, f.conn))
			base, err := message.UnpackUpdate(rfc4271ProofRelayBody(second, true, 77))
			require.NoError(t, err)
			attrs := bytes.Clone(base.PathAttributes)
			if tc.oversized {
				// An optional transitive attribute with Extended Length is valid;
				// its indivisible value cannot fit a 4096-octet destination UPDATE.
				attrs = append(attrs, atomicFailureExtendedAttr(0xc0, 230, make([]byte, 5000))...)
			}
			output := makeUpdateBody(nil, attrs, second)
			exportLibraryAnswer(t, f.r, func(string) rpc.FilterUpdateOutput {
				return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
			}, f.destination)
			f.destination.raiseExportPolicyFrom(batch.SentOwnerMessage + 1)
			chain := staleChainFor(f.destination, batch.SentOwnerMessage)
			outcome, modified, removals := adapter.decideStaleReadvertise(f.destination.forwardFacts().filterInfo, chain, input, batch.Stale)
			require.Equal(t, staleModify, outcome, "the failure must follow a policy-accepted marked subset")
			require.Equal(t, output, modified)
			require.Len(t, removals, 1)
			removal, err := message.UnpackUpdate(removals[0])
			require.NoError(t, err)
			require.Equal(t, first, removal.WithdrawnRoutes, "accepted policy must owe exactly A before B is encoded")
			require.Empty(t, removal.PathAttributes)
			require.Empty(t, removal.NLRI)
			if tc.writeFails {
				session := f.destination.session
				session.writeMu.Lock()
				session.conn = &atomicStaleRetirementFailConn{recordingConn: f.conn}
				session.bufWriter = bufio.NewWriterSize(session.conn, message.MaxMsgLen)
				session.writeMu.Unlock()
			}
			if tc.queued {
				f.destination.initialUpdateOwed.Store(true)
				t.Cleanup(func() {
					f.destination.initialUpdateOwed.Store(false)
					f.destination.wakeForwardOverflow()
				})
				require.True(t, f.destination.withdrawBehindForwards(), "the stale send must take the queued branch")
			}
			sent, sendErr := adapter.sendStaleReadvertiseUnit(t.Context(), announceTarget{peer: f.destination, session: f.destination.session},
				batch, facts, message.MaxMsgLen, make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen))
			if tc.queued {
				require.Equal(t, before, len(aigpSocketBodies(t, f.conn)), "the accepted retirement must honor the existing replay fence")
				f.destination.initialUpdateOwed.Store(false)
				f.destination.wakeForwardOverflow()
				forwardSocketBarrier(t, f.r)
			}
			if tc.oversized {
				require.False(t, sent, "retirement alone must not report B delivered")
				require.ErrorIs(t, sendErr, message.ErrAttributesTooLarge, "the actual splitter must reject B's indivisible attributes")
			} else {
				require.NoError(t, sendErr)
				require.True(t, sent)
			}
			written := aigpSocketBodies(t, f.conn)[before:]
			if tc.writeFails {
				require.ErrorIs(t, sendErr, io.ErrClosedPipe, "cleanup must retain its transport failure beside B's split failure")
				require.Empty(t, written)
				return
			}
			var kept, removed []byte
			if tc.wantRemoval {
				removed = first
			}
			if !tc.oversized {
				kept = second
			}
			atomicAdmissionAssertRetirement(t, written, kept, removed)
			for _, body := range written {
				update, err := message.UnpackUpdate(body)
				require.NoError(t, err)
				require.False(t, update.IsEndOfRIBAnyFamily(), "failed survivor cleanup must not invent an EOR")
			}
			f.destination.settings.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalAccept)
			f.destination.refreshForwardFacts()
			if tc.received && !tc.wantRemoval {
				before = len(aigpSocketBodies(t, f.conn))
				require.NoError(t, ownershipWriterForward(t, f.destination, f.source, makeUpdateBody(first, nil, nil), false))
				atomicAdmissionAssertRetirement(t, aigpSocketBodies(t, f.conn)[before:], nil, first)
			}
			later := []byte{24, 10, 44, 0}
			before = len(aigpSocketBodies(t, f.conn))
			require.NoError(t, f.destination.session.SendUpdate(ownershipWriterUpdate(t, rfc4271ProofRelayBody(later, true, 77))))
			atomicAdmissionAssertRetirement(t, aigpSocketBodies(t, f.conn)[before:], later, nil)
		})
	}
}
