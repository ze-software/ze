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
	r.startSessionTimeouts(1, 1)
}

func TestRunSessionTimeoutCanceled(t *testing.T) {
	r := newTestReactor(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.runSessionTimeout(1, 1, time.Hour, ctx)
}

func TestRunIdleTimeoutCanceled(t *testing.T) {
	r := newTestReactor(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.runIdleTimeout(1, 1, time.Hour, "ppp0", ctx)
}

func newTestReactor(t *testing.T) *L2TPReactor {
	t.Helper()
	addr := netip.MustParseAddrPort("127.0.0.1:0")
	listener := NewUDPListener(addr, nil)
	t.Cleanup(func() { _ = listener.Stop() })
	return NewL2TPReactor(listener, nil, ReactorParams{})
}
