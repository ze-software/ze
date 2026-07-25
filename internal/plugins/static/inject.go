// Design: plan/learned/710-gap-2-static-route-enhancements.md -- BFD integration and active NH tracking
// Related: doctor.go -- checkRouteSkipped reads routeManager.skipped + activeRouteManager

package static

import (
	"cmp"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"

	bfdapi "github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/core/redistevents"
	staticevents "github.com/ze-software/ze/internal/plugins/static/events"
)

type nhState struct {
	nh     nextHop
	active bool
	handle bfdapi.SessionHandle
	unsub  func()
}

type routeState struct {
	route    staticRoute
	nhStates []nhState
	done     chan struct{}
	emitted  bool
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
}

func newRouteManager(backend routeBackend) *routeManager {
	return &routeManager{
		backend: backend,
		routes:  make(map[routeKey]*routeState),
		skipped: make(map[routeKey]skippedRoute),
	}
}

// activeRouteManager points at the running static plugin's route manager, set in
// runStaticPlugin. The doctor check reads it to report routes the backend
// skipped at runtime. It is nil in the offline `ze doctor <config>` path (no
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

	return nil
}

func (rm *routeManager) applyRouteLocked(r staticRoute) error {
	key := routeKey{table: r.Table, prefix: r.Prefix}
	// Capture the route being replaced (if any) before teardown, so that if the
	// replacement is then skipped we can reclaim its orphaned FIB entry and
	// announcement (the skip branch below). teardownRouteLocked does NOT touch
	// the backend, so the old kernel route survives a replace.
	var replacedRoute staticRoute
	var hasReplaced, replacedEmitted, removeAlreadyEmitted bool
	if existing := rm.routes[key]; existing != nil {
		if existing.emitted && r.Action != actionForward {
			rm.emitRouteChange(redistevents.ActionRemove, existing.route)
			removeAlreadyEmitted = true
		}
		replacedRoute = existing.route
		hasReplaced = true
		replacedEmitted = existing.emitted
		rm.teardownRouteLocked(existing)
	}

	rs := &routeState{
		route: r,
		done:  make(chan struct{}),
	}

	if r.Action == actionForward {
		rs.nhStates = make([]nhState, len(r.NextHops))
		for i, nh := range r.NextHops {
			rs.nhStates[i] = nhState{nh: nh, active: true}
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
			if remErr := rm.backend.removeRoute(replacedRoute); remErr != nil {
				logger().Warn("static: replaced route removal failed on skip",
					"prefix", replacedRoute.Prefix, "table", replacedRoute.Table, "error", remErr)
			}
			// Withdraw the old announcement, but only if it was announced AND the
			// forward->non-forward branch above did not already emit its Remove.
			if replacedEmitted && !removeAlreadyEmitted {
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
	if err := rm.backend.removeRoute(rs.route); err != nil {
		logger().Warn("static: remove route failed", "prefix", rs.route.Prefix, "table", rs.route.Table, "error", err)
		return err
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
		if err := rm.backend.applyRoute(rs.route); err != nil {
			logger().Warn("static: apply route failed", "prefix", rs.route.Prefix, "table", rs.route.Table, "error", err)
			return err
		}
		return nil
	}

	active := activeNextHops(rs)
	if len(active) == 0 {
		if err := rm.backend.removeRoute(rs.route); err != nil {
			logger().Warn("static: withdraw route (all NHs down)", "prefix", rs.route.Prefix, "table", rs.route.Table, "error", err)
			return err
		}
		if rs.emitted {
			rm.emitRouteChange(redistevents.ActionRemove, rs.route)
			rs.emitted = false
		}
		return nil
	}

	programmed := rs.route
	programmed.NextHops = active
	if err := rm.backend.applyRoute(programmed); err != nil {
		logger().Warn("static: apply route failed", "prefix", programmed.Prefix, "table", programmed.Table, "error", err)
		return err
	}
	if !rs.emitted {
		rm.emitRouteChange(redistevents.ActionAdd, rs.route)
		rs.emitted = true
	}
	return nil
}

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
			Mode:    bfdapi.SingleHop,
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

		go rm.watchBFD(key, idx, ch, done)
	}
}

func (rm *routeManager) watchBFD(key routeKey, nhIdx int, ch <-chan bfdapi.StateChange, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case sc, ok := <-ch:
			if !ok {
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

			wasActive := rs.nhStates[nhIdx].active
			nowActive := sc.State == bfdapi.StateUp

			if wasActive != nowActive {
				rs.nhStates[nhIdx].active = nowActive
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

func (rm *routeManager) shutdown() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	for key, rs := range rm.routes {
		_ = rm.removeRouteLocked(rs)
		delete(rm.routes, key)
	}
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
	// a silent no-op (AC-3, ai/rules/fail-closed-guards.md).
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
// orchestrator can replay to a newly-established peer). Only forward routes in
// table 0 are redistribute sources; the guards match emitRouteChange.
func (rm *routeManager) emitRouteChangeID(action redistevents.RouteAction, r staticRoute, replayID uint64) {
	if r.Table != 0 {
		return
	}
	bus := getEventBus()
	if bus == nil {
		return
	}
	if r.Action != actionForward {
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
	})
	if _, err := staticevents.RouteChange.Emit(bus, b); err != nil {
		logger().Warn("static: route-change emit failed", "error", err)
	}
}

// reemitAll re-emits every currently-announced static route as an add tagged
// with replayID, so the redistribute orchestrator can replay them to a peer
// that establishes after the original emit. Reflects the CURRENT live set: only
// routes with emitted==true (forward, table 0, active next-hop) are re-emitted;
// a route withdrawn before the peer joined is absent. A zero replayID is a
// no-op (the orchestrator only allocates nonzero tokens).
func (rm *routeManager) reemitAll(replayID uint64) {
	if replayID == 0 {
		return
	}
	rm.mu.Lock()
	announced := make([]staticRoute, 0, len(rm.routes))
	for _, rs := range rm.routes {
		if rs.emitted {
			announced = append(announced, rs.route)
		}
	}
	rm.mu.Unlock()
	for i := range announced {
		rm.emitRouteChangeID(redistevents.ActionAdd, announced[i], replayID)
	}
}
