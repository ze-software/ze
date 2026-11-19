// Design: docs/architecture/l2tp/bng-5-pppoe.md -- per-session VPP route ownership
// Related: api_routes.go -- native source and path fields; session.go -- teardown
package vpp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// This bound exceeds the wire-carried route attributes of one subscriber and
// bounds retained cleanup work even for an internal caller constructing routes.
const sessionRouteMax = 1024

type sessionRoute struct {
	destination netip.Prefix
	gateway     netip.Addr
	metric      int
	protocol    rtproto.Proto
}

// AddAddressP2P stages the exact peer path without publishing native egress.
// The common owner MUST acknowledge the assignment before calling Connect;
// Connect MUST follow every newly installed NCP address pair.
func (s *session) AddAddressP2P(local, peer netip.Prefix) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if !local.IsValid() {
		return errors.New("pppoe-vpp: local address prefix is invalid")
	}
	if !peer.IsValid() {
		return errors.New("pppoe-vpp: peer address prefix is invalid")
	}
	if local.Addr().Is4() != peer.Addr().Is4() {
		return errors.New("pppoe-vpp: point-to-point address families differ")
	}
	if local.Addr().Is4In6() {
		return errors.New("pppoe-vpp: mapped IPv4 local address is unsupported")
	}
	if peer.Addr().Is4In6() {
		return errors.New("pppoe-vpp: mapped IPv4 peer address is unsupported")
	}
	if _, exists := s.addresses[local]; exists {
		return errors.New("pppoe-vpp: local address already owns a peer route")
	}
	if _, exists := s.withdrawals[local]; exists {
		return errors.New("pppoe-vpp: local address has a pending peer-route withdrawal")
	}
	route := sessionRoute{destination: netip.PrefixFrom(peer.Addr(), peer.Addr().BitLen()), protocol: rtproto.Iface}
	if err := s.stageRoute(route); err != nil {
		return err
	}
	s.addresses[local] = route
	return nil
}

func (s *session) RemoveAddress(local netip.Prefix) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	if !local.IsValid() {
		return errors.New("pppoe-vpp: local address prefix is invalid")
	}
	if local.Addr().Is4In6() {
		return errors.New("pppoe-vpp: mapped IPv4 local address is unsupported")
	}
	route, exists := s.withdrawals[local]
	if !exists {
		route, exists = s.addresses[local]
		if !exists {
			// The common owner retains cleanup before an add can partially
			// mutate. A refused add owns nothing, and its cleanup MUST remain
			// harmless without touching a path another owner installed.
			return nil
		}
		if s.withdrawals == nil {
			s.withdrawals = make(map[netip.Prefix]sessionRoute)
		}
		s.withdrawals[local] = route
		delete(s.addresses, local)
	}
	// Ending the pair cancels its publication intent before any operation can
	// fail. A possible native path remains independently owned in routes.
	delete(s.stagedRoutes, route)
	ipv6 := local.Addr().Is6()
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()
	if s.addressFamilyCount(ipv6) == 0 {
		if s.ready {
			if err := s.api.setIP(ctx, s.identity.NativeIndex, ipv6, false); err != nil {
				return err
			}
		}
		err := s.withdrawFamily(ctx, ipv6)
		if !s.hasNativeFamily(ipv6) {
			delete(s.withdrawals, local)
		}
		return err
	}
	if _, native := s.routes[route]; !native {
		delete(s.withdrawals, local)
		return nil
	}
	err := s.removeRoute(route)
	if _, retained := s.routes[route]; !retained {
		delete(s.withdrawals, local)
	}
	return err
}

func (s *session) AddRoute(destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	route := sessionRoute{destination: destination, gateway: gateway, metric: metric, protocol: protocol}
	if s.ready {
		if s.addressFamilyCount(destination.Addr().Is6()) != 0 {
			return s.addRoute(route)
		}
	}
	return s.stageRoute(route)
}

func (s *session) RemoveRoute(destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	return s.removeRoute(sessionRoute{destination: destination, gateway: gateway, metric: metric, protocol: protocol})
}

func (s *session) addRoute(route sessionRoute) error {
	if err := validateRoute(route); err != nil {
		return err
	}
	if _, exists := s.routes[route]; exists {
		return errors.New("pppoe-vpp: route already belongs to this session")
	}
	if _, exists := s.stagedRoutes[route]; exists {
		return errors.New("pppoe-vpp: route already awaits publication")
	}
	if s.routeCount() >= sessionRouteMax {
		return errors.New("pppoe-vpp: session route limit reached")
	}
	receipt, err := s.programRoute(context.Background(), true, route)
	if receipt != routeRefused {
		// Both acknowledged and uncertain adds remain teardown obligations.
		s.routes[route] = struct{}{}
	}
	return err
}

func (s *session) removeRoute(route sessionRoute) error {
	if err := validateRoute(route); err != nil {
		return err
	}
	if _, exists := s.routes[route]; !exists {
		if _, staged := s.stagedRoutes[route]; staged {
			delete(s.stagedRoutes, route)
			return nil
		}
		// A refused add may leave only the common owner's cleanup intent.
		// Exact absence here is a no-op, never a native delete by prefix.
		return nil
	}
	// A requested withdrawal cannot become an add again after a failed delete.
	// Only the exact native cleanup obligation survives this transition.
	delete(s.stagedRoutes, route)
	receipt, err := s.programRoute(context.Background(), false, route)
	if receipt == routeConfirmed {
		delete(s.routes, route)
		delete(s.stagedRoutes, route)
	}
	return err
}

