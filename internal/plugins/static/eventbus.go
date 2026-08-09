// Design: docs/architecture/static-routes.md -- event bus integration

package static

import (
	"sync/atomic"

	"github.com/ze-software/ze/pkg/ze"
)

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
