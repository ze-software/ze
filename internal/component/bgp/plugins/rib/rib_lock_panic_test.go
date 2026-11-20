// Design: docs/architecture/plugin/rib-storage-design.md -- receive path, peer and FlowSpec locks
// Related: rib_structured.go -- handleReceivedStructured, admitReceivedPeer, the deferred failSourceProcessed
// Related: rib_bestchange.go -- checkRouteBestChange, the peerMu read section
// Related: rib_flowspec_validation.go -- reconcileFlowSpecs, the flowSpecMu and peerMu sections

package rib

import (
	"bytes"
	"net/netip"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// lockPanicCause is what the test receive gate panics with, a type of its own
// so the test can tell its own panic from any other one the handler raises.
type lockPanicCause string

const lockPanicGate lockPanicCause = "test receive gate panicked"

// lockPanicBound is the failure deadline of one received UPDATE, never a
// success condition. Every call the handler makes is answered in this
// process, so a handler still running after the bound is wedged, not slow.
const lockPanicBound = 10 * time.Second

// lockPanicNextHop is the MP_REACH next hop of every non-FlowSpec route here.
var lockPanicNextHop = netip.MustParseAddr("192.0.2.254")

// lockPanicEvent builds one received UPDATE announcing raw for fam through
// MP_REACH_NLRI after attrs, with receipt and owner both owner and the
// physical connection 41. A FlowSpec announcement carries no next hop
// (RFC 8955 Section 4.1), every other one carries lockPanicNextHop.
func lockPanicEvent(t *testing.T, owner uint64, peer netip.Addr, id uint64, fam family.Family, raw, attrs []byte) *rpc.StructuredEvent {
	t.Helper()
	mp := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI)}
	var nextHop []byte
	if !ribevents.IsFlowSpec(fam) {
		nextHop = lockPanicNextHop.AsSlice()
	}
	mp = append(mp, byte(len(nextHop)))
	mp = append(mp, nextHop...)
	mp = append(mp, 0)
	mp = append(mp, raw...)
	attrs = append(bytes.Clone(attrs), 0x80, 14, byte(len(mp)))
	attrs = append(attrs, mp...)
	body := []byte{0, 0, byte(len(attrs) >> 8), byte(len(attrs))}
	body = append(body, attrs...)
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	require.NoError(t, err)
	wu := wireu.NewWireUpdate(body, ctxID)
	wu.SetSourceOwner(owner)
	wu.SetSourceReceipt(owner)
	aw, err := wu.Attrs()
	require.NoError(t, err)
	return &rpc.StructuredEvent{ConnectionID: 41,
		EventType: rpc.EventKindUpdate, PeerOwner: owner, PeerAddress: peer.String(), PeerAS: rpc.RemoteASOf(65001), LocalAS: 65000,
		SourceReceipt:  owner,
		RemoteRouterID: 0x01010101,
		RawMessage:     &bgptypes.RawMessage{Type: msgtype.TypeUPDATE, MessageID: id, SourceOwner: owner, SourceReceipt: owner, RawBytes: body, WireUpdate: wu, AttrsWire: aw},
	}
}

// receiveRecovering delivers event on a goroutine of its own. It answers what
// the handler panicked with (nil for a normal return) and returned=true, or
// returned=false when the handler did not come back within lockPanicBound.
func receiveRecovering(r *RIBManager, event *rpc.StructuredEvent) (cause any, returned bool) {
	done := make(chan any, 1)
	go func() {
		var recovered any
		defer func() { done <- recovered }()
		defer func() { recovered = recover() }()
		r.handleReceivedStructured(event)
	}()
	bound := time.NewTimer(lockPanicBound)
	defer bound.Stop()
	select {
	case cause = <-done:
		return cause, true
	case <-bound.C:
		return nil, false
	}
}

// lockPanicStacks is every goroutine's stack, so a wedge names its lock.
func lockPanicStacks() string {
	stacks := make([]byte, 1<<20)
	return string(stacks[:runtime.Stack(stacks, true)])
}

