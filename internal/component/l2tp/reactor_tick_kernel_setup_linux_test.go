// Design: docs/research/l2tpv2-ze-integration.md -- Kernel and PPP event handling
// Related: reactor.go -- handleTick, which retries a kernel setup the packet path could not hand out
// Related: reactor_kernel.go -- kernelSetupsLocked keeps kernelSetupNeeded when the socket lookup fails
//go:build linux

package l2tp

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTickEnqueuesRecoveredKernelSetup proves that a kernel setup the packet
// path could not hand out reaches the kernel worker through the tick that
// retries it, once and only once.
//
// Method: the listener is not started, so when a ZLB for the tunnel runs
// through the real handle path its collection cannot read the listener's
// socket and leaves kernelSetupNeeded set, as kernelSetupsLocked documents.
// The listener then starts and the real handleTick runs for the tunnel. The
// worker is never started, so its queue holds exactly what the reactor
// enqueued and the test reads it with no timing. A second tick proves the
// retry introduces no duplicate setup.
// MUTATION: discarding the setups handleTick collects (the code before this
// test) leaves the worker's queue empty after the tick.
func TestTickEnqueuesRecoveredKernelSetup(t *testing.T) {
	logger := discardLoggerForTest()
	ln := newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), logger)
	r := newL2TPReactor(ln, logger, reactorParams{
		Defaults: TunnelDefaults{HostName: "ze-test", FramingCapabilities: 0x3, RecvWindow: 16},
	})
	worker := newKernelWorker((&fakeKernelOps{}).ops(), make(chan kernelSetupFailed, 4),
		make(chan kernelSetupSucceeded, 4), logger)
	r.kernelWorker = worker

	peer := netip.MustParseAddrPort("10.0.0.7:1701")
	tun := mkTunnel(r, 100, 200, peer)
	sess := addEstablishedSession(t, tun, 1001, 2001, true)
	r.tunnelsMu.Lock()
	sess.kernelSetupNeeded = true
	r.tunnelsMu.Unlock()

	// The packet path: the socket lookup fails, so nothing is enqueued and
	// the need stays for a later dispatch.
	r.handle(rxPacket{from: peer, bytes: controlDatagram(100, 0, 0, 0, nil), release: func() {}})
	r.tunnelsMu.Lock()
	needed := sess.kernelSetupNeeded
	r.tunnelsMu.Unlock()
	require.True(t, needed, "a failed socket lookup must leave the setup owed")
	require.Empty(t, worker.eventCh, "the packet path had no socket, so it must enqueue nothing")

	require.NoError(t, ln.Start(t.Context()))
	t.Cleanup(func() { require.NoError(t, ln.Stop()) })
	socketFD, err := ln.SocketFD()
	require.NoError(t, err)

	r.handleTick(tickReq{tunnelID: 100})

	require.Len(t, worker.eventCh, 1, "the tick that collected the owed setup must enqueue it")
	setup, ok := (<-worker.eventCh).(kernelSetupEvent)
	require.True(t, ok, "the enqueued event must be the session's kernel setup")
	require.Same(t, sess, setup.session)
	require.Equal(t, uint16(100), setup.localTID)
	require.Equal(t, uint16(1001), setup.localSID)
	require.Equal(t, socketFD, setup.socketFD)
	r.tunnelsMu.Lock()
	needed = sess.kernelSetupNeeded
	r.tunnelsMu.Unlock()
	require.False(t, needed, "the enqueued setup is no longer owed")

	r.handleTick(tickReq{tunnelID: 100})
	require.Empty(t, worker.eventCh, "a setup already enqueued must not be enqueued again")
}
