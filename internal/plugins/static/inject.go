// Design: docs/architecture/static-routes.md -- BFD integration and active NH tracking
// Related: doctor.go -- checkRouteSkipped reads routeManager.skipped + activeRouteManager

package static

import (
	"cmp"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"

	bfdapi "github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	staticevents "github.com/ze-software/ze/internal/plugins/static/events"
)

// nhState is one next-hop's membership of its route's ECMP group. active is
// what programRouteLocked reads (activeNextHops); it is derived from the
// inputs below by refresh, and never written from one input alone.
type nhState struct {
	nh     nextHop
	active bool
	// bfdUp is BFD's verdict: true while the session is up, and while the
	// next-hop is not BFD-tracked at all.
	bfdUp bool
	// waiting is device presence: true for a next-hop the candidate declared
	// dynamic (nextHop.Dynamic) while its device is not known to be up. The
	// route manager's link watch of that name sets it (link.go): LinkUp clears
	// it, LinkDown (down or deleted) sets it. A waiting next-hop is never
	// resolved and never skipped, and stays out of the group.
	waiting bool
	handle  bfdapi.SessionHandle
	unsub   func()
}

// newNHState builds the membership a next-hop starts with: untracked by BFD
// (setupBFDLocked may track it next), and waiting when it names a declared
// dynamic interface whose device is not up (deviceUp, the link watch's state;
// ignored for any other next-hop).
func newNHState(nh nextHop, deviceUp bool) nhState {
	s := nhState{nh: nh, bfdUp: true, waiting: nh.Dynamic && !deviceUp}
	s.refresh()
	return s
}

// refresh derives active from the inputs and reports whether it changed, so
// the caller reprograms the route only on a change. A next-hop is active only
// while BFD allows it AND it is not waiting for its device.
func (s *nhState) refresh() bool {
	want := s.bfdUp && !s.waiting
	if want == s.active {
		return false
	}
	s.active = want
	return true
}

type routeState struct {
	route    staticRoute
	nhStates []nhState
	done     chan struct{}
	emitted  bool
	// programmed is the route exactly as it was last handed to
	// applyProgrammed, with its next-hops narrowed to the active members, or
	// nil while nothing of this route is installed. A withdrawal hands back
	// this value, never route: route can name a waiting member that was never
	// installed and whose device cannot be resolved to build the removal.
	programmed *staticRoute
}

type routeKey struct {
	table  uint32
	prefix netip.Prefix
}

// skippedRoute records a route the backend could not program, together with the
// reason. Per-route isolation (spec-fixit-static-per-route-isolation) keeps a
// failing route out of the FIB and the diff baseline while surfacing it here so
// `static show` and the doctor check can report it. A skipped route is
// re-attempted on the next apply and clears once it programs.
type skippedRoute struct {
	route  staticRoute
	reason string
}

type routeManager struct {
	mu      sync.Mutex
	backend routeBackend
	routes  map[routeKey]*routeState
	skipped map[routeKey]skippedRoute
	bfd     bfdapi.Service

	// loc and remote are where a MAIN-table route is installed: the shared
	// Loc-RIB in-process, or the engine over RPC when static runs forked. The
	// backend above now serves NAMED tables only. See locrib.go.
	loc    *locrib.RIB
	remote routeSink

	// links tells the manager whether the device of a declared on-demand
	// interface is up, and watches holds one subscription per such name that
	// a route in routes uses (link.go). linkWG counts the watchers, which
	// shutdown waits for once it has released mu.
	links   linkSource
	watches map[string]*linkWatch
	linkWG  sync.WaitGroup
}

func newRouteManager(backend routeBackend) *routeManager {
	return &routeManager{
		backend: backend,
		routes:  make(map[routeKey]*routeState),
		skipped: make(map[routeKey]skippedRoute),
		links:   ifaceLinks{},
		watches: make(map[string]*linkWatch),
	}
}

