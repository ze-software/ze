// Design: docs/architecture/core-design.md -- bgp-redistribute egress consumer
// Related: format.go -- canonical announce/withdraw command-text builders
//
// Package redistribute implements the bgp-redistribute plugin: the single
// EventBus subscriber that turns non-BGP protocol route-change events into
// BGP UPDATE announcements.
//
// Architecture (see plan/spec-bgp-redistribute.md):
//
//	protocol producer --(redistevents.RouteChangeBatch)--> EventBus --> bgp-redistribute
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

package redistribute

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
// Hyphenated form per plugin-design.md. The "-egress" suffix distinguishes
// this consumer (cross-protocol egress) from the existing
// `bgp-redistribute` plugin in `internal/component/bgp/redistribute/`,
// which registers the intra-BGP IngressFilter on the same evaluator.
const Name = "bgp-redistribute-egress"

// Subsystem is the dotted log subsystem key used by slogutil.
const Subsystem = "bgp.redistribute.egress"

// loggerPtr is the package-level logger, disabled by default. setLogger
// installs the engine's logger for this plugin via the registration's
// ConfigureEngineLogger callback.
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

// eventBusPtr stores the EventBus instance. Set by ConfigureEventBus before
// RunEngine. Read by run() at startup.
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

// pluginMetrics holds Prometheus counters for bgp-redistribute. Counters
// are filled by the metrics registration in metrics.go; the empty struct is
// reserved here so unit tests that exercise run() without a metrics registry
// still find a non-nil pointer.
type pluginMetrics struct {
	eventsReceived        metrics.Counter
	announcements         metrics.Counter
	withdrawals           metrics.Counter
	filteredProtocolTotal metrics.Counter
	filteredRuleTotal     metrics.Counter
}

// metricsPtr stores the metrics struct, set by setMetricsRegistry via
// ConfigureMetrics. nil is a valid value (handlers no-op when nil) so
// internal-mode plugins without metrics still work.
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

// getMetrics returns the active counter set, or nil. Hot-path callers must
// nil-check before incrementing.
func getMetrics() *pluginMetrics { return metricsPtr.Load() }

// updateRouteTimeout is the per-call deadline for plugin.UpdateRoute. Matches
// bgp-rib's existing 10s timeout (rib.go:502); kept short so a stalled
// dispatcher cannot back up the consumer goroutine.
const updateRouteTimeout = 10 * time.Second

// routeDispatcher is the slice of *sdk.Plugin we depend on. Extracting it
// as an interface lets tests inject a fake without standing up a full SDK
// instance and connection pair.
type routeDispatcher interface {
	UpdateRoute(ctx context.Context, peerSelector, command string) (uint32, uint32, error)
}

// bgpProtocolName is the canonical name for BGP in the redistevents
// registry. Looked up once at run() startup; if the lookup fails (BGP did
// not register) the consumer treats every protocol as non-BGP, which is
// safe because BGP does not produce route-change events today (consumer,
// not producer).
const bgpProtocolName = "bgp"

// run is the long-lived consumer goroutine. Enumerates non-BGP producers
// from the redistevents registry, builds local typed handles, subscribes,
// then blocks on ctx.Done() until shutdown. Subscription cleanup happens
// via defer.
//
// dispatcher is the engine-side route injector (typically *sdk.Plugin).
// Decoupled from sdk so unit tests can inject a recording fake.
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

// subscribe enumerates registered producers, skipping BGP, and registers
// per-protocol typed handlers on the bus. Returns the unsubscribe functions
// so run() can defer them. Each consumer-side events.Register call obtains
// a LOCAL handle bound to the same (namespace, eventType, T) tuple the
// producer registered -- no handle pointer crosses a plugin boundary.
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

// handleBatch is the per-event handler. Filters by protocol, consults the
// global redistribute evaluator, and dispatches one UpdateRoute RPC per
// accepted entry.
//
// The handler runs synchronously on the bus dispatcher thread per the
// EventBus contract (see eventbus.go); MUST NOT retain b past return.
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
		// No redistribute config -- drop. Common when the operator has not
		// enabled redistribute at all.
		return
	}

	famVal := family.Family{AFI: family.AFI(b.AFI), SAFI: family.SAFI(b.SAFI)}
	route := configredist.RedistRoute{
		Origin: name,
		Family: famVal,
		Source: name,
	}

	if !ev.Accept(route, bgpProtocolName) {
		// Whole batch rejected by the evaluator -- count one rule-rejection
		// per entry so the counter reflects entries-not-dispatched.
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

// dispatchEntry builds the command text for one entry and fires the
// UpdateRoute RPC with a bounded per-call timeout. Unknown / unspecified
// actions are dropped with a warn; invalid prefixes are likewise dropped at
// the consumer rather than letting "invalid Prefix" leak into the announce
// command text and trip the reactor's parser.
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

// buildCommand returns the announce/withdraw text for one entry plus an OK
// flag. ok=false signals an unrecognized action. Counter increments for the
// happy paths happen here so the test fixture sees them even when the
// dispatcher fails the RPC.
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
