// Design: docs/architecture/core-design.md -- redistribute orchestrator
//
// Package redistribute implements the redistribute-orchestrator plugin:
// the single EventBus subscriber that turns non-consumer protocol route-change
// events into dispatches to registered RedistConsumer implementations.
//
// Architecture:
//
//	protocol producer --(redistevents.RouteChangeBatch)--> EventBus --> redistribute-orchestrator
//	   |                                                                      |
//	   +---- L2TP, connected, future static/OSPF/ISIS ----+                   |
//	                                                       |                   v
//	                                                       |          for each consumer:
//	                                                       |            configredist.Accept(route, consumer.Name())
//	                                                       |                   |
//	                                                       |                   v
//	                                                       |            consumer.InjectRoute / WithdrawRoute
//	                                                       |                   |
//	                                                       |                   v
//	                                                       |            protocol-specific dispatch
//
// The plugin enumerates non-consumer producers via `redistevents.Producers()` at
// startup, builds its OWN local typed handles via
// `events.Register[*RouteChangeBatch](name, redistevents.EventType)`, and
// subscribes. No handle pointer crosses a plugin boundary.

package redistribute

import (
	"context"
	"log/slog"
	"sync/atomic"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	ribpolicy "github.com/ze-software/ze/internal/core/rib/policy"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/ze"
)

// Name is the canonical plugin name registered with the plugin registry.
const Name = "redistribute-orchestrator"

// Subsystem is the dotted log subsystem key used by slogutil.
const Subsystem = "redistribute.orchestrator"

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
	chainRejectedTotal    metrics.CounterVec
	replayTotal           metrics.CounterVec
}

var metricsPtr atomic.Pointer[pluginMetrics]

func setMetricsRegistry(reg metrics.Registry) {
	m := &pluginMetrics{
		eventsReceived:        reg.Counter("ze_bgp_redistribute_events_received", "Route-change batches received from the EventBus."),
		announcements:         reg.Counter("ze_bgp_redistribute_announcements", "Accepted add entries dispatched to consumers as announcements."),
		withdrawals:           reg.Counter("ze_bgp_redistribute_withdrawals", "Withdrawals dispatched to consumers: accepted remove entries, and held routes a rejected entry retires."),
		filteredProtocolTotal: reg.Counter("ze_bgp_redistribute_filtered_protocol_total", "Batches filtered by the consumer-protocol skip."),
		chainRejectedTotal:    reg.CounterVec("ze_redistribute_chain_rejected_total", "Routes an import rule covers and its policy chain rejected, by destination and source protocol.", []string{"destination", "source"}),
		replayTotal:           reg.CounterVec("ze_bgp_redistribute_replay_total", "Redistribute routes replayed to a BGP peer whose session just established or to a destination consumer that just registered, by source.", []string{"source"}),
	}
	metricsPtr.Store(m)
}

func getMetrics() *pluginMetrics { return metricsPtr.Load() }

func consumerProtocolIDs() map[redistevents.ProtocolID]bool {
	names := configredist.ConsumerNames()
	ids := make(map[redistevents.ProtocolID]bool, len(names))
	for _, n := range names {
		if id, ok := redistevents.ProtocolIDOf(n); ok {
			ids[id] = true
		}
	}
	return ids
}

func run(ctx context.Context) {
	bus := getEventBus()
	if bus == nil {
		var tb textbuf.Buffer
		logger().Warn(tb.Str(Name).Str(": no event bus configured").Slice())
		return
	}

	// Snapshot consumer protocol IDs so a producer batch is not dispatched back to
	// the same protocol's consumer. Other consumers still receive that producer,
	// e.g. BGP best-path changes redistributed into OSPF.
	skipIDs := consumerProtocolIDs()
	unsubs := subscribe(ctx, bus, skipIDs)
	defer func() {
		for _, u := range unsubs {
			u()
		}
	}()

	// Installed AFTER subscribe. A replay request this dispatcher is not yet
	// subscribed to answers nobody. The producers re-emit into a bus with no
	// orchestrator on it, which is the same loss in a different place.
	stopWatching := watchConsumers(bus, getReplayCoordinator())
	defer stopWatching()

	logger().Debug(Name+": running", "producers", len(unsubs))
	<-ctx.Done()
	var tb2 textbuf.Buffer
	logger().Debug(tb2.Str(Name).Str(": stopped").Slice())
}