// activeRouteManager points at the running static plugin's route manager, set in
// runStaticPlugin. The doctor check reads it to report routes the backend
// skipped at runtime. It is nil in the offline `ze doctor config <file>` path (no
// daemon), where there is no runtime skip state to report, and when static runs
// as an external forked plugin (the daemon process holds no route manager) --
// in those cases `static show` and the WARN logs remain the always-on skip
// surfaces. Set once per process; static runs at most one route manager.
var activeRouteManager atomic.Pointer[routeManager]

func (rm *routeManager) setBFD(svc bfdapi.Service) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.bfd = svc

	for _, rs := range rm.routes {
		if rs.route.Action == actionForward {
			rm.setupBFDLocked(rs)
		}
	}
}

func (rm *routeManager) applyRoutes(routes []staticRoute) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	newMap := make(map[routeKey]staticRoute, len(routes))
	for _, r := range routes {
		newMap[routeKey{table: r.Table, prefix: r.Prefix}] = r
	}

	// Per-route isolation (spec-fixit-static-per-route-isolation, A-2): a route
	// the backend cannot program is logged and skipped, never section-fatal. The
	// good routes stay programmed and applyRoutes returns nil. The skip is
	// surfaced (rm.skipped) so `static show` and the doctor check report it, and
	// the route is re-attempted on the next apply.
	for key, rs := range rm.routes {
		if _, keep := newMap[key]; !keep {
			if err := rm.removeRouteLocked(rs); err != nil {
				logger().Warn("static: route skipped, kept rest of section",
					"prefix", rs.route.Prefix, "table", rs.route.Table, "reason", err)
			}
			delete(rm.routes, key)
		}
	}
	// A previously-skipped route that is no longer in the config is dropped: it
	// was never programmed, so there is nothing to remove from the FIB.
	for key := range rm.skipped {
		if _, keep := newMap[key]; !keep {
			delete(rm.skipped, key)
		}
	}

	for _, r := range routes {
		key := routeKey{table: r.Table, prefix: r.Prefix}
		existing := rm.routes[key]
		if existing != nil && routesEqual(existing.route, r) {
			continue
		}
		if err := rm.applyRouteLocked(r); err != nil {
			logger().Warn("static: route skipped, kept rest of section",
				"prefix", r.Prefix, "table", r.Table, "reason", err)
		}
	}

	// The link watches follow the committed set: a name no held route uses
	// any more stops being watched (Data Flow step 10). A rollback re-applies
	// the previous set through here too, so it re-derives them.
	rm.pruneLinksLocked()
	return nil
}

func (rm *routeManager) applyRouteLocked(r staticRoute) error {
	key := routeKey{table: r.Table, prefix: r.Prefix}
	// Capture the route being replaced (if any) before teardown, so that if the
	// replacement is then skipped we can reclaim its orphaned FIB entry and
	// announcement (the skip branch below). teardownRouteLocked does NOT touch
	// the backend, so the old kernel route survives a replace.
	var replacedRoute staticRoute
	var replacedProgrammed *staticRoute
	var hasReplaced, replacedEmitted bool
	if existing := rm.routes[key]; existing != nil {
		replacedRoute = existing.route
		replacedProgrammed = existing.programmed
		hasReplaced = true
		replacedEmitted = existing.emitted
		rm.teardownRouteLocked(existing)
	}

	// The replaced route's install survives its teardown, so the new state
	// inherits it: a replacement with no active member then withdraws it.
	rs := &routeState{
		route:      r,
		done:       make(chan struct{}),
		programmed: replacedProgrammed,
	}

	if r.Action == actionForward {
		rs.nhStates = make([]nhState, len(r.NextHops))
		for i, nh := range r.NextHops {
			deviceUp := false
			if nh.Dynamic {
				deviceUp = rm.ensureLinkLocked(nh.Interface).up
			}
			rs.nhStates[i] = newNHState(nh, deviceUp)
		}
		rm.setupBFDLocked(rs)
	}

	rm.routes[key] = rs
	if err := rm.programRouteLocked(rs); err != nil {
		// Per-route isolation: the FIB program failed. Undo the half-built state
		// so the diff baseline (rm.routes) never retains an unprogrammed route --
		// this keeps the routesEqual short-circuit honest for the good routes
		// (650 R-10 / AC-5) and lets this route be re-attempted on the next apply.
		// Record it in rm.skipped so the skip is observable (AC-3), never silent.
		rm.teardownRouteLocked(rs)
		delete(rm.routes, key)
		rm.skipped[key] = skippedRoute{route: r, reason: err.Error()}
		// If this skip REPLACED an existing route, the old kernel entry and its
		// announcement are now orphaned (teardown left the backend untouched, and
		// the replacement failed to program). Withdraw both so the FIB, the
		// redistribute announcement, and `static show` all agree the prefix is
		// UNROUTED and skipped (re-attempted next apply). This is not the 650
		// flap case: the route is genuinely gone, so a Remove is correct.
		if hasReplaced {
			// The failed program left the inherited install in place, so
			// rs.programmed is still the replaced route's install, or nil when
			// none of it was installed.
			if rs.programmed != nil {
				if remErr := rm.withdrawProgrammed(*rs.programmed); remErr != nil {
					logger().Warn("static: replaced route removal failed on skip",
						"prefix", replacedRoute.Prefix, "table", replacedRoute.Table, "error", remErr)
				}
			}
			// Withdraw the old announcement, but only if it was announced.
			if replacedEmitted {
				rm.emitRouteChange(redistevents.ActionRemove, replacedRoute)
			}
		}
		return err
	}
	// A route that programs clears any prior skip for its key (it resolved).
	delete(rm.skipped, key)
	return nil
}

