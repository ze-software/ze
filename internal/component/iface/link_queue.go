// Design: docs/architecture/iface/management.md -- carrier events are queued per interface
// Related: register.go -- the subscribers that push here, the handlers the worker calls
// Related: rate.go -- the collect tick that feeds the carrier resync, and the counters

package iface

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/events"
	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/pkg/ze"
)

// linkEventClass names what a queue entry is about. It is half of the
// coalescing key, so a carrier transition never cancels a router transition.
type linkEventClass uint8

const (
	// linkEventCarrier is a carrier up or down for one interface.
	linkEventCarrier linkEventClass = iota
	// linkEventRouter is a router discovered or lost on one interface.
	linkEventRouter
	// linkEventResync asks the worker to compare acted-on route metric state
	// against live carrier state and repair a contradiction.
	linkEventResync
)

// linkEventKey identifies the subject whose LAST state the queue keeps. Two
// pushes with the same key coalesce: the later one wins, and the earlier one is
// never applied.
//
// The key is the interface for a carrier event, so a down followed by an up
// leaves one entry that says up. It is the (interface, router) pair for a
// router event, so two routers on one interface stay independent. It carries no
// subject for a resync, so at most one resync is ever pending.
type linkEventKey struct {
	class     linkEventClass
	ifaceName string
	routerIP  string
}

// linkEventValue is the state the queue holds for one key. Which field carries
// the meaning depends on the key's class, and each class ignores the fields it
// does not name.
type linkEventValue struct {
	// present is the state the subject ended in: carrier up for
	// linkEventCarrier, router discovered for linkEventRouter.
	present bool
	// payload is the raw router-event JSON that handleRouterDiscovered and
	// handleRouterLost parse. linkEventRouter only.
	payload string
	// carrier is live carrier state per interface, read from the interface
	// list the rate tracker already dumps. linkEventResync only. The queue
	// stores the reference, so a caller MUST NOT write to the map after the
	// push.
	carrier map[string]bool
}

// linkEventEntry is one drained key and the value it ended with.
type linkEventEntry struct {
	key   linkEventKey
	value linkEventValue
}

// linkEventQueue holds at most ONE pending event per subject and hands them to
// a worker goroutine, in arrival order of their keys.
//
// It replaces a 16-deep channel whose full buffer dropped events outright. A
// config commit was exactly when that buffer filled, because the worker takes
// the lock the commit holds across DHCP client stop and start. A dropped UP
// then left the DHCP default route at the deprioritized metric for as long as
// the link stayed up: the route handlers are idempotent by routeMetricState,
// which is what makes them safe against a duplicate event and helpless against
// a missing one.
//
// Coalescing loses nothing the consumers read. They act on the state a subject
// ENDED in, so the queue keeps that state and discards what it superseded.
// Memory is bounded by the number of subjects carrying an unconsumed event,
// because the worker deletes a key as it takes it.
type linkEventQueue struct {
	mu      sync.Mutex
	pending map[linkEventKey]linkEventValue
	order   []linkEventKey

	// wake is the cap-1 coalescing signal nonBlockingNotify writes. It is
	// never closed, so a push that races stop is a no-op rather than a send on
	// a closed channel -- the shape registryReconcileCh already uses.
	wake   chan struct{}
	stopCh chan struct{}
	done   chan struct{}

	stopOnce sync.Once
	log      *slog.Logger

	// workerBlocked is true while the worker is waiting for the lock a config
	// commit holds. It exists so a push can say whether it arrived DURING that
	// wait, which is the one fact the block counter cannot report: the worker
	// takes the lock once per drained entry, so at most one block is countable
	// per contiguous hold, and the periodic resync usually takes it. An event
	// queued while this is set met a held lock by definition, however many
	// other events did.
	workerBlocked atomic.Bool
}

