package netlink

import (
	"fmt"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// RouteProbeIPv4 checks an IPv4 route's complete kernel identity without
// creating, replacing or deleting a route. EEXIST identifies an exact match;
// ENOENT identifies a miss. Other errors do not establish either outcome.
// IPv6 is deliberately unsupported: its insertion path can change routes even
// when NLM_F_CREATE and NLM_F_REPLACE are absent.
func (h *Handle) RouteProbeIPv4(route *Route) error {
	if h == nil {
		return fmt.Errorf("IPv4 route identity probe requires a handle: %w", unix.EINVAL)
	}
	if route == nil {
		return fmt.Errorf("IPv4 route identity probe requires a route: %w", unix.EINVAL)
	}
	if route.Dst == nil {
		return fmt.Errorf("IPv4 route identity probe requires a destination: %w", unix.EINVAL)
	}
	if route.Dst.IP.To4() == nil {
		return unix.EAFNOSUPPORT
	}
	req := h.newNetlinkRequest(unix.RTM_NEWROUTE, unix.NLM_F_ACK)
	_, err := h.routeHandle(route, req, nl.NewRtMsg())
	return err
}