func (rm *routeManager) removeRouteLocked(rs *routeState) error {
	rm.teardownRouteLocked(rs)
	if rs.programmed != nil {
		if err := rm.withdrawProgrammed(*rs.programmed); err != nil {
			logger().Warn("static: remove route failed", "prefix", rs.route.Prefix, "table", rs.route.Table, "error", err)
			return err
		}
		rs.programmed = nil
	}
	if rs.emitted {
		rm.emitRouteChange(redistevents.ActionRemove, rs.route)
		rs.emitted = false
	}
	return nil
}

func (rm *routeManager) teardownRouteLocked(rs *routeState) {
	rm.releaseBFDLocked(rs)
	close(rs.done)
}

func (rm *routeManager) programRouteLocked(rs *routeState) error {
	if rs.route.Action != actionForward {
		if err := rm.applyProgrammed(rs.route); err != nil {
			logger().Warn("static: apply route failed", "prefix", rs.route.Prefix, "table", rs.route.Table, "error", err)
			return err
		}
		installed := rs.route
		rs.programmed = &installed
		// A blackhole or reject route is a redistribute source, as in FRR
		// (zebra_redistribute_check tests no next-hop type) and BIRD
		// (static_announce_rte announces every r->dest).
		if !rs.emitted {
			rm.emitRouteChange(redistevents.ActionAdd, rs.route)
			rs.emitted = true
		}
		return nil
	}

	// A route with no active member is withdrawn, and stays in rm.routes: it is
	// not a skip, so `static show` lists it with its members inactive. A route
	// none of whose members was ever active (every member waiting, or every BFD
	// session down from the start) has nothing installed to withdraw.
	active := activeNextHops(rs)
	if len(active) == 0 {
		if rs.programmed != nil {
			if err := rm.withdrawProgrammed(*rs.programmed); err != nil {
				logger().Warn("static: withdraw route (all NHs down)", "prefix", rs.route.Prefix, "table", rs.route.Table, "error", err)
				return err
			}
			rs.programmed = nil
		}
		if rs.emitted {
			rm.emitRouteChange(redistevents.ActionRemove, rs.route)
			rs.emitted = false
		}
		return nil
	}

	programmed := rs.route
	programmed.NextHops = active
	if err := rm.applyProgrammed(programmed); err != nil {
		logger().Warn("static: apply route failed", "prefix", programmed.Prefix, "table", programmed.Table, "error", err)
		return err
	}
	rs.programmed = &programmed
	if !rs.emitted {
		rm.emitRouteChange(redistevents.ActionAdd, rs.route)
		rs.emitted = true
	}
	return nil
}

