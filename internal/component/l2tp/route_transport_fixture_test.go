// Design: docs/architecture/l2tp/subscriber-session-model.md -- L2TP route events through the subscriber bridge

package l2tp

import (
	"log/slog"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/subscriberid"
	"github.com/ze-software/ze/pkg/ze"
)

func startL2TPRouteTestReader(t *testing.T, bus ze.EventBus, logger *slog.Logger, registry *subscriber.Registry) *subscriberRouteObserver {
	t.Helper()
	reader := StartSubscriberRouteObserver(logger, bus, subscriber.AccessL2TP, l2tpevents.ProtocolID, l2tpevents.RouteChange)
	t.Cleanup(reader.Stop)
	bridge := newSubscriberBridge(registry, bus, logger)
	t.Cleanup(bridge.stop)
	return reader.observer
}

// recordingRouteObserver captures the reactor's original Down coordinates and
// route changes from the bridge and route subscriber on one test bus.
type recordingRouteObserver struct {
	bus      ze.EventBus
	observer *subscriberRouteObserver
	routes   recordingBus
	mu       sync.Mutex
	down     [][2]uint16
}

func newRecordedActorRoutes(t *testing.T) *recordingRouteObserver {
	t.Helper()
	routes := &recordingRouteObserver{}
	routes.start(t, newSessionOrderBus(), slog.Default())
	return routes
}

func (r *recordingRouteObserver) start(t *testing.T, bus ze.EventBus, logger *slog.Logger) {
	t.Helper()
	r.bus = bus
	r.observer = startL2TPRouteTestReader(t, bus, logger, subscriber.NewRegistry())
	t.Cleanup(l2tpevents.RouteChange.Subscribe(bus, func(batch *redistevents.RouteChangeBatch) {
		if _, err := r.routes.Emit(l2tpevents.Namespace, redistevents.EventType, batch); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(l2tpevents.SessionDown.Subscribe(bus, func(p *l2tpevents.SessionDownPayload) {
		r.mu.Lock()
		r.down = append(r.down, [2]uint16{p.TunnelID, p.SessionID})
		r.mu.Unlock()
	}))
}

func (r *recordingRouteObserver) downs() [][2]uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	routes := make([][2]uint16, len(r.down))
	copy(routes, r.down)
	return routes
}

func (r *recordingRouteObserver) replay(t *testing.T, id uint64) {
	t.Helper()
	_, err := redistevents.ReplayRequestEvent.Emit(r.bus, &redistevents.ReplayRequest{ReplayID: id})
	require.NoError(t, err)
}

func seedL2TPRouteNotice(t *testing.T, bus ze.EventBus, birth subscriberid.L2TPBirth, tid, sid uint16, addr netip.Addr, network l2tpevents.NetworkLifetime) {
	t.Helper()
	_, err := l2tpevents.SessionIPAssigned.Emit(bus, &l2tpevents.SessionIPAssignedPayload{
		Birth: birth, SubscriberID: birth.String(), TunnelID: tid, SessionID: sid, PeerAddr: addr.String(), NetworkLifetime: network,
	})
	require.NoError(t, err)
}

func (r *recordingRouteObserver) seedHostRoute(t *testing.T, birth subscriberid.L2TPBirth, tid, sid uint16) redistevents.RouteChangeEntry {
	t.Helper()
	addr := netip.MustParseAddr("192.0.2.7")
	seedL2TPRouteNotice(t, r.bus, birth, tid, sid, addr, l2tpevents.NetworkLifetime{})
	adds := actorRouteEntries(t, r.routes.events(), redistevents.ActionAdd, 0)
	require.Len(t, adds, 1, "the fixture must install the original host route")
	require.Equal(t, netip.PrefixFrom(addr, 32), adds[0].Prefix)
	require.NotZero(t, adds[0].SourceInstance)
	return adds[0]
}

func (r *recordingRouteObserver) requireHostWithdrawal(t *testing.T, original redistevents.RouteChangeEntry) {
	t.Helper()
	original.Action = redistevents.ActionRemove
	require.Equal(t, []redistevents.RouteChangeEntry{original}, actorRouteEntries(t, r.routes.events(), redistevents.ActionRemove, 0))
}
