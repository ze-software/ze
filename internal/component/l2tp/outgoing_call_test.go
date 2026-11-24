package l2tp

import (
	"context"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// TestSubsystem_PlaceOutgoingCall_PreflightErrors -- AC-4 remote resolution.
//
// VALIDATES: the outgoing-call Service method rejects an unknown remote, a
// remote that does not permit outgoing calls, and the case where no listener
// (reactor) is available -- all before dialing.
func TestSubsystem_PlaceOutgoingCall_PreflightErrors(t *testing.T) {
	permitted := Remote{Name: "lns1", Address: netip.MustParseAddrPort("10.0.0.1:1701"), OutgoingCalls: true}
	denied := Remote{Name: "lns2", Address: netip.MustParseAddrPort("10.0.0.2:1701"), OutgoingCalls: false}
	s := &Subsystem{params: Parameters{Remotes: []Remote{permitted, denied}}, logger: slog.Default()}

	_, err := s.PlaceOutgoingCall("ghost", "555")
	require.ErrorIs(t, err, errNoOutgoingRemote)

	_, err = s.PlaceOutgoingCall("lns2", "555")
	require.ErrorIs(t, err, errRemoteNoOutgoing)

	// Permitted remote but no reactor wired -> no listener error.
	_, err = s.PlaceOutgoingCall("lns1", "555")
	require.ErrorIs(t, err, errNoReactorForOutgoing)
}

// TestOutgoingCall_SignalsSuccessOnOCCN -- AC-4 result surfacing.
//
// VALIDATES: a callResult channel installed on an LNS-outgoing session (as
// placeOutgoingCallSync does) receives a success outcome carrying local and
// remote SIDs the moment OCCN establishes the call, and is cleared after.
func TestOutgoingCall_SignalsSuccessOnOCCN(t *testing.T) {
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	localSID, _ := tun.placeOutgoingCall(now, callParams{callSerial: 7, framingType: 1, calledNumber: "5559999"}, logger)
	sess := tun.lookupSession(localSID)
	require.NotNil(t, sess)
	ch := make(chan callOutcome, 1)
	sess.callResult = ch

	tun.handleOCRP(sess, ocrpBody(808), now, logger)
	require.Equal(t, L2TPSessionWaitConnect, sess.state)
	require.Empty(t, ch, "no outcome until OCCN establishes the call")

	tun.handleOCCN(sess, buildOCCN(64000, 1), now, logger)
	require.Equal(t, L2TPSessionEstablished, sess.state)
	select {
	case o := <-ch:
		require.NoError(t, o.err)
		require.Equal(t, localSID, o.localSID)
		require.EqualValues(t, 808, o.remoteSID)
	default:
		t.Fatal("expected a success outcome signaled on OCCN")
	}
	require.Nil(t, sess.callResult, "callResult cleared after resolve")
}

// TestIncomingCall_SignalsSuccessOnICRP -- AC-3/AC-4 result surfacing (LAC).
//
// VALIDATES: a LAC-incoming session's callResult receives success when ICRP
// establishes the call.
func TestIncomingCall_SignalsSuccessOnICRP(t *testing.T) {
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	localSID, _ := tun.placeIncomingCall(now, callParams{callSerial: 42, framingType: 1}, logger)
	sess := tun.lookupSession(localSID)
	require.NotNil(t, sess)
	ch := make(chan callOutcome, 1)
	sess.callResult = ch

	tun.handleICRP(sess, icrpBody(707), now, logger)
	require.Equal(t, L2TPSessionEstablished, sess.state)
	select {
	case o := <-ch:
		require.NoError(t, o.err)
		require.Equal(t, localSID, o.localSID)
		require.EqualValues(t, 707, o.remoteSID)
	default:
		t.Fatal("expected a success outcome signaled on ICRP")
	}
}

// TestOutgoingCall_SignalsFailureOnTeardown -- AC-4 failure surfacing.
//
// VALIDATES: tearing a placed-but-unconnected call down delivers a failure
// outcome carrying the CDN Result Code, so the blocking RPC reports why.
func TestOutgoingCall_SignalsFailureOnTeardown(t *testing.T) {
	tun := newEstablishedTunnel(t, 0)
	now := time.Now()
	logger := slog.Default()

	localSID, _ := tun.placeOutgoingCall(now, callParams{callSerial: 7, framingType: 1}, logger)
	sess := tun.lookupSession(localSID)
	require.NotNil(t, sess)
	ch := make(chan callOutcome, 1)
	sess.callResult = ch

	tun.teardownSession(sess, cdnResultGeneralError, l2tpevents.TerminateCauseNASError, now, logger)
	select {
	case o := <-ch:
		require.Error(t, o.err)
		require.EqualValues(t, cdnResultGeneralError, o.resultCode)
	default:
		t.Fatal("expected a failure outcome signaled on teardown")
	}
}

// TestResolveCall_NilChannelIsNoOp -- fire-and-forget safety.
//
// VALIDATES: a session with no callResult (peer-initiated or config-driven
// relay) tears down without panicking; resolveCall is a no-op.
func TestResolveCall_NilChannelIsNoOp(t *testing.T) {
	sess := &L2TPSession{localSID: 5}
	require.NotPanics(t, func() {
		sess.resolveCall(callOutcome{err: errCallTornDown})
	})
}

// TestSubsystem_DialFromLoopbackNamesServer proves the server key reaches the
// runtime refusal through Start, and that a reload which moves the listener
// turns the fix into a restart.
//
// VALIDATES: a Subsystem started with server "lab" on 127.0.0.1 refuses
// PlaceOutgoingCall to 192.0.2.1 with ErrDialFromLoopback naming
// `(environment l2tp server lab)` and the `set` command. After a reload that
// sets "lab" to 0.0.0.0, the same call is refused naming the configured
// 0.0.0.0:1701 and "Restart ze to rebind the listener", with no `set` line.
// METHOD: real Start over an ephemeral loopback port; the kernel module probe
// is neutralized, as in TestSubsystem_StartEnabledWithListener.
// MUTATION: dropping ServerName from Start's reactorParams turns the first
// half red; dropping the setConfiguredListen loop from Reload the second.
func TestSubsystem_DialFromLoopbackNamesServer(t *testing.T) {
	defer SetProbeKernelModulesForTest(func() error { return nil })()

	s := NewSubsystem(&Parameters{
		Enabled:       true,
		HelloInterval: 60 * time.Second,
		ListenAddrs:   []netip.AddrPort{netip.MustParseAddrPort("127.0.0.1:0")},
		ListenNames:   []string{"lab"},
		Remotes: []Remote{{
			Name: "lns1", Address: netip.MustParseAddrPort("192.0.2.1:1701"), OutgoingCalls: true,
		}},
	})
	defer UseListenerKernelBackendForTest(s)()
	ctx := context.Background()
	require.NoError(t, s.Start(ctx, nil, nil))
	defer func() { require.NoError(t, s.Stop(ctx)) }()

	_, err := s.PlaceOutgoingCall("lns1", "555")
	require.ErrorIs(t, err, ErrDialFromLoopback)
	require.Contains(t, err.Error(), "(environment l2tp server lab)")
	require.Contains(t, err.Error(), "remote 192.0.2.1:1701")
	require.True(t, strings.HasSuffix(err.Error(), "\n  set environment l2tp server lab ip 0.0.0.0"), err.Error())

	require.NoError(t, s.Reload(ctx, dialSourceProvider("0.0.0.0", "192.0.2.1")))
	_, err = s.PlaceOutgoingCall("lns1", "555")
	require.ErrorIs(t, err, ErrDialFromLoopback)
	require.Contains(t, err.Error(), "(environment l2tp server lab)")
	require.Contains(t, err.Error(), "already sets this listener to 0.0.0.0:1701")
	require.True(t, strings.HasSuffix(err.Error(), "Restart ze to rebind the listener"), err.Error())
	require.NotContains(t, err.Error(), "set environment")
}

// TestSubsystem_ReactorForRemote_LoopbackFirstPicksRoutableSecond proves that
// when multiple listeners are configured, dialing selects a listener that can
// reach the remote rather than blindly picking s.reactors[0].
//
// VALIDATES: AC-7 (dial-ignores-which-listener-reaches-the-remote): with a
// loopback listener first and a wildcard listener second, an off-host remote
// selects the wildcard reactor, a loopback remote prefers the loopback reactor
// regardless of slice order, a wildcard-only subsystem still reaches loopback,
// and an IPv6 remote against IPv4-only listeners returns ErrNoReachingListener
// naming the remote and available listeners.
// PREVENTS: dialing from s.reactors[0] when it cannot reach the remote while a
// later listener can.
func TestSubsystem_ReactorForRemote_LoopbackFirstPicksRoutableSecond(t *testing.T) {
	ctx := context.Background()
	loopLn := newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), slog.Default())
	require.NoError(t, loopLn.Start(ctx))
	t.Cleanup(func() { _ = loopLn.Stop() })

	wanLn := newUDPListener(netip.MustParseAddrPort("0.0.0.0:0"), slog.Default())
	require.NoError(t, wanLn.Start(ctx))
	t.Cleanup(func() { _ = wanLn.Stop() })

	loopReactor := newL2TPReactor(loopLn, slog.Default(), reactorParams{ServerName: "loop"})
	wanReactor := newL2TPReactor(wanLn, slog.Default(), reactorParams{ServerName: "wan"})

	s := &Subsystem{reactors: []*l2tpReactor{loopReactor, wanReactor}, logger: slog.Default()}

	offHost := netip.MustParseAddrPort("192.0.2.1:1701")
	got, err := s.reactorForRemoteLocked(offHost, errNoReactorForOutgoing)
	require.NoError(t, err)
	require.Same(t, wanReactor, got, "off-host remote must skip loopback reactor[0] and pick wan reactor[1]")

	loopRemote := netip.MustParseAddrPort("127.0.0.1:17010")
	got, err = s.reactorForRemoteLocked(loopRemote, errNoReactorForOutgoing)
	require.NoError(t, err)
	require.Same(t, loopReactor, got, "loopback remote prefers the loopback reactor")

	// Reversed order: loopback remote still prefers loopback over wildcard.
	sReversed := &Subsystem{reactors: []*l2tpReactor{wanReactor, loopReactor}, logger: slog.Default()}
	got, err = sReversed.reactorForRemoteLocked(loopRemote, errNoReactorForOutgoing)
	require.NoError(t, err)
	require.Same(t, loopReactor, got, "loopback remote prefers loopback even when wildcard comes first")

	got, err = sReversed.reactorForRemoteLocked(offHost, errNoReactorForOutgoing)
	require.NoError(t, err)
	require.Same(t, wanReactor, got, "off-host remote picks wildcard reactor")

	// Wildcard alone still reaches loopback when no loopback listener exists.
	sWanOnly := &Subsystem{reactors: []*l2tpReactor{wanReactor}, logger: slog.Default()}
	got, err = sWanOnly.reactorForRemoteLocked(loopRemote, errNoReactorForOutgoing)
	require.NoError(t, err)
	require.Same(t, wanReactor, got, "non-loopback listener reaches loopback when no loopback listener matches")

	// IPv6 remote against IPv4-only listeners fails clearly with available listeners.
	v6Remote := netip.MustParseAddrPort("[2001:db8::1]:1701")
	got, err = s.reactorForRemoteLocked(v6Remote, errNoReactorForOutgoing)
	require.Nil(t, got)
	require.ErrorIs(t, err, ErrNoReachingListener)
	require.Contains(t, err.Error(), v6Remote.String())
	require.Contains(t, err.Error(), loopLn.Addr().String())
	require.Contains(t, err.Error(), wanLn.Addr().String())
}

