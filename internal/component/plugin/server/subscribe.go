// Design: docs/architecture/api/process-protocol.md — plugin process management
// Overview: register.go — RPC registration hub
// Related: engine_event.go — engine-side stream pub/sub, parallel registry to SubscriptionManager

package server

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
)

var (
	errMissingPeerSelector   = errors.New("missing peer selector")
	errMissingPluginName     = errors.New("missing plugin name")
	errMissingNamespace      = errors.New("missing namespace")
	errExpectedEventKeyword  = errors.New("expected 'event' keyword")
	errMissingEventType      = errors.New("missing event type")
	errMissingDirectionValue = errors.New("missing direction value")
	errEmptyPeerSelector     = errors.New("empty peer selector")
)

// Subscribe/unsubscribe handlers are in component/cmd/subscribe/subscribe.go.

// Command keywords for subscription parsing.
const (
	kwPeer      = "peer"
	kwEvent     = "event"
	kwDirection = "direction"
	nsBGP       = "bgp"
)

// Subscription represents an event subscription.
type Subscription struct {
	Namespace    events.NamespaceID // compact ID assigned at registration time
	EventType    events.EventTypeID // compact ID assigned at registration time
	Direction    events.Direction   // typed enum (DirReceived, DirSent, DirBoth)
	PeerFilter   *PeerFilter        // nil = all peers
	PluginFilter string             // plugin name filter (empty = all)
	// Runtime marks a subscription an operator made against a RUNNING daemon
	// with `request subscribe`, rather than one a plugin declared at ready.
	// It is a live override: it is delivered whether or not the peer's config
	// grants the type, and the next config apply discards it
	// (Server.DiscardRuntimeSubscriptions, which only a config apply calls -- a
	// peer joining or leaving republishes the index and leaves this standing).
	// The config is durable truth, so a runtime
	// change must not survive a reload -- that would make the config document
	// a lie about what the daemon does.
	Runtime bool
}

// PeerFilter specifies which peers to filter.
type PeerFilter struct {
	Selector string // "*", "10.0.0.1", "!10.0.0.1", "my-peer", "!my-peer"
}

// Matches returns true if the peer matches this filter.
// Matches against both the peer address (IP) and peer name.
func (pf *PeerFilter) Matches(peerAddr, peerName string) bool {
	if pf.Selector == "*" {
		return true
	}
	if pf.Selector != "" && pf.Selector[0] == '!' {
		// Exclusion: reject if either address or name matches the excluded value.
		excluded := pf.Selector[1:]
		return peerAddr != excluded && peerName != excluded
	}
	return peerAddr == pf.Selector || peerName == pf.Selector
}

// Matches returns true if this subscription matches the event.
// peerAddr is the peer's IP address; peerName is the configured peer name (may be empty).
func (s *Subscription) Matches(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr, peerName string) bool {
	if s.Namespace != ns {
		return false
	}
	if s.EventType != et {
		return false
	}
	if dir != events.DirUnspecified && s.Direction != events.DirBoth && s.Direction != dir {
		return false
	}
	if s.PeerFilter != nil {
		if !s.PeerFilter.Matches(peerAddr, peerName) {
			return false
		}
	}
	return true
}

// Equals returns true if two subscriptions are identical.
func (s *Subscription) Equals(other *Subscription) bool {
	if s.Namespace != other.Namespace || s.EventType != other.EventType || s.Direction != other.Direction {
		return false
	}
	if s.PluginFilter != other.PluginFilter {
		return false
	}
	if (s.PeerFilter == nil) != (other.PeerFilter == nil) {
		return false
	}
	if s.PeerFilter != nil && s.PeerFilter.Selector != other.PeerFilter.Selector {
		return false
	}
	return true
}

// SubscriptionManager tracks subscriptions per process.
type SubscriptionManager struct {
	mu            sync.RWMutex
	subscriptions map[*process.Process][]*Subscription
	// runtime counts the live overrides the map holds. The delivery path reads
	// it once per event to decide whether the override scan is worth making,
	// and it is zero on every daemon nobody has typed `request subscribe` at.
	runtime atomic.Int64
}

// newSubscriptionManager creates a new subscription manager.
func newSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[*process.Process][]*Subscription),
	}
}

// Add adds a subscription for a process.
func (sm *SubscriptionManager) Add(proc *process.Process, sub *Subscription) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.subscriptions[proc] = append(sm.subscriptions[proc], sub)
	if sub.Runtime {
		sm.runtime.Add(1)
	}
}

