// Design: docs/research/l2tpv2-ze-integration.md -- reactor -> PPP driver dispatch
// Related: reactor.go -- handleKernelSuccess, handlePPPEvent, SetPPPDriver
// Related: reactor_kernel_test.go -- collectKernelEventsLocked coverage

package l2tp

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

// discardLoggerForTest returns a logger that drops every record.
// Local helper to avoid dragging a production slogutil dependency
// into reactor tests.
func discardLoggerForTest() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// VALIDATES: clampReauthInterval applies the 5 s safety floor, ignores
//
//	non-positive values, and logs a WARN when falling back or
//	clamping. An empty value yields 0 (disabled); a malformed
//	duration yields 0 (disabled); a 1 ms value clamps to 5 s;
//	a 10 s value passes through verbatim.
//
// PREVENTS: regression where a 1 us operator typo in
//
//	ze.l2tp.auth.reauth-interval would launch a reauth
//	storm (ISSUE 2 from /ze-review 2026-04-17).
func TestClampReauthInterval(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"empty disables", "", 0},
		{"zero disables", "0s", 0},
		{"negative disables", "-1s", 0},
		{"malformed disables", "not-a-duration", 0},
		{"one microsecond clamps to floor", "1us", reauthIntervalFloor},
		{"one millisecond clamps to floor", "1ms", reauthIntervalFloor},
		{"floor passes through verbatim", reauthIntervalFloor.String(), reauthIntervalFloor},
		{"ten seconds passes through", "10s", 10 * time.Second},
		{"one hour passes through", "1h", time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := clampReauthInterval(discardLoggerForTest(), tc.raw)
			if got != tc.want {
				t.Errorf("clampReauthInterval(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// openPeerSocket binds a UDP socket on loopback ephemeral port. The
// returned addr is the peerAddr to plug into the tunnel so the reactor's
// listener.Send actually delivers to this socket; the returned conn is
// drained by the test. Cleanup closes the socket.
func openPeerSocket(t *testing.T) (*net.UDPConn, netip.AddrPort) {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	laddr, ok := c.LocalAddr().(*net.UDPAddr)
	require.True(t, ok)
	return c, netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(laddr.Port))
}

// fakePPPDriver records StartSession dispatches and lets tests inject
// ppp.Event values that the reactor consumes via its run loop.
type fakePPPDriver struct {
	sessionsIn chan ppp.StartSession
	eventsOut  chan ppp.Event
}

func newFakePPPDriver() *fakePPPDriver {
	return &fakePPPDriver{
		sessionsIn: make(chan ppp.StartSession, 4),
		eventsOut:  make(chan ppp.Event, 4),
	}
}

func (f *fakePPPDriver) SessionsIn() chan<- ppp.StartSession { return f.sessionsIn }
func (f *fakePPPDriver) EventsOut() <-chan ppp.Event         { return f.eventsOut }

func TestL2TPReactorDispatchesToPPPDriver(t *testing.T) {
	// VALIDATES: AC-2 -- reactor receives kernelSetupSucceeded and writes
	// a ppp.StartSession onto the driver's SessionsIn channel, carrying
	// the fds, IDs, lnsMode, and proxy LCP bytes verbatim.
	// PREVENTS: silently-dropped success events that leave PPP unaware
	// of a newly established kernel session.
	_, r, stop := newUnstartedReactor(t)
	defer stop()

	fake := newFakePPPDriver()
	r.SetPPPDriver(fake)

	peer := netip.MustParseAddrPort("10.0.0.7:1701")
	mkTunnel(r, 100, 200, peer)

	r.handleKernelSuccess(kernelSetupSucceeded{
		localTID:                   100,
		localSID:                   1001,
		lnsMode:                    true,
		sequencing:                 false,
		fds:                        pppSessionFDs{pppoxFD: 30, chanFD: 31, unitFD: 32, unitNum: 7},
		proxyInitialRecvLCPConfReq: []byte{0x01, 0x02},
		proxyLastSentLCPConfReq:    []byte{0x03},
		proxyLastRecvLCPConfReq:    []byte{0x04},
	})

	select {
	case start := <-fake.sessionsIn:
		require.Equal(t, uint16(100), start.TunnelID)
		require.Equal(t, uint16(1001), start.SessionID)
		require.Equal(t, 31, start.ChanFD)
		require.Equal(t, 32, start.UnitFD)
		require.Equal(t, 7, start.UnitNum)
		require.True(t, start.LNSMode)
		require.Equal(t, peer, start.PeerAddr)
		require.Equal(t, []byte{0x01, 0x02}, start.ProxyLCPInitialRecv)
		require.Equal(t, []byte{0x03}, start.ProxyLCPLastSent)
		require.Equal(t, []byte{0x04}, start.ProxyLCPLastRecv)
		require.Equal(t, 30*time.Second, start.AuthTimeout,
			"default ze.l2tp.auth.timeout (30s) should flow into StartSession.AuthTimeout")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ppp.StartSession dispatch")
	}
}

func TestL2TPReactorAuthTimeoutFromEnv(t *testing.T) {
	// VALIDATES: spec-l2tp-6b-auth Phase 3 -- ze.l2tp.auth.timeout env var
	// overrides the default 30s and is plumbed onto every new StartSession.
	// PREVENTS: operator setting auth-timeout in env and seeing no effect
	// until spec-l2tp-7-subsystem wires the YANG leaf.
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	t.Setenv("ze.l2tp.auth.timeout", "45s")
	env.ResetCache()

	_, r, stop := newUnstartedReactor(t)
	defer stop()

	fake := newFakePPPDriver()
	r.SetPPPDriver(fake)

	mkTunnel(r, 100, 200, netip.MustParseAddrPort("10.0.0.7:1701"))

	r.handleKernelSuccess(kernelSetupSucceeded{
		localTID: 100,
		localSID: 1001,
		lnsMode:  true,
		fds:      pppSessionFDs{pppoxFD: 30, chanFD: 31, unitFD: 32, unitNum: 7},
	})

	select {
	case start := <-fake.sessionsIn:
		require.Equal(t, 45*time.Second, start.AuthTimeout)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ppp.StartSession dispatch")
	}
}

func TestL2TPReactorAuthTimeoutInvalidEnvFallsBack(t *testing.T) {
	// VALIDATES: ze.l2tp.auth.timeout set to a value time.ParseDuration
	// cannot decode (operator typo, wrong units) falls back to 30s and does
	// not crash.
	// PREVENTS: a bad env value propagating as 0 or a partial parse into
	// ppp.StartSession.AuthTimeout, which would either disable the fail-
	// closed guard or time sessions out immediately.
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	t.Setenv("ze.l2tp.auth.timeout", "not-a-duration")
	env.ResetCache()

	_, r, stop := newUnstartedReactor(t)
	defer stop()

	fake := newFakePPPDriver()
	r.SetPPPDriver(fake)

	mkTunnel(r, 100, 200, netip.MustParseAddrPort("10.0.0.7:1701"))

	r.handleKernelSuccess(kernelSetupSucceeded{
		localTID: 100,
		localSID: 1001,
		lnsMode:  true,
		fds:      pppSessionFDs{pppoxFD: 30, chanFD: 31, unitFD: 32, unitNum: 7},
	})

	select {
	case start := <-fake.sessionsIn:
		require.Equal(t, 30*time.Second, start.AuthTimeout,
			"invalid env value must fall back to the 30s default")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ppp.StartSession dispatch")
	}
}

func TestL2TPReactorAuthTimeoutZeroPropagates(t *testing.T) {
	// VALIDATES: ze.l2tp.auth.timeout=0s is NOT interpreted by the reactor
	// as "use default". The reactor forwards 0 onto StartSession.AuthTimeout
	// verbatim; it is ppp that documents zero as "use package default (30s)"
	// in start_session.go. Pins the boundary so neither side accidentally
	// starts (or stops) doing the zero translation.
	// PREVENTS: double translation where reactor reads 0s, substitutes 30s,
	// then ppp sees 30s and cannot distinguish default from explicit.
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	t.Setenv("ze.l2tp.auth.timeout", "0s")
	env.ResetCache()

	_, r, stop := newUnstartedReactor(t)
	defer stop()

	fake := newFakePPPDriver()
	r.SetPPPDriver(fake)

	mkTunnel(r, 100, 200, netip.MustParseAddrPort("10.0.0.7:1701"))

	r.handleKernelSuccess(kernelSetupSucceeded{
		localTID: 100,
		localSID: 1001,
		lnsMode:  true,
		fds:      pppSessionFDs{pppoxFD: 30, chanFD: 31, unitFD: 32, unitNum: 7},
	})

	select {
	case start := <-fake.sessionsIn:
		require.Equal(t, time.Duration(0), start.AuthTimeout,
			"0s env must propagate as 0; ppp (not reactor) owns the zero-as-default contract")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ppp.StartSession dispatch")
	}
}

func TestL2TPReactorWithoutPPPDriverLogsAndDrops(t *testing.T) {
	// VALIDATES: when no PPP driver has been wired (non-Linux, test
	// paths, or iface backend absent), handleKernelSuccess does not
	// panic; the event is logged and dropped.
	// PREVENTS: nil-deref crash when kernel integration runs ahead of
	// iface backend availability.
	_, r, stop := newUnstartedReactor(t)
	defer stop()

	// r.pppDriver is nil by construction.
	r.handleKernelSuccess(kernelSetupSucceeded{
		localTID: 100,
		localSID: 1001,
		fds:      pppSessionFDs{pppoxFD: 30, chanFD: 31, unitFD: 32, unitNum: 7},
	})
}

func TestL2TPReactorPPPEventSessionDownSendsCDN(t *testing.T) {
	// VALIDATES: when the PPP driver emits EventSessionDown for an
	// established L2TP session, the reactor (a) removes the session
	// entry and (b) emits a CDN on the wire to the peer. Both assertions
	// are required -- "session removed" alone could pass on a broken
	// implementation that forgot to call listener.Send.
	// PREVENTS: sessions stuck at L2TPSessionEstablished in ze's view
	// while PPP has already torn them down; also silent regressions that
	// drop the CDN while still cleaning up local state.
	_, r, stop := newUnstartedReactor(t)
	defer stop()

	peerConn, peerAddr := openPeerSocket(t)
	tun := mkTunnel(r, 100, 200, peerAddr)
	sess := addEstablishedSession(tun, 1001, 2001, true)

	r.handlePPPEvent(ppp.EventSessionDown{
		TunnelID:  100,
		SessionID: 1001,
		Reason:    "test: peer Terminate-Request",
	})

	r.tunnelsMu.Lock()
	_, stillThere := tun.sessions[sess.localSID]
	r.tunnelsMu.Unlock()
	require.False(t, stillThere, "session must be removed after PPP SessionDown")

	// Read the CDN the reactor should have sent to peerAddr and verify
	// its header parses as an L2TP control packet whose first AVP is
	// Message-Type = CDN.
	require.NoError(t, peerConn.SetReadDeadline(time.Now().Add(2*time.Second)))
	buf := make([]byte, 4096)
	n, _, err := peerConn.ReadFromUDP(buf)
	require.NoError(t, err, "timed out waiting for CDN on peer socket")
	hdr, err := ParseMessageHeader(buf[:n])
	require.NoError(t, err)
	require.True(t, hdr.IsControl, "control bit must be set on CDN")
	body := buf[hdr.PayloadOff:int(hdr.Length)]
	it := NewAVPIterator(body)
	vendorID, attrType, _, value, ok := it.Next()
	require.True(t, ok, "payload must contain at least one AVP")
	require.NoError(t, it.Err())
	require.Equal(t, uint16(0), vendorID, "Message-Type AVP is vendor 0")
	require.Equal(t, AVPMessageType, attrType, "first AVP must be Message-Type (RFC 2661 S4.1)")
	mt, rerr := ReadAVPUint16(value)
	require.NoError(t, rerr)
	require.Equal(t, MsgCDN, MessageType(mt), "peer should receive a CDN message")
}

func TestL2TPReactorPPPEventInformationalIgnored(t *testing.T) {
	// VALIDATES: EventLCPUp / EventLCPDown / EventSessionUp do not tear
	// the session down; they are informational in 6a.
	// PREVENTS: LCP-reached-Opened being interpreted as a teardown
	// signal, which would immediately send a CDN after every session
	// came up.
	_, r, stop := newUnstartedReactor(t)
	defer stop()

	peer := netip.MustParseAddrPort("192.0.2.1:1701")
	tun := mkTunnel(r, 100, 200, peer)
	sess := addEstablishedSession(tun, 1001, 2001, true)

	r.handlePPPEvent(ppp.EventLCPUp{TunnelID: 100, SessionID: 1001, NegotiatedMRU: 1460})
	r.handlePPPEvent(ppp.EventSessionUp{TunnelID: 100, SessionID: 1001})
	r.handlePPPEvent(ppp.EventLCPDown{TunnelID: 100, SessionID: 1001, Reason: "echo-timeout"})

	r.tunnelsMu.Lock()
	_, stillThere := tun.sessions[sess.localSID]
	r.tunnelsMu.Unlock()
	// LCPDown is informational per spec wording; actual teardown happens
	// only on SessionDown / SessionRejected. If this changes in 6b, the
	// test flips with the code.
	require.True(t, stillThere, "informational events must NOT remove the session")
}

// newUnstartedReactorWithLogs is like newUnstartedReactor but returns
// the lockedBuffer so callers can assert on log output.
func newUnstartedReactorWithLogs(t *testing.T) (*UDPListener, *L2TPReactor, *lockedBuffer, func()) {
	t.Helper()
	buf := &lockedBuffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ln := NewUDPListener(netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 0), logger)
	require.NoError(t, ln.Start(context.Background()))
	r := NewL2TPReactor(ln, logger, ReactorParams{
		Defaults: TunnelDefaults{HostName: "ze-test", FramingCapabilities: 0x3, RecvWindow: 16},
	})
	stop := func() {
		_ = ln.Stop()
	}
	return ln, r, buf, stop
}

func TestL2TPReactorSessionIPAssignedLogsValidIPv4(t *testing.T) {
	// VALIDATES: handleSessionIPAssigned emits an Info log containing
	// the tunnel-id, session-id, username, and address when the event
	// carries a valid IPv4 peer address.
	// PREVENTS: silent session IP assignment with no operator-visible
	// evidence in the log stream.
	_, r, logs, stop := newUnstartedReactorWithLogs(t)
	defer stop()

	tun := mkTunnel(r, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"))
	sess := addEstablishedSession(tun, 1001, 2001, true)
	sess.username = "alice"

	r.handleSessionIPAssigned(ppp.EventSessionIPAssigned{
		TunnelID:  100,
		SessionID: 1001,
		Family:    ppp.AddressFamilyIPv4,
		Peer:      netip.MustParseAddr("10.100.0.2"),
	})

	got := logs.String()
	require.Contains(t, got, "l2tp: session IP assigned")
	require.Contains(t, got, "10.100.0.2")
	require.Contains(t, got, "alice")
}

func TestL2TPReactorSessionIPAssignedNoLogOnInvalidAddr(t *testing.T) {
	// VALIDATES: handleSessionIPAssigned does NOT emit the Info log
	// when neither Peer nor Local+InterfaceID resolve to a valid addr.
	// PREVENTS: spurious "session IP assigned" log noise on events
	// where no NCP address was actually negotiated.
	_, r, logs, stop := newUnstartedReactorWithLogs(t)
	defer stop()

	tun := mkTunnel(r, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"))
	addEstablishedSession(tun, 1001, 2001, true)

	r.handleSessionIPAssigned(ppp.EventSessionIPAssigned{
		TunnelID:  100,
		SessionID: 1001,
	})

	require.False(t, strings.Contains(logs.String(), "l2tp: session IP assigned"),
		"no log expected when addr is invalid")
}

func TestL2TPReactorSessionUpLogsWithNilEventBus(t *testing.T) {
	// VALIDATES: handleSessionUp emits the Info log even when the
	// reactor has no eventBus wired (nil). Before the fix, the method
	// returned early on nil eventBus, producing no log.
	// PREVENTS: regression to the old early-return that silenced the
	// PPP session-up log in standalone (no event-bus subscriber)
	// deployments.
	_, r, logs, stop := newUnstartedReactorWithLogs(t)
	defer stop()

	require.Nil(t, r.eventBus, "precondition: eventBus must be nil for this test")

	tun := mkTunnel(r, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"))
	sess := addEstablishedSession(tun, 1001, 2001, true)
	sess.pppInterface = "ppp0"

	r.handleSessionUp(ppp.EventSessionUp{TunnelID: 100, SessionID: 1001})

	got := logs.String()
	require.Contains(t, got, "l2tp: PPP session up")
	require.Contains(t, got, "ppp0")
}

func TestL2TPReactorSessionUpNoLogOnEmptyInterface(t *testing.T) {
	// VALIDATES: handleSessionUp returns early (no log, no event)
	// when the session has no pppInterface name set. This happens if
	// the kernel session setup didn't populate the interface name.
	// PREVENTS: empty-interface log spam or panics on zero-value
	// session fields.
	_, r, logs, stop := newUnstartedReactorWithLogs(t)
	defer stop()

	tun := mkTunnel(r, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"))
	addEstablishedSession(tun, 1001, 2001, true)
	// pppInterface left as zero value ""

	r.handleSessionUp(ppp.EventSessionUp{TunnelID: 100, SessionID: 1001})

	require.False(t, strings.Contains(logs.String(), "l2tp: PPP session up"),
		"no log expected when pppInterface is empty")
}