// TestSubsystem_PlaceOutgoingCall_IPv4AndIPv6SelectsMatchingReactor proves
// PlaceOutgoingCall dials from the matching address-family reactor when IPv4
// is listener 0 and IPv6 is listener 1.
//
// VALIDATES: AC-7 end-to-end across IPv4 and IPv6 listeners: an outgoing call
// to an IPv6 peer succeeds via the IPv6 reactor at index 1 rather than failing
// on the IPv4 reactor at index 0.
// PREVENTS: IPv6 outgoing calls failing when an IPv4 listener is declared first.
func TestSubsystem_PlaceOutgoingCall_IPv4AndIPv6SelectsMatchingReactor(t *testing.T) {
	ctx := context.Background()
	v4Ln := newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), slog.Default())
	require.NoError(t, v4Ln.Start(ctx))
	t.Cleanup(func() { _ = v4Ln.Stop() })

	v6Ln := newUDPListener(netip.MustParseAddrPort("[::1]:0"), slog.Default())
	require.NoError(t, v6Ln.Start(ctx))
	t.Cleanup(func() { _ = v6Ln.Stop() })

	v4Reactor := newL2TPReactor(v4Ln, slog.Default(), reactorParams{
		ServerName: "v4",
		Defaults:   TunnelDefaults{HostName: "ze-v4", FramingCapabilities: 0x3, RecvWindow: 16},
	})
	require.NoError(t, v4Reactor.Start())
	t.Cleanup(v4Reactor.Stop)

	v6Reactor := newL2TPReactor(v6Ln, slog.Default(), reactorParams{
		ServerName: "v6",
		Defaults:   TunnelDefaults{HostName: "ze-v6", FramingCapabilities: 0x3, RecvWindow: 16},
	})
	require.NoError(t, v6Reactor.Start())
	t.Cleanup(v6Reactor.Stop)

	v6Timer := newTunnelTimer(v6Reactor.tickCh, v6Reactor.updateCh)
	require.NoError(t, v6Timer.Start())
	t.Cleanup(v6Timer.Stop)

	peerConn, err := net.ListenUDP("udp6", &net.UDPAddr{IP: net.IPv6loopback, Port: 0})
	require.NoError(t, err)
	t.Cleanup(func() { _ = peerConn.Close() })
	udpAddr, ok := peerConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("peerConn.LocalAddr() has type %T, want *net.UDPAddr", peerConn.LocalAddr())
	}
	peerAddr := udpAddr.AddrPort()

	s := &Subsystem{
		params: Parameters{
			Remotes: []Remote{{
				Name:          "lns-v6",
				Address:       peerAddr,
				OutgoingCalls: true,
			}},
		},
		reactors: []*l2tpReactor{v4Reactor, v6Reactor},
		logger:   slog.Default(),
	}

	type callRes struct {
		res OutgoingCallResult
		err error
	}
	done := make(chan callRes, 1)
	go func() {
		res, callErr := s.PlaceOutgoingCall("lns-v6", "5551234")
		done <- callRes{res: res, err: callErr}
	}()

	buf := make([]byte, 2048)
	require.NoError(t, peerConn.SetReadDeadline(time.Now().Add(3*time.Second)))
	n, from, err := peerConn.ReadFromUDPAddrPort(buf)
	require.NoError(t, err)
	sccrq := append([]byte(nil), buf[:n]...)
	require.Equal(t, MsgSCCRQ, msgTypeOf(t, sccrq))
	require.Equal(t, v6Ln.Addr(), from, "SCCRQ must originate from the IPv6 listener, not IPv4 listener[0]")

	zeTIDBytes := extractAVP(t, sccrq, AVPAssignedTunnelID)
	zeTID := uint16(zeTIDBytes[0])<<8 | uint16(zeTIDBytes[1])
	_, err = peerConn.WriteToUDPAddrPort(buildSCCRPForSCCRQ(t, sccrq, 901), from)
	require.NoError(t, err)

	// Read until OCRQ (skipping SCCCN / ZLB ACK).
	var ocrq []byte
	for range 4 {
		require.NoError(t, peerConn.SetReadDeadline(time.Now().Add(3*time.Second)))
		n, _, err = peerConn.ReadFromUDPAddrPort(buf)
		require.NoError(t, err)
		pkt := append([]byte(nil), buf[:n]...)
		hdr, herr := ParseMessageHeader(pkt)
		require.NoError(t, herr)
		if hdr.PayloadOff < int(hdr.Length) && msgTypeOf(t, pkt) == MsgOCRQ {
			ocrq = pkt
			break
		}
	}
	require.NotEmpty(t, ocrq, "expected OCRQ over the IPv6 tunnel")
	zeSIDBytes := extractAVP(t, ocrq, AVPAssignedSessionID)
	zeSID := uint16(zeSIDBytes[0])<<8 | uint16(zeSIDBytes[1])

	sendCtrl := func(body []byte, ns, nr uint16) {
		pkt := make([]byte, ControlHeaderLen+len(body))
		WriteControlHeader(pkt, 0, uint16(len(pkt)), zeTID, zeSID, ns, nr)
		copy(pkt[ControlHeaderLen:], body)
		_, werr := peerConn.WriteToUDPAddrPort(pkt, from)
		require.NoError(t, werr)
	}
	sendCtrl(ocrpBody(808), 1, 3)
	sendCtrl(buildOCCN(64000, 1), 2, 3)

	select {
	case out := <-done:
		require.NoError(t, out.err)
		require.True(t, out.res.Established)
		require.Equal(t, zeSID, out.res.LocalSID)
		require.EqualValues(t, 808, out.res.RemoteSID)
	case <-time.After(3 * time.Second):
		t.Fatal("PlaceOutgoingCall timed out waiting for IPv6 call establishment")
	}

	v4Reactor.tunnelsMu.Lock()
	v4Tunnels := len(v4Reactor.tunnelsByLocalID)
	v4Reactor.tunnelsMu.Unlock()
	require.Zero(t, v4Tunnels, "IPv4 reactor[0] must remain unused")
}
