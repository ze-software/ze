// Design: docs/architecture/plugin/rib-storage-design.md -- source-policy lifecycle owner
// RFC naming: untagged -- tests machinery RFC 8212 default reject relies on; rows RFC8212-3-1/3-2 are {gap}, which refuse a proof tag until the spec closes them (AC-11).
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
// Related: source_policy.go -- the handler these tests drive

package rs

import (
	"context"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The source whose import policy changes, and another source that stays.
const (
	policySource = "10.0.0.1"
	policyOther  = "10.0.0.2"
)

// policyKey is the withdrawal-map key of an IPv4 unicast prefix.
func policyKey(prefix string) withdrawalKey {
	return withdrawalKey{fam: family.IPv4Unicast, prefix: netip.MustParsePrefix(prefix)}
}

// newPolicyRouteServer is a route server whose source and other peer are up,
// each holding routes admitted at epoch 5: the source two, the other one.
// Departures are recorded through updateRouteHook and stopped at cleanup.
func newPolicyRouteServer(t *testing.T) (*routeServer, *departureLog) {
	t.Helper()
	rs := newTestRouteServer(t)
	for _, peer := range []string{policySource, policyOther} {
		rs.peers[peer] = &PeerState{Address: peer, Up: true, StateSeen: true}
	}
	rs.withdrawals[policySource] = map[withdrawalKey]withdrawalEntry{
		policyKey("10.1.0.0/24"): {sourcePolicyEpoch: 5},
		policyKey("10.2.0.0/24"): {sourcePolicyEpoch: 5},
	}
	rs.withdrawals[policyOther] = map[withdrawalKey]withdrawalEntry{
		policyKey("10.1.0.0/24"): {sourcePolicyEpoch: 5},
	}
	log := &departureLog{}
	rs.updateRouteHook = log.record
	t.Cleanup(rs.stopDepartures)
	return rs, log
}

// departureLog records the departure commands the route server sends.
type departureLog struct {
	mu       sync.Mutex
	commands []string
	delay    time.Duration
}

func (l *departureLog) record(_, command string) {
	time.Sleep(l.delay)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.commands = append(l.commands, command)
}

func (l *departureLog) sent() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.commands...)
}

