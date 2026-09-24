// Design: docs/architecture/core-design.md -- FIB Linux route monitor
// Overview: fibkernel.go -- FIB kernel plugin
// Related: monitor.go -- external change handling (platform-independent)
//
// Registers as a routewatch consumer to detect external route modifications
// on ze-managed prefixes and trigger re-assertion. The shared routewatch
// Watcher owns the single netlink subscription; this consumer receives
// parsed RouteEvent values, including Ze-owned deletions.

//go:build linux

package fibkernel

import (
	"context"

	"github.com/ze-software/ze/internal/core/routewatch"
)

// runMonitor closes ready after registration. The caller MUST cancel ctx and
// join runMonitor before closing the route backend.
func (f *fibKernel) runMonitor(ctx context.Context, w *routewatch.Watcher, ready chan<- struct{}) {
	unreg := w.Register(f.handleExternalChange)
	defer unreg()

	w.Start(func(err error) {
		logger().Warn("routewatch: monitor error", "error", err)
	})
	close(ready)

	logger().Info("fib-kernel: route monitor started (routewatch consumer)")

	<-ctx.Done()

	logger().Info("fib-kernel: route monitor stopped")
}