// staticBFDMode is the hop mode of every next-hop BFD session: a static
// next-hop is a directly connected neighbor. Commit (verifyStaticSections)
// checks a next-hop's profile against this same mode.
const staticBFDMode = bfdapi.SingleHop

func (rm *routeManager) setupBFDLocked(rs *routeState) {
	if rm.bfd == nil {
		return
	}
	for i := range rs.nhStates {
		nhs := &rs.nhStates[i]
		if nhs.nh.BFDProfile == "" {
			continue
		}
		req := bfdapi.SessionRequest{
			Peer:    nhs.nh.Address,
			Mode:    staticBFDMode,
			Profile: nhs.nh.BFDProfile,
		}
		if nhs.nh.Interface != "" {
			req.Interface = nhs.nh.Interface
		}
		handle, err := rm.bfd.EnsureSession(req)
		if err != nil {
			logger().Warn("static: BFD session failed", "peer", nhs.nh.Address, "error", err)
			continue
		}
		nhs.handle = handle
		ch := handle.Subscribe()

		key := routeKey{table: rs.route.Table, prefix: rs.route.Prefix}
		idx := i
		done := rs.done
		nhs.unsub = func() {
			handle.Unsubscribe(ch)
		}

		go rm.watchBFD(key, idx, handle, ch, done)
	}
}

// watchBFD follows one next-hop's BFD session until the route is torn down
// (done) or the channel closes because the BFD engine stopped
// (endVanishedBFDSession).
func (rm *routeManager) watchBFD(key routeKey, nhIdx int, handle bfdapi.SessionHandle, ch <-chan bfdapi.StateChange, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case sc, ok := <-ch:
			if !ok {
				rm.endVanishedBFDSession(key, nhIdx, handle)
				return
			}
			rm.mu.Lock()
			rs, exists := rm.routes[key]
			if !exists || nhIdx >= len(rs.nhStates) {
				rm.mu.Unlock()
				continue
			}

			if isDone(rs.done) {
				rm.mu.Unlock()
				return
			}

			// A snapshot (sc.Initial) needs no branch here, and it is a FIX
			// rather than a hazard: this handler asks only "is the session Up",
			// records the answer as BFD's input, and programs when the
			// next-hop's activity changes. A snapshot of Down takes an
			// untracked next-hop out of the group, and a snapshot of Up keeps
			// one whose session another client had already brought up, rather
			// than waiting for a transition to learn it.
			// BFD is one input to the next-hop's activity: a next-hop still
			// waiting for its device stays inactive whatever BFD says.
			nhs := &rs.nhStates[nhIdx]
			nhs.bfdUp = sc.State == bfdapi.StateUp

			if nhs.refresh() {
				_ = rm.programRouteLocked(rs)
				logger().Info("static: BFD state change",
					"prefix", key.prefix,
					"table", key.table,
					"nh", rs.nhStates[nhIdx].nh.Address,
					"state", sc.State,
				)
			}
			rm.mu.Unlock()
		}
	}
}

// endVanishedBFDSession is watchBFD's answer to its channel closing while the
// next-hop still holds handle: the BFD engine stopped (engine.Loop.Stop closes
// every subscriber channel), so the session is gone. The next-hop forgets the
// handle, with nothing to release into a loop that no longer exists, and stops
// being BFD-tracked: BFD allows it again, as it does when static runs without
// BFD, so it is active unless it waits for its device, and the route is
// reprogrammed if that changed its next-hop set.
// RFC 5882 Section 4.2: "If a BFD session transitions from Up state to
// AdminDown ... clients SHOULD NOT take any control protocol action", and an
// engine stopping is BFD switched off on this system, not a path failure.
//
// The warning that the next hop is no longer BFD-tracked is logged only while
// the daemon runs (registry.DaemonStopping): at daemon stop the BFD engine can
// stop before static does.
//
// A route replaced or removed since keeps its own state: its next-hop no
// longer holds handle, or the route no longer exists.
func (rm *routeManager) endVanishedBFDSession(key routeKey, nhIdx int, handle bfdapi.SessionHandle) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rs, exists := rm.routes[key]
	if !exists {
		return
	}
	if nhIdx >= len(rs.nhStates) {
		return
	}
	nhs := &rs.nhStates[nhIdx]
	if nhs.handle != handle {
		return
	}
	nhs.handle = nil
	nhs.unsub = nil
	if registry.DaemonStopping() {
		logger().Debug("static: BFD session ended with the daemon stop",
			"prefix", key.prefix, "table", key.table, "nh", nhs.nh.Address)
	} else {
		logger().Warn("static: BFD session ended because the BFD engine stopped; next hop no longer BFD-tracked",
			"prefix", key.prefix, "table", key.table, "nh", nhs.nh.Address)
	}
	nhs.bfdUp = true
	if !nhs.refresh() {
		return
	}
	if err := rm.programRouteLocked(rs); err != nil {
		logger().Warn("static: reprogramming after the BFD engine stopped failed",
			"prefix", key.prefix, "table", key.table, "error", err)
	}
}