// policyPhase runs one lifecycle request of transaction tx1 for the source,
// captured at epoch 5 under session generation 3.
func policyPhase(t *testing.T, rs *routeServer, op rpc.SourcePolicyOperation, next uint64) error {
	t.Helper()
	input := rpc.SourcePolicyInput{TransactionID: "tx1", Operation: op,
		Source: rpc.SourcePolicyIdentity{Peer: policySource, PeerOwner: 1, SessionGeneration: 3,
			PolicyEpoch: 5, ReceiveFrontier: 9, TemplateGeneration: 1},
		NextPolicyEpoch: next, DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
	require.NoError(t, input.Validate(time.Now()))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	return rs.handleSourcePolicy(ctx, input)
}

// sourceEpochs is the epoch of each key the source's withdrawal map holds.
func sourceEpochs(rs *routeServer) map[string]uint64 {
	rs.withdrawalMu.Lock()
	defer rs.withdrawalMu.Unlock()
	epochs := make(map[string]uint64)
	for key, entry := range rs.withdrawals[policySource] {
		epochs[key.prefix.String()] = entry.sourcePolicyEpoch
	}
	return epochs
}

// slowWorkers replaces the worker pool with one whose handler takes 50 ms
// for every item but a settle marker, and counts the items it finished.
func slowWorkers(t *testing.T, rs *routeServer) *atomic.Int32 {
	t.Helper()
	rs.workers.Stop()
	var finished atomic.Int32
	rs.workers = newWorkerPool(func(key workerKey, item workItem) {
		if item.barrier == nil {
			time.Sleep(50 * time.Millisecond)
			finished.Add(1)
			return
		}
		rs.processForward(key, item)
	}, poolConfig{chanSize: 64, idleTimeout: 5 * time.Second})
	t.Cleanup(rs.workers.Stop)
	return &finished
}

// TestRFC8212SourcePolicyRSSettlesSource queues slow work for the source, a
// worker item and (for prepare) a departure of its previous session, then
// runs each phase that owes a settle.
//
// VALIDATES: prepare returns only after the queued departure was sent and the
// worker's queued item was handled; apply and abort return only after every
// relay queued on the source's worker (the receive store's eligibility relays
// arrive there) was handled.
// PREVENTS: a receipt answered while the source's forwards or relays are still
// in flight, so the engine's settle of destination writers misses them.
func TestRFC8212SourcePolicyRSSettlesSource(t *testing.T) {
	t.Run("prepare", func(t *testing.T) {
		rs, log := newPolicyRouteServer(t)
		finished := slowWorkers(t, rs)
		// Slower than the worker item, so only the departure wait covers it.
		log.delay = 150 * time.Millisecond
		require.True(t, rs.enqueueDeparture(policySource, map[withdrawalKey]withdrawalEntry{
			policyKey("10.9.0.0/24"): {generation: 2},
		}))
		require.True(t, rs.workers.Dispatch(workerKey{sourcePeer: policySource}, workItem{sourcePeer: policySource}))
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyPrepare, 0))
		require.Equal(t, int32(1), finished.Load(), "the queued item was handled before prepare answered")
		require.Len(t, log.sent(), 1, "the queued departure was sent before prepare answered")
	})
	t.Run("apply", func(t *testing.T) {
		rs, _ := newPolicyRouteServer(t)
		finished := slowWorkers(t, rs)
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyPrepare, 0))
		require.True(t, rs.workers.Dispatch(workerKey{sourcePeer: policySource}, workItem{sourcePeer: policySource}))
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyApply, 6))
		require.Equal(t, int32(1), finished.Load(), "the queued relay was handled before apply answered")
	})
	t.Run("abort", func(t *testing.T) {
		rs, _ := newPolicyRouteServer(t)
		finished := slowWorkers(t, rs)
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyPrepare, 0))
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyApply, 6))
		require.True(t, rs.workers.Dispatch(workerKey{sourcePeer: policySource}, workItem{sourcePeer: policySource}))
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyAbort, 7))
		require.Equal(t, int32(1), finished.Load(), "the queued relay was handled before abort answered")
	})
}

// TestRFC8212SourcePolicyRSPrepareRefusals prepares a source whose session is
// down, and a source holding a route forwarded under a newer epoch.
//
// VALIDATES: each refusal names the source and the reason, and installs no
// transaction, so a following apply is refused as never prepared.
// PREVENTS: pins taken for a session that ended, or under an epoch older than
// what the route server already forwarded.
func TestRFC8212SourcePolicyRSPrepareRefusals(t *testing.T) {
	t.Run("down", func(t *testing.T) {
		rs, _ := newPolicyRouteServer(t)
		rs.peers[policySource].Up = false
		err := policyPhase(t, rs, rpc.SourcePolicyPrepare, 0)
		require.ErrorContains(t, err, "bgp-rs: source "+policySource+" has no established session")
		require.ErrorContains(t, policyPhase(t, rs, rpc.SourcePolicyApply, 6), "has no prepared source-policy transaction")
	})
	t.Run("stale", func(t *testing.T) {
		rs, _ := newPolicyRouteServer(t)
		rs.withdrawals[policySource][policyKey("10.3.0.0/24")] = withdrawalEntry{sourcePolicyEpoch: 9}
		err := policyPhase(t, rs, rpc.SourcePolicyPrepare, 0)
		require.ErrorContains(t, err, "is older than a route already forwarded")
		require.ErrorContains(t, err, policySource)
		require.ErrorContains(t, policyPhase(t, rs, rpc.SourcePolicyApply, 10), "has no prepared source-policy transaction")
	})
}