// watchConsumers makes a consumer that registers late hold the producers'
// current set rather than nothing, and returns the function that stops
// watching. The caller MUST have subscribed to the producers first.
//
// Two arrivals are late, and both are owed. A consumer that registers after
// this dispatcher started is answered by the observer. A consumer that
// registered BEFORE it started is answered by the sweep. Its registration
// happened where no observer can see it, and the batch it missed reached no
// dispatcher either. Nothing orders the plugin startup tiers, so which of the
// two a deployment takes is not decided anywhere.
//
// A consumer that registers during the sweep is replayed twice. That costs one
// duplicate InjectRoute of a prefix the consumer already holds. Every consumer
// treats it as the add it already applied.
func watchConsumers(bus ze.EventBus, coord *replayCoordinator) func() {
	configredist.SetConsumerObserver(func(name string) {
		// A consumer registering here, first time or again, holds nothing yet:
		// the replay below is what it will hold.
		consumerHeld.forgetConsumer(name)
		coord.onConsumerRegistered(bus, name)
	})
	for _, name := range configredist.ConsumerNames() {
		coord.onConsumerRegistered(bus, name)
	}
	return func() { configredist.SetConsumerObserver(nil) }
}

func subscribe(ctx context.Context, bus ze.EventBus, skipIDs map[redistevents.ProtocolID]bool) []func() {
	prods := redistevents.Producers()
	out := make([]func(), 0, len(prods))
	for _, id := range prods {
		name := redistevents.ProtocolName(id)
		if name == "" {
			logger().Warn(Name+": producer with no name", "id", id)
			continue
		}
		handle := events.Register[*redistevents.RouteChangeBatch](name, redistevents.EventType)
		out = append(out, handle.Subscribe(bus, func(b *redistevents.RouteChangeBatch) {
			handleBatch(ctx, skipIDs, b)
		}))
	}
	return out
}

func handleBatch(ctx context.Context, skipIDs map[redistevents.ProtocolID]bool, b *redistevents.RouteChangeBatch) {
	if m := getMetrics(); m != nil {
		m.eventsReceived.Inc()
	}
	var tb textbuf.Buffer
	if b == nil {
		logger().Warn(tb.Str(Name).Str(": nil batch").Slice())
		return
	}
	if b.Protocol == redistevents.ProtocolUnspecified {
		logger().Warn(tb.Reset().Str(Name).Str(": batch with unspecified protocol").Slice())
		return
	}

	// Replay batches (nonzero ReplayID, echoed from a ReplayRequest) are
	// correlated to the one peer whose establishment triggered the request and
	// injected via the BGP consumer only. The incremental path below is
	// unchanged for the default ReplayID==0.
	if b.IsReplay() {
		handleReplayBatch(ctx, b)
		return
	}

	name := redistevents.ProtocolName(b.Protocol)
	if name == "" {
		logger().Warn(Name+": batch from unregistered ProtocolID", "id", b.Protocol)
		return
	}
	if skipIDs[b.Protocol] {
		logger().Debug(Name+": source protocol has a consumer; skipping only that consumer", "source", name)
	}

	ev := configredist.Global()
	if ev == nil {
		logger().Warn(Name+": no evaluator configured, dropping batch", "source", name)
		return
	}
	logger().Debug(Name+": processing batch", "source", name, "entries", len(b.Entries))

	famVal := family.Family{AFI: family.AFI(b.AFI), SAFI: family.SAFI(b.SAFI)}

	consumers := configredist.ConsumerNames()
	if len(consumers) == 0 {
		logger().Warn(Name+": no consumers registered, dropping batch", "source", name)
		return
	}
	for _, cname := range consumers {
		consumer, ok := configredist.LookupConsumer(cname)
		if !ok {
			logger().Warn(Name+": consumer not found", "consumer", cname)
			continue
		}
		// Loop prevention (per-consumer skip): don't fan a source protocol's
		// batch back into that same protocol's consumer; others still receive it.
		if redistevents.WouldLoop(name, cname) {
			if m := getMetrics(); m != nil {
				m.filteredProtocolTotal.Inc()
			}
			continue
		}
		logger().Debug(Name+": offering batch to consumer", "consumer", cname, "entries", len(b.Entries))
		for i := range b.Entries {
			entry := &b.Entries[i]
			// Acceptance is decided for each ROUTE, not for the batch: an import
			// rule's chain reads the entry's prefix, tag and BGP class.
			route := redistRoute(name, b.Protocol, famVal, entry)
			key := heldKey{consumer: cname, source: name, family: famVal, prefix: entry.Prefix}
			sets, accepted := ev.Accept(&route, cname)
			if !accepted {
				logger().Debug(Name+": evaluator rejected", "source", name, "consumer", cname, "origin", route.Origin, "family", famVal.String(), "prefix", entry.Prefix, "tag", entry.Tag)
				countChainRejected(ev, &route, cname, name)
				removeReplacedRoute(ctx, consumer, key, entry.Action)
				continue
			}
			// Empty peer selector: the incremental path fans out to all peers.
			dispatchEntryToConsumer(ctx, consumer, famVal, name, "", b.OriginASN, b.Community, entry, &route, sets)
			recordDispatched(key, entry.Action, &route)
		}
	}
}

