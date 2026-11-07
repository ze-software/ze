// Design: docs/architecture/static-routes.md -- device presence as an activity input
// Related: inject.go -- nhState.waiting, refresh and programRouteLocked, which a link event drives
// Related: internal/component/iface/resolve.go -- Subscribe and Resolve, the link facts static reads

package static

import (
	"github.com/ze-software/ze/internal/component/iface"
)

// linkSource is where the route manager learns whether the device of a
// declared on-demand interface (nextHop.Dynamic) is up. Production reads the
// shared iface resolver (ifaceLinks); a unit test drives the events itself.
//
// Both methods are process-local, like everything iface publishes: a static
// that runs forked hears nothing (warnIfExternal).
type linkSource interface {
	// subscribe returns the link events of the logical name and the function
	// that ends the subscription and closes the channel. The caller MUST call
	// that function.
	subscribe(name string) (<-chan iface.LinkEvent, func())
	// state reads the device as it is now: whether it is up, and its ifindex.
	state(name string) (up bool, index int)
}

// ifaceLinks is the production linkSource: the shared iface resolver.
type ifaceLinks struct{}

func (ifaceLinks) subscribe(name string) (<-chan iface.LinkEvent, func()) {
	return iface.Subscribe(name)
}

// state answers from iface.Resolve, the read every other consumer makes. A
// name that does not resolve has no device to program a route through: no
// backend is loaded, or the session that creates the device is not up.
func (ifaceLinks) state(name string) (bool, int) {
	b, err := iface.Resolve(name)
	// refuse-by-default: guards ensureLinkLocked -- an unresolvable name is an absent device, and a member through it waits
	if err != nil {
		return false, 0
	}
	return b.State == iface.LinkStateUp, b.Ifindex
}

// linkWatch follows one declared on-demand interface for every committed
// route that names it. One watch per name, however many routes and members
// use it.
//
// Lifecycle: ensureLinkLocked subscribes, reads the first state and starts
// watchLink; cancelLinkLocked closes done and ends the subscription; the
// goroutine exits on either. All fields are guarded by routeManager.mu.
type linkWatch struct {
	name   string
	up     bool
	index  int
	cancel func()
	done   chan struct{}
}

// ensureLinkLocked returns the watch of name, creating it on first use.
//
// The subscription is made BEFORE the first read, so a change between the two
// still arrives as an event and the watch ends in the device's real state.
//
// REQUIRES: the caller holds rm.mu.
func (rm *routeManager) ensureLinkLocked(name string) *linkWatch {
	if w := rm.watches[name]; w != nil {
		return w
	}
	events, cancel := rm.links.subscribe(name)
	w := &linkWatch{name: name, cancel: cancel, done: make(chan struct{})}
	w.up, w.index = rm.links.state(name)
	rm.watches[name] = w
	rm.linkWG.Add(1)
	go rm.watchLink(w, events)
	return w
}

// pruneLinksLocked cancels the watch of every name no route in rm.routes uses
// any more. rm.routes is the committed set, so an apply and the rollback that
// re-applies the previous set both leave exactly the watches their routes
// need. A skipped route is not in rm.routes: it is re-attempted only on the
// next apply, which subscribes again.
//
// REQUIRES: the caller holds rm.mu.
func (rm *routeManager) pruneLinksLocked() {
	used := make(map[string]struct{}, len(rm.watches))
	for _, rs := range rm.routes {
		for i := range rs.nhStates {
			if nh := rs.nhStates[i].nh; nh.Dynamic {
				used[nh.Interface] = struct{}{}
			}
		}
	}
	for name, w := range rm.watches {
		if _, keep := used[name]; keep {
			continue
		}
		rm.cancelLinkLocked(w)
	}
}

// cancelLinkLocked ends one watch. It does not wait for watchLink: that
// goroutine may be blocked on rm.mu, which the caller holds. shutdown waits
// for every watcher once it has released the lock.
//
// REQUIRES: the caller holds rm.mu.
func (rm *routeManager) cancelLinkLocked(w *linkWatch) {
	close(w.done)
	w.cancel()
	delete(rm.watches, w.name)
}

// watchLink follows one watch's events until the watch is canceled (done)
// or its subscription closes. One goroutine per watched name for the life of
// the watch, never one per event (ai/rules/goroutine-lifecycle.md).
func (rm *routeManager) watchLink(w *linkWatch, events <-chan iface.LinkEvent) {
	defer rm.linkWG.Done()
	for {
		select {
		case <-w.done:
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			rm.onLinkEvent(w, ev)
		}
	}
}

// onLinkEvent records one link event as the device state of every member
// that names the watched interface.
func (rm *routeManager) onLinkEvent(w *linkWatch, ev iface.LinkEvent) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	// The watch was canceled while this event waited for the lock: the
	// routes that used it are gone or no longer name it.
	if isDone(w.done) {
		return
	}
	switch ev.Kind {
	case iface.LinkUp:
		rm.setLinkLocked(w, true, ev.Index)
	case iface.LinkDown:
		// A deleted device arrives as down too (the iface monitor maps
		// RTM_DELLINK onto its down event).
		rm.setLinkLocked(w, false, ev.Index)
	case iface.LinkAppeared:
		// A created device is not yet known to be up: the monitor follows its
		// creation with the up or the down event, which decides.
	}
}

// setLinkLocked records the device state and reprograms each route whose
// membership changed. A device that is up again under a new ifindex (deleted
// and recreated, with the down event lost to a full subscriber channel)
// reprograms its routes even though the member was already active: the
// kernel flushed the old install with the old device.
//
// REQUIRES: the caller holds rm.mu.
func (rm *routeManager) setLinkLocked(w *linkWatch, up bool, index int) {
	moved := up && w.up && index != w.index
	if up == w.up && !moved {
		return
	}
	w.up = up
	w.index = index
	for _, rs := range rm.routes {
		if !markDevice(rs, w.name, up, moved) {
			continue
		}
		if err := rm.programRouteLocked(rs); err != nil {
			logger().Warn("static: reprogramming after a device change failed",
				"prefix", rs.route.Prefix, "table", rs.route.Table, "interface", w.name, "up", up, "error", err)
		}
	}
}

// markDevice sets the waiting input of every member of rs that names the
// device, and reports whether rs needs reprogramming: a member's activity
// changed, or an active member's device moved to a new ifindex.
//
// REQUIRES: the caller holds the route manager's lock.
func markDevice(rs *routeState, name string, up, moved bool) bool {
	reprogram := false
	for i := range rs.nhStates {
		nhs := &rs.nhStates[i]
		if !nhs.nh.Dynamic {
			continue
		}
		if nhs.nh.Interface != name {
			continue
		}
		nhs.waiting = !up
		if nhs.refresh() {
			reprogram = true
			continue
		}
		if moved && nhs.active {
			reprogram = true
		}
	}
	return reprogram
}
