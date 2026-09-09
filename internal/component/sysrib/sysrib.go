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
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/rib/igpcost"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/component/config"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/nexthop"
	"github.com/ze-software/ze/internal/core/rib/routetype"
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

// SetLocRIB wires the shared Loc-RIB and creates the NH resolver over it.
//
// A nil RIB removes both. Leaving the resolver in place would keep answering
// reachability from a Loc-RIB nothing writes to any more: every next-hop would
// resolve to unreachable, sysrib would withdraw the routes that depend on them,
// and getNHResolver would report a resolver to code that asks whether one is
// wired at all.
func SetLocRIB(r *locrib.RIB) {
	locRIBPtr.Store(r)
	if r == nil {
		nhResolverPtr.Store(nil)
		igpcost.Set(nil)
		return
	}

	resolver := newNHResolver(r)
	nhResolverPtr.Store(resolver)
	igpcost.Set(resolver.IGPMetric)
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
	nextHopInterface string // outgoing device for nextHop, empty for a gateway-only next-hop
	nextHopWeight    uint8  // nextHop's share of the multipath group, 0 for an unweighted one
	priority         int    // effective admin distance (lower wins)
	incomingPriority int    // original priority from protocol RIB (before override)
	metric           uint32
	labels           []uint32   // MPLS label stack (nil for unlabeled routes)
	srv6SID          netip.Addr // SRv6 SID from PrefixSID attribute (zero if absent)

	// routeType is the forwarding action the FIB programs: an ordinary next-hop,
	// or a discard. Unset means the protocol has no opinion and the FIB installs
	// an ordinary route. It rides the winner into BestChangeEntry.RouteType.
	//
	// It is NOT part of arbitration. recomputeBest still selects on priority,
	// then protocol name, so a discard route never beats a forwarding one on the
	// strength of being a discard. It IS part of the change comparison, because
	// a prefix that turns into a discard with every other field unchanged must
	// still reach the kernel.
	routeType routetype.Type

	// backupNextHop and backupLabels are the fast-reroute alternate for this
	// route's primary next-hop (an IP FRR backup + optional MPLS repair stack).
	// They ride the winner into BestChangeEntry.Backup as a DEDICATED backup
	// next-hop, never folded into the ECMP group.
	backupNextHop netip.Addr
	backupLabels  []uint32

	// osInstalled is true when the OPERATING SYSTEM creates this protocol's
	// forwarding entry, so Ze MUST NOT program one of its own. The protocol
	// DECLARES it (redistevents.RegisterOSInstalled) and this is the answer read
	// back by ID, never a guess from an invalid next-hop or a protocol name.
	//
	// It is not part of arbitration: an OS-installed route wins or loses on its
	// administrative distance like any other. What it changes is what winning
	// MEANS, in recordOSInstalledWinner.
	osInstalled bool

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
	ecmpNextHops []nexthop.NextHop
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
	// Used by the cascade worker to detect resolution changes. PRESENCE of the
	// key is what says Ze has an install outstanding for the prefix, and
	// programmedByZe is the one test: the ADDRESS is invalid for a route
	// reached over a device alone, which no validity test can tell from a
	// prefix Ze never programmed. A prefix is absent when the OS owns it, when
	// its next-hop stopped resolving, or when the operator withholds the
	// protocol that holds it; the route stays in the system RIB in each case.
	resolvedNH map[prefixKey]netip.Addr
	// adminDist maps protocol type (e.g., "ebgp", "ibgp", "static") to its
	// administrative distance. parseAdminDistanceConfig returns every protocol
	// the schema declares, so this is complete once configure has run, and is
	// nil only before it.
	adminDist map[string]int
	// distanceSpoken names the protocols this process has already reported an
	// unresolved distance for. A route arriving before configure, or a protocol
	// the schema does not declare, is worth ONE line each: without the set it
	// would be one line per route, and with no line at all the value that
	// decides what the kernel installs would change in silence
	// (ai/rules/principles.md).
	distanceSpoken map[string]bool
	mu             sync.RWMutex

	// fibMu guards the two fields below. It is a leaf lock: a holder MUST NOT
	// acquire mu, and every caller acquires it last. It is separate from mu
	// because fibPermitted WRITES the log-once set on every read of the
	// permission, so a caller holding mu in either mode can ask the question
	// without the answer needing mu's write side.
	fibMu sync.Mutex
	// fibPermit maps a protocol name to whether its routes are written to the
	// FIB. parseFIBImportConfig returns every protocol the registry holds, so
	// this is complete once configure has run, and is nil only before it.
	fibPermit map[string]bool
	// fibSpoken names the protocols this process has already reported as absent
	// from the permission set, for distanceSpoken's reason: one line for each,
	// rather than one per route or none at all.
	fibSpoken map[string]bool
}