// countChainRejected counts a route the evaluator refused under the policy
// chain of a rule that imports it. A route no rule imports into destination is
// not a chain's verdict, so it is left out (configredist.Evaluator.Imports).
func countChainRejected(ev *configredist.Evaluator, route *configredist.RedistRoute, destination, source string) {
	m := getMetrics()
	if m == nil {
		return
	}
	if !ev.Imports(route, destination) {
		return
	}
	m.chainRejectedTotal.With(destination, source).Inc()
}

// redistRoute builds the route an import rule judges for one entry: the
// protocol it came from, the source a rule names it by, and the view the
// rule's chain reads, with the producer's original values.
func redistRoute(protocol string, id redistevents.ProtocolID, fam family.Family, entry *redistevents.RouteChangeEntry) configredist.RedistRoute {
	return configredist.RedistRoute{
		Origin: protocol,
		Source: entrySource(protocol, entry),
		Route: ribpolicy.Route{
			Family:  fam,
			Prefix:  entry.Prefix,
			Source:  id,
			NextHop: entry.NextHop,
			Metric:  entry.Metric,
			Tag:     entry.Tag,
			// Every redistributed route carries a tag: a producer writes zero
			// for a route with no tag (RouteChangeEntry.Tag), and zero is a tag
			// like any other, so `match tag [ 0 ]` selects the untagged routes.
			// That keeps the retired `tag 0` leaf's meaning and agrees with
			// FRR's route_match_tag, which compares an untagged route's tag 0.
			HasTag: true,
			IsBGP:  entry.BGPClass != redistevents.BGPClassUnspecified,
			IsEBGP: entry.BGPClass == redistevents.BGPClassExternal,
		},
	}
}

// applySets answers the entry a consumer receives: the producer's entry with
// the tag and the metric the accepting chain set, when it set them.
func applySets(entry *redistevents.RouteChangeEntry, sets ribpolicy.Sets) redistevents.RouteChangeEntry {
	adjusted := *entry
	if sets.HasTag {
		adjusted.Tag = sets.Tag
	}
	if sets.HasMetric {
		adjusted.Metric = sets.Metric
	}
	return adjusted
}

// entrySource answers the source a rule matches an entry under: the BGP class
// of a BGP best path (`ebgp`, `ibgp`), which the producer stamps on the entry,
// and otherwise the batch protocol. A rule naming the protocol itself (`bgp`)
// still matches through the route's Origin.
func entrySource(protocol string, entry *redistevents.RouteChangeEntry) string {
	if class, ok := entry.BGPClass.SourceName(); ok {
		return class
	}
	return protocol
}

// dispatchEntryToConsumer hands one entry to consumer. peer is the single-peer
// selector for the replay path (empty means the normal all-peers fan-out).
// view is the route the import chain judged and sets what its accepting chain
// set; a withdraw carries neither and passes nil and the zero Sets.
func dispatchEntryToConsumer(ctx context.Context, consumer configredist.RedistConsumer, fam family.Family, source, peer string, originASN uint32, community []uint32, produced *redistevents.RouteChangeEntry, view *configredist.RedistRoute, sets ribpolicy.Sets) {
	adjusted := applySets(produced, sets)
	entry := &adjusted
	if !entry.Prefix.IsValid() {
		logger().Warn(Name+": skipping entry with invalid prefix", "action", entry.Action)
		return
	}
	if entry.Action == redistevents.ActionAdd {
		if m := getMetrics(); m != nil {
			m.announcements.Inc()
		}
		nhop := ""
		if entry.NextHop.IsValid() {
			nhop = entry.NextHop.String()
		}
		// Prefer the per-entry origin AS when the producer set one (BGP best-paths
		// each carry their own origin AS); fall back to the batch OriginASN for
		// producers that model themselves as a single-ASN virtual router (as112).
		effectiveOriginASN := originASN
		if entry.OriginAS != 0 {
			effectiveOriginASN = entry.OriginAS
		}
		var route ribpolicy.Route
		if view != nil {
			// The import chain's sets are on entry; the view carries them too.
			route = view.Route
			route.Tag = entry.Tag
			route.Metric = entry.Metric
		}
		consumer.InjectRoute(ctx, fam, configredist.RouteEntry{
			Prefix:    entry.Prefix.String(),
			NextHop:   nhop,
			Source:    source,
			Peer:      peer,
			OriginASN: effectiveOriginASN,
			Community: community,
			Tag:       entry.Tag,
			Metric:    entry.Metric,
			HasMetric: sets.HasMetric,
			Route:     route,
		})
		return
	}
	if entry.Action == redistevents.ActionRemove {
		if m := getMetrics(); m != nil {
			m.withdrawals.Inc()
		}
		consumer.WithdrawRoute(ctx, fam, entry.Prefix.String())
		return
	}
	logger().Warn(Name+": skipping entry with invalid action", "action", entry.Action, "prefix", entry.Prefix)
}
