// Design: docs/architecture/core-design.md -- System RIB plugin
//
// System RIB aggregates best routes from all protocol RIBs and selects
// the system-wide best per prefix by administrative distance (lower wins).
// Subscribes to (rib, best-change) on the EventBus, emits (sysrib, best-change).
package sysrib

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"sync"
	"sync/atomic"

	ribevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/events"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	"codeberg.org/thomas-mangin/ze/internal/core/rib/locrib"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysribevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/events"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// sysribMetrics holds Prometheus metrics for the system RIB plugin.
//
// routeChanges pre-binds one Counter per bgptypes.RouteAction at init time;
// the hot path does `m.routeChanges[c.Action].Inc()`, a zero-allocation
// array index. The underlying CounterVec still emits one time series per
// action label to Prometheus exposition.
type sysribMetrics struct {
	routesBest     metrics.Gauge
	routeChanges   [bgptypes.RouteActionCount]metrics.Counter
	eventsReceived metrics.Counter
}

// sysribMetricsPtr stores system RIB metrics, set by SetMetricsRegistry.
var sysribMetricsPtr atomic.Pointer[sysribMetrics]

// SetMetricsRegistry creates system RIB metrics from the given registry.
// Called via ConfigureMetrics callback before RunEngine.
func SetMetricsRegistry(reg metrics.Registry) {
	routeChangeVec := reg.CounterVec("ze_systemrib_route_changes_total", "Best-path changes emitted.", []string{"action"})
	m := &sysribMetrics{
		routesBest:     reg.Gauge("ze_systemrib_routes_best", "Current system-wide best route count."),
		eventsReceived: reg.Counter("ze_systemrib_events_received_total", "Protocol RIB events received."),
	}
	// Pre-bind the actions sysrib actually emits. Unspecified and Del are
	// never published from a system-RIB best-change, so their slots stay
	// nil; a publish of one would fall into the nil-guard in the hot-path
	// increment below.
	for _, a := range [...]bgptypes.RouteAction{
		bgptypes.RouteActionAdd,
		bgptypes.RouteActionUpdate,
		bgptypes.RouteActionWithdraw,
	} {
		m.routeChanges[a] = routeChangeVec.With(a.String())
	}
	sysribMetricsPtr.Store(m)
}

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// locRIBPtr stores the shared cross-protocol Loc-RIB. When set,
// sysrib consumes locrib.OnChange notifications instead of subscribing to
// the BGP-specific (rib, best-change) EventBus stream. Nil restores the
// legacy path for tests and environments that have not wired a Loc-RIB.
var locRIBPtr atomic.Pointer[locrib.RIB]

// SetLocRIB wires the shared Loc-RIB. Safe to call once at plugin setup;
// passing nil reverts to the legacy EventBus-subscription path.
func SetLocRIB(r *locrib.RIB) {
	locRIBPtr.Store(r)
}

func getLocRIB() *locrib.RIB { return locRIBPtr.Load() }

// eventBusPtr stores the EventBus instance.
var eventBusPtr atomic.Pointer[ze.EventBus]

func setEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

// clearEventBus removes any stored EventBus. Used by tests that share the
// package-level pointer between cases.
func clearEventBus() {
	eventBusPtr.Store(nil)
}

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

// protocolRoute is one protocol's best route for a prefix.
type protocolRoute struct {
	protocol         string
	protocolType     string // "ebgp", "ibgp", "static", etc. for admin distance lookup
	nextHop          netip.Addr
	priority         int // effective admin distance (lower wins)
	incomingPriority int // original priority from protocol RIB (before override)
	metric           uint32
}

// prefixKey identifies a unique prefix in the system RIB.
type prefixKey struct {
	family family.Family
	prefix netip.Prefix
}

// sysRIB selects across protocols by admin distance.
type sysRIB struct {
	// routes[prefixKey][protocol] = protocolRoute.
	routes map[prefixKey]map[string]*protocolRoute
	// best[prefixKey] = current system best route.
	best map[prefixKey]*protocolRoute
	// adminDist maps protocol type (e.g., "ebgp", "ibgp", "static") to
	// configured admin distance. Empty when no sysrib config is present,
	// in which case incoming priorities pass through unchanged.
	adminDist map[string]int
	mu        sync.RWMutex
}

func newSysRIB() *sysRIB {
	return &sysRIB{
		routes: make(map[prefixKey]map[string]*protocolRoute),
		best:   make(map[prefixKey]*protocolRoute),
	}
}