func isDone(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default: //nolint:staticcheck // non-blocking done check, not a silent ignore
		return false
	}
}

func (rm *routeManager) releaseBFDLocked(rs *routeState) {
	if rm.bfd == nil {
		return
	}
	for i := range rs.nhStates {
		nhs := &rs.nhStates[i]
		if nhs.unsub != nil {
			nhs.unsub()
			nhs.unsub = nil
		}
		if nhs.handle != nil {
			_ = rm.bfd.ReleaseSession(nhs.handle)
			nhs.handle = nil
		}
	}
}

// shutdown withdraws every route and ends every link watch, then waits for
// the watchers outside mu: a watcher that holds an event is blocked on mu,
// and exits on its done channel once it takes it.
func (rm *routeManager) shutdown() {
	rm.mu.Lock()
	for key, rs := range rm.routes {
		_ = rm.removeRouteLocked(rs)
		delete(rm.routes, key)
	}
	for _, w := range rm.watches {
		rm.cancelLinkLocked(w)
	}
	rm.mu.Unlock()
	rm.linkWG.Wait()
}

func activeNextHops(rs *routeState) []nextHop {
	var active []nextHop
	for _, nhs := range rs.nhStates {
		if nhs.active {
			active = append(active, nhs.nh)
		}
	}
	return active
}

type showRoute struct {
	Prefix      string   `json:"prefix"`
	Table       uint32   `json:"table"`
	Action      string   `json:"action"`
	NextHops    []showNH `json:"next-hops,omitempty"`
	Metric      uint32   `json:"metric"`
	Tag         uint32   `json:"tag,omitempty"`
	Description string   `json:"description,omitempty"`
	// Skipped is true for a route the backend could not program (per-route
	// isolation, spec-fixit-static-per-route-isolation). Such a route is NOT in
	// the FIB; SkipReason names why. The operator sees it here so a skip is never
	// a silent no-op (AC-3, ai/rules/evidence.md).
	Skipped    bool   `json:"skipped,omitempty"`
	SkipReason string `json:"skip-reason,omitempty"`
}

type showNH struct {
	Address    string `json:"address"`
	Interface  string `json:"interface,omitempty"`
	Weight     uint16 `json:"weight"`
	BFDProfile string `json:"bfd-profile,omitempty"`
	Active     bool   `json:"active"`
}

