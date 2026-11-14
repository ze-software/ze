// Design: docs/architecture/core-design.md -- System RIB plugin
// Related: nhresolver.go -- recursive next-hop resolution using Loc-RIB LPM
// Related: ecmp.go -- ECMP path collection from equal-cost protocol routes
//
// System RIB consumes the daemon authority's selected routing values and
// computes forwarding, dependency and replay effects for the active SDK owner.
// Admission and cross-protocol selection remain in the daemon's Loc-RIB.
package sysrib

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/component/config"
	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/nexthop"
	"github.com/ze-software/ze/internal/core/rib/routetype"
	"github.com/ze-software/ze/internal/core/slogutil"
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
		eventsReceived: reg.Counter("ze_systemrib_events_received_total", "Nonempty authoritative routing-change batches received."),
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

// protocolRoute is one protocol's best route for a prefix.
type protocolRoute struct {
	protocol         string
	protocolType     string // "ebgp", "ibgp", "static", etc. for admin distance lookup
	nextHop          netip.Addr
	nextHopInterface string // outgoing device for nextHop, empty for a gateway-only next-hop
	nextHopWeight    uint8  // nextHop's share of the multipath group, 0 for an unweighted one
	nextHopOnLink    bool   // protocol-established adjacency, never recursively replaced
	priority         int    // effective admin distance (lower wins)
	metric           uint32
	labels           []uint32   // MPLS label stack (nil for unlabeled routes)
	srv6SID          netip.Addr // SRv6 SID from PrefixSID attribute (zero if absent)

	// routeType is the forwarding action the FIB programs: an ordinary next-hop,
	// or a discard. Unset means the protocol has no opinion and the FIB installs
	// an ordinary route. It rides the winner into BestChangeEntry.RouteType.
	//
	// Authority has already selected this path. A discard action does not
	// perform a second ranking here. It IS part of the change comparison, because
	// a prefix that turns into a discard with every other field unchanged must
	// still reach the kernel.
	routeType routetype.Type

	// backupNextHop and backupLabels are the fast-reroute alternate for this
	// route's primary next-hop (an IP FRR backup + optional MPLS repair stack).
	// They ride the winner into BestChangeEntry.Backup as a DEDICATED backup
	// next-hop, never folded into the ECMP group.
	backupNextHop netip.Addr
	backupLabels  []uint32
	// backupInterface and backupOnLink are the backup's outgoing device and
	// on-link flag (an alternate across an unnumbered link).
	backupInterface string
	backupOnLink    bool

	// osInstalled is true when the OPERATING SYSTEM creates this protocol's
	// forwarding entry, so Ze MUST NOT program one of its own. The protocol
	// DECLARES it (redistevents.RegisterOSInstalled) and this is the answer read
	// back by ID, never a guess from an invalid next-hop or a protocol name.
	//
	// It is not part of arbitration: an OS-installed route wins or loses on its
	// administrative distance like any other. What it changes is what winning
	// MEANS, in recordOSInstalledWinner.
	osInstalled bool

	// ecmpNextHops carries the authority's complete selected forwarding group.
	// Aligned metadata retains each member's owner, labels and Service SID;
	// the consumer never reconstructs membership from unselected candidates.
	ecmpNextHops []nexthop.NextHop
	// ecmpMetadata is aligned with ecmpNextHops when the authority supplied
	// each sibling's own labels and SID. Nil means the producer's one Path
	// supplies the same metadata for all its explicitly declared targets.
	ecmpMetadata []projectionMemberMetadata
}

// prefixKey identifies a unique prefix in the system RIB.
type prefixKey struct {
	family family.Family
	prefix netip.Prefix
}

// installedPath is what Ze last programmed for a prefix. proved says the
// resolver proved a path to nextHop at that moment, and false says it proved
// none and the entry carries the target its producer named, which every door
// programs deliberately (fibVerdict).
//
// A cascade is the one reader of proved, and it is what tells a path that was
// LOST from one the resolver never covered. Without it the two states look
// identical at the next cascade, because a gateway inside a CONNECTED covering
// route resolves to itself: the address Ze programmed is the producer's own
// either way.
type installedPath struct {
	nextHop netip.Addr
	iface   string
	weight  uint8
	labels  []uint32
	srv6SID netip.Addr
	onLink  bool
	proved  bool
}