// parseAdminDistanceConfig extracts the admin-distance map from the sysrib
// config section JSON. Returns an empty map if no admin-distance block is present.
func parseAdminDistanceConfig(jsonData string) (map[string]int, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonData), &tree); err != nil {
		return nil, fmt.Errorf("unmarshal sysrib config: %w", err)
	}

	sysribTree, ok := tree["rib"].(map[string]any)
	if !ok {
		return make(map[string]int), nil
	}

	adTree, ok := sysribTree["admin-distance"].(map[string]any)
	if !ok {
		return make(map[string]int), nil
	}

	result := make(map[string]int, len(adTree))
	for proto, v := range adTree {
		num, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("admin-distance %s: expected number, got %T", proto, v)
		}
		result[proto] = int(num)
	}

	return result, nil
}

// incomingBatch aliases the (bgp-rib, best-change) payload type. sysrib
// receives one of these per BGP best-change and fans it out to the FIB
// plugins after admin-distance arbitration.
type incomingBatch = ribevents.BestChangeBatch

// incomingChange aliases a single entry in the incoming batch.
type incomingChange = ribevents.BestChangeEntry

// outgoingChange aliases the exported payload entry type so functions in
// this file keep their current signatures while producing the exported
// payload shape used by fib plugins.
type outgoingChange = sysribevents.BestChangeEntry

// outgoingBatch aliases the exported payload type. The producer builds one
// batch per family and emits via the typed BestChange handle.
type outgoingBatch = sysribevents.BestChangeBatch

// effectivePriority returns the configured admin distance for a protocol type
// if one exists, otherwise returns the incoming priority unchanged.
func (s *sysRIB) effectivePriority(protocolType string, incomingPriority int) int {
	if len(s.adminDist) == 0 {
		return incomingPriority
	}
	if d, ok := s.adminDist[protocolType]; ok {
		return d
	}
	return incomingPriority
}

