// Design: docs/architecture/core-design.md -- redistribute orchestrator
//
// Package redistributeegress implements the redistribute-orchestrator plugin:
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

package redistributeegress

import (
	"context"
	"log/slog"
	"sync/atomic"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
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
	filteredRuleTotal     metrics.Counter
	replayTotal           metrics.CounterVec
}

var metricsPtr atomic.Pointer[pluginMetrics]

func setMetricsRegistry(reg metrics.Registry) {
	m := &pluginMetrics{
		eventsReceived:        reg.Counter("ze_bgp_redistribute_events_received", "Route-change batches received from the EventBus."),
		announcements:         reg.Counter("ze_bgp_redistribute_announcements", "Accepted add entries dispatched to consumers as announcements."),
		withdrawals:           reg.Counter("ze_bgp_redistribute_withdrawals", "Accepted remove entries dispatched to consumers as withdrawals."),
		filteredProtocolTotal: reg.Counter("ze_bgp_redistribute_filtered_protocol_total", "Batches filtered by the consumer-protocol skip."),
		filteredRuleTotal:     reg.Counter("ze_bgp_redistribute_filtered_rule_total", "Entries rejected by the redistribute evaluator."),
		replayTotal:           reg.CounterVec("ze_bgp_redistribute_replay_total", "Redistribute routes replayed to a newly-established peer, by source.", []string{"source"}),
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

	logger().Debug(Name+": running", "producers", len(unsubs))
	<-ctx.Done()
	var tb2 textbuf.Buffer
	logger().Debug(tb2.Str(Name).Str(": stopped").Slice())
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
	route := configredist.RedistRoute{
		Origin: name,
		Family: famVal,
		Source: name,
	}

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
		if !ev.Accept(route, cname) {
			logger().Debug(Name+": evaluator rejected", "source", name, "consumer", cname, "origin", route.Origin, "family", famVal.String())
			if m := getMetrics(); m != nil {
				for range b.Entries {
					m.filteredRuleTotal.Inc()
				}
			}
			continue
		}
		logger().Debug(Name+": dispatching to consumer", "consumer", cname, "entries", len(b.Entries))
		for i := range b.Entries {
			// Empty peer selector: the incremental path fans out to all peers.
			dispatchEntryToConsumer(ctx, consumer, famVal, name, "", b.OriginASN, b.Community, &b.Entries[i])
		}
	}
}

// dispatchEntryToConsumer dispatches one entry to a consumer. peer is the
// single-peer selector for the replay path (empty means the normal all-peers
// fan-out).
func dispatchEntryToConsumer(ctx context.Context, consumer configredist.RedistConsumer, fam family.Family, source, peer string, originASN uint32, community []uint32, entry *redistevents.RouteChangeEntry) {
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
		consumer.InjectRoute(ctx, fam, configredist.RouteEntry{
			Prefix:    entry.Prefix.String(),
			NextHop:   nhop,
			Source:    source,
			Peer:      peer,
			OriginASN: effectiveOriginASN,
			Community: community,
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
