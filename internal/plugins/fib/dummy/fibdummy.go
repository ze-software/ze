// Design: docs/architecture/core-design.md -- FIB dummy route table and event handler
// Related: register.go -- plugin registration, lifecycle, and EventBus wiring
// Related: show.go -- command answer shapes and builders
package fibdummy

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// routeKey identifies one held route by its forwarding table and masked prefix.
type routeKey struct {
	table  uint32
	prefix netip.Prefix
}

// heldRoute stores the scalar attributes of one system-RIB route held in memory
// by fib-dummy and programmed nowhere. No per-path slices are retained and no
// strings are constructed during apply so steady-state replaces allocate nothing.
type heldRoute struct {
	nextHop   netip.Addr
	iface     string
	protocol  string
	routeType sysribevents.RouteType
	paths     int
}

// dummyFIB holds the best routes the system RIB selected, counts changes, and
// writes them to no forwarding table. Safe for concurrent use.
type dummyFIB struct {
	mu              sync.RWMutex
	routes          map[routeKey]heldRoute
	since           time.Time
	onInvalidChange func(prefix string)

	installs            atomic.Uint64
	replaces            atomic.Uint64
	removals            atomic.Uint64
	invalidChanges      atomic.Uint64
	labelEntriesRefused atomic.Uint64
}

func newDummyFIB(since time.Time) *dummyFIB {
	return &dummyFIB{
		routes: make(map[routeKey]heldRoute),
		since:  since,
	}
}

func hasPrimaryTarget(c *sysribevents.BestChangeEntry) bool {
	if c.NextHop.IsValid() {
		return true
	}
	if c.Interface != "" {
		return true
	}
	return c.SRv6SID.IsValid()
}

func buildHeldRoute(c *sysribevents.BestChangeEntry) heldRoute {
	nextHop := c.NextHop
	iface := c.Interface
	paths := len(c.ECMPPaths)
	if hasPrimaryTarget(c) {
		paths++
	}
	if len(c.ECMPPaths) > 0 {
		if !nextHop.IsValid() {
			nextHop = c.ECMPPaths[0].NextHop
		}
		if iface == "" {
			iface = c.ECMPPaths[0].Interface
		}
	}
	return heldRoute{
		nextHop:   nextHop,
		iface:     iface,
		protocol:  c.Protocol,
		routeType: c.RouteType,
		paths:     paths,
	}
}

// applyBatch updates the held route table, counters, and metrics for one
// (system-rib, best-change) batch. Safe for concurrent use.
//
// Steady-state replaces of an already-held (table, prefix) allocate nothing:
// no strings are constructed, no slices are copied, and nothing is logged.
func (f *dummyFIB) applyBatch(batch *sysribevents.BestChangeBatch) {
	if batch == nil {
		return
	}

	var installs, replaces, removals uint64
	firstInvalid := false

	f.mu.Lock()
	for i := range batch.Changes {
		c := &batch.Changes[i]
		if !c.Prefix.IsValid() {
			if f.invalidChanges.Add(1) == 1 {
				firstInvalid = true
			}
			continue
		}
		key := routeKey{
			table:  c.TableID,
			prefix: c.Prefix.Masked(),
		}
		switch c.Action.Verb() {
		case routeaction.VerbInstall:
			f.routes[key] = buildHeldRoute(c)
			installs++
		case routeaction.VerbReplace:
			f.routes[key] = buildHeldRoute(c)
			replaces++
		case routeaction.VerbRemove:
			delete(f.routes, key)
			removals++
		case routeaction.VerbSkip:
			// Unspecified or unknown action: ignore without mutating the table.
		default:
			// routeaction.Verb is open at consumer boundaries; ignore unknown verbs.
		}
	}
	held := len(f.routes)
	if installs > 0 {
		f.installs.Add(installs)
	}
	if replaces > 0 {
		f.replaces.Add(replaces)
	}
	if removals > 0 {
		f.removals.Add(removals)
	}
	if m := metricsPtr.Load(); m != nil {
		if installs > 0 {
			m.routeInstalls.Add(float64(installs))
		}
		if replaces > 0 {
			m.routeReplaces.Add(float64(replaces))
		}
		if removals > 0 {
			m.routeRemovals.Add(float64(removals))
		}
		if installs+replaces+removals > 0 {
			m.routesHeld.Set(float64(held))
		}
	}
	onInvalid := f.onInvalidChange
	f.mu.Unlock()

	if firstInvalid {
		if onInvalid != nil {
			onInvalid("")
		}
	}
}

// handleBestChangeJSON decodes one JSON-encoded (system-rib, best-change) batch
// delivered over the forked plugin RPC transport and applies it.
func (f *dummyFIB) handleBestChangeJSON(data json.RawMessage) error {
	var batch sysribevents.BestChangeBatch
	if err := json.Unmarshal(data, &batch); err != nil {
		return fmt.Errorf("fib-dummy: decode best-change batch: %w", err)
	}
	f.applyBatch(&batch)
	return nil
}

// handleEvent decodes one enveloped event string delivered to a forked
// fib-dummy process and dispatches (system-rib, best-change) batches.
func (f *dummyFIB) handleEvent(text string) error {
	envelope, err := rpc.ParseEventEnvelope(text)
	if err != nil {
		return fmt.Errorf("fib-dummy: decode event envelope: %w", err)
	}
	if envelope.Namespace != sysribevents.Namespace {
		return nil
	}
	if envelope.Event != sysribevents.EventBestChange {
		return nil
	}
	return f.handleBestChangeJSON(envelope.Payload)
}
