// Design: docs/architecture/l2tp/subscriber-session-model.md -- framed routes through the session interface
// RFC: rfc/full/rfc2865.txt -- RFC 2865 Section 5.22 (Framed-Route)
// RFC: rfc/full/rfc3162.txt -- RFC 3162 Section 2.5 (Framed-IPv6-Route)
// Related: ncp.go -- onNCPOpened installs, withdrawNCPFamilyLocked withdraws
// Related: ipv6_service.go -- the IPv6 service owns the Framed-IPv6-Route receipts
// Related: session_handle.go -- SessionHandle.AddRoute and RemoveRoute, the per-session data plane

package ppp

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"slices"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// framedRouteReceipt is the exact route a session asked its data plane to
// install for one Framed-Route. Removal names the same four values, so the
// data plane withdraws only what this session installed, in the same shape:
// a device route is withdrawn as a device route.
type framedRouteReceipt struct {
	destination netip.Prefix
	// gateway is the subscriber's address, or the invalid Addr for a device
	// route through the session interface (newFramedRouteReceipt).
	gateway netip.Addr
	metric  int
}

// nextHop names the receipt's next hop for an operator message.
func (r framedRouteReceipt) nextHop() string {
	if !r.gateway.IsValid() {
		return "through the session interface"
	}
	return "via " + r.gateway.String()
}

// newFramedRouteReceipt resolves the route the session installs through its
// own interface. subscriber is the peer's address in the route's family: the
// address IPCP assigned for IPv4, the peer's link-local address for IPv6.
// RFC 2865 Section 5.22: "Whenever the gateway address is specified as "0.0.0.0" the IP address of the user SHOULD be used as the gateway address."
// RFC 3162 Section 2.5: "Whenever the gateway address is the IPv6 unspecified address the IP address of the user SHOULD be used as the gateway address."
// A gateway equal to the subscriber stays a route via the subscriber. Any
// other gateway becomes a device route through the session interface: the
// session is a point-to-point link whose only neighbor is the subscriber,
// so a packet sent to any next hop on it reaches the subscriber, exactly as
// a route via the subscriber does. Linux refuses a gateway no connected
// route covers, and Ze's owner-guarded session routes cannot carry
// RTNH_F_ONLINK (kernel patch 0008 refuses any rtm flag), so the device
// route is the one form that installs the route the server authorized.
func newFramedRouteReceipt(route l2tpevents.FramedRoute, subscriber netip.Addr) (framedRouteReceipt, error) {
	if !route.Prefix.IsValid() {
		return framedRouteReceipt{}, errors.New("framed route has no destination prefix")
	}
	ipv4 := route.Prefix.Addr().Is4()
	// refuse-by-default: guards pppSession.installNCPFramedRoutesIPv4 and IPv6Service.installFramedRoutes -- a zero Gateway names no gateway and is never read as the subscriber
	if !route.Gateway.IsValid() {
		return framedRouteReceipt{}, errors.New("framed route " + route.Prefix.String() + " carries no gateway")
	}
	if route.Gateway.Is4() != ipv4 {
		return framedRouteReceipt{}, errors.New("framed route " + route.Prefix.String() + " has a gateway of another family")
	}
	gateway := route.Gateway
	if gateway.IsUnspecified() {
		gateway = subscriber
	}
	if !gateway.IsValid() {
		return framedRouteReceipt{}, errors.New("framed route " + route.Prefix.String() + " names the subscriber, which has no address")
	}
	if gateway.Is4() != ipv4 {
		return framedRouteReceipt{}, errors.New("framed route " + route.Prefix.String() + " names the subscriber, whose address is of another family")
	}
	if uint64(route.Metric) > uint64(math.MaxInt) {
		return framedRouteReceipt{}, errors.New("framed route " + route.Prefix.String() + " metric exceeds the native width")
	}
	if gateway != subscriber {
		gateway = netip.Addr{}
	}
	return framedRouteReceipt{destination: route.Prefix.Masked(), gateway: gateway, metric: int(route.Metric)}, nil
}

// installNCPFramedRoutesIPv4 installs each IPv4 Framed-Route of the accepted
// profile through this session's interface, after its address. A route the
// data plane refuses fails the session: Ze never serves a subscriber a
// subset of the routes the server authorized. The receipt is recorded before
// each call, so the same gated withdrawal removes an uncertain add.
// It MUST run under the same owner gate as installNCPIPv4 and Close, and
// MUST release that gate before assignment publication.
// RFC 2865 Section 5.22: "This Attribute provides routing information to be configured for the user on the NAS."
func (s *pppSession) installNCPFramedRoutesIPv4() error {
	s.nativeReleaseMu.Lock()
	defer s.nativeReleaseMu.Unlock()
	s.mu.Lock()
	closed := s.filesClosed
	s.mu.Unlock()
	if closed {
		return ErrSessionNotFound
	}
	select {
	case <-s.stopCh:
		return ErrSessionNotFound
	case <-s.sessStop:
		return ErrSessionNotFound
	default:
	}
	if err := checkFramedRoutesLimit(s.authenticatedFramedRoutes); err != nil {
		return err
	}
	// Framed-IPv6-Route entries go to the IPv6 service, which installs them
	// once the peer's link-local address exists (framedRoutesOfFamily).
	for _, route := range framedRoutesOfFamily(s.authenticatedFramedRoutes, AddressFamilyIPv4) {
		receipt, err := newFramedRouteReceipt(route, s.peerIPv4)
		if err != nil {
			return err
		}
		// A server may list one route twice; the data plane owns it once.
		if slices.Contains(s.installedFramedIPv4, receipt) {
			continue
		}
		s.installedFramedIPv4 = append(s.installedFramedIPv4, receipt)
		if err := s.dataplane.AddRoute(receipt.destination, receipt.gateway, receipt.metric, rtproto.Iface); err != nil {
			return fmt.Errorf("framed route %s %s was refused, so the subscriber cannot be served all its authorized routes; "+
				"correct the subscriber's RADIUS Framed-Route (a 0.0.0.0 gateway names the subscriber): %w",
				receipt.destination, receipt.nextHop(), err)
		}
	}
	return nil
}

