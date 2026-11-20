// Design: docs/architecture/l2tp/subscriber-session-model.md -- shared subscriber route events

package l2tp

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/redistevents"
)

type subscriberRouteNotice struct {
	protocol redistevents.ProtocolID
	replayID uint64
	entry    redistevents.RouteChangeEntry
}

func TestSubscriberRouteEventsKeepAccessAndLifetimeApart(t *testing.T) {
	t.Cleanup(redistevents.PreserveForTest())
	bus := newSessionOrderBus()
	const fixtureSource = "d14-pppoe-route-fixture"
	pppoeProtocol := redistevents.RegisterProtocol(fixtureSource)
	pppoeRouteChange := events.Register[*redistevents.RouteChangeBatch](fixtureSource, redistevents.EventType)
	l2tpReader := StartSubscriberRouteObserver(slog.Default(), bus, subscriber.AccessL2TP, l2tpevents.ProtocolID, l2tpevents.RouteChange)
	t.Cleanup(l2tpReader.Stop)
	pppoeReader := StartSubscriberRouteObserver(slog.Default(), bus, subscriber.AccessPPPoE, pppoeProtocol, pppoeRouteChange)
	t.Cleanup(pppoeReader.Stop)

	takeNotices := func(event *events.Event[*redistevents.RouteChangeBatch]) *[]subscriberRouteNotice {
		var out []subscriberRouteNotice
		t.Cleanup(event.Subscribe(bus, func(batch *redistevents.RouteChangeBatch) {
			for _, entry := range batch.Entries {
				out = append(out, subscriberRouteNotice{protocol: batch.Protocol, replayID: batch.ReplayID, entry: entry})
			}
		}))
		return &out
	}
	l2tpNotices := takeNotices(l2tpevents.RouteChange)
	pppoeNotices := takeNotices(pppoeRouteChange)

	previous, err := (l2tpevents.NetworkLifetime{}).Next()
	require.NoError(t, err)
	current, err := previous.Next()
	require.NoError(t, err)
	birth := fixedRouteFixtureBirth(t, 707)
	address := netip.MustParseAddr("192.0.2.7")
	lns := subscriber.Session{ID: birth.String(), L2TPBirth: birth, AccessType: subscriber.AccessL2TP, TunnelID: 7, SessionID: 9, IPv4Addr: address}
	access := subscriber.Session{ID: subscriber.SessionID(subscriber.AccessPPPoE, 7, 9), AccessType: subscriber.AccessPPPoE, AccessIfIndex: 7, PPPoESID: 9, IPv4Addr: address}
	lnsFramed := l2tpevents.FramedRoute{Prefix: netip.MustParsePrefix("198.51.100.0/24"), Metric: 5}
	accessFramed := l2tpevents.FramedRoute{Prefix: netip.MustParsePrefix("2001:db8:7::/48"), Metric: 7}

	_, err = subevents.SessionIPAssigned.Emit(bus, &subevents.SessionIPAssignedPayload{
		Session: lns, Address: address, FramedRoutes: []l2tpevents.FramedRoute{lnsFramed}, NetworkLifetime: current,
	})
	require.NoError(t, err)
	require.Len(t, *l2tpNotices, 2, "L2TP assignment must advertise both its own host and framed route")
	require.Empty(t, *pppoeNotices, "L2TP assignment must never be exported as PPPoE")
	_, err = subevents.SessionIPAssigned.Emit(bus, &subevents.SessionIPAssignedPayload{
		Session: access, Address: address, FramedRoutes: []l2tpevents.FramedRoute{accessFramed}, NetworkLifetime: current,
	})
	require.NoError(t, err)
	require.Len(t, *l2tpNotices, 2, "PPPoE assignment must never be exported as L2TP")
	// The access framed route is IPv6, so the IPv4 install announces the host
	// alone; the IPv6 install below announces the framed route (AC-4).
	require.Len(t, *pppoeNotices, 1, "PPPoE IPv4 assignment must advertise its host and not the IPv6 framed route")
	lnsInstance, accessInstance := (*l2tpNotices)[0].entry.SourceInstance, (*pppoeNotices)[0].entry.SourceInstance
	require.NotZero(t, lnsInstance)
	require.NotZero(t, accessInstance)
	require.NotEqual(t, lnsInstance, accessInstance, "equal labels and IPs still own distinct contributions")
	host := netip.PrefixFrom(address, 32)
	assertSubscriberRoutePair(t, *l2tpNotices, l2tpevents.ProtocolID, lnsInstance, 0, redistevents.ActionAdd, host, lnsFramed)

	access.IPv6InterfaceID[7] = 9
	_, err = subevents.SessionIPAssigned.Emit(bus, &subevents.SessionIPAssignedPayload{
		Session: access, NetworkLifetime: current,
	})
	require.NoError(t, err)
	require.Len(t, *pppoeNotices, 2, "an IPv6CP identifier must add its framed route and not re-advertise the IPv4 host")
	assertSubscriberRoutePair(t, *pppoeNotices, pppoeProtocol, accessInstance, 0, redistevents.ActionAdd, host, accessFramed)

	_, err = subevents.SessionDown.Emit(bus, &subevents.SessionDownPayload{Session: access, NetworkLifetime: previous})
	require.NoError(t, err)
	require.Len(t, *pppoeNotices, 2, "a stale network down must leave the current contribution intact")
	const liveReplayID uint64 = 707
	_, err = redistevents.ReplayRequestEvent.Emit(bus, &redistevents.ReplayRequest{ReplayID: liveReplayID})
	require.NoError(t, err)
	require.Len(t, *pppoeNotices, 4, "the live PPPoE host and framed route must replay")
	assertSubscriberRoutePair(t, (*pppoeNotices)[2:], pppoeProtocol, accessInstance, liveReplayID, redistevents.ActionAdd, host, accessFramed)
	require.Len(t, *l2tpNotices, 4, "the live L2TP host and framed route must replay")
	assertSubscriberRoutePair(t, (*l2tpNotices)[2:], l2tpevents.ProtocolID, lnsInstance, liveReplayID, redistevents.ActionAdd, host, lnsFramed)

	_, err = subevents.SessionDown.Emit(bus, &subevents.SessionDownPayload{Session: access, NetworkLifetime: current})
	require.NoError(t, err)
	require.Len(t, *pppoeNotices, 6)
	assertSubscriberRoutePair(t, (*pppoeNotices)[4:], pppoeProtocol, accessInstance, 0, redistevents.ActionRemove, host, accessFramed)

	const survivorReplayID uint64 = 708
	_, err = redistevents.ReplayRequestEvent.Emit(bus, &redistevents.ReplayRequest{ReplayID: survivorReplayID})
	require.NoError(t, err)
	require.Len(t, *pppoeNotices, 6, "withdrawn PPPoE contributions must not replay")
	require.Len(t, *l2tpNotices, 6, "the surviving L2TP host and framed route must replay")
	assertSubscriberRoutePair(t, (*l2tpNotices)[4:], l2tpevents.ProtocolID, lnsInstance, survivorReplayID, redistevents.ActionAdd, host, lnsFramed)
	_, err = subevents.SessionDown.Emit(bus, &subevents.SessionDownPayload{Session: lns, NetworkLifetime: current})
	require.NoError(t, err)
	require.Len(t, *l2tpNotices, 8)
	assertSubscriberRoutePair(t, (*l2tpNotices)[6:], l2tpevents.ProtocolID, lnsInstance, 0, redistevents.ActionRemove, host, lnsFramed)
}

