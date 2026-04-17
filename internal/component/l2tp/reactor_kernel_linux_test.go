package l2tp

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newUnstartedReactor builds a reactor + bound listener WITHOUT starting
// the reactor goroutine. Kernel-integration tests drive internal helpers
// (collectKernelEventsLocked, handleKernelError) directly, so they do not
// need the run loop and must not race with it.
//
// SetKernelWorker is documented as "must be called before Start()"; this
// helper lets tests honor that contract.
func newUnstartedReactor(t *testing.T) (*UDPListener, *L2TPReactor, func()) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(&lockedBuffer{}, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ln := NewUDPListener(netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0), logger)
	require.NoError(t, ln.Start(context.Background()))
	r := NewL2TPReactor(ln, logger, ReactorParams{
		Defaults: TunnelDefaults{HostName: "ze-test", FramingCapabilities: 0x3, RecvWindow: 16},
	})
	stop := func() {
		_ = ln.Stop()
	}
	return ln, r, stop
}

// addEstablishedSession inserts a session in the established state into
// the tunnel. Used by reactor kernel-integration tests that need a
// pre-built state graph without driving the FSM through real packets.
func addEstablishedSession(tun *L2TPTunnel, localSID, remoteSID uint16, lns bool) *L2TPSession {
	sess := &L2TPSession{
		localSID:  localSID,
		remoteSID: remoteSID,
		state:     L2TPSessionEstablished,
		lnsMode:   lns,
	}
	tun.addSession(sess)
	return sess
}

// mkTunnel builds a tunnel in the established state and registers it
// with the reactor's tunnel maps.
func mkTunnel(r *L2TPReactor, localTID, remoteTID uint16, peer netip.AddrPort) *L2TPTunnel {
	tun := newTunnel(localTID, remoteTID, peer,
		ReliableConfig{MaxRetransmit: 3, RTimeout: time.Second, RTimeoutCap: 4 * time.Second, RecvWindow: 4},
		r.logger, time.Now())
	tun.state = L2TPTunnelEstablished
	r.tunnelsMu.Lock()
	r.tunnelsByLocalID[tun.localTID] = tun
	r.tunnelsByPeer[peerKey{addr: tun.peerAddr, tid: tun.remoteTID}] = tun
	r.tunnelsMu.Unlock()
	return tun
}

func TestReactorCollectsKernelSetupEvent(t *testing.T) {
	// VALIDATES: collectKernelEventsLocked converts kernelSetupNeeded
	// flags into kernelSetupEvent entries with the right IDs, peer addr,
	// LNS mode, and sequencing flag.
	// PREVENTS: reactor silently ignoring established sessions when the
	// kernel worker is configured.
	ln, r, stop := newUnstartedReactor(t)
	defer stop()

	fake := &fakeKernelOps{}
	errCh := make(chan kernelSetupFailed, 4)
	successCh := make(chan kernelSetupSucceeded, 4)
	w := newKernelWorker(fake.ops(), errCh, successCh, r.logger)
	w.Start()
	defer w.Stop()
	r.SetKernelWorker(w, errCh, successCh)

	peer := netip.MustParseAddrPort("10.0.0.7:1701")
	tun := mkTunnel(r, 100, 200, peer)
	sess := addEstablishedSession(tun, 1001, 2001, true)
	sess.kernelSetupNeeded = true
	sess.sequencingRequired = true

	socketFD, err := ln.SocketFD()
	require.NoError(t, err)

	r.tunnelsMu.Lock()
	setups, teardowns := r.collectKernelEventsLocked(tun)
	r.tunnelsMu.Unlock()

	require.Len(t, setups, 1, "one setup event expected")
	require.Empty(t, teardowns, "no teardowns expected")
	ev := setups[0]
	require.Equal(t, uint16(100), ev.localTID)
	require.Equal(t, uint16(200), ev.remoteTID)
	require.Equal(t, uint16(1001), ev.localSID)
	require.Equal(t, uint16(2001), ev.remoteSID)
	require.Equal(t, peer, ev.peerAddr)
	require.Equal(t, socketFD, ev.socketFD)
	require.True(t, ev.lnsMode)
	require.True(t, ev.sequencing)
	require.False(t, sess.kernelSetupNeeded, "flag must be cleared after collection")
}

func TestReactorCollectsTeardownEvent(t *testing.T) {
	// VALIDATES: collectKernelEventsLocked drains the tunnel's
	// pendingKernelTeardowns list.
	// PREVENTS: CDN teardowns that fail to propagate to the kernel
	// worker, which would leave kernel resources behind.
	_, r, stop := newUnstartedReactor(t)
	defer stop()

	fake := &fakeKernelOps{}
	errCh := make(chan kernelSetupFailed, 4)
	successCh := make(chan kernelSetupSucceeded, 4)
	w := newKernelWorker(fake.ops(), errCh, successCh, r.logger)
	w.Start()
	defer w.Stop()
	r.SetKernelWorker(w, errCh, successCh)

	tun := mkTunnel(r, 101, 201, netip.MustParseAddrPort("10.0.0.8:1701"))
	tun.pendingKernelTeardowns = []kernelTeardownEvent{
		{localTID: 101, localSID: 1001},
		{localTID: 101, localSID: 1002},
	}

	r.tunnelsMu.Lock()
	setups, teardowns := r.collectKernelEventsLocked(tun)
	r.tunnelsMu.Unlock()

	require.Empty(t, setups)
	require.Len(t, teardowns, 2)
	require.Empty(t, tun.pendingKernelTeardowns, "teardown list must be drained")
}

