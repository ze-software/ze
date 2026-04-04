// Design: docs/architecture/core-design.md -- FIB Linux route monitor
// Overview: fibkernel.go -- FIB kernel plugin
// Related: monitor.go -- external change handling (platform-independent)
// Related: backend_linux.go -- Linux netlink backend
//
// Monitors kernel routing table changes via netlink multicast groups
// RTNLGRP_IPV4_ROUTE and RTNLGRP_IPV6_ROUTE. Detects external route
// modifications (from other daemons or manual ip route commands) and
// triggers re-assertion of ze-managed routes.

//go:build linux

package fibkernel

import (
	"context"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// runMonitor subscribes to kernel route change notifications via netlink
// multicast groups. Filters by rtm_protocol: ignores ze's own changes,
// calls handleExternalChange for modifications on ze-managed prefixes.
// Blocks until ctx is canceled.
func (f *fibKernel) runMonitor(ctx context.Context) {
	updates := make(chan netlink.RouteUpdate, 64)
	done := make(chan struct{})
	defer close(done) // Always signal netlink to stop, preventing goroutine leak.

	opts := netlink.RouteSubscribeOptions{
		ListExisting:  false,
		ErrorCallback: func(err error) { logger().Warn("fib-kernel: monitor error", "error", err) },
	}
	if err := netlink.RouteSubscribeWithOptions(updates, done, opts); err != nil {
		logger().Error("fib-kernel: route subscribe failed", "error", err)
		return
	}

	logger().Info("fib-kernel: route monitor started")

	for {
		select {
		case <-ctx.Done():
			logger().Info("fib-kernel: route monitor stopped")
			return

		case update, ok := <-updates:
			if !ok {
				logger().Warn("fib-kernel: route update channel closed")
				return
			}

			// Ignore ze's own route changes.
			if update.Protocol == rtprotZE {
				continue
			}

			// Handle both route additions/replacements and deletions on ze-managed prefixes.
			if update.Type != unix.RTM_NEWROUTE && update.Type != unix.RTM_DELROUTE {
				continue
			}

			if update.Dst == nil {
				continue
			}

			prefix := update.Dst.String()
			var nextHop string
			if update.Gw != nil {
				nextHop = update.Gw.String()
			}

			f.handleExternalChange(prefix, nextHop, int(update.Protocol))
		}
	}
}