func newSysRIB() *sysRIB {
	return &sysRIB{
		distanceSpoken: make(map[string]bool),
		fibSpoken:      make(map[string]bool),
		routes:         make(map[prefixKey]map[string]*protocolRoute),
		best:           make(map[prefixKey]*protocolRoute),
		lastECMP:       make(map[prefixKey][]sysribevents.ECMPPath),
		resolvedNH:     make(map[prefixKey]netip.Addr),
	}
}

// programmedByZe reports whether Ze has an install outstanding for a prefix:
// it emitted an Add or an Update for it and has emitted no Withdraw since.
//
// The test is the PRESENCE of the key, never the validity of the address it
// holds. A route reached over a device alone resolves to an invalid address,
// and Ze programs it: `static { route { forward { interface X } } }` is a
// forwarding entry like any other. Reading the address would file every such
// route under "Ze programmed nothing", which leaves it in the kernel when
// something later takes the prefix away from it.
//
// REQUIRES: the caller holds s.mu.
func (s *sysRIB) programmedByZe(key prefixKey) bool {
	_, programmed := s.resolvedNH[key]
	return programmed
}

// adminDistanceSchemaPath is where the one declaration of every protocol's
// administrative distance lives. The YANG carries the numbers because it has to
// anyway, for validation, for the editor's completion and for the generated
// config reference; a Go copy beside it would be the second declaration
// (ai/rules/principles.md).
const adminDistanceSchemaPath = "rib/distance"

// parseAdminDistanceConfig returns every protocol's administrative distance:
// the operator's value where one is written, and the schema's declared value
// everywhere else.
//
// The map is COMPLETE whether or not the operator wrote the block, and that is
// the point of it. It used to come back empty for a config with no `rib`
// section, and effectivePriority read that emptiness as permission to trust
// whatever the producing protocol had stamped. So which declaration decided
// depended on whether a block written for some other protocol happened to
// exist, and no log line marked the switch.
//
// A YANG `default` does not arrive on its own. config.ApplyDefaults is what
// puts one into a tree, and the peer path is the only other caller
// (applyPeerSchemaDefaults, internal/component/bgp/config/peers.go), so a
// consumer reading a config section directly sees the leaf simply missing.
//
// The protocol set is DISCOVERED from the schema rather than listed here, so a
// protocol added to the YANG gets a distance with no edit to this file.
//
// An unreadable schema is an ERROR rather than an empty map. This value decides
// which route the kernel installs, so refusing to configure is the safe
// direction and guessing is not: a silent empty map is the defect this function
// was rewritten to remove, and reintroducing it here would move it rather than
// fix it. applyPeerSchemaDefaults swallows the same two errors; do not copy that.
func parseAdminDistanceConfig(jsonData string) (map[string]int, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonData), &tree); err != nil {
		return nil, fmt.Errorf("unmarshal sysrib config: %w", err)
	}

	declared := map[string]any{}
	if sysribTree, ok := tree["rib"].(map[string]any); ok {
		if adTree, ok := sysribTree["distance"].(map[string]any); ok {
			declared = adTree
		}
	}

	schema, err := config.YANGSchema()
	if err != nil {
		return nil, fmt.Errorf("distance: load schema: %w", err)
	}
	node, err := schema.Lookup(adminDistanceSchemaPath)
	if err != nil {
		return nil, fmt.Errorf("distance: resolve %s: %w", adminDistanceSchemaPath, err)
	}
	config.ApplyDefaults(declared, node)

	result := make(map[string]int, len(declared))
	for proto, v := range declared {
		num, err := adminDistanceValue(v)
		if err != nil {
			return nil, fmt.Errorf("distance %s: %w", proto, err)
		}
		result[proto] = num
	}

	return result, nil
}

