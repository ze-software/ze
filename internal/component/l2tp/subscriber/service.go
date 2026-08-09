// Design: docs/architecture/l2tp/subscriber-session-model.md -- service locator

package subscriber

import "sync/atomic"

// Service exposes the subscriber registry to CLI handlers and other
// components without import coupling. Same pattern as l2tp.PublishService.
type Service struct {
	Registry *Registry
}

// DefaultRegistry is the process-wide subscriber session registry.
// Both L2TP and PPPoE subsystems add/remove sessions here.
var DefaultRegistry = NewRegistry()

var svc atomic.Pointer[Service]

func init() {
	svc.Store(&Service{Registry: DefaultRegistry})
}

func LookupService() *Service { return svc.Load() }
