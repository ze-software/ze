package plugin

import (
	"fmt"
	"net/netip"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/slogutil"
)

// subscribeLogger is the subscription subsystem logger (lazy initialization).
// Controlled by ze.log.subscribe environment variable.
var subscribeLogger = slogutil.LazyLogger("subscribe")

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
	if len(pf.Selector) > 0 && pf.Selector[0] == '!' {
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
	if direction != "" && s.Direction != DirectionBoth && s.Direction != direction {
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
	subscriptions map[*Process][]*Subscription
}

// NewSubscriptionManager creates a new subscription manager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[*Process][]*Subscription),
	}
}

// Add adds a subscription for a process.
func (sm *SubscriptionManager) Add(proc *Process, sub *Subscription) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.subscriptions[proc] = append(sm.subscriptions[proc], sub)
}

// Remove removes a subscription for a process.
// Returns true if the subscription was found and removed.
func (sm *SubscriptionManager) Remove(proc *Process, sub *Subscription) bool {
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
func (sm *SubscriptionManager) Count(proc *Process) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.subscriptions[proc])
}

// ClearProcess removes all subscriptions for a process.
func (sm *SubscriptionManager) ClearProcess(proc *Process) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.subscriptions, proc)
}

// GetMatching returns all processes with subscriptions matching the event.
func (sm *SubscriptionManager) GetMatching(namespace, eventType, direction, peer string) []*Process {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subscribeLogger().Debug("GetMatching", "namespace", namespace, "event", eventType, "dir", direction, "peer", peer, "totalProcs", len(sm.subscriptions))

	var result []*Process
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
func (sm *SubscriptionManager) GetSubscriptions(proc *Process) []*Subscription {
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
		Direction: DirectionBoth, // default
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
	if ns != NamespaceBGP && ns != NamespaceRIB {
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
		case DirectionReceived, DirectionSent, DirectionBoth:
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
	if len(s) > 0 && s[0] == '!' {
		s = s[1:]
		// Check for double exclusion
		if len(s) > 0 && s[0] == '!' {
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
	case NamespaceBGP:
		if !validBgpEvents[eventType] {
			return fmt.Errorf("invalid bgp event type: %s (valid: update, open, notification, keepalive, refresh, state, negotiated)", eventType)
		}
	case NamespaceRIB:
		if !validRibEvents[eventType] {
			return fmt.Errorf("invalid rib event type: %s (valid: cache, route)", eventType)
		}
	default:
		return fmt.Errorf("invalid namespace: %s", namespace)
	}
	return nil
}

// subscribeRPCs returns RPC registrations for handlers defined in this file.
// Part of the ze-bgp module — aggregated by BgpPluginRPCs().
func subscribeRPCs() []RPCRegistration {
	return []RPCRegistration{
		{"ze-bgp:subscribe", "subscribe", handleSubscribe, "Subscribe to events"},
		{"ze-bgp:unsubscribe", "unsubscribe", handleUnsubscribe, "Unsubscribe from events"},
	}
}

// handleSubscribe handles the "subscribe" command.
func handleSubscribe(ctx *CommandContext, args []string) (*Response, error) {
	sub, err := ParseSubscription(args)
	if err != nil {
		return &Response{
			Status: StatusError,
			Data:   err.Error(),
		}, err
	}

	if ctx.Process == nil {
		return &Response{
			Status: StatusError,
			Data:   "subscribe requires a process context",
		}, fmt.Errorf("no process context")
	}

	if ctx.Subscriptions() == nil {
		return &Response{
			Status: StatusError,
			Data:   "subscription manager not available",
		}, fmt.Errorf("no subscription manager")
	}

	ctx.Subscriptions().Add(ctx.Process, sub)

	return &Response{
		Status: StatusDone,
		Data: map[string]any{
			"namespace": sub.Namespace,
			"event":     sub.EventType,
			"direction": sub.Direction,
		},
	}, nil
}

// handleUnsubscribe handles the "unsubscribe" command.
func handleUnsubscribe(ctx *CommandContext, args []string) (*Response, error) {
	sub, err := ParseSubscription(args)
	if err != nil {
		return &Response{
			Status: StatusError,
			Data:   err.Error(),
		}, err
	}

	if ctx.Process == nil {
		return &Response{
			Status: StatusError,
			Data:   "unsubscribe requires a process context",
		}, fmt.Errorf("no process context")
	}

	if ctx.Subscriptions() == nil {
		return &Response{
			Status: StatusError,
			Data:   "subscription manager not available",
		}, fmt.Errorf("no subscription manager")
	}

	removed := ctx.Subscriptions().Remove(ctx.Process, sub)

	return &Response{
		Status: StatusDone,
		Data: map[string]any{
			"removed":   removed,
			"namespace": sub.Namespace,
			"event":     sub.EventType,
		},
	}, nil
}