// installedEntry retains the complete primary path so a cascade can observe
// a device, label or weight change even when the resolved address is unchanged.
func installedEntry(entry *outgoingChange, verdict fibVerdict) installedPath {
	return installedPath{
		nextHop: entry.NextHop, iface: entry.Interface, onLink: entry.OnLink,
		weight: entry.Weight, labels: entry.Labels, srv6SID: entry.SRv6SID,
		proved: verdict == fibProved,
	}
}

func (p installedPath) matches(entry *outgoingChange) bool {
	return p.nextHop == entry.NextHop && p.iface == entry.Interface &&
		p.onLink == entry.OnLink && p.weight == entry.Weight &&
		p.srv6SID == entry.SRv6SID && labelsEqual(p.labels, entry.Labels)
}

// sysRIB retains selected values and the forwarding state of one consumer.
type sysRIB struct {
	// resolver belongs to this consumer generation. Runtime installs it only
	// after its selected reader is synchronized; a replacement cannot change
	// another generation's resolver or dependency tracking.
	resolver atomic.Pointer[nhResolver]
	// Each prefix contains only its one authority-selected protocol value.
	routes map[prefixKey]map[string]*protocolRoute
	// best[prefixKey] = current system best route.
	best map[prefixKey]*protocolRoute
	// lastECMP tracks the last emitted ECMP path set per prefix for
	// suppressing duplicate emissions when only ECMP membership changes.
	lastECMP map[prefixKey][]sysribevents.ECMPPath
	// installed carries the entry Ze has outstanding in the FIB for a prefix.
	// Used by the cascade worker to detect resolution changes. PRESENCE of the
	// key is what says Ze has an install outstanding for the prefix, and
	// programmedByZe is the one test: the ADDRESS is invalid for a route
	// reached over a device alone, which no validity test can tell from a
	// prefix Ze never programmed. A prefix is absent when the OS owns it, when
	// its next-hop stopped resolving, or when the operator withholds the
	// protocol that holds it; the route stays in the system RIB in each case.
	installed map[prefixKey]installedPath
	// adminDist maps protocol type (e.g., "ebgp", "ibgp", "static") to its
	// administrative distance. parseAdminDistanceConfig returns every protocol
	// the schema declares, so this is complete once configure has run, and is
	// nil only before it.
	adminDist map[string]int
	mu        sync.RWMutex

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
	// from the permission set: one line for each rather than one per route.
	fibSpoken map[string]bool
}

func newSysRIB() *sysRIB {
	s := &sysRIB{
		fibSpoken: make(map[string]bool),
		routes:    make(map[prefixKey]map[string]*protocolRoute),
		best:      make(map[prefixKey]*protocolRoute),
		lastECMP:  make(map[prefixKey][]sysribevents.ECMPPath),
		installed: make(map[prefixKey]installedPath),
	}
	return s
}

func (s *sysRIB) nextHopResolver() *nhResolver { return s.resolver.Load() }

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
	_, programmed := s.installed[key]
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

// outgoingChange aliases the exported payload entry type so functions in
// this file keep their current signatures while producing the exported
// payload shape used by fib plugins.
type outgoingChange = sysribevents.BestChangeEntry

// outgoingBatch aliases the exported payload type. orderForwarding cuts one
// publication into these batches, and the daemon's routing-view output emits
// each via the typed BestChange handle (route_view_output.go, publish).
type outgoingBatch = sysribevents.BestChangeBatch

