package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: R-5 / coupling #1. routeInbound keys the owner-loop hand-off on SA
// identity (ps.ownedSA == packet's SA), not the peer name. A packet for a DIFFERENT
// SA of the same peer -- the parallel half-open re-init -- is NOT delivered to the
// established SA's owner loop (where it would decrypt under the wrong keys); it is
// handled inline instead. RFC 7296 Section 2.4.
// PREVENTS: the naive one-line responderBusy fix, which would misroute the accepted
// re-init to the old SA and silently fail to decrypt.
func TestRouteInboundKeysOnSANotPeer(t *testing.T) {
	log := slogutil.DiscardLogger()
	table := NewSATable()

	owned := testSA()
	owned.PeerName = "ze"
	owned.State = StateEstablished
	other := testSA() // a distinct SA of the SAME peer (the parallel handshake)
	other.PeerName = "ze"
	other.State = StateSAInitReceived

	ps := &PeerSession{peerName: "ze", inbound: make(chan transport.Packet, inboundQueueDepth)}
	ps.ownedSA.Store(owned)
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	// A packet for the parallel SA must NOT reach the owner loop. Short data makes the
	// inline handleInbound return without side effects; we only assert the routing.
	routeInbound(other, transport.Packet{Data: make([]byte, 8)}, table, nil, log)
	select {
	case <-ps.inbound:
		t.Fatal("a parallel-SA packet was wrongly delivered to the established SA's owner loop")
	default:
	}

	// A packet for the owned SA does reach the owner loop.
	routeInbound(owned, transport.Packet{Data: []byte("owned-sa-packet")}, table, nil, log)
	select {
	case <-ps.inbound:
	case <-time.After(time.Second):
		t.Fatal("the owned SA's packet did not reach the owner inbound queue")
	}
}

// VALIDATES: spec-fixit-ike-resource-lifetime-leaks AC-2 -- when runEngine takes an error
// exit after the bypass is installed, no IKE bypass policy is left in the kernel policy
// table: every policy start installed is released.
// PREVENTS: four node-wide XFRM policies surviving the daemon that installed them. The
// release sat in the shutdown tail, BELOW the `return 1` that a failed plugin handshake
// takes, so a start that failed after installIKEBypass left ze's IKE ports exempt from
// IPsec processing for a daemon that is no longer running. The policies carry no peer
// identity and are not reference-counted, so nothing on the box ever removed them.
//
// It drives runEngine itself rather than the removal helper: the helper is already proven
// by TestInstallAndRemoveIKEBypassCoverBothFamilies, and the defect was the WIRING between
// the two -- which exits reach it (ai/rules/evidence.md).
func TestRunRemovesIKEBypassOnEveryErrorExit(t *testing.T) {
	dp := &bypassDP{}
	const backend = "test-ike-bypass-record"
	if err := dataplane.Register(backend, func() (dataplane.Dataplane, error) { return dp, nil }); err != nil {
		t.Fatalf("register the recording dataplane backend: %v", err)
	}

	origName := ikeDataplaneFn
	ikeDataplaneFn = func() string { return backend }
	origLog := loggerPtr.Load()
	setLogger(slogutil.DiscardLogger())

	// runEngine writes process-wide state that outlives it. Put it back so a later
	// test in this package reads what it would have read.
	t.Cleanup(func() {
		ikeDataplaneFn = origName
		loggerPtr.Store(origLog)
		activeTablePtr.Store(nil)
		setActivePeers(nil)
		reEstablishFn.Store(nil)
		_ = dataplane.CloseBackend()
	})

	// The engine end of the plugin pipe is closed before runEngine reads it, so the
	// SDK fails at stage 1 (declare-registration) and runEngine takes its error exit.
	// Every error exit above the shutdown tail has that shape.
	ours, theirs := net.Pipe()
	if err := theirs.Close(); err != nil {
		t.Fatalf("close the engine end of the plugin pipe: %v", err)
	}

	if code := runEngine(ours); code != 1 {
		t.Fatalf("runEngine returned %d on a broken plugin connection, want 1 (the error exit under test)", code)
	}

	if len(dp.installed) == 0 {
		t.Fatal("engine start installed no bypass policy, so the removal asserted below proves nothing")
	}

	want := map[spKey]bool{}
	for _, p := range dp.installed {
		want[keyOf(p)] = true
	}
	got := map[spKey]bool{}
	for _, p := range dp.removed {
		got[keyOf(p)] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("the error exit left the bypass policy %+v installed; it is node-wide, so it outlives this process", k)
		}
	}
	if len(got) != len(want) {
		t.Errorf("the error exit released %d distinct policies, start installed %d", len(got), len(want))
	}
}