// processEvent handles a batch of protocol RIB changes received from the
// EventBus. Returns the outgoing changes the caller should publish on the
// (sysrib, best-change) channel, plus the family the changes belong to.
// batch is the typed payload delivered by the bgp-rib BestChange handle.
func (s *sysRIB) processEvent(batch *incomingBatch) (family.Family, []outgoingChange) {
	if batch == nil {
		logger().Warn("sysrib: nil batch")
		return family.Family{}, nil
	}
	proto := batch.Protocol
	fam := batch.Family
	if proto == "" || (fam == family.Family{}) {
		logger().Warn("sysrib: event missing protocol or family")
		return family.Family{}, nil
	}

	if m := sysribMetricsPtr.Load(); m != nil {
		m.eventsReceived.Inc()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var outChanges []outgoingChange

	for _, c := range batch.Changes {
		if !c.Prefix.IsValid() {
			logger().Warn("sysrib: skipping change with empty prefix")
			continue
		}
		if c.Action != bgptypes.RouteActionAdd && c.Action != bgptypes.RouteActionUpdate && c.Action != bgptypes.RouteActionWithdraw {
			logger().Warn("sysrib: unrecognized action", "action", c.Action, "prefix", c.Prefix)
			continue
		}

		key := prefixKey{family: fam, prefix: c.Prefix}

		if c.Action == bgptypes.RouteActionAdd || c.Action == bgptypes.RouteActionUpdate {
			// Use per-change protocol type for admin distance override.
			// Falls back to batch-level protocol if per-change type is absent.
			protoType := c.ProtocolType.String()
			if c.ProtocolType == bgptypes.BGPProtocolUnspecified {
				protoType = proto
			}
			priority := s.effectivePriority(protoType, c.Priority)

			if s.routes[key] == nil {
				s.routes[key] = make(map[string]*protocolRoute)
			}
			s.routes[key][proto] = &protocolRoute{
				protocol:         proto,
				protocolType:     protoType,
				nextHop:          c.NextHop,
				priority:         priority,
				incomingPriority: c.Priority,
				metric:           c.Metric,
			}
		} else if c.Action == bgptypes.RouteActionWithdraw && s.routes[key] != nil {
			delete(s.routes[key], proto)
			if len(s.routes[key]) == 0 {
				delete(s.routes, key)
			}
		}

		if change := s.recomputeBest(key); change != nil {
			outChanges = append(outChanges, *change)
		}
	}

	if m := sysribMetricsPtr.Load(); m != nil {
		for _, c := range outChanges {
			if ctr := m.routeChanges[c.Action]; ctr != nil {
				ctr.Inc()
			}
		}
		m.routesBest.Set(float64(len(s.best)))
	}

	return fam, outChanges
}

// reapplyAdminDistances recalculates effective priorities for all stored routes
// using the current adminDist map, then recomputes best for each prefix.
// Returns outgoing changes grouped by family. Caller MUST NOT hold s.mu.
func (s *sysRIB) reapplyAdminDistances() map[family.Family][]outgoingChange {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Recalculate effective priority for every stored route.
	for _, protocols := range s.routes {
		for _, route := range protocols {
			route.priority = s.effectivePriority(route.protocolType, route.incomingPriority)
		}
	}

	// Recompute best for all prefixes; collect changes by family.
	changesByFamily := make(map[family.Family][]outgoingChange)
	for key := range s.routes {
		if change := s.recomputeBest(key); change != nil {
			changesByFamily[key.family] = append(changesByFamily[key.family], *change)
		}
	}

	if m := sysribMetricsPtr.Load(); m != nil {
		for _, changes := range changesByFamily {
			for _, c := range changes {
				if ctr := m.routeChanges[c.Action]; ctr != nil {
					ctr.Inc()
				}
			}
		}
		m.routesBest.Set(float64(len(s.best)))
	}

	return changesByFamily
}

// recomputeBest selects the system-wide best route for a prefix.
// Returns an outgoing change if the system best changed, nil otherwise.
// Caller MUST hold s.mu.
func (s *sysRIB) recomputeBest(key prefixKey) *outgoingChange {
	protocols := s.routes[key]
	prev := s.best[key]

	if len(protocols) == 0 {
		if prev != nil {
			delete(s.best, key)
			return &outgoingChange{
				Action: bgptypes.RouteActionWithdraw,
				Prefix: key.prefix,
			}
		}
		return nil
	}

	// Select lowest priority (admin distance). Deterministic tiebreak by protocol name.
	var winner *protocolRoute
	for _, route := range protocols {
		if winner == nil || route.priority < winner.priority ||
			(route.priority == winner.priority && route.protocol < winner.protocol) {
			winner = route
		}
	}

	if prev == nil {
		s.best[key] = winner
		return &outgoingChange{
			Action:   bgptypes.RouteActionAdd,
			Prefix:   key.prefix,
			NextHop:  winner.nextHop,
			Protocol: winner.protocol,
		}
	}

	if prev.protocol == winner.protocol && prev.nextHop == winner.nextHop &&
		prev.priority == winner.priority && prev.metric == winner.metric {
		// Update the pointer so s.best[key] tracks the current route object
		// even when the values are unchanged (the old struct may be stale).
		s.best[key] = winner
		return nil
	}

	s.best[key] = winner
	return &outgoingChange{
		Action:   bgptypes.RouteActionUpdate,
		Prefix:   key.prefix,
		NextHop:  winner.nextHop,
		Protocol: winner.protocol,
	}
}

// publishChanges emits one event on (system-rib, best-change) via the
// typed BestChange handle. In-process FIB plugins receive the *BestChangeBatch
// directly; external plugin processes receive JSON marshaled by the bus.
func publishChanges(changes []outgoingChange, fam family.Family) {
	eb := getEventBus()
	if eb == nil {
		return
	}

	batch := &outgoingBatch{
		Family:  fam,
		Changes: changes,
	}
	if _, err := sysribevents.BestChange.Emit(eb, batch); err != nil {
		logger().Warn("sysrib: emit failed", "error", err)
	}
}

// replayBest publishes the current system best table as batch events.
// Used for full-table replay when a downstream subscriber requests it.
func (s *sysRIB) replayBest() {
	eb := getEventBus()
	if eb == nil {
		return
	}

	s.mu.RLock()
	changesByFamily := make(map[family.Family][]outgoingChange)
	for key, route := range s.best {
		changesByFamily[key.family] = append(changesByFamily[key.family], outgoingChange{
			Action:   bgptypes.RouteActionAdd,
			Prefix:   key.prefix,
			NextHop:  route.nextHop,
			Protocol: route.protocol,
		})
	}
	s.mu.RUnlock()

	for famName, changes := range changesByFamily {
		batch := &outgoingBatch{
			Family:  famName,
			Replay:  true,
			Changes: changes,
		}
		if _, err := sysribevents.BestChange.Emit(eb, batch); err != nil {
			logger().Warn("sysrib: replay emit failed", "error", err)
		}
	}

	logger().Info("sysrib: replay published", "families", len(changesByFamily))
}

// run consumes best-path changes and blocks until ctx is canceled. In-process
// setups wire a shared Loc-RIB via SetLocRIB; sysrib reacts to its OnChange
// callback. Forked setups (each plugin in its own process) leave Loc-RIB
// unwired because processes cannot share a struct; sysrib falls back to the
// BGP EventBus stream. Both wire the same downstream emission.
func (s *sysRIB) run(ctx context.Context) {
	eb := getEventBus()
	if eb == nil {
		logger().Warn("sysrib: no event bus configured")
		return
	}

	var unsubBest func()
	source := "eventbus"
	if loc := getLocRIB(); loc != nil {
		source = "locrib"
		unsubBest = loc.OnChange(func(c locrib.Change) {
			batch := changeToBatch(c)
			if batch == nil {
				return
			}
			fam, changes := s.processEvent(batch)
			if len(changes) > 0 {
				publishChanges(changes, fam)
			}
		})
		// Snapshot existing state so prefixes inserted before OnChange
		// was registered are carried into sysrib. A live Change arriving
		// between subscribe and this walk is idempotent on processEvent
		// (upsert semantics).
		for _, fam := range loc.Families() {
			loc.Iterate(fam, func(pfx netip.Prefix, g locrib.PathGroup) bool {
				if g.Best < 0 || g.Best >= len(g.Paths) {
					return true
				}
				s.replayPath(fam, pfx, g.Paths[g.Best])
				return true
			})
		}
	} else {
		unsubBest = ribevents.BestChange.Subscribe(eb, func(batch *incomingBatch) {
			fam, changes := s.processEvent(batch)
			if len(changes) > 0 {
				publishChanges(changes, fam)
			}
		})
		if _, err := ribevents.ReplayRequest.Emit(eb); err != nil {
			logger().Warn("sysrib: replay-request emit failed", "error", err)
		}
	}
	defer unsubBest()

	// Subscribe to (system-rib, replay-request) from downstream consumers
	// (e.g., fib-kernel). On request, replay the entire system best table.
	unsubReplay := sysribevents.ReplayRequest.Subscribe(eb, s.replayBest)
	defer unsubReplay()

	logger().Info("sysrib: running", "source", source)
	<-ctx.Done()
	logger().Info("sysrib: stopped")
}

// showRIB returns the current system RIB state as JSON.
func (s *sysRIB) showRIB() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type entry struct {
		Prefix   netip.Prefix  `json:"prefix"`
		Family   family.Family `json:"family"`
		NextHop  netip.Addr    `json:"next-hop,omitzero"`
		Protocol string        `json:"protocol"`
		Priority int           `json:"priority"`
	}

	entries := make([]entry, 0, len(s.best))
	for key, route := range s.best {
		entries = append(entries, entry{
			Prefix:   key.prefix,
			Family:   key.family,
			NextHop:  route.nextHop,
			Protocol: route.protocol,
			Priority: route.priority,
		})
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// changeToBatch converts a locrib.Change into the BestChangeBatch shape
// sysrib's processEvent consumes. One Change -> one single-entry batch.
// Returns nil for unspecified / unrecognized ChangeKind.
func changeToBatch(c locrib.Change) *incomingBatch {
	var action bgptypes.RouteAction
	switch c.Kind {
	case locrib.ChangeAdd:
		action = ribevents.BestChangeAdd
	case locrib.ChangeUpdate:
		action = ribevents.BestChangeUpdate
	case locrib.ChangeRemove:
		action = ribevents.BestChangeWithdraw
	case locrib.ChangeUnspecified:
		return nil
	default:
		return nil
	}
	var nextHop netip.Addr
	var priority int
	var metric uint32
	if c.Kind != locrib.ChangeRemove {
		nextHop = c.Best.NextHop
		priority = int(c.Best.AdminDistance)
		metric = c.Best.Metric
	}
	return &incomingBatch{
		Protocol: redistevents.ProtocolName(c.Best.Source),
		Family:   c.Family,
		Changes: []incomingChange{{
			Action:       action,
			Prefix:       c.Prefix,
			NextHop:      nextHop,
			Priority:     priority,
			Metric:       metric,
			ProtocolType: bgpProtocolTypeFromPath(c.Best),
		}},
	}
}

// replayPath seeds sysrib with an already-present best from locrib at startup.
// Runs the change through processEvent as a synthetic Add so admin-distance
// overrides and downstream emission work the same as any live change.
func (s *sysRIB) replayPath(fam family.Family, pfx netip.Prefix, p locrib.Path) {
	batch := changeToBatch(locrib.Change{
		Family: fam,
		Prefix: pfx,
		Kind:   locrib.ChangeAdd,
		Best:   p,
	})
	if batch == nil {
		return
	}
	famStr, changes := s.processEvent(batch)
	if len(changes) > 0 {
		publishChanges(changes, famStr)
	}
}

// bgpProtocolTypeFromPath derives the BGP protocol type for a locrib Path.
// Only BGP paths produce a meaningful result; non-BGP sources return
// BGPProtocolUnspecified (the caller uses the batch-level protocol name
// for admin-distance lookup in that case).
func bgpProtocolTypeFromPath(p locrib.Path) bgptypes.BGPProtocolType {
	name := redistevents.ProtocolName(p.Source)
	if name != "bgp" {
		return bgptypes.BGPProtocolUnspecified
	}
	switch p.AdminDistance {
	case 20:
		return bgptypes.BGPProtocolEBGP
	case 200:
		return bgptypes.BGPProtocolIBGP
	}
	return bgptypes.BGPProtocolUnspecified
}
