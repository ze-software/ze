// Design: docs/architecture/core-design.md -- bgp-redistribute egress consumer
//
// Package redistributeegress implements the bgp-redistribute-egress plugin:
// the single EventBus subscriber that turns non-BGP protocol route-change
// events into BGP UPDATE announcements.
//
// Architecture (see plan/spec-bgp-redistribute.md):
//
//	protocol producer --(redistevents.RouteChangeBatch)--> EventBus --> bgp-redistribute-egress
//	   |                                                                      |
//	   +---- L2TP, connected, future static/OSPF/ISIS ----+                   |
//	                                                       |                   v
//	                                                       |          configredist.Accept(route, "bgp")
//	                                                       |                   |
//	                                                       |                   v
//	                                                       |          plugin.UpdateRoute(ctx, "*", text)
//	                                                       |                   |
//	                                                       |                   v
//	                                                       |          reactor per-peer dispatch
//	                                                       |                   |
//	                                                       |                   v
//	                                                       |          UPDATE on wire (NEXT_HOP=Self)
//
// The plugin enumerates non-BGP producers via `redistevents.Producers()` at
// startup, builds its OWN local typed handles via
// `events.Register[*RouteChangeBatch](name, redistevents.EventType)`, and
// subscribes. No handle pointer crosses a plugin boundary.

package redistributeegress

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	configredist "codeberg.org/thomas-mangin/ze/internal/component/config/redistribute"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Name is the canonical plugin name registered with the plugin registry.
const Name = "bgp-redistribute-egress"

// Subsystem is the dotted log subsystem key used by slogutil.
const Subsystem = "bgp.redistribute.egress"

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

var eventBusPtr atomic.Pointer[ze.EventBus]

func setEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

type pluginMetrics struct {
	eventsReceived        metrics.Counter
	announcements         metrics.Counter
	withdrawals           metrics.Counter
	filteredProtocolTotal metrics.Counter
	filteredRuleTotal     metrics.Counter
}

var metricsPtr atomic.Pointer[pluginMetrics]

func setMetricsRegistry(reg metrics.Registry) {
	m := &pluginMetrics{
		eventsReceived:        reg.Counter("ze_bgp_redistribute_events_received", "Route-change batches received from the EventBus."),
		announcements:         reg.Counter("ze_bgp_redistribute_announcements", "Accepted add entries dispatched to peers as announcements."),
		withdrawals:           reg.Counter("ze_bgp_redistribute_withdrawals", "Accepted remove entries dispatched to peers as withdrawals."),
		filteredProtocolTotal: reg.Counter("ze_bgp_redistribute_filtered_protocol_total", "Batches filtered by the BGP-protocol skip (own protocol)."),
		filteredRuleTotal:     reg.Counter("ze_bgp_redistribute_filtered_rule_total", "Entries rejected by the redistribute evaluator."),
	}
	metricsPtr.Store(m)
}

func getMetrics() *pluginMetrics { return metricsPtr.Load() }

const updateRouteTimeout = 10 * time.Second

type routeDispatcher interface {
	UpdateRoute(ctx context.Context, peerSelector, command string) (uint32, uint32, error)
}

const bgpProtocolName = "bgp"

func run(ctx context.Context, dispatcher routeDispatcher) {
	bus := getEventBus()
	if bus == nil {
		logger().Warn(Name + ": no event bus configured")
		return
	}

	bgpID, _ := redistevents.ProtocolIDOf(bgpProtocolName)

	unsubs := subscribe(ctx, dispatcher, bus, bgpID)
	defer func() {
		for _, u := range unsubs {
			u()
		}
	}()

	logger().Info(Name+": running", "non-bgp-producers", len(unsubs))
	<-ctx.Done()
	logger().Info(Name + ": stopped")
}

func subscribe(ctx context.Context, dispatcher routeDispatcher, bus ze.EventBus, bgpID redistevents.ProtocolID) []func() {
	prods := redistevents.Producers()
	out := make([]func(), 0, len(prods))
	for _, id := range prods {
		if id == bgpID {
			continue
		}
		name := redistevents.ProtocolName(id)
		if name == "" {
			logger().Warn(Name+": producer with no name", "id", id)
			continue
		}
		handle := events.Register[*redistevents.RouteChangeBatch](name, redistevents.EventType)
		out = append(out, handle.Subscribe(bus, func(b *redistevents.RouteChangeBatch) {
			handleBatch(ctx, dispatcher, bgpID, b)
		}))
	}
	return out
}

func handleBatch(ctx context.Context, dispatcher routeDispatcher, bgpID redistevents.ProtocolID, b *redistevents.RouteChangeBatch) {
	if m := getMetrics(); m != nil {
		m.eventsReceived.Inc()
	}
	if b == nil {
		logger().Warn(Name + ": nil batch")
		return
	}
	if b.Protocol == redistevents.ProtocolUnspecified {
		logger().Warn(Name + ": batch with unspecified protocol")
		return
	}
	if b.Protocol == bgpID {
		if m := getMetrics(); m != nil {
			m.filteredProtocolTotal.Inc()
		}
		return
	}

	name := redistevents.ProtocolName(b.Protocol)
	if name == "" {
		logger().Warn(Name+": batch from unregistered ProtocolID", "id", b.Protocol)
		return
	}

	ev := configredist.Global()
	if ev == nil {
		return
	}

	famVal := family.Family{AFI: family.AFI(b.AFI), SAFI: family.SAFI(b.SAFI)}
	route := configredist.RedistRoute{
		Origin: name,
		Family: famVal,
		Source: name,
	}

	if !ev.Accept(route, bgpProtocolName) {
		if m := getMetrics(); m != nil {
			for range b.Entries {
				m.filteredRuleTotal.Inc()
			}
		}
		return
	}

	famStr := famVal.String()
	for i := range b.Entries {
		dispatchEntry(ctx, dispatcher, famStr, &b.Entries[i])
	}
}

func dispatchEntry(ctx context.Context, dispatcher routeDispatcher, fam string, entry *redistevents.RouteChangeEntry) {
	if !entry.Prefix.IsValid() {
		logger().Warn(Name+": skipping entry with invalid prefix", "action", entry.Action)
		return
	}
	cmd, ok := buildCommand(fam, entry)
	if !ok {
		logger().Warn(Name+": skipping entry with invalid action", "action", entry.Action, "prefix", entry.Prefix)
		return
	}

	cctx, cancel := context.WithTimeout(ctx, updateRouteTimeout)
	defer cancel()
	if _, _, err := dispatcher.UpdateRoute(cctx, "*", cmd); err != nil {
		logger().Warn(Name+": update-route failed", "error", err, "command", cmd)
	}
}

func buildCommand(fam string, entry *redistevents.RouteChangeEntry) (string, bool) {
	if entry.Action == redistevents.ActionAdd {
		if m := getMetrics(); m != nil {
			m.announcements.Inc()
		}
		return formatAnnounce(fam, entry), true
	}
	if entry.Action == redistevents.ActionRemove {
		if m := getMetrics(); m != nil {
			m.withdrawals.Inc()
		}
		return formatWithdraw(fam, entry), true
	}
	return "", false
}