func TestSubscriberRouteStopWithdrawsLiveContributions(t *testing.T) {
	bus := newSessionOrderBus()
	reader := StartSubscriberRouteObserver(slog.Default(), bus, subscriber.AccessL2TP, l2tpevents.ProtocolID, l2tpevents.RouteChange)
	var notices []subscriberRouteNotice
	var replayDuringStopErr error
	replayDuringStop := false
	t.Cleanup(l2tpevents.RouteChange.Subscribe(bus, func(batch *redistevents.RouteChangeBatch) {
		for _, entry := range batch.Entries {
			notices = append(notices, subscriberRouteNotice{protocol: batch.Protocol, replayID: batch.ReplayID, entry: entry})
			if entry.Action == redistevents.ActionRemove && !replayDuringStop {
				replayDuringStop = true
				_, replayDuringStopErr = redistevents.ReplayRequestEvent.Emit(bus, &redistevents.ReplayRequest{ReplayID: 709})
			}
		}
	}))

	birth := fixedRouteFixtureBirth(t, 709)
	address := netip.MustParseAddr("192.0.2.9")
	host := netip.PrefixFrom(address, 32)
	framed := l2tpevents.FramedRoute{Prefix: netip.MustParsePrefix("198.51.100.0/24"), Metric: 5}
	lns := subscriber.Session{ID: birth.String(), L2TPBirth: birth, AccessType: subscriber.AccessL2TP, TunnelID: 7, SessionID: 9}
	network, err := (l2tpevents.NetworkLifetime{}).Next()
	require.NoError(t, err)
	_, err = subevents.SessionIPAssigned.Emit(bus, &subevents.SessionIPAssignedPayload{
		Session: lns, Address: address, FramedRoutes: []l2tpevents.FramedRoute{framed}, NetworkLifetime: network,
	})
	require.NoError(t, err)
	require.Len(t, notices, 2)
	instance := notices[0].entry.SourceInstance
	require.NotZero(t, instance)
	assertSubscriberRoutePair(t, notices, l2tpevents.ProtocolID, instance, 0, redistevents.ActionAdd, host, framed)

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		reader.Stop()
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("route reader stop deadlocked when a removal recipient requested a replay")
	}
	require.Len(t, notices, 4, "stop must withdraw both contributions even before the terminal notice arrives")
	assertSubscriberRoutePair(t, notices[2:], l2tpevents.ProtocolID, instance, 0, redistevents.ActionRemove, host, framed)
	require.True(t, replayDuringStop)
	require.NoError(t, replayDuringStopErr)

	_, err = subevents.SessionDown.Emit(bus, &subevents.SessionDownPayload{Session: lns, NetworkLifetime: network})
	require.NoError(t, err)
	_, err = redistevents.ReplayRequestEvent.Emit(bus, &redistevents.ReplayRequest{ReplayID: 710})
	require.NoError(t, err)
	reader.Stop()
	require.Len(t, notices, 4, "late down, replay and repeated stop must not change the retired contribution")
}

func assertSubscriberRoutePair(t *testing.T, notices []subscriberRouteNotice, protocol redistevents.ProtocolID, source uint32, replay uint64, action redistevents.RouteAction, host netip.Prefix, framed l2tpevents.FramedRoute) {
	t.Helper()
	require.Equal(t, []subscriberRouteNotice{
		{protocol: protocol, replayID: replay, entry: redistevents.RouteChangeEntry{Action: action, Prefix: host, SourceInstance: source}},
		{protocol: protocol, replayID: replay, entry: redistevents.RouteChangeEntry{Action: action, Prefix: framed.Prefix, Metric: framed.Metric, SourceInstance: source}},
	}, notices)
}
