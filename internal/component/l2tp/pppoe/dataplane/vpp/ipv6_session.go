// Design: docs/architecture/l2tp/bng-5-pppoe.md -- negotiated IPv6 carrier binding
// Related: session_routes.go -- address-pair ownership; ipv6_carrier.go -- lifetime
package vpp

import (
	"context"
	"errors"
	"io"
	"net/netip"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// OpenIPv6Carrier MUST follow the common service's post-ACK Connect. IPv6CP's
// actual link-local pair identifies this point-to-point link; neither access
// MAC nor a newly constructed interface identifier can stand in for it.
// RFC 5072 Section 5: "The negotiated interface identifier is used by the
// local end of the PPP link to autoconfigure an IPv6 link-local unicast
// address for the PPP interface."
func (s *session) OpenIPv6Carrier(ctx context.Context) (ppp.IPv6Carrier, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, io.ErrClosedPipe
	}
	var local, peer netip.Addr
	for address, route := range s.addresses {
		if !address.Addr().Is6() {
			continue
		}
		if !address.Addr().IsLinkLocalUnicast() {
			continue
		}
		if !route.destination.Addr().Is6() {
			return nil, errors.New("pppoe-vpp: negotiated IPv6 peer has another address family")
		}
		if !route.destination.Addr().IsLinkLocalUnicast() {
			return nil, errors.New("pppoe-vpp: negotiated IPv6 link-local peer is unavailable")
		}
		if _, staged := s.stagedRoutes[route]; staged {
			continue
		}
		if _, published := s.routes[route]; !published {
			continue
		}
		if s.retiringPeerRoute(route) {
			continue
		}
		if local.IsValid() {
			return nil, errors.New("pppoe-vpp: multiple IPv6 link-local address pairs")
		}
		local, peer = address.Addr(), route.destination.Addr()
	}
	if !local.IsValid() {
		return nil, errors.New("pppoe-vpp: IPv6CP link-local address pair is not installed")
	}
	return s.socket.openIPv6Carrier(local, peer)
}