// recomputeForwarding evaluates the FIB effect of the authority's selected
// route. A nil selection withdraws the prefix. The caller MUST hold s.mu and
// MUST update the complete reader batch before resolving any changed prefix.
func (s *sysRIB) recomputeForwarding(key prefixKey, winner *protocolRoute) *outgoingChange {
	protocols := s.routes[key]
	prev := s.best[key]
	s.untrackNextHopGroup(key)
	defer s.trackNextHopGroup(key)

	if winner == nil {
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
		delete(s.installed, key)
		s.untrackNextHops(key.prefix, prev)
		if !programmed {
			return nil
		}
		return &outgoingChange{
			Action: routeaction.Withdraw,
			Prefix: key.prefix,
		}
	}

	if winner.osInstalled {
		return s.recordOSInstalledWinner(key, prev, winner, protocols)
	}

	if !s.fibPermitted(winner.protocol) {
		return s.recordWithheldWinner(key, prev, winner, protocols)
	}

	if prev == nil {
		// installed is a SUBSET of best: every path that writes one writes the
		// other, and every path that removes a prefix from best removes it from
		// both. So no previous winner means no install outstanding, and the
		// entry below is an Add rather than an Update. The Add is unconditional
		// on that invariant, which TestInstalledIsASubsetOfBest asserts.
		s.best[key] = winner
		entry, verdict := s.fibEntry(key, winner)
		// A route whose SRv6 SID does not resolve is not programmed. The
		// deferred tracking stands, so the prefix is re-evaluated
		// when the SID becomes reachable.
		if verdict == fibForbidden {
			return nil
		}
		s.lastECMP[key] = entry.ECMPPaths
		s.installed[key] = installedEntry(&entry, verdict)
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
	// says the prefix is Ze's to program. installed says whether it is
	// programmed now.
	programmed := s.programmedByZe(key)
	s.best[key] = winner

	// What the FIB owes is fibEntry's answer, the one the cascade and the
	// replay act on, and this path chooses only the ACTION -- the one thing
	// fibEntry cannot see. Computing the payload here answered a SECOND way:
	// the winner's raw gateway where the resolved address is programmed, and a
	// group with no reachability filter, so a member a cascade had dropped came
	// back into the kernel multipath on the next change for the prefix, and a
	// replay reported an address the live path never sent.
	entry, verdict := s.fibEntry(key, winner)
	if verdict == fibForbidden {
		delete(s.lastECMP, key)
		delete(s.installed, key)
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
	// another interface or changed its share of a weighted group.
	// A proved alternate also compares the installed primary's full metadata:
	// an unchanged configured gateway can resolve onto a different device or
	// member. An unproved answer stays with the cascade, which withdraws a
	// previously proved path rather than reinstalling its unresolved gateway.
	if prev.protocol == winner.protocol && prev.nextHop == winner.nextHop &&
		prev.nextHopInterface == winner.nextHopInterface &&
		prev.nextHopWeight == winner.nextHopWeight &&
		prev.nextHopOnLink == winner.nextHopOnLink &&
		prev.priority == winner.priority && prev.metric == winner.metric &&
		prev.backupNextHop == winner.backupNextHop &&
		prev.backupInterface == winner.backupInterface &&
		prev.backupOnLink == winner.backupOnLink &&
		labelsEqual(prev.backupLabels, winner.backupLabels) &&
		prev.srv6SID == winner.srv6SID && prev.routeType == winner.routeType &&
		labelsEqual(prev.labels, winner.labels) && !ecmpChanged(s.lastECMP[key], entry.ECMPPaths) &&
		(verdict != fibProved || s.installed[key].matches(&entry)) {
		return nil
	}

	s.lastECMP[key] = entry.ECMPPaths
	s.installed[key] = installedEntry(&entry, verdict)
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
	// It is NOT a rule about the verb. A replay (replayChanges) emits an Add for
	// every prefix it replays, so a fib-kernel restart over a kernel table Ze's own entries
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
// declared nothing. Current selected inputs have registered source identities;
// this ownership lookup never grants kernel ownership to an unknown name.
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
	s.lastECMP[key] = ecmpPaths(s.ecmpCollect(protocols, winner))

	// Ze programs nothing here, so the previous winner's tracking is released
	// and this winner's is never taken (fibimport.go, trackNextHops).
	if prev != nil {
		s.untrackNextHops(key.prefix, prev)
	}

	if !hadZeRoute {
		return nil
	}
	delete(s.installed, key)
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
func (s *sysRIB) srv6SIDResolvable(sid netip.Addr) bool {
	r := s.nextHopResolver()
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

// fibVerdict is what fibEntry decided about a prefix's forwarding path. ONE of
// its three values refuses the write, and every caller tests for that one and
// no other: a live change, a cascade, the permission sweep and a replay cannot
// disagree about whether a prefix is programmable.
//
// The other two both PROGRAM, and they differ in what the install RECORDS
// rather than in what any caller does next (installedPath). Reading that
// difference as a second answer to "may I program this" is what made a cascade
// withdraw a prefix the live path had deliberately programmed
// (plan/journal/guard-added-to-one-half-of-a-pair.md).
//
// The count starts at one, so the Go zero value names no verdict. No
// fibVerdict can hold it, and the guarantee is at the PRODUCER: fibEntry is
// the only one, and every return it makes names a constant. It MUST stay
// there, because no reader supplies it: every caller tests for fibForbidden,
// so a zero reaching one is PROGRAMMED. A second producer owes the same named
// returns.
type fibVerdict uint8

const (
	// fibProved says the resolver proved the path to the entry's next-hop, or
	// the target is direct and there was nothing to resolve. A cascade that
	// later proves no path for such an install says the path was LOST.
	fibProved fibVerdict = iota + 1
	// fibUnproved says the resolver proves no path for the prefix. The entry
	// still carries the target its producer named, and every caller programs
	// it, because the Loc-RIB is not the router's whole picture of
	// reachability: an OSPF or IS-IS next-hop on a link whose connected route
	// no plugin inserted resolves to nothing and is on-link all the same
	// (test/ospf/ospf-route-install.ci).
	fibUnproved
	// fibForbidden says nothing may be programmed for the prefix at all.
	// RFC 9252 Section 5 is the one rule that says so today.
	fibForbidden
)

// fibEntry answers what the FIB owes for a prefix RIGHT NOW: the entry to
// program, and the resolver's verdict on its forwarding path. It READS the RIB
// and the resolver and writes nothing, so the four callers take one answer
// rather than computing it four times. recomputeForwarding emits it for a live
// change, the cascade compares it against what Ze last emitted and publishes
// the difference, the permission sweep does the same for a prefix whose
// permission moved (fibimport.go, fibStateChange), and a replay reports it as
// it stands.
//
// The Action is Add. A caller that knows Ze already holds the prefix overwrites
// it with Update, because that is the one thing this function cannot see.
//
// The entry is empty ONLY for fibForbidden. The other verdict carries the entry
// to program.
//
// REQUIRES: the caller holds s.mu, and best is the prefix's winner. Lock
// ordering: the resolver acquires Loc-RIB shard read locks under that mutex.
// The Loc-RIB OnChange handler therefore queues to a channel and the worker in
// the current routing-view consumer processes it outside authority locks, which
// would deadlock against the LPM lookup below.
func (s *sysRIB) fibEntry(key prefixKey, best *protocolRoute) (outgoingChange, fibVerdict) {
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
	if best.srv6SID.IsValid() && !s.srv6SIDResolvable(best.srv6SID) {
		return outgoingChange{}, fibForbidden
	}

	protocols := s.routes[key]
	r := s.nextHopResolver()
	if r == nil {
		return fibChange(key, best, routePath(best, best.nextHop), s.ecmpCollect(protocols, best)), fibProved
	}
	if !best.nextHop.IsValid() {
		// Device-only and discard primaries need no gateway resolution, but
		// recursive siblings still need their own reachability checked.
		return fibChange(key, best, routePath(best, best.nextHop), s.ecmpCollectResolved(protocols, best, r)), fibProved
	}

	path, reachable := resolveMember(routePath(best, best.nextHop), r)
	if reachable {
		return fibChange(key, best, path, s.ecmpCollectResolved(protocols, best, r)), fibProved
	}

	// The winner's gateway is unreachable, so an equal-cost member that still
	// resolves carries the prefix.
	paths := s.ecmpCollectResolved(protocols, best, r)
	if len(paths) == 0 {
		// The resolver covers nothing for this prefix. The entry is the group
		// SELECTION over the producers' own targets, which is what Ze programs
		// for a prefix it has never resolved, and what a replay must hand back
		// for one it programmed that way.
		return fibChange(key, best, routePath(best, best.nextHop), s.ecmpCollect(protocols, best)), fibUnproved
	}
	// The promoted member is the path the packets take now, so the entry
	// carries the member's WHOLE path -- its gateway, its device, its share,
	// its label stack and its SID -- and keeps from the winner only what the
	// prefix owns. Overriding fields by name here carried the winner's label
	// stack onto another protocol's path, which is an MPLS misforward
	// (plan/journal/helper-bypassed-by-an-open-coded-copy.md).
	return fibChange(key, best, paths[0], paths[1:]), fibProved
}

// cascadeRecompute re-resolves the NH for a prefix whose covering route
// changed. Returns an outgoing change if the state Ze holds for the prefix
// differs from what fibEntry now says the FIB owes, nil otherwise.
// cascadeChanges is the one caller.
//
// What the prefix then owes is fibInstall's answer, the SAME answer the
// permission sweep takes (fibimport.go, fibStateChange). The guards below are
// the cascade's own, and they decide which prefixes a cascade has nothing to
// say about. They do not change the answer for the ones it does.
//
// Caller MUST hold s.mu.
func (s *sysRIB) cascadeRecompute(key prefixKey) *outgoingChange {
	best := s.best[key]
	if best == nil {
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
	// reachable has nothing to install and nothing to update. recomputeForwarding
	// already declined it and cleared the resolved next-hop, and re-resolving
	// here would put the install back through the other door.
	if !s.fibPermitted(best.protocol) {
		return nil
	}

	// A cascade is the resolver telling Ze that a covering route moved, so with
	// no resolver wired there is nothing to re-resolve. fibEntry answers for
	// that state too, and the permission sweep is the caller that wants the
	// answer (fibimport.go, fibStateChange).
	if s.nextHopResolver() == nil {
		return nil
	}

	entry, verdict := s.fibEntry(key, best)

	// A cascade is reachability NEWS, and this is the ONE thing it reads that
	// the other doors do not. A prefix programmed over a path the resolver
	// PROVED, for which it now proves none, has LOST that path: the gateway is
	// no longer covered, the kernel cannot forward over it, and the entry goes.
	//
	// A prefix the resolver never covered is in the very state it was
	// installed in, so the same verdict is no news at all. Withdrawing there
	// took out what the live rule had deliberately programmed, and for good:
	// an identical re-announcement is a no-op at recomputeForwarding, and a cascade
	// fires only when a covering route changes, so a gateway the Loc-RIB never
	// covers had nothing left to put the prefix back
	// (plan/journal/guard-added-to-one-half-of-a-pair.md). The two states are
	// indistinguishable at the address, because a gateway inside a connected
	// covering route resolves to itself, which is why the install records
	// which one it was (installedPath).
	// A previous cascade may already have withdrawn this path. Another
	// unresolved cover is not evidence to install it again; wait for proof.
	if verdict == fibUnproved && !s.programmedByZe(key) {
		return nil
	}
	if verdict == fibUnproved && s.installed[key].proved {
		delete(s.installed, key)
		delete(s.lastECMP, key)
		return &outgoingChange{
			Action: routeaction.Withdraw,
			Prefix: key.prefix,
		}
	}

	return s.fibInstall(key, entry, verdict)
}

// fibInstall turns fibEntry's answer into the change a prefix owes, and into
// the verb it owes it under. It is the tail BOTH doors onto one install
// decision take -- cascadeRecompute where a covering route moved,
// fibStateChange where the operator changed a permission -- so no state can be
// programmed through one and withdrawn through the other. Each door keeps its
// own guards above the call, because which prefixes they have nothing to say
// about differs; what the FIB owes for the ones they do does not.
//
// Nil where the FIB already holds what the RIB says, and where a forbidden
// prefix was never programmed.
//
// REQUIRES: the caller holds s.mu, entry and verdict are fibEntry's answer for
// this prefix's winner, and the caller has established that the prefix is Ze's
// to program.
func (s *sysRIB) fibInstall(key prefixKey, entry outgoingChange, verdict fibVerdict) *outgoingChange {
	// Whether Ze has an install outstanding is the PRESENCE of the key, never
	// the validity of the address it holds. A member named by a device alone
	// resolves to an invalid address, and a promotion writes that address here,
	// so a caller reading validity would file a programmed prefix under "Ze
	// programmed nothing" and leave the kernel forwarding on a group whose
	// members are all gone.
	programmed := s.programmedByZe(key)

	// RFC 9252 Section 5 is the one rule that forbids the write outright, and
	// fibEntry is where the SID is read. A prefix Ze holds an install for is
	// withdrawn, as recomputeForwarding withdraws it: the kernel entry encapsulates
	// to a SID no route reaches. One Ze never programmed is left alone, because
	// a Withdraw for it asks a FIB writer to delete an entry it never made.
	if verdict == fibForbidden {
		delete(s.installed, key)
		delete(s.lastECMP, key)
		if !programmed {
			return nil
		}
		return &outgoingChange{
			Action: routeaction.Withdraw,
			Prefix: key.prefix,
		}
	}

	if programmed && s.installed[key].matches(&entry) &&
		!ecmpChanged(s.lastECMP[key], entry.ECMPPaths) {
		// The FIB holds this entry already, so nothing is published. The PROOF
		// can still have moved under an identical entry, because a gateway
		// inside a connected covering route resolves to itself, and a cascade
		// reads the proof to tell a path that was LOST from one the resolver
		// never covered. So it is recorded whether or not anything is emitted.
		s.installed[key] = installedEntry(&entry, verdict)
		return nil
	}

	s.installed[key] = installedEntry(&entry, verdict)
	s.lastECMP[key] = entry.ECMPPaths
	// An Update names an entry to replace and a prefix Ze holds no install for
	// has none, so the verb follows the install rather than the change:
	// recomputeForwarding says what each one becomes at the kernel writer.
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
func (s *sysRIB) ecmpCollectResolved(protocols map[string]*protocolRoute, winner *protocolRoute, r *nhResolver) []forwardingPath {
	var paths []forwardingPath
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
		path, reachable := resolveECMPMember(memberPath(route, route.nextHop, route.nextHopInterface, route.nextHopWeight, route.nextHopOnLink), r)
		if !reachable {
			continue
		}
		paths = append(paths, path)
	}
	// Intra-protocol equal-cost siblings of the winner (Loc-RIB path-group
	// expansion, isis-9), filtered to those whose next-hop resolves.
	for i, nh := range winner.ecmpNextHops {
		if !namesATarget(nh.Addr, nh.Interface) {
			continue
		}
		if sameTarget(nh, winner) {
			if siblingProtocol(winner, i) == winner.protocol {
				continue
			}
		}
		if !s.fibPermitted(siblingProtocol(winner, i)) {
			continue
		}
		path, reachable := resolveECMPMember(siblingPath(winner, i), r)
		if !reachable {
			continue
		}
		paths = append(paths, path)
	}
	return finishECMP(paths)
}

// resolveECMPMember requires the member's own SID to resolve as well as its
// gateway. A surviving gateway cannot carry a promoted SID that reaches
// nothing. SID dependencies remain tracked so a cover can restore the member.
func resolveECMPMember(path forwardingPath, r *nhResolver) (forwardingPath, bool) {
	if path.srv6SID.IsValid() {
		if !r.Resolve(path.srv6SID).Resolved {
			return forwardingPath{}, false
		}
	}
	return resolveMember(path, r)
}

// resolveMember resolves one multipath member's gateway to the directly-reachable
// address the FIB programs, and reports whether the member is usable.
//
// A member named by a DEVICE alone is already direct: there is no address to
// look up, so it is kept as it stands. Running it through the resolver would
// report it unreachable and drop it, which is how a device-only next-hop
// disappears from a group that names both kinds.
func resolveMember(path forwardingPath, r *nhResolver) (forwardingPath, bool) {
	if !path.NextHop.IsValid() {
		return path, path.Interface != ""
	}
	if path.OnLink {
		return path, true
	}
	resolved := r.Resolve(path.NextHop)
	if !resolved.Resolved {
		return forwardingPath{}, false
	}
	path.NextHop = resolved.DirectNH
	if path.Interface == "" {
		path.Interface = resolved.Interface
	}
	path.OnLink = resolved.OnLink
	return path, true
}

// cascadeChanges re-evaluates every prefix that depends on the given NHs and
// answers with the owned changes, without publishing. It handles multi-level
// cascades: a re-evaluated prefix that itself covers other tracked NHs brings
// their dependents in too. The routing-view worker publishes the result with
// the selected changes that caused it, after these locks are released.
func (s *sysRIB) cascadeChanges(nhs []netip.Addr) map[family.Family][]outgoingChange {
	r := s.nextHopResolver()
	if r == nil {
		return nil
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

	return changesByFamily
}

// replayChanges renders the same programmed state as live forwarding, keyed by
// family. Its caller MUST publish through the active generation's
// acknowledged sink.
func (s *sysRIB) replayChanges() map[family.Family][]outgoingChange {
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
		entry, verdict := s.fibEntry(key, route)
		if verdict == fibForbidden {
			continue
		}
		changesByFamily[key.family] = append(changesByFamily[key.family], entry)
	}
	s.mu.RUnlock()
	return changesByFamily
}

// showNHTable returns the NH resolver tracking table as JSON.
func (s *sysRIB) showNHTable() (any, error) {
	r := s.nextHopResolver()
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
		paths := ecmpPaths(s.ecmpRIBGroup(s.routes[key], route))
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
			ECMPPaths: ecmpPaths(s.ecmpRIBGroup(s.routes[key], route)),
		}
		if route.routeType != 0 {
			e.RouteType = route.routeType.String()
		}
		entries = append(entries, e)
	}

	return entries, nil
}
