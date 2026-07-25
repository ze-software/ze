package trafficstat

import (
	"sync"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/observation"
)

var initOnce sync.Once

// EnsureGlobal creates the process-wide trafficstat service on first
// call. Safe to call from multiple goroutines and from init().
func EnsureGlobal() *Service {
	initOnce.Do(func() {
		if mr := registry.GetMetricsRegistry(); mr != nil {
			BindMetrics(mr)
		}
		svc := NewService(observation.Global())
		SetGlobal(svc)
	})
	return Global()
}
