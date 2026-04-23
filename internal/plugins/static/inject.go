// Design: plan/spec-static-routes.md -- BFD integration and active NH tracking

package static

import (
	"net/netip"
	"sync"

	bfdapi "codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
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

func (rm *routeManager) applyRoutes(routes []staticRoute) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	newMap := make(map[netip.Prefix]staticRoute, len(routes))
	for _, r := range routes {
		newMap[r.Prefix] = r
	}

	for pfx, rs := range rm.routes {
		if _, keep := newMap[pfx]; !keep {
			rm.removeRouteLocked(rs)
			delete(rm.routes, pfx)
		}
	}

	for _, r := range routes {
		existing := rm.routes[r.Prefix]
		if existing != nil && routesEqual(existing.route, r) {
			continue
		}
		rm.applyRouteLocked(r)
	}
}

func (rm *routeManager) applyRouteLocked(r staticRoute) {
	if existing := rm.routes[r.Prefix]; existing != nil {
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
	rm.programRouteLocked(rs)
}

func (rm *routeManager) removeRouteLocked(rs *routeState) {
	rm.teardownRouteLocked(rs)
	if err := rm.backend.removeRoute(rs.route); err != nil {
		logger().Warn("static: remove route failed", "prefix", rs.route.Prefix, "error", err)
	}
}

func (rm *routeManager) teardownRouteLocked(rs *routeState) {
	rm.releaseBFDLocked(rs)
	close(rs.done)
}

func (rm *routeManager) programRouteLocked(rs *routeState) {
	if rs.route.Action != actionForward {
		if err := rm.backend.applyRoute(rs.route); err != nil {
			logger().Warn("static: apply route failed", "prefix", rs.route.Prefix, "error", err)
		}
		return
	}

	active := activeNextHops(rs)
	if len(active) == 0 {
		if err := rm.backend.removeRoute(rs.route); err != nil {
			logger().Warn("static: withdraw route (all NHs down)", "prefix", rs.route.Prefix, "error", err)
		}
		return
	}

	programmed := rs.route
	programmed.NextHops = active
	if err := rm.backend.applyRoute(programmed); err != nil {
		logger().Warn("static: apply route failed", "prefix", programmed.Prefix, "error", err)
	}
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
				rm.programRouteLocked(rs)
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
		rm.removeRouteLocked(rs)
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