// withdrawNCPFramedRoutesIPv4 removes every retained receipt, newest first.
// A failed removal keeps that receipt and every older one for the retry.
// Caller MUST hold nativeReleaseMu, and MUST withdraw before the address the
// routes resolve through.
func (s *pppSession) withdrawNCPFramedRoutesIPv4() error {
	for index, receipt := range slices.Backward(s.installedFramedIPv4) {
		if err := s.dataplane.RemoveRoute(receipt.destination, receipt.gateway, receipt.metric, rtproto.Iface); err != nil {
			return err
		}
		s.installedFramedIPv4 = s.installedFramedIPv4[:index]
	}
	s.installedFramedIPv4 = nil
	return nil
}

// checkFramedRoutesLimit refuses an accepted profile carrying more framed
// routes, both families counted together, than one session installs. The
// RADIUS extraction keeps one entry past the limit as an overflow witness, so
// the whole set is refused here before any route reaches the data plane,
// rather than serving the subscriber a truncated authorization.
func checkFramedRoutesLimit(routes []l2tpevents.FramedRoute) error {
	if len(routes) > l2tpevents.FramedRoutesMax {
		return fmt.Errorf("the RADIUS authorization carries more than %d framed routes, the most one session installs, so none was installed; "+
			"reduce the subscriber's Framed-Route and Framed-IPv6-Route list to at most %d entries, then reconnect the subscriber",
			l2tpevents.FramedRoutesMax, l2tpevents.FramedRoutesMax)
	}
	return nil
}

// framedRoutesOfFamily returns the routes whose destination is of family, in
// the server's order. Each family's routes are installed by the owner of that
// family's addressing: IPCP for IPv4, the IPv6 service for IPv6.
func framedRoutesOfFamily(routes []l2tpevents.FramedRoute, family AddressFamily) []l2tpevents.FramedRoute {
	var selected []l2tpevents.FramedRoute
	for _, route := range routes {
		if route.Prefix.Addr().Is4() == (family == AddressFamilyIPv4) {
			selected = append(selected, route)
		}
	}
	return selected
}

// errFramedRouteRefused marks an IPv6 service start that failed because a
// Framed-IPv6-Route could not be installed. Unlike every other IPv6 service
// failure it fails the session: Ze never serves a subscriber a subset of the
// routes the server authorized.
var errFramedRouteRefused = errors.New("RADIUS Framed-IPv6-Route installation")

// installFramedRoutes installs each Framed-IPv6-Route of the accepted profile
// through the session interface, after the link-local pair, with the
// unspecified gateway replaced by the peer's link-local address and any
// gateway other than the peer installed as a device route. The receipt
// is recorded before each call, so Stop withdraws an uncertain add.
// RFC 3162 Section 2.5: "This Attribute provides routing information to be configured for the user on the NAS."
func (s *IPv6Service) installFramedRoutes() error {
	peerLL := linkLocalFromInterfaceID(s.cfg.PeerInterfaceID)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, route := range s.cfg.FramedRoutes {
		receipt, err := newFramedRouteReceipt(route, peerLL)
		if err != nil {
			return fmt.Errorf("%w: %w", errFramedRouteRefused, err)
		}
		// A server may list one route twice; the data plane owns it once.
		if slices.Contains(s.framedRoutes, receipt) {
			continue
		}
		s.framedRoutes = append(s.framedRoutes, receipt)
		if err := s.cfg.Dataplane.AddRoute(receipt.destination, receipt.gateway, receipt.metric, rtproto.Iface); err != nil {
			return fmt.Errorf("%w: framed route %s %s was refused, so the subscriber cannot be served all its authorized routes; "+
				"correct the subscriber's RADIUS Framed-IPv6-Route (a :: gateway names the subscriber): %w",
				errFramedRouteRefused, receipt.destination, receipt.nextHop(), err)
		}
	}
	return nil
}

// removeFramedRoutes withdraws every retained Framed-IPv6-Route receipt,
// newest first. A failed removal keeps that receipt and every older one for
// the retry. Callers MUST withdraw these before the link-local pair.
func (s *IPv6Service) removeFramedRoutes() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, receipt := range slices.Backward(s.framedRoutes) {
		if err := s.cfg.Dataplane.RemoveRoute(receipt.destination, receipt.gateway, receipt.metric, rtproto.Iface); err != nil {
			return err
		}
		s.framedRoutes = s.framedRoutes[:index]
	}
	s.framedRoutes = nil
	return nil
}
