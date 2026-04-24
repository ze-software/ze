// Design: plan/spec-static-routes.md -- BFD integration and active NH tracking

package static

import (
	"errors"
	"net/netip"
	"slices"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/core/redistevents"
	bfdapi "codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
	staticevents "codeberg.org/thomas-mangin/ze/internal/plugins/static/events"
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

type routeManager struct {
	mu      sync.Mutex
	backend routeBackend
	routes  map[netip.Prefix]*routeState
	bfd     bfdapi.Service
}

func newRouteManager(backend routeBackend) *routeManager {
	return &routeManager{
		backend: backend,
		routes:  make(map[netip.Prefix]*routeState),
	}
}

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

	newMap := make(map[netip.Prefix]staticRoute, len(routes))
	for _, r := range routes {
		newMap[r.Prefix] = r
	}

	var errs []error
	for pfx, rs := range rm.routes {
		if _, keep := newMap[pfx]; !keep {
			if err := rm.removeRouteLocked(rs); err != nil {
				errs = append(errs, err)
			}
			delete(rm.routes, pfx)
		}
	}

	for _, r := range routes {
		existing := rm.routes[r.Prefix]
		if existing != nil && routesEqual(existing.route, r) {
			continue
		}
		if err := rm.applyRouteLocked(r); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (rm *routeManager) applyRouteLocked(r staticRoute) error {
	if existing := rm.routes[r.Prefix]; existing != nil {
		if existing.emitted && r.Action != actionForward {
			rm.emitRouteChange(redistevents.ActionRemove, existing.route)
		}
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

	rm.routes[r.Prefix] = rs
	return rm.programRouteLocked(rs)
}

func (rm *routeManager) removeRouteLocked(rs *routeState) error {
	rm.teardownRouteLocked(rs)
	if err := rm.backend.removeRoute(rs.route); err != nil {
		logger().Warn("static: remove route failed", "prefix", rs.route.Prefix, "error", err)
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
			logger().Warn("static: apply route failed", "prefix", rs.route.Prefix, "error", err)
			return err
		}
		return nil
	}

	active := activeNextHops(rs)
	if len(active) == 0 {
		if err := rm.backend.removeRoute(rs.route); err != nil {
			logger().Warn("static: withdraw route (all NHs down)", "prefix", rs.route.Prefix, "error", err)
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
		logger().Warn("static: apply route failed", "prefix", programmed.Prefix, "error", err)
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

		prefix := rs.route.Prefix
		idx := i
		done := rs.done
		nhs.unsub = func() {
			handle.Unsubscribe(ch)
		}

		go rm.watchBFD(prefix, idx, ch, done)
	}
}

func (rm *routeManager) watchBFD(prefix netip.Prefix, nhIdx int, ch <-chan bfdapi.StateChange, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case sc, ok := <-ch:
			if !ok {
				return
			}
			rm.mu.Lock()
			rs, exists := rm.routes[prefix]
			if !exists || nhIdx >= len(rs.nhStates) {
				rm.mu.Unlock()
				continue
			}

			select {
			case <-rs.done:
				rm.mu.Unlock()
				return
			default:
			}

			wasActive := rs.nhStates[nhIdx].active
			nowActive := sc.State == packet.StateUp

			if wasActive != nowActive {
				rs.nhStates[nhIdx].active = nowActive
				_ = rm.programRouteLocked(rs)
				logger().Info("static: BFD state change",
					"prefix", prefix,
					"nh", rs.nhStates[nhIdx].nh.Address,
					"state", sc.State,
				)
			}
			rm.mu.Unlock()
		}
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
	for pfx, rs := range rm.routes {
		_ = rm.removeRouteLocked(rs)
		delete(rm.routes, pfx)
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
	Action      string   `json:"action"`
	NextHops    []showNH `json:"next-hops,omitempty"`
	Metric      uint32   `json:"metric"`
	Tag         uint32   `json:"tag,omitempty"`
	Description string   `json:"description,omitempty"`
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

	out := make([]showRoute, 0, len(rm.routes))
	for _, rs := range rm.routes {
		sr := showRoute{
			Prefix:      rs.route.Prefix.String(),
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
	slices.SortFunc(out, func(a, b showRoute) int {
		if a.Prefix < b.Prefix {
			return -1
		}
		if a.Prefix > b.Prefix {
			return 1
		}
		return 0
	})
	return out
}

func (rm *routeManager) emitRouteChange(action redistevents.RouteAction, r staticRoute) {
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
	})
	if _, err := staticevents.RouteChange.Emit(bus, b); err != nil {
		logger().Warn("static: route-change emit failed", "error", err)
	}
}
