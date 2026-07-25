// Design: docs/architecture/core-design.md -- kernel route redistribution
// Related: events/events.go -- typed EventBus handle for route-change

package kernel

import (
	"context"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/routewatch"
	kernelevents "github.com/ze-software/ze/internal/plugins/kernel/events"
	"github.com/ze-software/ze/pkg/ze"
)

const (
	rtprotKernel   = 2
	rtprotRedirect = 1
)

type routeObserver struct {
	bus ze.EventBus

	mu        sync.Mutex
	announced map[netip.Prefix]struct{}
}

func newRouteObserver(bus ze.EventBus) *routeObserver {
	return &routeObserver{
		bus:       bus,
		announced: make(map[netip.Prefix]struct{}),
	}
}

func (o *routeObserver) handleRouteEvent(ev routewatch.RouteEvent) {
	if ev.Protocol == rtprotKernel || ev.Protocol == rtprotRedirect {
		return
	}

	switch ev.Action {
	case routewatch.ActionAdd:
		o.mu.Lock()
		o.announced[ev.Prefix] = struct{}{}
		o.mu.Unlock()
		o.emit(redistevents.ActionAdd, ev.Prefix, ev.NextHop, ev.Metric)

	case routewatch.ActionRemove:
		o.mu.Lock()
		delete(o.announced, ev.Prefix)
		o.mu.Unlock()
		o.emit(redistevents.ActionRemove, ev.Prefix, ev.NextHop, ev.Metric)
	}
}

func (o *routeObserver) emit(action redistevents.RouteAction, prefix netip.Prefix, nextHop netip.Addr, metric uint32) {
	if o.bus == nil {
		return
	}
	var fam family.Family
	if prefix.Addr().Is4() {
		fam = family.IPv4Unicast
	} else {
		fam = family.IPv6Unicast
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = kernelevents.ProtocolID
	b.AFI = uint16(fam.AFI)
	b.SAFI = uint8(fam.SAFI)
	b.Entries = append(b.Entries, redistevents.RouteChangeEntry{
		Action:  action,
		Prefix:  prefix,
		NextHop: nextHop,
		Metric:  metric,
	})
	if _, err := kernelevents.RouteChange.Emit(o.bus, b); err != nil {
		logger().Warn("kernel: route-change emit failed", "error", err)
	}
}

func (o *routeObserver) withdrawAll() {
	o.mu.Lock()
	prefixes := make([]netip.Prefix, 0, len(o.announced))
	for p := range o.announced {
		prefixes = append(prefixes, p)
	}
	o.announced = make(map[netip.Prefix]struct{})
	o.mu.Unlock()

	for _, p := range prefixes {
		o.emit(redistevents.ActionRemove, p, netip.Addr{}, 0)
	}
}

func (o *routeObserver) run(ctx context.Context) {
	w := routewatch.Global()

	unreg := w.Register(o.handleRouteEvent)

	w.Start(func(err error) {
		logger().Warn("routewatch: error", "error", err)
	})

	logger().Info("kernel: route observer started")

	<-ctx.Done()

	unreg()
	o.withdrawAll()

	logger().Info("kernel: route observer stopped")
}
