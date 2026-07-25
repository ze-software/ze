// Design: docs/architecture/core-design.md -- EventBus atomic storage

package as112

import (
	"sync/atomic"

	"github.com/ze-software/ze/pkg/ze"
)

// eventBusPtr holds the hub-owned EventBus injected via the ConfigureEventBus
// registration hook (register.go). The as112 redistribute producer reads it
// through getEventBus to emit route-change batches on the in-process bus.
var eventBusPtr atomic.Pointer[ze.EventBus]

func setEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}