// VALIDATES: a panic raised while checkRouteBestChange holds peerMu.RLock
// reaches the caller of handleReceivedStructured, the call returns, the read
// lock is released and the source's unapplied UPDATE is recorded as a gap.
// Method: the receive gate (ribevents.RegisterValidationLookup, the seam the
// selection already calls under peerMu.RLock from gatherPrefixCandidatesLocked)
// panics for every unicast query; one unicast UPDATE whose next hop resolves
// through the Loc-RIB reaches that gate, and the handler runs on its own
// goroutine against a bound.
// PREVENTS: the read lock staying held while the handler's deferred
// failSourceProcessed waits for peerMu.Lock on the same goroutine, which turns
// a BUG panic into a silent, permanent hang of the RIB.
func TestReceivePanicUnderPeerReadLockPropagates(t *testing.T) {
	if ribevents.ValidationEnabled() {
		t.Fatal("test requires no receive validation owner")
	}
	previousBus := eventBusPtr.Load()
	t.Cleanup(func() { eventBusPtr.Store(previousBus) })
	r := newTestRIBManagerWithBus(newTestEventBus())
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	wedged := false
	t.Cleanup(func() {
		// SetLocRIB takes peerMu.Lock, which a wedged handler never lets go.
		if !wedged {
			r.SetLocRIB(nil)
		}
	})
	loc.Insert(family.IPv4Unicast, netip.PrefixFrom(lockPanicNextHop, lockPanicNextHop.BitLen()), locrib.Path{
		Source: redistevents.RegisterProtocol("connected"), Interface: "lock-panic0",
	})
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	ribevents.RegisterValidationLookup(func(route ribevents.ValidationRoute, _ uint64) bool {
		if route.Family == family.IPv4Unicast {
			panic(lockPanicGate)
		}
		return true
	}, nil)

	peer := netip.MustParseAddr("192.0.2.1")
	attrs := flowValidationAttrs(flowValidationPath(2, 65001), netip.Addr{}, 0, nil)
	cause, returned := receiveRecovering(r, lockPanicEvent(t, 1, peer, 10, family.IPv4Unicast, []byte{24, 203, 0, 113}, attrs))
	if !returned {
		wedged = true
		t.Fatalf("the received UPDATE did not return within %s after the receive gate panicked under peerMu.RLock: "+
			"the handler is wedged on the read lock the panic left held\n%s", lockPanicBound, lockPanicStacks())
	}
	require.Equal(t, lockPanicGate, cause, "the panic raised under peerMu.RLock must reach the caller unchanged")
	require.True(t, r.peerMu.TryLock(), "the panic left peerMu held")
	meta := r.peerMeta[1]
	var failed bool
	if meta != nil {
		failed = meta.processingFailed
	}
	r.peerMu.Unlock()
	require.NotNil(t, meta, "the UPDATE's peer metadata must be recorded before selection")
	require.True(t, failed, "the deferred cleanup must record the unapplied UPDATE as the source's gap")
}

// VALIDATES: a panic raised while reconcileFlowSpecs holds flowSpecMu reaches
// the caller of handleReceivedStructured and leaves neither flowSpecMu nor
// peerMu held.
// Method: a covering unicast route is stored with the receive gate answering
// normally; the gate is then armed to panic for unicast queries, and a FlowSpec
// UPDATE for a destination that route covers makes reconcileFlowSpecs ask the
// gate about it (dropIneligible), which it does holding flowSpecMu alone.
// PREVENTS: flowSpecMu staying held after the panic, so every later
// reconciliation, from any UPDATE, validation change or replay, waits on it
// forever and FlowSpec selection silently stops.
func TestReceivePanicUnderFlowSpecLockReleasesIt(t *testing.T) {
	if ribevents.ValidationEnabled() {
		t.Fatal("test requires no receive validation owner")
	}
	r, _ := flowValidationFixture(t)
	var armed atomic.Bool
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	ribevents.RegisterValidationLookup(func(route ribevents.ValidationRoute, _ uint64) bool {
		if armed.Load() && route.Family == family.IPv4Unicast {
			panic(lockPanicGate)
		}
		return true
	}, nil)

	peer := netip.MustParseAddr("192.0.2.1")
	sequence := flowValidationPath(2, 65001)
	cause, returned := receiveRecovering(r, lockPanicEvent(t, 1, peer, 1, family.IPv4Unicast, []byte{8, 10}, flowValidationAttrs(sequence, netip.Addr{}, 0, nil)))
	require.True(t, returned, "the covering unicast UPDATE must return\n%s", lockPanicStacks())
	require.Nil(t, cause, "the covering unicast UPDATE must store without a panic")

	armed.Store(true)
	cause, returned = receiveRecovering(r, lockPanicEvent(t, 1, peer, 2, flowspecFamily, flowspecNLRI(16, 10, 1), flowValidationAttrs(sequence, netip.Addr{}, 100, nil)))
	armed.Store(false)
	require.True(t, returned, "the FlowSpec UPDATE did not return within %s\n%s", lockPanicBound, lockPanicStacks())
	require.Equal(t, lockPanicGate, cause, "the panic raised in reconcileFlowSpecs must reach the caller unchanged")
	require.True(t, r.flowSpecMu.TryLock(), "the panic left flowSpecMu held: every later FlowSpec reconciliation would wait on it forever")
	r.flowSpecMu.Unlock()
	require.True(t, r.peerMu.TryLock(), "the panic left peerMu held")
	r.peerMu.Unlock()
}