// newLinkEventQueue returns a queue with no worker behind it. The caller MUST
// call start before any push is expected to reach a handler, and MUST call
// stop once for that start.
func newLinkEventQueue(log *slog.Logger) *linkEventQueue {
	return &linkEventQueue{
		pending: make(map[linkEventKey]linkEventValue),
		wake:    make(chan struct{}, 1),
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
		log:     log,
	}
}

// push records value as the pending state for key and wakes the worker. It
// never blocks and it never discards a subject's final state, so an event bus
// handler MAY call it on the emitter's goroutine.
func (q *linkEventQueue) push(key linkEventKey, value linkEventValue) {
	q.mu.Lock()
	_, coalesced := q.pending[key]
	if !coalesced {
		q.order = append(q.order, key)
	}
	q.pending[key] = value
	q.mu.Unlock()

	// A superseded resync is not reportable: the resync is a periodic signal
	// whose whole contract is that one pass absorbs every tick behind it, the
	// same contract nonBlockingNotify has. A superseded carrier or router
	// event is reportable, because it says the worker fell behind the kernel.
	if coalesced && key.class != linkEventResync {
		countLinkEventCoalesced(key.ifaceName)
		q.log.Debug("interface: link event superseded before the worker took it",
			"iface", key.ifaceName, "router", key.routerIP)
	}
	// Counted whether or not it coalesced, and for a resync too: the question
	// is what ARRIVED during the hold, not what survived it.
	if q.workerBlocked.Load() {
		countLinkEventQueuedWhileBlocked(blockedLabel(key))
	}
	nonBlockingNotify(q.wake)
}

// setWorkerBlocked records whether the worker is waiting for the config-commit
// lock. The worker MUST clear it once it holds the lock, so the window this
// reports is the wait and not the work.
func (q *linkEventQueue) setWorkerBlocked(blocked bool) {
	q.workerBlocked.Store(blocked)
}

// pushCarrier queues the carrier state ifaceName ended in.
func (q *linkEventQueue) pushCarrier(ifaceName string, up bool) {
	q.push(linkEventKey{class: linkEventCarrier, ifaceName: ifaceName}, linkEventValue{present: up})
}

// pushRouter queues a router transition. data is the raw event JSON the
// handlers parse; payload is the same JSON already decoded, which is what the
// key needs to keep two routers on one interface independent.
func (q *linkEventQueue) pushRouter(payload RouterEventPayload, data string, discovered bool) {
	q.push(
		linkEventKey{class: linkEventRouter, ifaceName: payload.Name, routerIP: payload.RouterIP},
		linkEventValue{present: discovered, payload: data},
	)
}

// resyncBlockedLabel is the metric label a block is counted under when the
// worker was about to handle a resync. A resync is about every interface at
// once, so it carries no name, and labelling its block with the zero value put
// it on a `name=""` series that reads as a bug and hid a real one: a test
// summing only `name="<device>"` saw zero through a genuine block, because the
// resync ticks every second and is what usually meets a held lock.
//
// A label an operator can read is the point. `blockedLabel` is the only place
// that turns a key into one, and TestBlockedLabelIsNeverEmpty pins it.
const resyncBlockedLabel = "(resync)"

// blockedLabel names the metric series a worker block is counted under. It
// never answers the empty string: an empty Prometheus label value tells an
// operator nothing and tempts a reader into filtering it out.
func blockedLabel(key linkEventKey) string {
	if key.ifaceName != "" {
		return key.ifaceName
	}
	if key.class == linkEventResync {
		return resyncBlockedLabel
	}
	return "(unnamed)"
}

// pushResync queues a comparison of acted-on route metric state against live
// carrier state. The caller MUST NOT write to carrier after the call.
func (q *linkEventQueue) pushResync(carrier map[string]bool) {
	q.push(linkEventKey{class: linkEventResync}, linkEventValue{carrier: carrier})
}

