// Design: docs/architecture/core-design.md -- FIB Linux route monitor
// Overview: fibkernel.go -- FIB kernel plugin
// Related: monitor.go -- external change handling (platform-independent)
//
// Registers as a routewatch consumer to detect external route modifications
// on ze-managed prefixes and trigger re-assertion. The shared routewatch
// Watcher owns the single netlink subscription; this consumer receives
// parsed RouteEvent values with Ze-owned routes already filtered.

//go:build linux

package fibkernel

import (
	"context"

	"github.com/ze-software/ze/internal/core/routewatch"
)

func (f *fibKernel) runMonitor(ctx context.Context) {
	w := routewatch.Global()

	unreg := w.Register(func(ev routewatch.RouteEvent) {
		var nextHop string
		if ev.NextHop.IsValid() {
			nextHop = ev.NextHop.String()
		}
		f.handleExternalChange(ev.Prefix.String(), nextHop, ev.Protocol)
	})
	defer unreg()

	w.Start(func(err error) {
		logger().Warn("routewatch: monitor error", "error", err)
	})

	logger().Info("fib-kernel: route monitor started (routewatch consumer)")

	<-ctx.Done()

	logger().Info("fib-kernel: route monitor stopped")
}
