// Design: docs/architecture/api/process-protocol.md — plugin process management
// Overview: register.go — RPC registration hub

package server

import (
	"fmt"
	"net/netip"
	"sync"

	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
)

// Subscribe/unsubscribe handlers are in component/cmd/subscribe/subscribe.go.

// Subscription represents an event subscription.
type Subscription struct {
	Namespace    string      // "bgp" or "rib"
	EventType    string      // "update", "state", etc.
	Direction    string      // "received", "sent", "both" (empty = both)
	PeerFilter   *PeerFilter // nil = all peers
	PluginFilter string      // plugin name filter (empty = all)
}

// PeerFilter specifies which peers to filter.
type PeerFilter struct {
	Selector string // "*", "10.0.0.1", "!10.0.0.1"
}

// Matches returns true if the peer matches this filter.
func (pf *PeerFilter) Matches(peer string) bool {
	if pf.Selector == "*" {
		return true
	}
	if pf.Selector != "" && pf.Selector[0] == '!' {
		// Exclusion selector
		return peer != pf.Selector[1:]
	}
	return peer == pf.Selector
}

// Matches returns true if this subscription matches the event.
func (s *Subscription) Matches(namespace, eventType, direction, peer string) bool {
	// Namespace must match
	if s.Namespace != namespace {
		return false
	}

	// Event type must match
	if s.EventType != eventType {
		return false
	}

	// Direction filter (only for events that have direction)
	if direction != "" && s.Direction != plugin.DirectionBoth && s.Direction != direction {
		return false
	}

	// Peer filter
	if s.PeerFilter != nil {
		if !s.PeerFilter.Matches(peer) {
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
}

// NewSubscriptionManager creates a new subscription manager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[*process.Process][]*Subscription),
	}
}

// Add adds a subscription for a process.
func (sm *SubscriptionManager) Add(proc *process.Process, sub *Subscription) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.subscriptions[proc] = append(sm.subscriptions[proc], sub)
}

// Remove removes a subscription for a process.
// Returns true if the subscription was found and removed.
func (sm *SubscriptionManager) Remove(proc *process.Process, sub *Subscription) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subs := sm.subscriptions[proc]
	for i, s := range subs {
		if s.Equals(sub) {
			sm.subscriptions[proc] = append(subs[:i], subs[i+1:]...)
			return true
		}
	}
	return false
}

// Count returns the number of subscriptions for a process.
func (sm *SubscriptionManager) Count(proc *process.Process) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.subscriptions[proc])
}

// ClearProcess removes all subscriptions for a process.
func (sm *SubscriptionManager) ClearProcess(proc *process.Process) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.subscriptions, proc)
}

// GetMatching returns all processes with subscriptions matching the event.
func (sm *SubscriptionManager) GetMatching(namespace, eventType, direction, peer string) []*process.Process {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*process.Process
	for proc, subs := range sm.subscriptions {
		for _, sub := range subs {
			if sub.Matches(namespace, eventType, direction, peer) {
				result = append(result, proc)
				break // Only add proc once, even if multiple subs match
			}
		}
	}
	return result
}

// GetSubscriptions returns all subscriptions for a process.
func (sm *SubscriptionManager) GetSubscriptions(proc *process.Process) []*Subscription {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subs := sm.subscriptions[proc]
	result := make([]*Subscription, len(subs))
	copy(result, subs)
	return result
}

// ParseSubscription parses a subscribe/unsubscribe command.
// Format: [peer <sel> | plugin <name>] <namespace> event <type> [direction received|sent|both].
func ParseSubscription(args []string) (*Subscription, error) {
	sub := &Subscription{
		Direction: plugin.DirectionBoth,
	}

	i := 0

	// Optional peer/plugin filter
	if len(args) > i && args[i] == "peer" {
		if len(args) < i+2 {
			return nil, fmt.Errorf("missing peer selector")
		}
		selector := args[i+1]
		if err := validatePeerSelector(selector); err != nil {
			return nil, err
		}
		sub.PeerFilter = &PeerFilter{Selector: selector}
		i += 2
	} else if len(args) > i && args[i] == cmdPlugin {
		if len(args) < i+2 {
			return nil, fmt.Errorf("missing plugin name")
		}
		sub.PluginFilter = args[i+1]
		i += 2
	}

	// Namespace
	if len(args) <= i {
		return nil, fmt.Errorf("missing namespace")
	}
	ns := args[i]
	if ns != plugin.NamespaceBGP && ns != plugin.NamespaceRIB {
		return nil, fmt.Errorf("invalid namespace: %s (valid: bgp, rib)", ns)
	}
	sub.Namespace = ns
	i++

	// "event" keyword
	if len(args) <= i || args[i] != "event" {
		return nil, fmt.Errorf("expected 'event' keyword")
	}
	i++

	// Event type
	if len(args) <= i {
		return nil, fmt.Errorf("missing event type")
	}
	eventType := args[i]
	if err := validateEventType(ns, eventType); err != nil {
		return nil, err
	}
	sub.EventType = eventType
	i++

	// Optional direction
	if len(args) > i && args[i] == "direction" {
		if len(args) <= i+1 {
			return nil, fmt.Errorf("missing direction value")
		}
		dir := args[i+1]
		switch dir {
		case plugin.DirectionReceived, plugin.DirectionSent, plugin.DirectionBoth:
			sub.Direction = dir
		default:
			return nil, fmt.Errorf("invalid direction: %s (valid: received, sent, both)", dir)
		}
	}

	return sub, nil
}

// validatePeerSelector validates a peer selector.
func validatePeerSelector(selector string) error {
	if selector == "*" {
		return nil
	}

	// Check for exclusion prefix
	s := selector
	if s != "" && s[0] == '!' {
		s = s[1:]
		// Check for double exclusion
		if s != "" && s[0] == '!' {
			return fmt.Errorf("invalid peer selector: %s (double exclusion)", selector)
		}
	}

	// Check for double glob
	if s == "*" && len(selector) > 1 {
		return fmt.Errorf("invalid peer selector: %s", selector)
	}

	// If not glob, must be valid IP
	if s != "*" {
		if _, err := netip.ParseAddr(s); err != nil {
			return fmt.Errorf("invalid peer selector: %s (not a valid IP address)", selector)
		}
	}

	return nil
}

// validateEventType validates an event type for a namespace.
func validateEventType(namespace, eventType string) error {
	switch namespace {
	case plugin.NamespaceBGP:
		if !plugin.ValidBgpEvents[eventType] {
			return fmt.Errorf("invalid bgp event type: %s (valid: update, open, notification, keepalive, refresh, state, negotiated, eor, congested, resumed)", eventType)
		}
	case plugin.NamespaceRIB:
		if !plugin.ValidRibEvents[eventType] {
			return fmt.Errorf("invalid rib event type: %s (valid: cache, route)", eventType)
		}
	default:
		return fmt.Errorf("invalid namespace: %s", namespace)
	}
	return nil
}
