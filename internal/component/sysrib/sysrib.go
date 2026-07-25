// Design: docs/architecture/core-design.md -- System RIB plugin
// Related: nhresolver.go -- recursive next-hop resolution using Loc-RIB LPM
// Related: ecmp.go -- ECMP path collection from equal-cost protocol routes
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
	"slices"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/rib/igpcost"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

// sysribMetrics holds Prometheus metrics for the system RIB plugin.
//
// routeChanges pre-binds one Counter per routeaction.Action at init time;
// the hot path does `m.routeChanges[c.Action].Inc()`, a zero-allocation
// array index. The underlying CounterVec still emits one time series per
// action label to Prometheus exposition.
type sysribMetrics struct {
	routesBest     metrics.Gauge
	routeChanges   [routeaction.Count]metrics.Counter
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
	for _, a := range [...]routeaction.Action{
		routeaction.Add,
		routeaction.Update,
		routeaction.Withdraw,
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

// locRIBPtr stores the shared cross-protocol Loc-RIB.
var locRIBPtr atomic.Pointer[locrib.RIB]

// nhResolverPtr stores the NH resolver, created when a Loc-RIB is wired.
var nhResolverPtr atomic.Pointer[nhResolver]

// SetLocRIB wires the shared Loc-RIB and creates the NH resolver.
func SetLocRIB(r *locrib.RIB) {
	locRIBPtr.Store(r)
	if r != nil {
		resolver := newNHResolver(r)
		nhResolverPtr.Store(resolver)
		igpcost.Set(resolver.IGPMetric)
	}
}

func getLocRIB() *locrib.RIB { return locRIBPtr.Load() }

func getNHResolver() *nhResolver { return nhResolverPtr.Load() }

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
	labels           []uint32   // MPLS label stack (nil for unlabeled routes)
	srv6SID          netip.Addr // SRv6 SID from PrefixSID attribute (zero if absent)

	// backupNextHop and backupLabels are the fast-reroute alternate for this
	// route's primary next-hop (an IP FRR backup + optional MPLS repair stack).
	// They ride the winner into BestChangeEntry.Backup as a DEDICATED backup
	// next-hop, never folded into the ECMP group.
	backupNextHop netip.Addr
	backupLabels  []uint32

	// ecmpNextHops are INTRA-protocol equal-cost sibling next-hops for this
	// prefix from the SAME protocol source, excluding nextHop (the winner).
	//
	// A Loc-RIB Change carries only the single best Path (locrib.Change.Best), so
	// a protocol that inserts one locrib.Path per equal-cost next-hop (IS-IS ECMP,
	// distinct Instance) would collapse to a single next-hop here, since
	// s.routes[key] is keyed by protocol string. This field is the committed
	// path-group expansion (isis-9, umbrella A-2): on the Loc-RIB ingest path,
	// sysrib reads the full PathGroup and records the winner's equal-cost siblings
	// here, so ecmpCollect surfaces them in BestChangeEntry.ECMPPaths and the
	// kernel installs a multipath route. Empty for single-Path prefixes and on the
	// forked EventBus path (no shared Loc-RIB), so existing sources are unaffected.
	ecmpNextHops []netip.Addr
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
	// lastECMP tracks the last emitted ECMP path set per prefix for
	// suppressing duplicate emissions when only ECMP membership changes.
	lastECMP map[prefixKey][]sysribevents.ECMPPath
	// resolvedNH tracks the last emitted resolved next-hop per prefix.
	// Used by the cascade worker to detect resolution changes. A valid
	// address means the route is FIB-installed; absence means the NH was
	// unreachable and the route is FIB-withdrawn (but still RIB-present).
	resolvedNH map[prefixKey]netip.Addr
	// adminDist maps protocol type (e.g., "ebgp", "ibgp", "static") to
	// configured admin distance. Empty when no sysrib config is present,
	// in which case incoming priorities pass through unchanged.
	adminDist map[string]int
	mu        sync.RWMutex
}

func newSysRIB() *sysRIB {
	return &sysRIB{
		routes:     make(map[prefixKey]map[string]*protocolRoute),
		best:       make(map[prefixKey]*protocolRoute),
		lastECMP:   make(map[prefixKey][]sysribevents.ECMPPath),
		resolvedNH: make(map[prefixKey]netip.Addr),
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
	if fam == (family.Family{}) {
		logger().Warn("sysrib: event missing family")
		return family.Family{}, nil
	}

	if m := sysribMetricsPtr.Load(); m != nil {
		m.eventsReceived.Inc()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var outChanges []outgoingChange

	for i := range batch.Changes {
		c := batch.Changes[i]
		if !c.Prefix.IsValid() {
			logger().Warn("sysrib: skipping change with empty prefix")
			continue
		}
		if c.Action != routeaction.Add && c.Action != routeaction.Update && c.Action != routeaction.Withdraw {
			logger().Warn("sysrib: unrecognized action", "action", c.Action, "prefix", c.Prefix)
			continue
		}

		key := prefixKey{family: fam, prefix: c.Prefix}

		if c.Action == routeaction.Add || c.Action == routeaction.Update {
			if proto == "" {
				logger().Warn("sysrib: event missing protocol", "prefix", c.Prefix)
				continue
			}
			// Use per-change protocol type for admin distance override.
			// Falls back to batch-level protocol if per-change type is absent.
			protoType := c.ProtocolType.String()
			if c.ProtocolType == routeaction.ProtocolUnspecified {
				protoType = proto
			}
			priority := s.effectivePriority(protoType, c.Priority)

			// Loc-RIB vs event-bus storage (see the gated store after this literal).
			// A unified Loc-RIB has already arbitrated across every source and emits
			// exactly ONE authoritative best per prefix, so a Loc-RIB-sourced change
			// REPLACES the whole per-prefix entry: a best switching from protocol A to
			// B drops A's now-stale slot. That was the ghost-entry -- A used to linger
			// until its own withdraw and could wrongly win recomputeBest after an
			// admin-distance reconfig. Intra-protocol ECMP siblings ride on
			// pr.ecmpNextHops (not separate map entries), so a single slot preserves
			// ECMP. The event-bus fallback (each protocol emits independently) keeps
			// the per-protocol upsert so its cross-protocol admin-distance arbitration
			// still works.
			pr := &protocolRoute{
				protocol:         proto,
				protocolType:     protoType,
				nextHop:          c.NextHop,
				priority:         priority,
				incomingPriority: c.Priority,
				metric:           c.Metric,
				labels:           c.Labels,
				srv6SID:          c.SRv6SID,
				backupNextHop:    c.BackupNextHop,
				backupLabels:     c.BackupRepairLabels,
				// Intra-protocol equal-cost siblings (isis-9 ECMP, umbrella A-2)
				// are now carried on the Loc-RIB Change (computed at emit while the
				// PathGroup is in hand under the shard lock), so there is no
				// per-change loc.Lookup here. Nil on the forked EventBus path and
				// for single-Path prefixes.
				ecmpNextHops: c.ECMPNextHops,
			}
			if batch.FromLocRIB {
				// Loc-RIB authoritative single best: replace the whole per-prefix
				// entry so a prior best from a different protocol cannot linger as a
				// ghost and wrongly win recomputeBest after an admin-distance change.
				s.routes[key] = map[string]*protocolRoute{proto: pr}
			} else {
				if s.routes[key] == nil {
					s.routes[key] = make(map[string]*protocolRoute)
				}
				s.routes[key][proto] = pr
			}
		} else if c.Action == routeaction.Withdraw {
			if proto == "" {
				delete(s.routes, key)
			} else if s.routes[key] != nil {
				delete(s.routes[key], proto)
				if len(s.routes[key]) == 0 {
					delete(s.routes, key)
				}
			}
		}

		if change := s.recomputeBest(key); change != nil {
			outChanges = append(outChanges, *change)
		}
	}

	if m := sysribMetricsPtr.Load(); m != nil {
		for i := range outChanges {
			if ctr := m.routeChanges[outChanges[i].Action]; ctr != nil {
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
			for i := range changes {
				if ctr := m.routeChanges[changes[i].Action]; ctr != nil {
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
			delete(s.lastECMP, key)
			delete(s.resolvedNH, key)
			if r := getNHResolver(); r != nil {
				if prev.nextHop.IsValid() {
					r.Untrack(prev.nextHop, key.prefix)
				}
				if prev.srv6SID.IsValid() {
					r.Untrack(prev.srv6SID, key.prefix)
				}
			}
			return &outgoingChange{
				Action: routeaction.Withdraw,
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
		ecmpPaths := ecmpCollect(protocols, winner)
		s.lastECMP[key] = ecmpPaths
		resolved := resolveNextHop(winner.nextHop)
		if r := getNHResolver(); r != nil {
			if winner.nextHop.IsValid() {
				r.Track(winner.nextHop, key.prefix)
			}
			if winner.srv6SID.IsValid() {
				r.Track(winner.srv6SID, key.prefix)
				if !srv6SIDResolvable(winner.srv6SID) {
					return nil
				}
			}
		}
		s.resolvedNH[key] = resolved
		return &outgoingChange{
			Action:    routeaction.Add,
			Prefix:    key.prefix,
			NextHop:   resolved,
			Protocol:  winner.protocol,
			Labels:    winner.labels,
			SRv6SID:   winner.srv6SID,
			Metric:    winner.metric,
			ECMPPaths: ecmpPaths,
			Backup:    backupPaths(winner),
		}
	}

	ecmpPaths := ecmpCollect(protocols, winner)
	if prev.protocol == winner.protocol && prev.nextHop == winner.nextHop &&
		prev.priority == winner.priority && prev.metric == winner.metric &&
		prev.srv6SID == winner.srv6SID &&
		labelsEqual(prev.labels, winner.labels) && !ecmpChanged(s.lastECMP[key], ecmpPaths) {
		s.best[key] = winner
		return nil
	}

	if r := getNHResolver(); r != nil {
		if prev.nextHop.IsValid() && prev.nextHop != winner.nextHop {
			r.Untrack(prev.nextHop, key.prefix)
		}
		if winner.nextHop.IsValid() && winner.nextHop != prev.nextHop {
			r.Track(winner.nextHop, key.prefix)
		}
		if prev.srv6SID.IsValid() && prev.srv6SID != winner.srv6SID {
			r.Untrack(prev.srv6SID, key.prefix)
		}
		if winner.srv6SID.IsValid() && winner.srv6SID != prev.srv6SID {
			r.Track(winner.srv6SID, key.prefix)
		}
		if winner.srv6SID.IsValid() && !srv6SIDResolvable(winner.srv6SID) {
			s.best[key] = winner
			s.lastECMP[key] = ecmpPaths
			delete(s.resolvedNH, key)
			return &outgoingChange{
				Action: routeaction.Withdraw,
				Prefix: key.prefix,
			}
		}
	}

	resolved := resolveNextHop(winner.nextHop)
	s.best[key] = winner
	s.lastECMP[key] = ecmpPaths
	s.resolvedNH[key] = resolved
	return &outgoingChange{
		Action:    routeaction.Update,
		Prefix:    key.prefix,
		NextHop:   resolved,
		Protocol:  winner.protocol,
		Labels:    winner.labels,
		SRv6SID:   winner.srv6SID,
		Metric:    winner.metric,
		ECMPPaths: ecmpPaths,
		Backup:    backupPaths(winner),
	}
}

// resolveNextHop attempts recursive resolution of nh via the NH resolver.
// Returns the directly-connected next-hop if resolution succeeds, or nh unchanged.
// Lock ordering: callers hold sysRIB.mu; the resolver acquires Loc-RIB
// shard read locks internally. The Loc-RIB OnChange handler queues to a
// channel (run's worker goroutine) to avoid acquiring sysRIB.mu under the
// shard write lock, which would deadlock with this LPM call.
func resolveNextHop(nh netip.Addr) netip.Addr {
	r := getNHResolver()
	if r == nil || !nh.IsValid() {
		return nh
	}
	res := r.Resolve(nh)
	if res.Resolved {
		return res.DirectNH
	}
	return nh
}

// srv6SIDResolvable checks whether an SRv6 SID has a covering route in the
// Loc-RIB per RFC 9252 Section 5 resolvability requirement.
func srv6SIDResolvable(sid netip.Addr) bool {
	r := getNHResolver()
	if r == nil {
		return true // no resolver configured: permissive
	}
	return r.Resolve(sid).Resolved
}

func labelsEqual(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// cascadeRecompute re-resolves the NH for a prefix whose covering route
// changed. Returns an outgoing change if the resolved NH differs from
// the last emitted state, nil otherwise. Caller MUST hold s.mu.
func (s *sysRIB) cascadeRecompute(key prefixKey) *outgoingChange {
	best := s.best[key]
	if best == nil || !best.nextHop.IsValid() {
		return nil
	}

	r := getNHResolver()
	if r == nil {
		return nil
	}

	prevResolved := s.resolvedNH[key]
	res := r.Resolve(best.nextHop)
	protocols := s.routes[key]

	if !res.Resolved {
		// Winner's NH is unreachable. Check for reachable ECMP members.
		ecmpPaths := ecmpCollectResolved(protocols, best, r)
		if len(ecmpPaths) > 0 {
			promoted := ecmpPaths[0].NextHop
			remaining := ecmpPaths[1:]
			if promoted == prevResolved && !ecmpChanged(s.lastECMP[key], remaining) {
				return nil
			}
			s.resolvedNH[key] = promoted
			s.lastECMP[key] = remaining
			action := routeaction.Update
			if !prevResolved.IsValid() {
				action = routeaction.Add
			}
			return &outgoingChange{
				Action:    action,
				Prefix:    key.prefix,
				NextHop:   promoted,
				Protocol:  best.protocol,
				Labels:    best.labels,
				SRv6SID:   best.srv6SID,
				Metric:    best.metric,
				ECMPPaths: remaining,
				Backup:    backupPaths(best),
			}
		}
		if prevResolved.IsValid() {
			delete(s.resolvedNH, key)
			delete(s.lastECMP, key)
			return &outgoingChange{
				Action: routeaction.Withdraw,
				Prefix: key.prefix,
			}
		}
		return nil
	}

	// RFC 9252 Section 5: SRv6 SID must be resolvable.
	if best.srv6SID.IsValid() && !srv6SIDResolvable(best.srv6SID) {
		if prevResolved.IsValid() {
			delete(s.resolvedNH, key)
			delete(s.lastECMP, key)
			return &outgoingChange{
				Action: routeaction.Withdraw,
				Prefix: key.prefix,
			}
		}
		return nil
	}

	newResolved := res.DirectNH
	ecmpPaths := ecmpCollectResolved(protocols, best, r)
	if newResolved == prevResolved && !ecmpChanged(s.lastECMP[key], ecmpPaths) {
		return nil
	}

	s.resolvedNH[key] = newResolved
	s.lastECMP[key] = ecmpPaths
	action := routeaction.Update
	if !prevResolved.IsValid() {
		action = routeaction.Add
	}
	return &outgoingChange{
		Action:    action,
		Prefix:    key.prefix,
		NextHop:   newResolved,
		Protocol:  best.protocol,
		Labels:    best.labels,
		SRv6SID:   best.srv6SID,
		Metric:    best.metric,
		ECMPPaths: ecmpPaths,
		Backup:    backupPaths(best),
	}
}

// ecmpCollectResolved collects ECMP paths with resolved NHs, filtering
// out members whose NHs are unreachable.
func ecmpCollectResolved(protocols map[string]*protocolRoute, winner *protocolRoute, r *nhResolver) []sysribevents.ECMPPath {
	var paths []sysribevents.ECMPPath
	for _, route := range protocols {
		if route == winner {
			continue
		}
		if route.priority != winner.priority || route.metric != winner.metric {
			continue
		}
		if !route.nextHop.IsValid() {
			continue
		}
		memberRes := r.Resolve(route.nextHop)
		if !memberRes.Resolved {
			continue
		}
		paths = append(paths, sysribevents.ECMPPath{
			NextHop: memberRes.DirectNH,
			Weight:  1,
			Labels:  route.labels,
		})
	}
	// Intra-protocol equal-cost siblings of the winner (Loc-RIB path-group
	// expansion, isis-9), filtered to those whose next-hop resolves.
	for _, nh := range winner.ecmpNextHops {
		if nh == winner.nextHop || !nh.IsValid() {
			continue
		}
		memberRes := r.Resolve(nh)
		if !memberRes.Resolved {
			continue
		}
		paths = append(paths, sysribevents.ECMPPath{
			NextHop: memberRes.DirectNH,
			Weight:  1,
			Labels:  winner.labels,
		})
	}
	if len(paths) == 0 {
		return nil
	}
	slices.SortFunc(paths, func(a, b sysribevents.ECMPPath) int {
		return a.NextHop.Compare(b.NextHop)
	})
	paths = dedupECMP(paths)
	if len(paths) > sysribevents.MaxECMPPaths-1 {
		paths = paths[:sysribevents.MaxECMPPaths-1]
	}
	return paths
}

// processCascade re-evaluates all prefixes that depend on the given NHs.
// Handles multi-level cascades: if a re-evaluated prefix itself covers
// other tracked NHs, those dependents are also re-evaluated.
func (s *sysRIB) processCascade(nhs []netip.Addr) {
	r := getNHResolver()
	if r == nil {
		return
	}

	seen := make(map[prefixKey]bool)
	var workList []prefixKey
	for _, nh := range nhs {
		for _, dep := range r.Dependents(nh) {
			key := prefixKey{family: familyForPrefix(dep), prefix: dep}
			if !seen[key] {
				seen[key] = true
				workList = append(workList, key)
			}
		}
	}

	s.mu.Lock()
	changesByFamily := make(map[family.Family][]outgoingChange)
	for len(workList) > 0 {
		key := workList[0]
		workList = workList[1:]
		change := s.cascadeRecompute(key)
		if change == nil {
			continue
		}
		changesByFamily[key.family] = append(changesByFamily[key.family], *change)
		for _, nh := range r.CoveredNHs(key.prefix) {
			for _, dep := range r.Dependents(nh) {
				k := prefixKey{family: familyForPrefix(dep), prefix: dep}
				if !seen[k] {
					seen[k] = true
					workList = append(workList, k)
				}
			}
		}
	}
	s.mu.Unlock()

	for fam, changes := range changesByFamily {
		publishChanges(changes, fam)
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

// replayBest publishes the current system best table as batch events. Used for
// full-table replay when a downstream subscriber (e.g. a FIB backend) requests
// it. This hop is broadcast, so the request's token is ignored except to stamp
// it onto the batches (replay.Broadcast), which makes IsReplay() report true.
func (s *sysRIB) replayBest(req *replay.Request) {
	eb := getEventBus()
	if eb == nil {
		return
	}

	s.mu.RLock()
	changesByFamily := make(map[family.Family][]outgoingChange)
	for key, route := range s.best {
		// RFC 9252 Section 5: skip routes with unresolvable SRv6 SIDs.
		if route.srv6SID.IsValid() && !srv6SIDResolvable(route.srv6SID) {
			continue
		}
		changesByFamily[key.family] = append(changesByFamily[key.family], outgoingChange{
			Action:    routeaction.Add,
			Prefix:    key.prefix,
			NextHop:   resolveNextHop(route.nextHop),
			Protocol:  route.protocol,
			Labels:    route.labels,
			SRv6SID:   route.srv6SID,
			Metric:    route.metric,
			ECMPPaths: s.lastECMP[key],
			Backup:    backupPaths(route),
		})
	}
	s.mu.RUnlock()

	for famName, changes := range changesByFamily {
		batch := &outgoingBatch{
			Family:   famName,
			ReplayID: req.ReplayID,
			Changes:  changes,
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
//
// Loc-RIB OnChange handlers run under the shard write lock. To avoid
// deadlock (processEvent -> resolveNextHop -> LPM re-locks the same
// shard), the handler queues changes to a channel and a separate
// worker goroutine processes them outside the lock. The cascade
// (re-evaluating dependent prefixes when a covering route changes) is
// handled inline in the same worker.
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

		changeCh := make(chan locrib.Change, 4096)
		unsubBest = loc.OnChange(func(c locrib.Change) {
			select {
			case changeCh <- c:
			default: // channel full: bounded, overflow logged
				logger().Warn("sysrib: change channel full, dropping event", "prefix", c.Prefix)
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
				s.replayPath(fam, pfx, g.Paths[g.Best], g.ECMPNextHops(g.Paths[g.Best]))
				return true
			})
		}

		// Single long-lived worker: processes Loc-RIB changes and
		// cascades outside the shard lock.
		var workerWG sync.WaitGroup
		workerWG.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case c := <-changeCh:
					s.processLocRIBChange(c)
				}
			}
		})
		defer workerWG.Wait()
	} else {
		unsubBest = ribevents.BestChange.Subscribe(eb, func(batch *incomingBatch) {
			fam, changes := s.processEvent(batch)
			if len(changes) > 0 {
				publishChanges(changes, fam)
			}
		})
		// Broadcast hop: ask the BGP RIB to replay its whole best-path table.
		if _, err := ribevents.ReplayRequest.Emit(eb, &replay.Request{ReplayID: replay.Broadcast}); err != nil {
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

// processLocRIBChange handles a single Loc-RIB change: converts it to
// the internal batch shape, runs admin-distance arbitration, publishes
// downstream, and triggers NH cascade if the changed prefix covers any
// tracked next-hops.
func (s *sysRIB) processLocRIBChange(c locrib.Change) {
	batch := changeToBatch(c)
	if batch == nil {
		return
	}
	fam, changes := s.processEvent(batch)
	if len(changes) > 0 {
		publishChanges(changes, fam)
	}
	if r := getNHResolver(); r != nil {
		if nhs := r.CoveredNHs(c.Prefix); len(nhs) > 0 {
			s.processCascade(nhs)
		}
	}
}

// showNHTable returns the NH resolver tracking table as JSON.
func (s *sysRIB) showNHTable() (any, error) {
	r := getNHResolver()
	if r == nil {
		// Marshaled once by the SDK: return an empty slice, not a JSON
		// string literal (which would double-encode on the wire).
		return []any{}, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	type nhEntry struct {
		NextHop    netip.Addr     `json:"next-hop"`
		Resolved   bool           `json:"resolved"`
		DirectNH   netip.Addr     `json:"direct-nh,omitzero"`
		IGPMetric  uint32         `json:"igp-metric,omitempty"`
		Dependents []netip.Prefix `json:"dependents"`
	}

	entries := make([]nhEntry, 0, len(r.tracking))
	for nh, deps := range r.tracking {
		res := r.Resolve(nh)
		prefixes := make([]netip.Prefix, 0, len(deps))
		for pfx := range deps {
			prefixes = append(prefixes, pfx)
		}
		entries = append(entries, nhEntry{
			NextHop:    nh,
			Resolved:   res.Resolved,
			DirectNH:   res.DirectNH,
			IGPMetric:  res.Metric,
			Dependents: prefixes,
		})
	}

	return entries, nil
}

// showECMPGroups returns the current ECMP groups as JSON.
func (s *sysRIB) showECMPGroups() (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type ecmpEntry struct {
		Prefix netip.Prefix            `json:"prefix"`
		Family family.Family           `json:"family"`
		Paths  []sysribevents.ECMPPath `json:"paths"`
	}

	var entries []ecmpEntry
	for key, paths := range s.lastECMP {
		if len(paths) == 0 {
			continue
		}
		entries = append(entries, ecmpEntry{
			Prefix: key.prefix,
			Family: key.family,
			Paths:  paths,
		})
	}

	return entries, nil
}

// showRIB returns the current system RIB state as JSON.
func (s *sysRIB) showRIB() (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type entry struct {
		Prefix    netip.Prefix            `json:"prefix"`
		Family    family.Family           `json:"family"`
		NextHop   netip.Addr              `json:"next-hop,omitzero"`
		Protocol  string                  `json:"protocol"`
		Priority  int                     `json:"priority"`
		ECMPPaths []sysribevents.ECMPPath `json:"ecmp-paths,omitempty"`
	}

	entries := make([]entry, 0, len(s.best))
	for key, route := range s.best {
		entries = append(entries, entry{
			Prefix:    key.prefix,
			Family:    key.family,
			NextHop:   route.nextHop,
			Protocol:  route.protocol,
			Priority:  route.priority,
			ECMPPaths: s.lastECMP[key],
		})
	}

	return entries, nil
}

// changeToBatch converts a locrib.Change into the BestChangeBatch shape
// sysrib's processEvent consumes. One Change -> one single-entry batch.
// Returns nil for unspecified / unrecognized ChangeKind.
func changeToBatch(c locrib.Change) *incomingBatch {
	var action routeaction.Action
	switch c.Kind {
	case locrib.ChangeAdd:
		action = ribevents.BestChangeAdd
	case locrib.ChangeUpdate:
		action = ribevents.BestChangeUpdate
	case locrib.ChangeRemove:
		// INVARIANT (locrib/change.go ChangeRemove + Best doc): ChangeRemove fires
		// ONLY when the last valid path for the prefix goes away, i.e. the Loc-RIB
		// PathGroup is fully empty, and Best is the zero Path. Because Best is zero,
		// the Protocol field below resolves to ProtocolName(0) == "" -- the
		// empty-string sentinel -- which makes processEvent delete EVERY protocol
		// entry for the prefix (the proto=="" branch). That all-protocols delete is
		// correct precisely because the PathGroup is empty: there is no surviving
		// protocol whose sysrib entry we would wrongly drop. If locrib ever started
		// emitting ChangeRemove on a partial withdraw (PathGroup non-empty), this
		// would over-delete; the assertion below catches that contract break.
		action = ribevents.BestChangeWithdraw
	case locrib.ChangeUnspecified:
		return nil
	default:
		return nil
	}
	var nextHop netip.Addr
	var priority int
	var metric uint32
	var labels []uint32
	if c.Kind != locrib.ChangeRemove {
		nextHop = c.Best.NextHop
		priority = int(c.Best.AdminDistance)
		metric = c.Best.Metric
		// Carry the MPLS label stack so labeled-unicast routes program a kernel
		// MPLS push entry rather than a plain IP route (the Loc-RIB now retains
		// Labels; without this they were dropped here).
		labels = c.Best.Labels
	}
	protocol := redistevents.ProtocolName(c.Best.Source)
	// Assert the ChangeRemove invariant cheaply: a Remove MUST carry the zero Path
	// (empty protocol sentinel) so processEvent's proto=="" branch deletes every
	// protocol entry for an already-empty PathGroup. A non-empty protocol on Remove
	// means locrib broke the contract and would cause a per-protocol delete instead;
	// log it rather than silently mis-program the FIB. Diagnostic-only; does not
	// change the produced batch.
	if c.Kind == locrib.ChangeRemove && protocol != "" {
		logger().Warn("sysrib: ChangeRemove carried a non-empty protocol; locrib invariant broken",
			"prefix", c.Prefix, "protocol", protocol)
	}
	return &incomingBatch{
		Protocol:   protocol,
		Family:     c.Family,
		FromLocRIB: true,
		Changes: []incomingChange{{
			Action:       action,
			Prefix:       c.Prefix,
			NextHop:      nextHop,
			Priority:     priority,
			Metric:       metric,
			Labels:       labels,
			ProtocolType: bgpProtocolTypeFromPath(c.Best),
			// Intra-source equal-cost siblings computed at Loc-RIB emit; sysrib
			// builds the ECMP group from these instead of re-looking-up the RIB.
			// Always nil on ChangeRemove (locrib leaves Change.ECMP nil there).
			ECMPNextHops: c.ECMP,
			// Fast-reroute backup (carry-through, never an ECMP sibling). Zero on
			// ChangeRemove because c.Best is the zero Path there.
			BackupNextHop:      c.Best.BackupNextHop,
			BackupRepairLabels: c.Best.BackupRepairLabels,
		}},
	}
}

// replayPath seeds sysrib with an already-present best from locrib at startup.
// Runs the change through processEvent as a synthetic Add so admin-distance
// overrides and downstream emission work the same as any live change. ECMP is
// supplied from the PathGroup snapshot so pre-existing multipath groups do not
// collapse to the primary next-hop on replay.
func (s *sysRIB) replayPath(fam family.Family, pfx netip.Prefix, p locrib.Path, ecmp []netip.Addr) {
	batch := changeToBatch(locrib.Change{
		Family: fam,
		Prefix: pfx,
		Kind:   locrib.ChangeAdd,
		Best:   p,
		ECMP:   ecmp,
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
func bgpProtocolTypeFromPath(p locrib.Path) routeaction.ProtocolType {
	name := redistevents.ProtocolName(p.Source)
	if name != "bgp" {
		return routeaction.ProtocolUnspecified
	}
	// Read the producer's eBGP/iBGP classification directly. Deriving it from
	// AdminDistance (20/200) silently lost the class whenever the operator
	// overrode bgp/admin-distance, making this replay path disagree with the
	// live event-bus ProtocolType. The BGP RIB sets Path.IsEBGP from the peer
	// ASN relationship; mirror its 2-state resolve() (iBGP unless eBGP).
	if p.IsEBGP {
		return routeaction.ProtocolEBGP
	}
	return routeaction.ProtocolIBGP
}
