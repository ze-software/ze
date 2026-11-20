// Design: docs/architecture/l2tp/subscriber-session-model.md -- L2TP subscriber event bridge
//
// Goal: prove the L2TP subscriber bridge keeps the registry entry equal to
// the open families: a release clears that family's fields and is
// re-published on the subscriber namespace, and the next assignment sets the
// fields again. Method: publish the L2TP notices on a bus the bridge reads,
// then read the registry and the subscriber-namespace releases.

package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
)

// TestSubscriberBridgeFamilyReleaseClearsAddress (AC-4, AC-8 on L2TP).
// Fails before the release subscription: the bridge never cleared a family.
func TestSubscriberBridgeFamilyReleaseClearsAddress(t *testing.T) {
	bus := newTestBus()
	reg := subscriber.NewRegistry()
	bridge := newSubscriberBridge(reg, bus, slog.Default())
	defer bridge.stop()

	birth := newFixtureBirth(t)
	id := birth.String()
	identifier := [8]byte{0x02, 0, 0, 0, 0, 0, 0, 0x90}
	ipv4 := &l2tpevents.SessionIPAssignedPayload{
		Birth: birth, SubscriberID: id, TunnelID: 1, SessionID: 90, PeerAddr: "192.0.2.90",
		DNSPrimary: netip.MustParseAddr("192.0.2.53"), DNSSecondary: netip.MustParseAddr("192.0.2.54"),
	}
	ipv6 := &l2tpevents.SessionIPAssignedPayload{
		Birth: birth, SubscriberID: id, TunnelID: 1, SessionID: 90, InterfaceID: identifier,
	}
	var released []*subevents.SessionIPReleasedPayload
	subevents.SessionIPReleased.Subscribe(bus, func(p *subevents.SessionIPReleasedPayload) { released = append(released, p) })
	release := func(family l2tpevents.AddressFamily) {
		_, err := l2tpevents.SessionIPReleased.Emit(bus, &l2tpevents.SessionIPReleasedPayload{
			Birth: birth, SubscriberID: id, TunnelID: 1, SessionID: 90, Family: family,
		})
		require.NoError(t, err)
	}
	entry := func() subscriber.Session {
		sess, ok := reg.Get(id)
		require.True(t, ok, "a family release keeps the registry entry")
		return sess
	}

	_, err := l2tpevents.SessionIPAssigned.Emit(bus, ipv4)
	require.NoError(t, err)
	_, err = l2tpevents.SessionIPAssigned.Emit(bus, ipv6)
	require.NoError(t, err)

	release(l2tpevents.AddressFamilyIPv4)
	sess := entry()
	require.False(t, sess.IPv4Addr.IsValid())
	require.False(t, sess.DNSPrimary.IsValid())
	require.False(t, sess.DNSSecondary.IsValid())
	require.Equal(t, identifier, sess.IPv6InterfaceID, "IPv4 release keeps the IPv6 family")
	require.Len(t, released, 1)
	require.Equal(t, l2tpevents.AddressFamilyIPv4, released[0].Family)
	require.Equal(t, id, released[0].Session.ID)
	require.False(t, released[0].Session.IPv4Addr.IsValid())

	_, err = l2tpevents.SessionIPAssigned.Emit(bus, ipv4)
	require.NoError(t, err)
	sess = entry()
	require.Equal(t, netip.MustParseAddr("192.0.2.90"), sess.IPv4Addr, "the reinstall sets the address again")
	require.Equal(t, netip.MustParseAddr("192.0.2.53"), sess.DNSPrimary)

	release(l2tpevents.AddressFamilyIPv6)
	sess = entry()
	require.Equal(t, [8]byte{}, sess.IPv6InterfaceID)
	require.Equal(t, netip.MustParseAddr("192.0.2.90"), sess.IPv4Addr, "IPv6 release keeps the IPv4 family")
	require.Len(t, released, 2)
	require.Equal(t, l2tpevents.AddressFamilyIPv6, released[1].Family)
}
