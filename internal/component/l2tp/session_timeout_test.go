// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- session/idle timeout tests

package l2tp

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCancelSessionTimeoutsNilSafe(t *testing.T) {
	sess := &L2TPSession{}
	cancelSessionTimeouts(sess)
}

func TestCancelSessionTimeoutsCancelsContext(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	sess := &L2TPSession{
		sessionTimeoutCancel: cancel1,
		idleTimeoutCancel:    cancel2,
	}
	cancelSessionTimeouts(sess)
	require.Error(t, ctx1.Err(), "session timeout context should be canceled")
	require.Error(t, ctx2.Err(), "idle timeout context should be canceled")
	require.Nil(t, sess.sessionTimeoutCancel, "cancel func should be cleared")
	require.Nil(t, sess.idleTimeoutCancel, "cancel func should be cleared")
}

func TestStartSessionTimeoutsNoMetadata(t *testing.T) {
	r := newTestReactor(t)
	tunnel := &L2TPTunnel{localTID: 1, sessions: make(map[uint16]*L2TPSession)}
	sess := &L2TPSession{localSID: 1, timeoutLifetime: &sessionTimeoutLifetime{}}
	tunnel.sessions[1] = sess
	r.tunnelsByLocalID[1] = tunnel
	r.startSessionTimeouts(tunnel, sess, sess.timeoutLifetime)
}

func TestRunSessionTimeoutCanceled(t *testing.T) {
	r := newTestReactor(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.runSessionTimeout(&L2TPTunnel{localTID: 1}, &L2TPSession{localSID: 1}, time.Hour, ctx)
}

func TestRunIdleTimeoutCanceled(t *testing.T) {
	r := newTestReactor(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.runIdleTimeout(&L2TPTunnel{localTID: 1}, &L2TPSession{localSID: 1}, time.Hour, "ppp0", ctx)
}

func newTestReactor(t *testing.T) *l2tpReactor {
	t.Helper()
	addr := netip.MustParseAddrPort("127.0.0.1:0")
	listener := newUDPListener(addr, nil)
	t.Cleanup(func() { _ = listener.Stop() })
	return newL2TPReactor(listener, nil, reactorParams{})
}
