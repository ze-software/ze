// Design: docs/architecture/iface/logical-name-resolution.md -- the shared resolver and its link events
// Related: resolve.go -- bindEvents, the binding a config apply makes through bindResolverEvents
// Related: export_test.go -- BindResolverEventsForTest, the same binding for this package's own tests

package iface

import "github.com/ze-software/ze/pkg/ze"

// BindResolverEventsForConsumerTest subscribes the shared resolver to eb, the
// binding a config apply makes through bindResolverEvents, for a CONSUMER
// package's Linux test. A function in export_test.go is invisible outside this
// package, and static's TestStaticInterfaceRouteFollowsDevice follows a device
// that is deleted and recreated: only the bound resolver drops the cached
// binding of the deleted device, so the route is rebuilt through the new
// ifindex. No production code calls it.
//
// The returned function removes the subscriptions and lets a later test bind
// again; a test MUST call it in its cleanup, because the resolver is
// process-wide and binds only once.
func BindResolverEventsForConsumerTest(eb ze.EventBus) func() {
	return globalResolver.bindEventsUndoable(eb)
}

// bindEventsUndoable is bindEvents plus the function that undoes it, which a
// config apply never needs: the daemon binds once for its whole life.
func (r *resolver) bindEventsUndoable(eb ze.EventBus) func() {
	r.bindEvents(eb)
	return func() {
		r.mu.Lock()
		unsub := r.unsub
		r.unsub = nil
		r.bound = false
		r.mu.Unlock()
		for _, u := range unsub {
			u()
		}
	}
}
