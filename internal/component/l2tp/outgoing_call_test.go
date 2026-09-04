package l2tp

import (
	"log/slog"
	"net/netip"
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