// Remove removes a subscription for a process.
// Returns true if the subscription was found and removed.
func (sm *SubscriptionManager) Remove(proc *process.Process, sub *Subscription) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subs := sm.subscriptions[proc]
	for i, s := range subs {
		if s.Equals(sub) {
			// The STORED subscription says whether an override is going, not
			// the one the command parsed: `request unsubscribe` produces the
			// same shape whichever half it removes.
			if s.Runtime {
				sm.runtime.Add(-1)
			}
			sm.subscriptions[proc] = append(subs[:i], subs[i+1:]...)
			return true
		}
	}
	return false
}

// hasRuntimeOverride reports whether any live override exists. One atomic load,
// so the delivery path pays nothing for a feature nobody is using.
func (sm *SubscriptionManager) hasRuntimeOverride() bool {
	return sm.runtime.Load() > 0
}

// matchesRuntimeOverride reports whether one process holds a runtime override
// covering this event. Only reached when hasRuntimeOverride is true and the
// config's own grant already said no.
func (sm *SubscriptionManager) matchesRuntimeOverride(proc *process.Process, ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr, peerName string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for _, sub := range sm.subscriptions[proc] {
		if sub.Runtime && sub.Matches(ns, et, dir, peerAddr, peerName) {
			return true
		}
	}
	return false
}

// clearRuntimeOverrides drops every live override. A config apply rebuilds the
// durable truth, and an override that survived it would make the config
// document a lie about what the daemon delivers.
func (sm *SubscriptionManager) clearRuntimeOverrides() {
	if !sm.hasRuntimeOverride() {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for proc, subs := range sm.subscriptions {
		kept := subs[:0]
		for _, sub := range subs {
			if !sub.Runtime {
				kept = append(kept, sub)
			}
		}
		sm.subscriptions[proc] = kept
	}
	sm.runtime.Store(0)
}

// processNamed returns the running process registered under one plugin name,
// or nil when no process of that name has subscribed to anything. The name is
// the one an operator writes in `attach process <name>`.
func (sm *SubscriptionManager) processNamed(name string) *process.Process {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for proc := range sm.subscriptions {
		if proc.Name() == name {
			return proc
		}
	}
	return nil
}

// declaredDirection reports whether a process declared one event type for one
// peer, and in which direction. Directions union: a plugin declaring the type
// twice gets the wider of the two.
func (sm *SubscriptionManager) declaredDirection(proc *process.Process, ns events.NamespaceID, et events.EventTypeID, peerAddr, peerName string) (bool, events.Direction) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	found := false
	dir := events.DirUnspecified
	for _, sub := range sm.subscriptions[proc] {
		if sub.Namespace != ns || sub.EventType != et {
			continue
		}
		if sub.PeerFilter != nil && !sub.PeerFilter.Matches(peerAddr, peerName) {
			continue
		}
		if !found {
			found, dir = true, sub.Direction
			continue
		}
		if dir != sub.Direction {
			dir = events.DirBoth
		}
	}
	return found, dir
}

// declaredTypes returns every event type a process declared for one peer.
func (sm *SubscriptionManager) declaredTypes(proc *process.Process, ns events.NamespaceID, peerAddr, peerName string) []events.EventTypeID {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var out []events.EventTypeID
	for _, sub := range sm.subscriptions[proc] {
		if sub.Namespace != ns {
			continue
		}
		if sub.PeerFilter != nil && !sub.PeerFilter.Matches(peerAddr, peerName) {
			continue
		}
		if !slices.Contains(out, sub.EventType) {
			out = append(out, sub.EventType)
		}
	}
	return out
}

// declaredUnattached returns the names of the processes that declared at least
// one subscription in ns and are not in attached.
//
// It answers the one question the per-edge report cannot: a process no peer
// attaches has NO edge, so every loop over the index skips it and the operator
// is told nothing while the program receives nothing. Sorted, so the report is
// stable for a reader and for a test.
func (sm *SubscriptionManager) declaredUnattached(ns events.NamespaceID, attached map[string]struct{}, only *process.Process) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var out []string
	for proc, subs := range sm.subscriptions {
		if only != nil && proc != only {
			continue
		}
		if _, ok := attached[proc.Name()]; ok {
			continue
		}
		for _, sub := range subs {
			if sub.Namespace == ns {
				out = append(out, proc.Name())
				break
			}
		}
	}
	slices.Sort(out)
	return out
}

// Count returns the number of subscriptions for a process.
func (sm *SubscriptionManager) Count(proc *process.Process) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.subscriptions[proc])
}