// TestRFC8212SourcePolicyRSFinalize applies and finalizes the source's
// transaction, then takes the source's session down.
//
// VALIDATES: finalize removes the source's denied keys from the withdrawal map
// and leaves the other source's; the departure that follows owes none of the
// denied routes, so nothing is sent for them; a stale abort after finalize is
// refused.
// PREVENTS: an accepted denial that keeps the denied routes for a later
// recovery, and a later departure that re-reconciles routes already settled.
func TestRFC8212SourcePolicyRSFinalize(t *testing.T) {
	rs, log := newPolicyRouteServer(t)
	require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyPrepare, 0))
	require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyApply, 6))
	require.Len(t, sourceEpochs(rs), 2, "apply keeps the routes for abort")
	require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyFinalize, 0))
	require.Empty(t, sourceEpochs(rs), "finalize drops the denied routes")
	require.Len(t, rs.withdrawals[policyOther], 1, "the other source keeps its route")
	require.ErrorContains(t, policyPhase(t, rs, rpc.SourcePolicyAbort, 7), "is finalized")

	rs.handleStateDown(policySource, 3)
	rs.stopDepartures()
	require.Empty(t, log.sent(), "no departure is owed for finalized routes")
}

// TestRFC8212SourcePolicyRSDepartureControl is the control of the finalize
// test: the same session goes down after an abort.
//
// VALIDATES: an aborted transaction leaves both routes in the inventory, so
// the departure carries both prefixes.
// PREVENTS: the finalize test passing because no departure is ever sent.
func TestRFC8212SourcePolicyRSDepartureControl(t *testing.T) {
	rs, log := newPolicyRouteServer(t)
	require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyPrepare, 0))
	require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyApply, 6))
	require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyAbort, 7))
	rs.handleStateDown(policySource, 3)
	rs.stopDepartures()
	sent := strings.Join(log.sent(), "\n")
	require.Contains(t, sent, "10.1.0.0/24")
	require.Contains(t, sent, "10.2.0.0/24")
}

// TestRFC8212SourcePolicyRSAbort aborts before apply and after apply.
//
// VALIDATES: a zero target releases the pins only: every epoch stays 5 and a
// later finalize is refused. A nonzero target restamps each pinned key still
// held to the target, never a key the source withdrew meanwhile and never
// another source's; a later finalize is refused.
// PREVENTS: a prepare-failure abort that moves the epoch, and an abort that
// leaves restored routes stamped with the superseded epoch.
func TestRFC8212SourcePolicyRSAbort(t *testing.T) {
	t.Run("before apply", func(t *testing.T) {
		rs, _ := newPolicyRouteServer(t)
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyPrepare, 0))
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyAbort, 0))
		require.Equal(t, map[string]uint64{"10.1.0.0/24": 5, "10.2.0.0/24": 5}, sourceEpochs(rs))
		require.ErrorContains(t, policyPhase(t, rs, rpc.SourcePolicyFinalize, 0), "was not applied")
	})
	t.Run("after apply", func(t *testing.T) {
		rs, _ := newPolicyRouteServer(t)
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyPrepare, 0))
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyApply, 6))
		rs.withdrawalMu.Lock()
		delete(rs.withdrawals[policySource], policyKey("10.2.0.0/24"))
		rs.withdrawalMu.Unlock()
		require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyAbort, 7))
		require.Equal(t, map[string]uint64{"10.1.0.0/24": 7}, sourceEpochs(rs))
		require.Equal(t, uint64(5), rs.withdrawals[policyOther][policyKey("10.1.0.0/24")].sourcePolicyEpoch)
		require.ErrorContains(t, policyPhase(t, rs, rpc.SourcePolicyFinalize, 0), "was not applied")
	})
}

// TestRFC8212SourcePolicyRSDownDropsTransaction prepares the source, then its
// session goes down.
//
// VALIDATES: the down drops the transaction, so a later apply is refused as
// never prepared, naming the source.
// PREVENTS: a phase of the ended session applied to its successor.
func TestRFC8212SourcePolicyRSDownDropsTransaction(t *testing.T) {
	rs, _ := newPolicyRouteServer(t)
	require.NoError(t, policyPhase(t, rs, rpc.SourcePolicyPrepare, 0))
	rs.handleStateDown(policySource, 3)
	err := policyPhase(t, rs, rpc.SourcePolicyApply, 6)
	require.ErrorContains(t, err, "bgp-rs: source "+policySource+" has no prepared source-policy transaction")
}
