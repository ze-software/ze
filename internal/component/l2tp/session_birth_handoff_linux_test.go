// Design: docs/architecture/l2tp/subscriber-session-model.md -- actual L2TP admission to PPP handoff
//go:build linux

package l2tp

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/callsink"
)

// Only native operations are substituted. ICRQ mints the logical birth, ICCN
// establishes the row, the worker emits its original descriptor receipt and
// handleKernelSuccess hands that receipt to a genuine ProductionDriver.
// MUTATION: rebuilding SubscriberID from TID/SID in handleKernelSuccess makes
// the actual admitted control disagree with the birth minted by ICRQ.
func TestL2TPKernelHandoffUsesOriginalAdmittedBirth(t *testing.T) {
	reactor := newReactorForSnapshot(t)
	reactor.params.LCP = testLCPConfig()
	reactor.params.CQMEchoInterval = time.Hour
	reactor.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), reactor.logger)
	require.NoError(t, reactor.listener.Start(t.Context()))
	t.Cleanup(func() { require.NoError(t, reactor.listener.Stop()) })
	insertEstablishedTunnel(t, reactor, 100, 200, netip.MustParseAddrPort("127.0.0.1:1701"))
	tunnel := reactor.tunnelsByLocalID[100]
	var packet [BufSize]byte
	length := writeICRQBody(packet[:], 70, 70, 0, "", "")
	require.NotEmpty(t, tunnel.handleICRQ(packet[:length], reactor.params.Clock(), reactor.logger))
	require.Len(t, tunnel.sessions, 1)
	var original *L2TPSession
	for _, session := range tunnel.sessions {
		original = session
	}
	require.NotNil(t, original)
	birth := original.Birth()
	require.True(t, birth.IsValid(), "the real ICRQ producer must mint the birth before publication")
	options := []byte{1, 4, 5, 220, 5, 6, 0xca, 0xfe, 0xba, 0xbe}
	proxy := &iccnProxy{lcp: callsink.ProxyLCP{InitialReceived: options, LastSent: options, LastReceived: options}}
	length = writeICCNBody(packet[:], 64000, 1, proxy)
	tunnel.handleICCN(original, packet[:length], reactor.params.Clock(), reactor.logger)
	require.Equal(t, L2TPSessionEstablished, original.state)
	require.True(t, original.kernelSetupNeeded)
	fake := &fakeKernelOps{}
	succeeded := make(chan kernelSetupSucceeded, 4)
	worker := newKernelWorker(fake.ops(), make(chan kernelSetupFailed, 4), succeeded, reactor.logger)
	t.Cleanup(worker.teardownAll)
	reactor.kernelWorker = worker
	reactor.tunnelsMu.Lock()
	setups := reactor.kernelSetupsLocked(tunnel)
	reactor.tunnelsMu.Unlock()
	require.Len(t, setups, 1)
	require.Same(t, original, setups[0].session)
	worker.setupSession(setups[0])
	var success kernelSetupSucceeded
	select {
	case success = <-succeeded:
	case <-time.After(time.Second):
		t.Fatal("real worker did not return its successful original receipt")
	}
	worker.ops.pppConfig = logicalPPPConfigForTest(success.owner)
	driver := ppp.NewProductionDriver(reactor.logger)
	require.NoError(t, driver.Start())
	native, peer := net.Pipe()
	owner := &currentCoreHandle{Conn: native, identity: ppp.SessionIdentity{InterfaceName: "ppp0", NativeIndex: 1}}
	t.Cleanup(func() {
		require.NoError(t, peer.Close())
		driver.Stop()
	})
	reactor.pppDriver = driver
	reactor.newPPPSession = func(pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
		return owner, owner, nil
	}
	reactor.handleKernelSuccess(success)
	var opened ppp.EventLCPUp
	select {
	case event := <-driver.EventsOut():
		var ok bool
		opened, ok = event.(ppp.EventLCPUp)
		require.True(t, ok, "actual Driver first event = %T, want LCP-up", event)
	case <-time.After(time.Second):
		t.Fatal("actual Driver did not admit the real worker handoff")
	}
	require.NotEqual(t, ppp.SessionControl{}, opened.Control)
	require.Equal(t, original.pppAttempt, opened.Control.Attempt())
	require.Equal(t, birth.String(), opened.Control.SubscriberID(), "the real handoff must retain the admitted logical birth instead of rebuilding wire labels")
	require.Equal(t, birth, opened.Birth)
	require.Equal(t, birth.String(), opened.SubscriberID)
}