// drain removes every pending entry and returns it in arrival order of the
// keys. A key pushed again while it waited is returned once, carrying the
// state of its LAST push.
func (q *linkEventQueue) drain() []linkEventEntry {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.order) == 0 {
		return nil
	}
	out := make([]linkEventEntry, 0, len(q.order))
	for _, key := range q.order {
		out = append(out, linkEventEntry{key: key, value: q.pending[key]})
		delete(q.pending, key)
	}
	q.order = q.order[:0]
	return out
}

// start runs the worker goroutine. apply is called once per drained entry, on
// the worker's goroutine, and it is the only place route work happens: no event
// bus handler reaches a handler directly, which is what keeps the monitor's
// read loop free of the config-apply lock.
//
// The caller MUST call stop exactly once for each start.
func (q *linkEventQueue) start(apply func(linkEventKey, linkEventValue)) {
	go func() {
		defer close(q.done)
		for {
			select {
			case <-q.wake:
				q.applyAll(apply)
			case <-q.stopCh:
				// select does not prefer wake over stop when both are ready,
				// so an event that raced the stop would otherwise never be
				// applied. Drain once more before leaving.
				q.applyAll(apply)
				return
			}
		}
	}()
}

func (q *linkEventQueue) applyAll(apply func(linkEventKey, linkEventValue)) {
	for _, entry := range q.drain() {
		apply(entry.key, entry.value)
	}
}

// stop ends the worker and waits for it to apply what is left. It MUST be
// called once for each start, and MUST NOT be called for a queue that was never
// started: nothing would ever close done and the wait would not return.
// Calling it twice is safe.
func (q *linkEventQueue) stop() {
	q.stopOnce.Do(func() { close(q.stopCh) })
	<-q.done
}

// subscribeLinkEvents registers the carrier and router subscribers on eb. Each
// one parses its payload and pushes; none of them takes a lock, calls a
// backend, or reaches a route handler.
//
// That is a requirement rather than a style: EmitEngineEvent
// (internal/component/plugin/server/engine_event.go) dispatches a subscriber
// synchronously on the caller's goroutine, and for these events that caller is
// the netlink monitor's read loop. Work done here stops the loop, and the
// kernel-side subscription queue overflows behind it.
//
// onCarrierUp, when not nil, is called for an interface whose carrier came up,
// on the emitter's goroutine. It MUST NOT block: the production caller only
// signals a coalescing channel.
//
// The returned unsubscribe funcs belong to the caller's shutdown list.
func subscribeLinkEvents(eb ze.EventBus, q *linkEventQueue, onCarrierUp func(string)) []func() {
	carrier := func(eventType string, up bool) func() {
		return eb.Subscribe(ifaceevents.Namespace, eventType, events.AsString(func(data string) {
			var ev struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(data), &ev); err != nil || ev.Name == "" {
				return
			}
			q.pushCarrier(ev.Name, up)
			if up && onCarrierUp != nil {
				onCarrierUp(ev.Name)
			}
		}))
	}
	router := func(eventType string, discovered bool) func() {
		return eb.Subscribe(ifaceevents.Namespace, eventType, events.AsString(func(data string) {
			if payload, ok := parseRouterEvent(data); ok {
				q.pushRouter(payload, data, discovered)
			}
		}))
	}
	return []func(){
		carrier(ifaceevents.EventDown, false),
		carrier(ifaceevents.EventUp, true),
		router(ifaceevents.EventRouterDiscovered, true),
		router(ifaceevents.EventRouterLost, false),
	}
}

