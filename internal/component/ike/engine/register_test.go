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

// VALIDATES: an engine that exits before its first configuration apply installed no SPD
// entry and removed none: Ze's SPD, the IKE bypass and the catch-all, appears on the
// first apply and not at engine start (owner decision 2026-10-20, Q6 B), and a catch-all
// another policy manager left in place survives the failed start.
// PREVENTS: the IKE bypass going in at engine start, before any configuration, which
// made Ze hold SPD entries with no catch-all beneath them; and the earlier defect, a
// start failing after that install leaving four node-wide bypass policies behind.
//
// It drives runEngine itself rather than a helper, because the defect class is the
// WIRING: which step of the start installs, and which exits release.
func TestRunInstallsNoSPDEntryBeforeTheFirstApply(t *testing.T) {
	dp := &bypassDP{}
	// This engine has not applied a configuration yet and does not own the
	// catch-all left by another policy manager. Startup failure must preserve it.
	foreign, err := unmatchedPolicies(net.IPv4zero, dataplane.SPActionBypass)
	if err != nil {
		t.Fatal(err)
	}
	if err := dp.InstallPolicy(foreign[0]); err != nil {
		t.Fatal(err)
	}
	dp.installed = nil
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

	if len(dp.installed) != 0 {
		t.Errorf("an engine that applied no configuration installed %d SPD entries: %+v", len(dp.installed), dp.installed)
	}
	if len(dp.removed) != 0 {
		t.Errorf("an engine that applied no configuration removed %d SPD entries: %+v", len(dp.removed), dp.removed)
	}
	if len(dp.live) != 1 {
		t.Fatalf("startup failure changed the pre-existing policy set: %+v", dp.live)
	}
	if kept, ok := dp.live[keyOf(foreign[0])]; !ok || kept.Action != foreign[0].Action {
		t.Fatal("startup failure removed or changed a catch-all it never installed")
	}
}
