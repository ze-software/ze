//go:build linux

// Design: docs/guide/plugins.md -- installed-route repair replay.
// Related: backend_linux.go -- ordinary replacement keeps its ownership scan.
// Upstream: github.com/vishvananda/netlink v1.3.1, route_linux.go;
// https://github.com/torvalds/linux/blob/v7.3/net/ipv4/fib_trie.c

package fibkernel

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/opererr"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// replayRouteOperations admits only a fresh identity probe and a noncreating
// change. Implementations require initialized, nonnil values, including typed
// nils held by an interface. No operation may retain or mutate the route.
type replayRouteOperations interface {
	RouteProbeIPv4(*netlink.Route) error
	RouteChange(*netlink.Route) error
}

func (n *netlinkBackend) reassertRichRoute(r RichRoute) (bool, error) {
	route, err := buildRichRoute(r)
	if err != nil {
		return true, err
	}
	return reassertOwnedRoute(n.handle, route)
}

// reassertOwnedRoute requires a nonnil rendered route. A fresh exact identity
// permits a change without CREATE, never a cache-only skip or a blind replace.
// Probe misses and a route removed before the change use the full ownership
// path. IPv6 retains that path because its no-flags insertion can mutate state.
func reassertOwnedRoute(ops replayRouteOperations, route *netlink.Route) (bool, error) {
	if route.Dst == nil {
		return false, nil
	}
	if route.Dst.IP.To4() == nil {
		return false, nil
	}
	err := ops.RouteProbeIPv4(route)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if err == nil {
		return true, opererr.Wrap(unix.EIO,
			fmt.Sprintf("route %s replay ownership was not confirmed", route.Dst),
			"The kernel returned an unexpected acknowledgement, so the replay stopped before changing forwarding",
			"ze show fib kernel")
	}
	if !errors.Is(err, unix.EEXIST) {
		return true, err
	}
	err = ops.RouteChange(route)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if errors.Is(err, unix.ESRCH) {
		return false, nil
	}
	return true, err
}