func (s *session) programRoute(ctx context.Context, add bool, route sessionRoute) (routeReceipt, error) {
	family := 0
	if route.destination.Addr().Is6() {
		family = 1
	}
	return s.api.route(ctx, add, s.identity.NativeIndex, s.tables[family], s.source, route.destination, route.gateway, route.metric)
}

// A staged pair cannot be a published family witness. A retained failed
// withdrawal has already left addresses, so another family's Connect cannot
// reopen it. The common PPP owner remains the sole NCP automaton.
func (s *session) addressFamilyCount(ipv6 bool) int {
	count := 0
	for local, route := range s.addresses {
		if local.Addr().Is6() == ipv6 {
			if _, staged := s.stagedRoutes[route]; staged {
				continue
			}
			count++
		}
	}
	return count
}

func (s *session) hasAddressFamily(ipv6 bool) bool {
	for local := range s.addresses {
		if local.Addr().Is6() == ipv6 {
			return true
		}
	}
	return false
}

func (s *session) hasWithdrawalFamily(ipv6 bool) bool {
	for local := range s.withdrawals {
		if local.Addr().Is6() == ipv6 {
			return true
		}
	}
	return false
}

func (s *session) hasNativeFamily(ipv6 bool) bool {
	for route := range s.routes {
		if route.destination.Addr().Is6() == ipv6 {
			return true
		}
	}
	return false
}

func (s *session) retiringPeerRoute(route sessionRoute) bool {
	for _, peer := range s.withdrawals {
		if peer == route {
			return true
		}
	}
	return false
}

// The last pair's withdrawal removes native egress for that family too.
// Generic route intents wait for a newly acknowledged pair; retiring peer
// paths never become new publication intents. A failed native delete keeps
// both cleanup and generic re-publication obligations, with Connect blocked
// by the family's retained withdrawal owner until cleanup is confirmed.
func (s *session) withdrawFamily(ctx context.Context, ipv6 bool) error {
	var result error
	for route := range s.routes {
		if route.destination.Addr().Is6() != ipv6 {
			continue
		}
		if !s.retiringPeerRoute(route) {
			if s.stagedRoutes == nil {
				s.stagedRoutes = make(map[sessionRoute]struct{})
			}
			s.stagedRoutes[route] = struct{}{}
		}
		receipt, err := s.programRoute(ctx, false, route)
		if receipt == routeConfirmed {
			delete(s.routes, route)
		}
		result = errors.Join(result, err)
	}
	return result
}

// Resource accounting counts the union: an uncertain staged add owns both a
// publication obligation and one possible native path, never two resources.
func (s *session) routeCount() int {
	count := len(s.routes)
	for route := range s.stagedRoutes {
		if _, owned := s.routes[route]; !owned {
			count++
		}
	}
	return count
}

func (s *session) stageRoute(route sessionRoute) error {
	if err := validateRoute(route); err != nil {
		return err
	}
	if _, owned := s.routes[route]; owned {
		return errors.New("pppoe-vpp: route already belongs to this session")
	}
	if _, staged := s.stagedRoutes[route]; staged {
		return errors.New("pppoe-vpp: route already awaits publication")
	}
	if s.routeCount() >= sessionRouteMax {
		return errors.New("pppoe-vpp: session route limit reached")
	}
	if s.stagedRoutes == nil {
		s.stagedRoutes = make(map[sessionRoute]struct{})
	}
	s.stagedRoutes[route] = struct{}{}
	return nil
}

// VPP's fib_path_list_copy_and_path_add discards an exact duplicate path.
// Reasserting an uncertain add therefore confirms this same owned intent
// without stacking a reference or replacing another subscriber's path.
func (s *session) publishRoutes(ctx context.Context, ipv6 bool) error {
	var result error
	for route := range s.stagedRoutes {
		if route.destination.Addr().Is6() != ipv6 {
			continue
		}
		receipt, err := s.programRoute(ctx, true, route)
		if receipt != routeRefused {
			s.routes[route] = struct{}{}
		}
		if receipt == routeConfirmed {
			delete(s.stagedRoutes, route)
		}
		result = errors.Join(result, err)
	}
	return result
}

func validateRoute(route sessionRoute) error {
	if !route.destination.IsValid() {
		return errors.New("pppoe-vpp: route destination is invalid")
	}
	if route.destination.Addr().Is4In6() {
		return errors.New("pppoe-vpp: mapped IPv4 route is unsupported")
	}
	if route.destination != route.destination.Masked() {
		return errors.New("pppoe-vpp: route destination has host bits")
	}
	if route.gateway.IsValid() {
		if route.gateway.Is4In6() {
			return errors.New("pppoe-vpp: mapped IPv4 gateway is unsupported")
		}
		if route.gateway.Is4() != route.destination.Addr().Is4() {
			return errors.New("pppoe-vpp: route gateway has another address family")
		}
	}
	if route.metric < 0 {
		return errors.New("pppoe-vpp: route metric is negative")
	}
	if route.metric > 255 {
		return fmt.Errorf("pppoe-vpp: route metric %d exceeds VPP path preference", route.metric)
	}
	if route.protocol == rtproto.Any {
		return errors.New("pppoe-vpp: route has no explicit producer ownership")
	}
	return nil
}