// clearProcess removes all subscriptions for a process.
func (sm *SubscriptionManager) clearProcess(proc *process.Process) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, sub := range sm.subscriptions[proc] {
		if sub.Runtime {
			sm.runtime.Add(-1)
		}
	}
	delete(sm.subscriptions, proc)
}

// GetMatching returns all processes with subscriptions matching the event.
// peerName is the configured peer name (may be empty for non-BGP events or emit-event RPCs).
func (sm *SubscriptionManager) GetMatching(ns events.NamespaceID, et events.EventTypeID, dir events.Direction, peerAddr, peerName string) []*process.Process {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*process.Process
	for proc, subs := range sm.subscriptions {
		for _, sub := range subs {
			if sub.Matches(ns, et, dir, peerAddr, peerName) {
				result = append(result, proc)
				break // Only add proc once, even if multiple subs match
			}
		}
	}
	return result
}

// ParseSubscription parses a subscribe/unsubscribe command.
// Format: [peer <sel> | plugin <name>] [<namespace>] event <type> [direction received|sent|both].
// Namespace defaults to "bgp" when peer is set.
func ParseSubscription(args []string) (*Subscription, error) {
	sub := &Subscription{
		Direction: events.DirBoth,
	}

	i := 0

	// Optional peer/plugin filter
	if len(args) > i && args[i] == kwPeer {
		if len(args) < i+2 {
			return nil, errMissingPeerSelector
		}
		selector := args[i+1]
		if err := validatePeerSelector(selector); err != nil {
			return nil, err
		}
		sub.PeerFilter = &PeerFilter{Selector: selector}
		i += 2
	} else if len(args) > i && args[i] == cmdPlugin {
		if len(args) < i+2 {
			return nil, errMissingPluginName
		}
		sub.PluginFilter = args[i+1]
		i += 2
	}

	// Namespace (implicit "bgp" when peer filter is set and next token is kwEvent).
	if len(args) <= i {
		return nil, errMissingNamespace
	}
	ns := args[i]
	if sub.PeerFilter != nil && ns == kwEvent {
		ns = nsBGP
	} else {
		if !events.IsValidNamespace(ns) {
			return nil, fmt.Errorf("invalid namespace: %s (valid: %s)", ns, events.ValidNamespaceNames())
		}
		i++
	}
	sub.Namespace = events.LookupNamespaceID(ns)

	// kwEvent keyword
	if len(args) <= i || args[i] != kwEvent {
		return nil, errExpectedEventKeyword
	}
	i++

	// Event type
	if len(args) <= i {
		return nil, errMissingEventType
	}
	eventType := args[i]
	if err := validateEventType(ns, eventType); err != nil {
		return nil, err
	}
	sub.EventType = events.LookupEventTypeID(eventType)
	i++

	// Optional direction
	if len(args) > i && args[i] == kwDirection {
		if len(args) <= i+1 {
			return nil, errMissingDirectionValue
		}
		dir := args[i+1]
		switch dir {
		case events.DirectionReceived, events.DirectionSent, events.DirectionBoth:
			sub.Direction = events.ParseDirection(dir)
		default:
			return nil, fmt.Errorf("invalid direction: %s (valid: received, sent, both)", dir)
		}
	}

	return sub, nil
}

// validatePeerSelector validates a peer selector.
// Accepts: "*" (all), "!<sel>" (exclusion), IP addresses, peer names.
func validatePeerSelector(selector string) error {
	if selector == "" {
		return errEmptyPeerSelector
	}

	if selector == "*" {
		return nil
	}

	// Check for exclusion prefix
	s := selector
	if s[0] == '!' {
		s = s[1:]
		if s == "" {
			return fmt.Errorf("invalid peer selector: %s (empty after exclusion)", selector)
		}
		// Check for double exclusion
		if s[0] == '!' {
			return fmt.Errorf("invalid peer selector: %s (double exclusion)", selector)
		}
	}

	return nil
}

// validateEventType validates an event type for a namespace.
// Both namespaces and event types are derived from events.ValidEvents.
func validateEventType(namespace, eventType string) error {
	if !events.IsValidNamespace(namespace) {
		return fmt.Errorf("invalid namespace: %s (valid: %s)", namespace, events.ValidNamespaceNames())
	}
	if !events.IsValidEvent(namespace, eventType) {
		return fmt.Errorf("invalid %s event type: %s (valid: %s)", namespace, eventType, events.ValidEventNames(namespace))
	}
	return nil
}