func TestReactorKernelDisabledReturnsNil(t *testing.T) {
	// VALIDATES: with no kernel worker configured (non-Linux path or
	// subsystem start without kernel support), collectKernelEventsLocked
	// returns nil instead of producing events for a nil worker.
	// PREVENTS: nil-deref when kernel integration is disabled.
	_, r, stop := newUnstartedReactor(t)
	defer stop()

	tun := mkTunnel(r, 102, 202, netip.MustParseAddrPort("10.0.0.9:1701"))
	sess := addEstablishedSession(tun, 1001, 2001, true)
	sess.kernelSetupNeeded = true
	tun.pendingKernelTeardowns = []kernelTeardownEvent{{localTID: 102, localSID: 9}}

	r.tunnelsMu.Lock()
	setups, teardowns := r.collectKernelEventsLocked(tun)
	r.tunnelsMu.Unlock()

	require.Nil(t, setups)
	require.Nil(t, teardowns)
	require.True(t, sess.kernelSetupNeeded, "flag must NOT clear when worker absent")
}

func TestReactorHandleKernelErrorSendsCDN(t *testing.T) {
	// VALIDATES: AC-13 -- a kernel setup failure routed back to the
	// reactor results in the session being torn down (session removed,
	// CDN enqueued to peer).
	// PREVENTS: sessions stuck established in ze's view while the
	// kernel has no corresponding resources.
	_, r, stop := newUnstartedReactor(t)
	defer stop()

	// Use the test discard address (RFC 6890 documentation prefix) so
	// the test cannot accidentally deliver a CDN to a real L2TP daemon
	// listening on the loopback. Send still succeeds at the syscall
	// layer; only the assertion on session state matters here.
	peer := netip.MustParseAddrPort("192.0.2.1:1701")
	tun := mkTunnel(r, 103, 203, peer)
	sess := addEstablishedSession(tun, 1001, 2001, true)

	r.handleKernelError(kernelSetupFailed{localTID: 103, localSID: 1001})

	r.tunnelsMu.Lock()
	_, stillThere := tun.sessions[sess.localSID]
	r.tunnelsMu.Unlock()
	require.False(t, stillThere, "session must be removed after kernel error")
}

func TestReactorDiscardTunnelReturnsKernelTeardowns(t *testing.T) {
	// VALIDATES: discardTunnelLocked drains and returns the
	// just-discarded tunnel's pendingKernelTeardowns. Closes the leak
	// where tie-breaker losers' kernel resources were never reaped.
	// PREVENTS: kernel state leak when a peer wins a tie-breaker race
	// against a tunnel that already had an established session.
	_, r, stop := newUnstartedReactor(t)
	defer stop()

	tun := mkTunnel(r, 200, 300, netip.MustParseAddrPort("192.0.2.10:1701"))
	addEstablishedSession(tun, 5001, 6001, true)
	addEstablishedSession(tun, 5002, 6002, false)

	r.tunnelsMu.Lock()
	teardowns := r.discardTunnelLocked(tun, "test")
	r.tunnelsMu.Unlock()

	if len(teardowns) != 2 {
		t.Fatalf("expected 2 teardowns from established sessions, got %d", len(teardowns))
	}
	// Tunnel must be removed from the maps.
	r.tunnelsMu.Lock()
	_, byID := r.tunnelsByLocalID[200]
	r.tunnelsMu.Unlock()
	if byID {
		t.Fatal("tunnel should have been removed from tunnelsByLocalID")
	}
	// pendingKernelTeardowns must be drained on the (now orphaned) tunnel.
	if tun.pendingKernelTeardowns != nil {
		t.Fatalf("pendingKernelTeardowns must be cleared, got %v", tun.pendingKernelTeardowns)
	}
}

func TestReactorHandleKernelErrorSessionGone(t *testing.T) {
	// VALIDATES: AC-13 -- an error for a session that was already
	// removed (CDN from peer arrived concurrently) is a no-op.
	// PREVENTS: nil deref / double teardown when the reactor and peer
	// race to remove the same session.
	_, r, stop := newUnstartedReactor(t)
	defer stop()

	// No tunnel registered for TID 999 -- handleKernelError must survive.
	r.handleKernelError(kernelSetupFailed{localTID: 999, localSID: 5})

	// Tunnel exists, but session does not -- also a no-op.
	// Use TEST-NET-1 (RFC 5737) so any send from teardownSession does not
	// reach a real L2TP daemon on this machine.
	_ = mkTunnel(r, 104, 204, netip.MustParseAddrPort("192.0.2.2:1701"))
	r.handleKernelError(kernelSetupFailed{localTID: 104, localSID: 99})
}
