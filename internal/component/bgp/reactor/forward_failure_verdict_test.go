// Tests for the verdict forwardUpdateCore returns when it dispatches to nobody.
//
// Two answers are possible and they mean opposite things. errAllDestinationsSuppressed
// says every destination was skipped by a POLICY decision, which is a correct outcome
// the caller may treat as success. errNoEstablishedPeersToForwardTo says the route was
// DROPPED, which the stored-route relay's completeness check must be able to see.
//
// Every branch below is a step that COULD NOT RUN. Each one must therefore reach the
// second answer, and the tests assert both halves: the drop sentinel is returned AND
// the suppression sentinel is not. Asserting only "an error" would leave a future
// suppressedCount++ on any of these paths green while it reopened the fail-open.
//
// Related: forward_community_test.go -- the stopped-pool dispatch branch,
// forward_modbuf_leak_test.go -- the buildFwdBody branch.

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// verdictFixture is one reactor around one destination, with that destination's
// Outgoing Peer Pool registered and its forward pool running. Only the pieces
// forwardUpdateCore reads are populated.
type verdictFixture struct {
	adapter *reactorAPIAdapter
	reactor *Reactor
	update  *ReceivedUpdate
	id      uint64
	dst     *Peer
}

func newVerdictFixture(t *testing.T, dst *Peer, payload []byte, srcCtxID bgpctx.ContextID) *verdictFixture {
	t.Helper()

	cache := newRecentUpdateCache(100)
	update, id := newLeakTestUpdate(t, cache, payload, srcCtxID)

	pool := newFwdPool(func(_ fwdKey, _ []fwdItem) {},
		fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)
	key := fwdKey{peerAddr: dst.Settings().PeerKey()}
	pool.registerOutgoingPool(key, 4096)

	r := &Reactor{
		attrModHandlers: attrModHandlersWithDefaults(),
		recentUpdates:   cache,
		peers:           map[netip.AddrPort]*Peer{key.peerAddr: dst},
		fwdPool:         pool,
	}
	return &verdictFixture{adapter: &reactorAPIAdapter{r: r}, reactor: r, update: update, id: id, dst: dst}
}

func (f *verdictFixture) forward() error {
	return f.adapter.forwardUpdateCore(f.update, f.id, []*Peer{f.dst}, forwardSourceInfo{
		resolved: true, globalLocalAS: 65000,
	})
}

// truncatedAttrPayload is an UPDATE body whose Total Path Attribute Length names
// more bytes than the message carries. Every tolerant scan in forwardUpdateCore
// reads it as "no attributes"; buildWithdrawalPayload refuses it outright, which
// is the branch under test.
func truncatedAttrPayload() []byte {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], 0)      // withdrawn routes length
	binary.BigEndian.PutUint16(payload[2:4], 0xFFFF) // attribute length, never present
	return payload
}

// unparseableASPathPayload advertises a prefix and carries an AS_PATH attribute
// whose declared length runs past the attribute section, so ASPathEdit.Record
// cannot index it. The prefix matters: without an advertised route the EBGP
// prepend rail is not the one that runs (aspath_slot.go, recordWithdrawOnly).
func unparseableASPathPayload() []byte {
	origin := []byte{0x40, 0x01, 0x01, 0x00}
	aspath := []byte{0x40, 0x02, 0x10, 0x02, 0x01} // declares 16 octets, carries 2
	attrs := append(append([]byte{}, origin...), aspath...)
	return buildUpdatePayload(attrs, []byte{24, 10, 0, 0})
}

// TestForwardZeroDispatchFailureIsADropNotASuppression drives one failure branch
// per producer that can leave forwardUpdateCore with nothing dispatched.
//
// VALIDATES: spec-fixit-stored-route-relay-hardening AC-10 -- a step that could
// not run reports errNoEstablishedPeersToForwardTo, never
// errAllDestinationsSuppressed.
// PREVENTS: the fail-open the sentinel exists to close. The relay reads
// errAllDestinationsSuppressed as "this route was correctly withheld from every
// destination" and counts the replay complete, so a rebuild failure, an
// unresolvable AS_PATH or a failed withdrawal conversion counted as suppression
// would report a replay that lost routes as a replay that finished.
func TestForwardZeroDispatchFailureIsADropNotASuppression(t *testing.T) {
	srcCtx := bgpctx.EncodingContextForASN4(true)
	srcCtxID, err := bgpctx.Registry.Register(srcCtx)
	require.NoError(t, err)

	t.Run("the modified payload cannot be built", func(t *testing.T) {
		dst := makeNextHopSelfIBGPPeer(t, "10.0.0.2", srcCtx, srcCtxID)
		f := newVerdictFixture(t, dst, modBufTestPayload(), srcCtxID)
		// Next-hop-self records a NEXT_HOP operation, and with no handler for it
		// the rebuild fails: modifyFailureNoHandler (forward_modify_failure.go).
		f.reactor.attrModHandlers = map[uint8]filterapi.AttrModHandler{}

		got := f.forward()
		require.ErrorIs(t, got, errNoEstablishedPeersToForwardTo,
			"a policy that could not be applied is a drop")
		assert.NotErrorIs(t, got, errAllDestinationsSuppressed,
			"the destination was not skipped by a decision; the rebuild failed")
	})

	t.Run("the withdrawal conversion fails", func(t *testing.T) {
		dst := makeNextHopSelfIBGPPeer(t, "10.0.0.3", srcCtx, srcCtxID)
		f := newVerdictFixture(t, dst, truncatedAttrPayload(), srcCtxID)
		// RFC 9494: the LLGR egress filter converts an announce to a withdrawal
		// for a destination that cannot hold the stale route.
		f.reactor.orderedEgressSteps = orderedEgressStepsFromFuncs(
			func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, mods *filterapi.ModAccumulator) bool {
				mods.SetWithdraw()
				return true
			})

		got := f.forward()
		require.ErrorIs(t, got, errNoEstablishedPeersToForwardTo,
			"a withdrawal that could not be built is a drop")
		assert.NotErrorIs(t, got, errAllDestinationsSuppressed,
			"the filter ACCEPTED this destination; only the conversion failed")
	})

	t.Run("the AS_PATH cannot be resolved", func(t *testing.T) {
		dst := makeDualASPeer(t, "10.0.0.4", srcCtx, srcCtxID)
		f := newVerdictFixture(t, dst, unparseableASPathPayload(), srcCtxID)

		got := f.forward()
		require.ErrorIs(t, got, errNoEstablishedPeersToForwardTo,
			"an EBGP destination whose AS_PATH will not index is a drop")
		assert.NotErrorIs(t, got, errAllDestinationsSuppressed,
			"nothing decided against this peer; the prepend could not be recorded")
	})

	// The control. The same rail, the same fixture, a destination an egress filter
	// genuinely refuses. Without it every NotErrorIs above could pass because the
	// suppression sentinel is never returned at all.
	t.Run("a policy decision IS a suppression", func(t *testing.T) {
		dst := makeNextHopSelfIBGPPeer(t, "10.0.0.5", srcCtx, srcCtxID)
		f := newVerdictFixture(t, dst, modBufTestPayload(), srcCtxID)
		f.reactor.orderedEgressSteps = orderedEgressStepsFromFuncs(
			func(_, _ filterapi.PeerFilterInfo, _ []byte, _ map[string]any, _ *filterapi.ModAccumulator) bool {
				return false
			})

		got := f.forward()
		assert.ErrorIs(t, got, errAllDestinationsSuppressed,
			"a filter that decided against this peer is policy, and the caller may treat it as success")
	})
}
