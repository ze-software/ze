package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
)

// VALIDATES: AC-21 -- Subsystem.Start registers the l2tp source.
func TestRegisterL2TPSourcesRegistersSource(t *testing.T) {
	// sync.Once means the registration may already have happened in
	// another test. Call it explicitly, then assert lookup.
	RegisterL2TPSources()

	src, ok := redistribute.LookupSource("l2tp")
	require.True(t, ok, "l2tp source must be registered")
	require.Equal(t, "l2tp", src.Name)
	require.Equal(t, "l2tp", src.Protocol)
}

// VALIDATES: AC-22 -- IPv4 session-up records the /32 address.
func TestRouteObserverInjectsIPv4(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default())

	o.OnSessionIPUp(42, "alice", netip.MustParseAddr("192.0.2.7"))

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(1), injected)
	require.Equal(t, uint64(0), withdrawn)
	require.Equal(t, 1, active)
}

// VALIDATES: AC-23 -- IPv6 session-up records the /128 address.
func TestRouteObserverInjectsIPv6(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default())

	o.OnSessionIPUp(43, "bob", netip.MustParseAddr("2001:db8::1"))

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(1), injected)
	require.Equal(t, uint64(0), withdrawn)
	require.Equal(t, 1, active)
}

// VALIDATES: dual-stack subscriber gets one record with both addresses.
func TestRouteObserverTracksBothFamiliesPerSession(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default())

	o.OnSessionIPUp(44, "carol", netip.MustParseAddr("192.0.2.7"))
	o.OnSessionIPUp(44, "carol", netip.MustParseAddr("2001:db8::7"))

	injected, _, active := o.Stats()
	require.Equal(t, uint64(2), injected)
	require.Equal(t, 1, active, "one session, two families, still one record")
}

// VALIDATES: AC-24 -- session-down withdraws every family for that SID.
func TestRouteObserverWithdrawsOnDown(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default())

	o.OnSessionIPUp(45, "dave", netip.MustParseAddr("192.0.2.8"))
	o.OnSessionIPUp(45, "dave", netip.MustParseAddr("2001:db8::8"))
	o.OnSessionDown(45)

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(2), injected)
	require.Equal(t, uint64(2), withdrawn, "dual-stack subscriber withdraws twice")
	require.Equal(t, 0, active)
}

// VALIDATES: OnSessionDown for an unknown SID is a no-op, not an error.
func TestRouteObserverSessionDownUnknownID(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default())

	o.OnSessionDown(9999) // never reported up

	injected, withdrawn, active := o.Stats()
	require.Equal(t, uint64(0), injected)
	require.Equal(t, uint64(0), withdrawn)
	require.Equal(t, 0, active)
}

// VALIDATES: OnSessionIPUp with an invalid address is a no-op.
func TestRouteObserverSkipsInvalidAddr(t *testing.T) {
	o := newSubscriberRouteObserver(slog.Default())

	o.OnSessionIPUp(46, "eve", netip.Addr{})

	injected, _, active := o.Stats()
	require.Equal(t, uint64(0), injected)
	require.Equal(t, 0, active)
}

// VALIDATES: the RouteObserver interface is satisfied by
// subscriberRouteObserver. Compile-time proof.
func TestSubscriberRouteObserverSatisfiesInterface(t *testing.T) {
	var _ RouteObserver = (*subscriberRouteObserver)(nil)
}