func (rm *routeManager) showRoutes() []showRoute {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	out := make([]showRoute, 0, len(rm.routes)+len(rm.skipped))
	for _, rs := range rm.routes {
		sr := showRoute{
			Prefix:      rs.route.Prefix.String(),
			Table:       rs.route.Table,
			Action:      rs.route.Action.String(),
			Metric:      rs.route.Metric,
			Tag:         rs.route.Tag,
			Description: rs.route.Description,
		}
		for _, nhs := range rs.nhStates {
			sr.NextHops = append(sr.NextHops, showNH{
				Address:    nhs.nh.Address.String(),
				Interface:  nhs.nh.Interface,
				Weight:     nhs.nh.Weight,
				BFDProfile: nhs.nh.BFDProfile,
				Active:     nhs.active,
			})
		}
		out = append(out, sr)
	}
	// Skipped routes are not in the FIB (rm.routes); surface them here marked
	// skipped so the operator can see which prefixes are unrouted and why.
	for _, sk := range rm.skipped {
		sr := showRoute{
			Prefix:      sk.route.Prefix.String(),
			Table:       sk.route.Table,
			Action:      sk.route.Action.String(),
			Metric:      sk.route.Metric,
			Tag:         sk.route.Tag,
			Description: sk.route.Description,
			Skipped:     true,
			SkipReason:  sk.reason,
		}
		for _, nh := range sk.route.NextHops {
			sr.NextHops = append(sr.NextHops, showNH{
				Address:    nh.Address.String(),
				Interface:  nh.Interface,
				Weight:     nh.Weight,
				BFDProfile: nh.BFDProfile,
				Active:     false,
			})
		}
		out = append(out, sr)
	}
	slices.SortFunc(out, func(a, b showRoute) int {
		if c := cmp.Compare(a.Prefix, b.Prefix); c != 0 {
			return c
		}
		return cmp.Compare(a.Table, b.Table)
	})
	return out
}

// skippedRoutes returns a deterministic snapshot of the routes the backend could
// not program. Used by the doctor check to report skipped prefixes.
func (rm *routeManager) skippedRoutes() []skippedRoute {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	out := make([]skippedRoute, 0, len(rm.skipped))
	for _, sk := range rm.skipped {
		out = append(out, sk)
	}
	slices.SortFunc(out, func(a, b skippedRoute) int {
		if c := cmp.Compare(a.route.Prefix.String(), b.route.Prefix.String()); c != 0 {
			return c
		}
		return cmp.Compare(a.route.Table, b.route.Table)
	})
	return out
}

func (rm *routeManager) emitRouteChange(action redistevents.RouteAction, r staticRoute) {
	rm.emitRouteChangeID(action, r, 0)
}

// emitRouteChangeID emits a single-route batch tagged with replayID (0 for the
// normal incremental path; nonzero echoes a redistribute ReplayRequest so the
// orchestrator can replay to a newly-established peer or to a destination
// consumer that registered after this producer emitted). Every route in table 0
// is a redistribute source, forward, blackhole and reject alike.
func (rm *routeManager) emitRouteChangeID(action redistevents.RouteAction, r staticRoute, replayID uint64) {
	if r.Table != 0 {
		return
	}
	bus := getEventBus()
	if bus == nil {
		return
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = staticevents.ProtocolID
	b.ReplayID = replayID
	if r.Prefix.Addr().Is4() {
		b.AFI = 1
		b.SAFI = 1
	} else {
		b.AFI = 2
		b.SAFI = 1
	}
	b.Entries = append(b.Entries, redistevents.RouteChangeEntry{
		Action: action,
		Prefix: r.Prefix,
		Metric: r.Metric,
		Table:  r.Table,
		Tag:    r.Tag,
	})
	if _, err := staticevents.RouteChange.Emit(bus, b); err != nil {
		logger().Warn("static: route-change emit failed", "error", err)
	}
}

// reemitAll re-emits every currently-announced static route as an add tagged
// with replayID, so the redistribute orchestrator can replay them to a peer
// that establishes after the original emit. Reflects the CURRENT live set: only
// routes with emitted==true (table 0; a forward route needs an active next-hop)
// are re-emitted;
// a route withdrawn before the peer joined is absent. A zero replayID is a
// no-op (the orchestrator only allocates nonzero tokens).
func (rm *routeManager) reemitAll(replayID uint64) {
	if replayID == 0 {
		return
	}
	// Emit under rm.mu, the lock every incremental emit runs under: released
	// between the snapshot and the emit, a Remove of a snapshotted route could
	// reach the orchestrator before this replay's Add of it, and the consumer
	// would keep a route no longer configured.
	rm.mu.Lock()
	defer rm.mu.Unlock()
	for _, rs := range rm.routes {
		if rs.emitted {
			rm.emitRouteChangeID(redistevents.ActionAdd, rs.route, replayID)
		}
	}
}
