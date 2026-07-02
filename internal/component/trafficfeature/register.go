package trafficfeature

import (
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/core/observation"
)

var initOnce sync.Once

// EnsureGlobal creates the process-wide trafficfeature service on first call.
// Safe to call from multiple goroutines and from init(). Mirrors trafficstat.
func EnsureGlobal() *Service {
	initOnce.Do(func() {
		if mr := registry.GetMetricsRegistry(); mr != nil {
			if r, ok := mr.(metrics.Registry); ok {
				BindMetrics(r)
			}
		}
		svc := NewService(observation.Global())
		SetGlobal(svc)
	})
	return Global()
}