// applyLinkEvent is the whole of what the link worker does with one drained
// entry: it is the only route work any of these events reach, and it runs on
// the worker's goroutine, never on an emitter's.
//
// The caller MUST hold dhcpMu.
func applyLinkEvent(key linkEventKey, value linkEventValue, active map[dhcpUnitKey]dhcpEntry, routers map[routerKey]routerEntry, priorities map[string]int, log *slog.Logger) {
	switch key.class {
	case linkEventCarrier:
		if value.present {
			handleLinkUp(key.ifaceName, active, log)
			handleLinkUpIPv6(key.ifaceName, routers, log)
		} else {
			handleLinkDown(key.ifaceName, active, log)
			handleLinkDownIPv6(key.ifaceName, routers, log)
		}
	case linkEventRouter:
		if value.present {
			handleRouterDiscovered(value.payload, routers, priorities, log)
		} else {
			handleRouterLost(value.payload, routers, log)
		}
	case linkEventResync:
		resyncCarrierState(value.carrier, active, routers, log)
	}
}

// carrierFromInterfaces reads live carrier state out of an interface list.
//
// The test is the one the monitor applies: linkToInfo
// (internal/plugins/iface/netlink/show_linux.go) maps OperUp, and OperUnknown
// with IFF_UP, onto the state string "up", which is the same pair isLinkUp
// (internal/plugins/iface/netlink/monitor_linux.go) turns into an up event.
// The comparison is case-insensitive because config_apply.go already reads
// backends that spell the state "UP".
func carrierFromInterfaces(ifs []InterfaceInfo) map[string]bool {
	carrier := make(map[string]bool, len(ifs))
	for i := range ifs {
		carrier[ifs[i].Name] = strings.EqualFold(ifs[i].State, "up")
	}
	return carrier
}

// resyncCarrierState repairs every interface whose acted-on route metric state
// CONTRADICTS its live carrier, and touches nothing else. It returns how many
// interfaces it repaired.
//
// It is the second reader of live carrier state, and it exists because the
// event stream is not always the one that carries the truth. A failed AddRoute
// leaves routeMetricUnknown and the route at neither metric, and a device that
// was already down when ze started its client never sends a transition. Without
// this pass the route stays where it is until a carrier event that may never
// come.
//
// Only a definite contradiction is repaired. routeMetricUnknown says ze does
// not know where the route is, and a lease sets it on every renewal, so acting
// on it here would move a route the DHCP client has just installed, on every
// tick.
//
// The caller MUST hold dhcpMu.
func resyncCarrierState(carrier map[string]bool, active map[dhcpUnitKey]dhcpEntry, routers map[routerKey]routerEntry, log *slog.Logger) int {
	repaired := 0
	seen := make(map[string]struct{})
	repair := func(ifaceName string) {
		if _, done := seen[ifaceName]; done {
			return
		}
		seen[ifaceName] = struct{}{}
		up, known := carrier[ifaceName]
		if !known {
			return // the device is gone, and its routes went with it
		}
		if !carrierContradicted(ifaceName, up, active, routers) {
			return
		}
		log.Info("interface: carrier resync repairing route metric", "iface", ifaceName, "carrier-up", up)
		if up {
			handleLinkUp(ifaceName, active, log)
			handleLinkUpIPv6(ifaceName, routers, log)
		} else {
			handleLinkDown(ifaceName, active, log)
			handleLinkDownIPv6(ifaceName, routers, log)
		}
		countCarrierResync(ifaceName)
		repaired++
	}
	for key := range active {
		repair(key.ifaceName)
	}
	for key := range routers {
		repair(key.ifaceName)
	}
	return repaired
}

// carrierContradicted reports whether ze acted on a carrier state for ifaceName
// that live carrier contradicts: a route left at the base metric with the link
// down, or at the deprioritized metric with the link up.
func carrierContradicted(ifaceName string, up bool, active map[dhcpUnitKey]dhcpEntry, routers map[routerKey]routerEntry) bool {
	contradiction := routeMetricDeprioritized
	if !up {
		contradiction = routeMetricBase
	}
	for key, entry := range active {
		if key.ifaceName == ifaceName && entry.gateway != "" && entry.metricState == contradiction {
			return true
		}
	}
	for key, entry := range routers {
		if key.ifaceName == ifaceName && entry.metricState == contradiction {
			return true
		}
	}
	return false
}