// adminDistanceValue reads the three shapes a distance arrives in: a JSON
// number, a native int from direct tree delivery, and the string a YANG default
// carries (LeafNode.Default is declared as a string).
func adminDistanceValue(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	case string:
		parsed, err := strconv.Atoi(n)
		if err != nil {
			return 0, fmt.Errorf("expected number, got %q", n)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}

// incomingBatch aliases the (bgp-rib, best-change) payload type. sysrib
// receives one of these per BGP best-change and fans it out to the FIB
// plugins after distance arbitration.
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

// effectivePriority returns a protocol's administrative distance, which decides
// which route the kernel installs when two protocols hold one prefix.
//
// The answer is the declaration's, and the incoming priority is a fallback for
// two states that are not supposed to happen: a route arriving before the
// configure callback has run, and a protocol the schema does not declare.
// Neither is silent any more. Until 2026-09-04 both were, and the emptiness of
// the map was itself a fallback condition, so which of two declarations decided
// depended on whether an operator had written a block for some other protocol.
//
// REQUIRES: the caller holds s.mu for writing. The spoken set is written here.
func (s *sysRIB) effectivePriority(protocolType string, incomingPriority int) int {
	if d, ok := s.adminDist[protocolType]; ok {
		return d
	}

	if !s.distanceSpoken[protocolType] {
		s.distanceSpoken[protocolType] = true
		reason := "protocol is not declared in the schema"
		if len(s.adminDist) == 0 {
			reason = "a route arrived before the distance configuration"
		}
		logger().Warn("sysrib: no declared administrative distance, using the value the protocol stamped",
			"protocol", protocolType, "reason", reason, "stamped", incomingPriority)
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
			if len(c.NLRI) > 0 {
				// A family whose NLRI is not a CIDR prefix (VPN, EVPN, MVPN,
				// MUP, flowspec, VPLS, BGP-LS). The route is named by its wire
				// bytes and there is no prefix to key the FIB by, so sysrib has
				// nothing to arbitrate. Expected, not a fault: warning here
				// would log once per VPN route on any box carrying one.
				continue
			}
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
			// distance reconfig. Intra-protocol ECMP siblings ride on
			// pr.ecmpNextHops (not separate map entries), so a single slot preserves
			// ECMP. The event-bus fallback (each protocol emits independently) keeps
			// the per-protocol upsert so its cross-protocol distance arbitration
			// still works.
			pr := &protocolRoute{
				protocol:         proto,
				protocolType:     protoType,
				osInstalled:      protocolInstalledByOS(proto),
				nextHop:          c.NextHop,
				nextHopInterface: c.Interface,
				nextHopWeight:    c.Weight,
				priority:         priority,
				incomingPriority: c.Priority,
				metric:           c.Metric,
				labels:           c.Labels,
				srv6SID:          c.SRv6SID,
				routeType:        c.RouteType,
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
				// ghost and wrongly win recomputeBest after a distance change.
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
		if prev == nil {
			return nil
		}
		// The producer took its last route for the prefix away. A Withdraw is
		// owed only where Ze has an install outstanding: a best route and a
		// programmed route are different things, and three states hold the
		// first without the second -- a protocol the operator withholds, a
		// prefix whose entry the OS owns, and a next-hop that stopped
		// resolving. Withdrawing one of those tells a FIB writer to remove an
		// entry it never made, which the kernel writer reports as a FIB sync
		// failure for every prefix (internal/plugins/fib/kernel/fibkernel.go,
		// processEvent).
		programmed := s.programmedByZe(key)
		delete(s.best, key)
		delete(s.lastECMP, key)
		delete(s.resolvedNH, key)
		untrackNextHops(key.prefix, prev)
		if !programmed {
			return nil
		}
		return &outgoingChange{
			Action: routeaction.Withdraw,
			Prefix: key.prefix,
		}
	}

	// Select lowest priority (admin distance). Deterministic tiebreak by protocol name.
	var winner *protocolRoute
	for _, route := range protocols {
		if winner == nil || route.priority < winner.priority ||
			(route.priority == winner.priority && route.protocol < winner.protocol) {
			winner = route
		}
	}

	if winner.osInstalled {
		return s.recordOSInstalledWinner(key, prev, winner, protocols)
	}

	if !s.fibPermitted(winner.protocol) {
		return s.recordWithheldWinner(key, prev, winner, protocols)
	}

	if prev == nil {
		// resolvedNH is a SUBSET of best: every path that writes one writes the
		// other, and every path that removes a prefix from best removes it from
		// both. So no previous winner means no install outstanding, and the
		// entry below is an Add rather than an Update. The Add is unconditional
		// on that invariant, which TestResolvedNextHopIsASubsetOfBest asserts.
		s.best[key] = winner
		trackNextHops(key.prefix, winner)
		entry, path := s.fibEntry(key, winner)
		// RFC 9252 Section 5: a route whose SRv6 SID does not resolve is not
		// programmed. The tracking above stands, so the prefix is re-evaluated
		// when the SID becomes reachable.
		if path == fibPathForbidden {
			return nil
		}
		s.lastECMP[key] = entry.ECMPPaths
		s.resolvedNH[key] = entry.NextHop
		return &entry
	}

	// The resolver tracks the next-hop of every route Ze MEANS to program, and
	// only those: a cascade re-evaluates what the kernel holds, and a prefix
	// whose entry the OS owns must not be re-evaluated into a second install.
	// So the two paths that decline to program (recordWithheldWinner and
	// recordOSInstalledWinner) release the tracking rather than take it, and
	// the winner that takes the prefix back re-establishes it here, whether or
	// not the next-hop changed. Tracking the withheld winner instead would put
	// the entry back under a route Ze does not program, and would need a second
	// guard in cascadeRecompute to keep the OS-installed prefix out.
	//
	// A route Ze means to program is not always programmed: fibEntry forbids a
	// prefix whose SRv6 SID does not resolve, and the cascade withdraws one
	// whose next-hop stopped resolving. The tracking stands through both, so
	// the prefix is re-evaluated when the resolution comes back. The tracking
	// says the prefix is Ze's to program. resolvedNH says whether it is
	// programmed now.
	programmed := s.programmedByZe(key)
	if r := getNHResolver(); r != nil {
		if prev.nextHop.IsValid() && prev.nextHop != winner.nextHop {
			r.Untrack(prev.nextHop, key.prefix)
		}
		if winner.nextHop.IsValid() && (!programmed || winner.nextHop != prev.nextHop) {
			r.Track(winner.nextHop, key.prefix)
		}
		if prev.srv6SID.IsValid() && prev.srv6SID != winner.srv6SID {
			r.Untrack(prev.srv6SID, key.prefix)
		}
		if winner.srv6SID.IsValid() && (!programmed || winner.srv6SID != prev.srv6SID) {
			r.Track(winner.srv6SID, key.prefix)
		}
	}
	s.best[key] = winner

	// What the FIB owes is fibEntry's answer, the one the cascade and the
	// replay act on, and this path chooses only the ACTION -- the one thing
	// fibEntry cannot see. Computing the payload here answered a SECOND way:
	// the winner's raw gateway where the resolved address is programmed, and a
	// group with no reachability filter, so a member a cascade had dropped came
	// back into the kernel multipath on the next change for the prefix, and a
	// replay reported an address the live path never sent.
	entry, path := s.fibEntry(key, winner)
	if path == fibPathForbidden {
		delete(s.lastECMP, key)
		delete(s.resolvedNH, key)
		// Nothing is programmed for the prefix now, so no group is
		// outstanding either. The Withdraw is owed only where Ze had an
		// install to remove: the previous winner's SID may have been
		// unresolvable too, and a Withdraw for a prefix Ze never programmed
		// is a FIB sync failure at the writer.
		if !programmed {
			return nil
		}
		return &outgoingChange{
			Action: routeaction.Withdraw,
			Prefix: key.prefix,
		}
	}

	// The no-op test. Every field the FIB programs is compared here, the
	// forwarding action included. A prefix that turns into a discard with its
	// next-hop, metric and labels unchanged is a change the kernel must see.
	// Leaving routeType out suppresses the case RFC 7999 exists for, and leaving
	// the outgoing device or the weight out suppresses a route that moved to
	// another interface or changed its share of a weighted group. What the
	// RESOLVER decided is not compared here and MUST NOT be: this path runs on
	// a producer's route change, and a resolution that moved under an unchanged
	// route is the cascade's to publish. Comparing it here re-programs a prefix
	// a promotion moved onto a member, over the gateway the resolver declared
	// unreachable, one step before the cascade withdraws it.
	if prev.protocol == winner.protocol && prev.nextHop == winner.nextHop &&
		prev.nextHopInterface == winner.nextHopInterface &&
		prev.nextHopWeight == winner.nextHopWeight &&
		prev.priority == winner.priority && prev.metric == winner.metric &&
		prev.srv6SID == winner.srv6SID && prev.routeType == winner.routeType &&
		labelsEqual(prev.labels, winner.labels) && !ecmpChanged(s.lastECMP[key], entry.ECMPPaths) {
		return nil
	}

	s.lastECMP[key] = entry.ECMPPaths
	s.resolvedNH[key] = entry.NextHop
	// An Update names an entry to replace, and a prefix Ze holds no install for
	// has none: the previous winner's protocol was withheld, the OS owned the
	// entry, or its next-hop stopped resolving. The two verbs reach the kernel
	// as different netlink requests, so the choice is visible. An Add becomes
	// RouteAdd, which carries NLM_F_EXCL and fails EEXIST on a prefix the table
	// already holds; an Update becomes RouteReplace, which overwrites whatever
	// holds it (internal/plugins/fib/kernel/backend_linux.go, addRoute and
	// replaceRoute). The refusal is the wanted behavior HERE, where an Add says
	// Ze holds no install: the table still holding the prefix means a delete
	// that failed and raised a fib-sync-failure, or an entry another writer
	// owns, so the Add reports the conflict and leaves that entry intact where
	// a Replace would clobber a route Ze never installed. mplsentry.go chooses
	// Add over Replace for this path's reason.
	//
	// It is NOT a rule about the verb. replayBest emits an Add for every prefix
	// it replays, so a fib-kernel restart over a kernel table Ze's own entries
	// survived answers EEXIST once per prefix, and raises one fib-sync-failure
	// for each.
	if programmed {
		entry.Action = routeaction.Update
	}
	return &entry
}

// protocolInstalledByOS answers whether the OS creates this protocol's forwarding
// entries, for a protocol named on an incoming change.
//
// An unknown name answers false, which means Ze programs the route normally. That
// is failing OPEN and it is deliberate: reading "nobody declared this protocol"
// as "do not program it" would blackhole every route from a protocol that forgot
// to register, which is a far worse outcome than a duplicate entry for one that
// declared nothing. The name is spoken once per protocol in effectivePriority's
// warning when the declaration does not name it either.
func protocolInstalledByOS(protocol string) bool {
	id, ok := redistevents.ProtocolIDOf(protocol)
	if !ok {
		return false
	}
	osInstalled, _ := redistevents.OSInstalled(id)
	return osInstalled
}

// recordOSInstalledWinner records a winner whose forwarding entry the OPERATING
// SYSTEM creates, and answers with the change that leaves the kernel holding
// exactly one route for the prefix.
//
// Ze installs nothing: the OS already has the route, and a second entry from Ze
// would be the two-writer collision this arbitration exists to remove. The winner
// still enters the system RIB, so `show rib` reports which protocol holds the
// prefix and a later withdraw of the OS-installed path hands it to the next best.
//
// When Ze HAD programmed the prefix for a previous winner, that entry is now
// stale beside the OS's own and is WITHDRAWN. Silence would leave both, which is
// the defect wearing a quieter face.
//
// REQUIRES: the caller holds s.mu for writing.
func (s *sysRIB) recordOSInstalledWinner(key prefixKey, prev, winner *protocolRoute, protocols map[string]*protocolRoute) *outgoingChange {
	hadZeRoute := s.programmedByZe(key)
	s.best[key] = winner
	s.lastECMP[key] = s.ecmpCollect(protocols, winner)

	// Ze programs nothing here, so the previous winner's tracking is released
	// and this winner's is never taken (fibimport.go, trackNextHops).
	if prev != nil {
		untrackNextHops(key.prefix, prev)
	}

	if !hadZeRoute {
		return nil
	}
	delete(s.resolvedNH, key)
	logger().Info("sysrib: withdrawing the route Ze programmed, the operating system owns this prefix now",
		"prefix", key.prefix, "protocol", winner.protocol)
	return &outgoingChange{
		Action: routeaction.Withdraw,
		Prefix: key.prefix,
	}
}

// srv6SIDResolvable answers whether an SRv6 SID has a covering route in the
// Loc-RIB. That is the reachability question RFC 9252 Section 5 asks, and
// fibEntry says why asking it there does not meet the section's requirement.
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

// fibPath is what the resolver decided about a prefix's forwarding path. Every
// caller of fibEntry branches on it, so the three outcomes carry names rather
// than a bool that would read the same for two of them.
type fibPath uint8

// The count starts at one, so the Go zero value names no verdict. No fibPath
// can hold it, and the guarantee is at the PRODUCER: fibEntry is the only one,
// and every return it makes names a constant. It MUST stay there, because no
// reader supplies it. recomputeBest, replayBest and fibStateChange test for
// fibPathForbidden, so a zero reaching them is PROGRAMMED; cascadeRecompute
// tests for fibPathReachable, so a zero reaching it WITHDRAWS the prefix. A
// second producer owes the same named returns.
const (
	// fibPathReachable says the entry's target is reachable: the resolver
	// proved it, or the target is direct and there was nothing to resolve.
	fibPathReachable fibPath = iota + 1
	// fibPathUnreachable says the resolver proves no path for the prefix. The
	// entry still carries the target its producer named, because the Loc-RIB
	// is not the router's whole picture of reachability: an OSPF or IS-IS
	// next-hop on a link whose connected route no plugin inserted resolves to
	// nothing and is on-link all the same (test/ospf/ospf-route-install.ci).
	// What the verdict MEANS depends on the question the caller asked. A
	// cascade asks about reachability, so for a prefix Ze has already
	// programmed this verdict says the path was LOST and cascadeRecompute
	// withdraws it. The permission sweep asks which protocols reach the FIB,
	// which is no news about a path, so fibStateChange programs the entry as
	// recomputeBest does.
	fibPathUnreachable
	// fibPathForbidden says nothing may be programmed for the prefix at all.
	// RFC 9252 Section 5 is the one rule that says so today.
	fibPathForbidden
)

// fibEntry answers what the FIB owes for a prefix RIGHT NOW: the entry to
// program, and the resolver's verdict on its forwarding path. It READS the RIB
// and the resolver and writes nothing, so the four callers take one answer
// rather than computing it four times. recomputeBest emits it for a live
// change, the cascade compares it against what Ze last emitted and publishes
// the difference, the permission sweep does the same for a prefix whose
// permission moved (fibimport.go, fibStateChange), and a replay reports it as
// it stands.
//
// The Action is Add. A caller that knows Ze already holds the prefix overwrites
// it with Update, because that is the one thing this function cannot see.
//
// The entry is empty ONLY for fibPathForbidden. Every other verdict carries the
// entry to program, and the verdict says how much the resolver proved about it.
//
// REQUIRES: the caller holds s.mu, and best is the prefix's winner. Lock
// ordering: the resolver acquires Loc-RIB shard read locks under that mutex.
// The Loc-RIB OnChange handler therefore queues to a channel and the worker in
// run() processes it, rather than taking s.mu under the shard write lock, which
// would deadlock against the LPM lookup below.
func (s *sysRIB) fibEntry(key prefixKey, best *protocolRoute) (outgoingChange, fibPath) {
	// A resolvability check, and NOT the one RFC 9252 Section 5 requires. That
	// section binds the ingress PE to check the SRv6 Service SID BEFORE the
	// received prefix is considered for the BGP best path computation. Both
	// selections have already run when this function is reached, so a route
	// whose SID reaches nothing has already WON the prefix, and the refusal
	// below declines the FIB WRITE for it. The next-best route does not get the
	// prefix, so traffic for it is dropped where a conformant implementation
	// forwards it.
	//
	// The ledger carries that as the gap RFC9252-5-2 (rfc/short/rfc9252.md),
	// and plan/immediate/spec-srv6-bestpath-resolvability.md is where the check
	// moves in front of selection. This refusal MUST NOT be read as the MUST
	// met.
	//
	// It does cover the promotion below: a prefix whose SID is unreachable is
	// not programmed over an equal-cost member either.
	if best.srv6SID.IsValid() && !srv6SIDResolvable(best.srv6SID) {
		return outgoingChange{}, fibPathForbidden
	}

	protocols := s.routes[key]
	r := getNHResolver()
	if r == nil || !best.nextHop.IsValid() {
		// Nothing to resolve. No resolver is wired, or the winner names a
		// device alone and is already direct, so the entry is the winner's own
		// target with the group SELECTION collected.
		return fibChange(key, best, best.nextHop, s.ecmpCollect(protocols, best)), fibPathReachable
	}

	res := r.Resolve(best.nextHop)
	if res.Resolved {
		return fibChange(key, best, res.DirectNH, s.ecmpCollectResolved(protocols, best, r)), fibPathReachable
	}

	// The winner's gateway is unreachable, so an equal-cost member that still
	// resolves carries the prefix.
	paths := s.ecmpCollectResolved(protocols, best, r)
	if len(paths) == 0 {
		// The resolver covers nothing for this prefix. The entry is the group
		// SELECTION over the producers' own targets, which is what Ze programs
		// for a prefix it has never resolved, and what a replay must hand back
		// for one it programmed that way.
		return fibChange(key, best, best.nextHop, s.ecmpCollect(protocols, best)), fibPathUnreachable
	}
	entry := fibChange(key, best, paths[0].NextHop, paths[1:])
	// The promoted member brings its own device and share: it is the primary
	// next-hop now, and the winner's are the ones the resolver just declared
	// unreachable.
	entry.Interface = paths[0].Interface
	entry.Weight = paths[0].Weight
	return entry, fibPathReachable
}

// cascadeRecompute re-resolves the NH for a prefix whose covering route
// changed. Returns an outgoing change if the state Ze holds for the prefix
// differs from what fibEntry now says the FIB owes, nil otherwise.
//
// A cascade is reachability NEWS, and that is what makes this different from
// the permission sweep, which asks fibEntry the same question and reads the
// answer another way (fibimport.go, fibStateChange). processCascade is the one
// caller.
//
// Caller MUST hold s.mu.
func (s *sysRIB) cascadeRecompute(key prefixKey) *outgoingChange {
	best := s.best[key]
	if best == nil || !best.nextHop.IsValid() {
		return nil
	}

	// A prefix whose forwarding entry the OS creates is not Ze's to install, so
	// there is nothing to re-resolve and an Add here would be the second entry
	// this arbitration exists to prevent. The resolver never reaches one --
	// recordOSInstalledWinner released its tracking -- but the permission sweep
	// calls in directly.
	if best.osInstalled {
		return nil
	}

	// A withheld protocol's prefix is not programmed, so a next-hop that became
	// reachable has nothing to install and nothing to update. recomputeBest
	// already declined it and cleared the resolved next-hop, and re-resolving
	// here would put the install back through the other door.
	if !s.fibPermitted(best.protocol) {
		return nil
	}

	// A cascade is the resolver telling Ze that a covering route moved, so with
	// no resolver wired there is nothing to re-resolve. fibEntry answers for
	// that state too, and the permission sweep is the caller that wants the
	// answer (fibimport.go, fibStateChange).
	if getNHResolver() == nil {
		return nil
	}

	// Whether Ze has an install outstanding is the PRESENCE of the key, never
	// the validity of the address it holds. A member named by a device alone
	// resolves to an invalid address, and a promotion writes that address here,
	// so a later cascade reading validity would file a programmed prefix under
	// "Ze programmed nothing" and leave the kernel forwarding on a group whose
	// members are all gone.
	programmed := s.programmedByZe(key)

	// A cascade is reachability NEWS: the resolver says the path to a next-hop
	// this prefix depends on changed. So a verdict of anything but reachable
	// says the prefix has LOST the path it was programmed over, and the entry
	// goes. A prefix Ze never programmed keeps nothing, so it owes nothing.
	entry, path := s.fibEntry(key, best)
	if path != fibPathReachable {
		if !programmed {
			return nil
		}
		delete(s.resolvedNH, key)
		delete(s.lastECMP, key)
		return &outgoingChange{
			Action: routeaction.Withdraw,
			Prefix: key.prefix,
		}
	}

	if programmed && entry.NextHop == s.resolvedNH[key] && !ecmpChanged(s.lastECMP[key], entry.ECMPPaths) {
		return nil
	}

	s.resolvedNH[key] = entry.NextHop
	s.lastECMP[key] = entry.ECMPPaths
	if programmed {
		entry.Action = routeaction.Update
	}
	return &entry
}

// ecmpCollectResolved collects ECMP paths with resolved NHs, filtering
// out members whose NHs are unreachable, and out members from a protocol
// `rib { fib-withhold }` names, for ecmpCollect's reason.
//
// REQUIRES: the caller holds s.mu.
func (s *sysRIB) ecmpCollectResolved(protocols map[string]*protocolRoute, winner *protocolRoute, r *nhResolver) []sysribevents.ECMPPath {
	var paths []sysribevents.ECMPPath
	for _, route := range protocols {
		if route == winner {
			continue
		}
		if route.priority != winner.priority || route.metric != winner.metric {
			continue
		}
		if !namesATarget(route.nextHop, route.nextHopInterface) {
			continue
		}
		if !s.fibPermitted(route.protocol) {
			continue
		}
		directNH, reachable := resolveMember(route.nextHop, route.nextHopInterface, r)
		if !reachable {
			continue
		}
		paths = append(paths, sysribevents.ECMPPath{
			NextHop:   directNH,
			Interface: route.nextHopInterface,
			Weight:    ecmpWeight(route.nextHopWeight),
			Labels:    route.labels,
		})
	}
	// Intra-protocol equal-cost siblings of the winner (Loc-RIB path-group
	// expansion, isis-9), filtered to those whose next-hop resolves.
	for _, nh := range winner.ecmpNextHops {
		if sameTarget(nh, winner) || !namesATarget(nh.Addr, nh.Interface) {
			continue
		}
		directNH, reachable := resolveMember(nh.Addr, nh.Interface, r)
		if !reachable {
			continue
		}
		paths = append(paths, sysribevents.ECMPPath{
			NextHop:   directNH,
			Interface: nh.Interface,
			Weight:    ecmpWeight(nh.Weight),
			Labels:    winner.labels,
		})
	}
	return finishECMP(paths)
}

// resolveMember resolves one multipath member's gateway to the directly-reachable
// address the FIB programs, and reports whether the member is usable.
//
// A member named by a DEVICE alone is already direct: there is no address to
// look up, so it is kept as it stands. Running it through the resolver would
// report it unreachable and drop it, which is how a device-only next-hop
// disappears from a group that names both kinds.
func resolveMember(addr netip.Addr, iface string, r *nhResolver) (netip.Addr, bool) {
	if !addr.IsValid() {
		return addr, iface != ""
	}
	res := r.Resolve(addr)
	if !res.Resolved {
		return netip.Addr{}, false
	}
	return res.DirectNH, true
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
		// A replay is the one path that reads the TABLE rather than a change,
		// and the table holds a best route for prefixes Ze programs nothing
		// for. The install Ze has outstanding is the one test that names them
		// all: the OS owns the entry, the operator withholds the protocol, or
		// the next-hop stopped resolving. Replaying any of them as an Add hands
		// a reconnecting FIB plugin a route Ze declined to install.
		if !s.programmedByZe(key) {
			continue
		}
		// What to program is fibEntry's answer, the one the live path emitted
		// and the cascade acts on. Reading the winner's own next-hop, device
		// and share instead reports a third state: after a promotion the
		// winner's gateway is the address the resolver declared unreachable,
		// and the member carrying the prefix is the one this loop would leave
		// out. An unproven path is replayed as it was programmed, because the
		// prefix reached the loop by being programmed; only the RFC 9252
		// refusal leaves the FIB nothing to be handed back.
		entry, path := s.fibEntry(key, route)
		if path == fibPathForbidden {
			continue
		}
		changesByFamily[key.family] = append(changesByFamily[key.family], entry)
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
// the internal batch shape, runs distance arbitration, publishes
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

// showECMPGroups returns the equal-cost groups the system RIB holds as JSON.
//
// The group is computed from the RIB rather than read from lastECMP, which
// holds what was last EMITTED to the FIB. A prefix held by a protocol
// `rib { fib-withhold }` names emits nothing, and reading the emitted state
// would report no group for it -- a FIB filter answering a RIB question, when a
// withheld route is not a dropped route.
func (s *sysRIB) showECMPGroups() (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type ecmpEntry struct {
		Prefix netip.Prefix            `json:"prefix"`
		Family family.Family           `json:"family"`
		Paths  []sysribevents.ECMPPath `json:"paths"`
	}

	var entries []ecmpEntry
	for key, route := range s.best {
		paths := s.ecmpRIBGroup(s.routes[key], route)
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
		Prefix   netip.Prefix  `json:"prefix"`
		Family   family.Family `json:"family"`
		NextHop  netip.Addr    `json:"next-hop,omitzero"`
		Protocol string        `json:"protocol"`
		Priority int           `json:"priority"`
		// RouteType names the forwarding action for a route that does not
		// forward: "blackhole", "unreachable", "prohibit". Rendered as the name
		// rather than the Linux number, and OMITTED for an ordinary route, so an
		// operator confirming that a blackhole took effect reads the key's
		// presence rather than every value.
		RouteType string                  `json:"route-type,omitempty"`
		ECMPPaths []sysribevents.ECMPPath `json:"ecmp-paths,omitempty"`
	}

	entries := make([]entry, 0, len(s.best))
	for key, route := range s.best {
		e := entry{
			Prefix:   key.prefix,
			Family:   key.family,
			NextHop:  route.nextHop,
			Protocol: route.protocol,
			Priority: route.priority,
			// The RIB view, for showECMPGroups's reason: this command reports
			// what the system RIB holds, and a withheld protocol's prefix keeps
			// its equal-cost paths there.
			ECMPPaths: s.ecmpRIBGroup(s.routes[key], route),
		}
		if route.routeType != 0 {
			e.RouteType = route.routeType.String()
		}
		entries = append(entries, e)
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
	var iface string
	var weight uint8
	var priority int
	var metric uint32
	var labels []uint32
	var routeTyp routetype.Type
	if c.Kind != locrib.ChangeRemove {
		nextHop = c.Best.NextHop
		// A route whose next-hop is a device, or one member of a weighted group,
		// loses both facts here unless they are carried. The kernel then gets a
		// route with no next-hop at all, or an equal share where the operator
		// asked for a proportion.
		iface = c.Best.Interface
		weight = c.Best.Weight
		priority = int(c.Best.AdminDistance)
		metric = c.Best.Metric
		// Carry the MPLS label stack so labeled-unicast routes program a kernel
		// MPLS push entry rather than a plain IP route (the Loc-RIB now retains
		// Labels; without this they were dropped here).
		labels = c.Best.Labels
		// Carry the forwarding action for the same reason. This is the only
		// translation from a Loc-RIB Change into the batch processEvent consumes.
		// A field dropped here is invisible to every in-process deployment.
		// Left unset on ChangeRemove, where c.Best is the zero Path and a
		// withdraw needs no forwarding action.
		routeTyp = c.Best.RouteType
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
			Interface:    iface,
			Weight:       weight,
			Priority:     priority,
			Metric:       metric,
			Labels:       labels,
			RouteType:    routeTyp,
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
// Runs the change through processEvent as a synthetic Add so distance
// overrides and downstream emission work the same as any live change. ECMP is
// supplied from the PathGroup snapshot so pre-existing multipath groups do not
// collapse to the primary next-hop on replay.
func (s *sysRIB) replayPath(fam family.Family, pfx netip.Prefix, p locrib.Path, ecmp []nexthop.NextHop) {
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
// for distance lookup in that case).
func bgpProtocolTypeFromPath(p locrib.Path) routeaction.ProtocolType {
	name := redistevents.ProtocolName(p.Source)
	if name != "bgp" {
		return routeaction.ProtocolUnspecified
	}
	// Read the producer's eBGP/iBGP classification directly. Deriving it from
	// AdminDistance (20/200) silently lost the class whenever the operator
	// overrode rib/distance, making this replay path disagree with the
	// live event-bus ProtocolType. The BGP RIB sets Path.IsEBGP from the peer
	// ASN relationship; mirror its 2-state resolve() (iBGP unless eBGP).
	if p.IsEBGP {
		return routeaction.ProtocolEBGP
	}
	return routeaction.ProtocolIBGP
}
