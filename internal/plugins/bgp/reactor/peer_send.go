// Design: docs/architecture/core-design.md — BGP UPDATE sending and watchdog operations
// Related: peer.go — Peer struct and FSM state machine

package reactor

import (
	"errors"
	"fmt"
	"net/netip"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
)

// SendUpdate sends a BGP UPDATE message to this peer.
// Returns ErrNotConnected if no session is active.
// Returns an error if the session is not in ESTABLISHED state.
func (p *Peer) SendUpdate(update *message.Update) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.SendUpdate(update)
}

// SendAnnounce sends a BGP UPDATE message for announcing a route.
// Eliminates large buffer allocations by writing directly to session buffer.
// Returns ErrNotConnected if no session is active.
//
// RFC 4271 Section 4.3 - UPDATE Message Format.
// RFC 4760 Section 3 - MP_REACH_NLRI for IPv6 routes.
// RFC 7911 - ADD-PATH encoding based on negotiated capabilities.
func (p *Peer) SendAnnounce(route bgptypes.RouteSpec, localAS uint32) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	isIBGP := p.settings.IsIBGP()
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	if route.Prefix.Addr().Is6() {
		family = nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}
	}
	asn4 := p.asn4()
	addPath := p.addPathFor(family)

	return session.SendAnnounce(route, localAS, isIBGP, asn4, addPath)
}

// SendWithdraw sends a BGP UPDATE message for withdrawing a route.
// Eliminates large buffer allocations by writing directly to session buffer.
// Returns ErrNotConnected if no session is active.
//
// RFC 4271 Section 4.3 - UPDATE Message Format (Withdrawn Routes for IPv4).
// RFC 4760 Section 4 - MP_UNREACH_NLRI for IPv6 withdrawals.
// RFC 7911 - ADD-PATH encoding based on negotiated capabilities.
func (p *Peer) SendWithdraw(prefix netip.Prefix) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	if prefix.Addr().Is6() {
		family = nlri.Family{AFI: nlri.AFIIPv6, SAFI: nlri.SAFIUnicast}
	}
	addPath := p.addPathFor(family)

	return session.SendWithdraw(prefix, addPath)
}

// SendRawUpdateBody sends a pre-encoded UPDATE message body (without BGP header).
// Used for zero-copy forwarding when encoding contexts match.
// Returns ErrNotConnected if no session is active.
func (p *Peer) SendRawUpdateBody(body []byte) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.SendRawUpdateBody(body)
}

// SendRawMessage sends raw bytes to the peer.
// If msgType is 0, payload is a full BGP packet.
// If msgType is non-zero, payload is message body only.
func (p *Peer) SendRawMessage(msgType uint8, payload []byte) error {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return ErrNotConnected
	}

	return session.SendRawMessage(msgType, payload)
}

// sendUpdateWithSplit sends an UPDATE, splitting if it exceeds maxSize.
// Uses SplitUpdateWithAddPath to chunk oversized messages into multiple UPDATEs.
// The family parameter is used to determine Add-Path state for correct NLRI parsing.
// Returns nil on success, first error encountered on failure.
//
// RFC 4271 Section 4.3: Each split UPDATE is self-contained with full attributes.
// RFC 7911: Add-Path requires 4-byte path identifier before each NLRI.
// RFC 8654: Respects peer's max message size (4096 or 65535).
func (p *Peer) sendUpdateWithSplit(update *message.Update, maxSize int, family nlri.Family) error {
	// Determine Add-Path state for this family using sendCtx
	// RFC 7911: Add-Path is negotiated per AFI/SAFI
	addPath := p.sendCtx != nil && p.sendCtx.AddPath(family)

	chunks, err := message.SplitUpdateWithAddPath(update, maxSize, addPath)
	if err != nil {
		// Attributes too large or single NLRI too large - cannot send
		return fmt.Errorf("splitting update: %w", err)
	}

	for _, chunk := range chunks {
		if err := p.SendUpdate(chunk); err != nil {
			return err
		}
	}
	return nil
}

// ErrWatchdogNotFound is returned when a watchdog group doesn't exist.
var ErrWatchdogNotFound = errors.New("watchdog group not found")

// AnnounceWatchdog sends all routes in the named watchdog group that are currently withdrawn.
// Routes are moved from withdrawn (-) to announced (+) state.
// State is updated even when disconnected, so routes will be in correct state on reconnect.
// Returns ErrWatchdogNotFound if the watchdog group doesn't exist.
func (p *Peer) AnnounceWatchdog(name string) error {
	p.mu.Lock()
	session := p.session
	routes, exists := p.settings.WatchdogGroups[name]

	if !exists {
		p.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}

	// Ensure watchdogState is initialized for this group
	if p.watchdogState == nil {
		p.watchdogState = make(map[string]map[string]bool)
	}
	if p.watchdogState[name] == nil {
		p.watchdogState[name] = make(map[string]bool)
		// Initialize state for all routes in group
		for i := range routes {
			wr := &routes[i]
			p.watchdogState[name][wr.RouteKey()] = !wr.InitiallyWithdrawn
		}
	}
	p.mu.Unlock()

	// If not connected, just update state (will be sent on reconnect)
	if session == nil {
		p.mu.Lock()
		for i := range routes {
			wr := &routes[i]
			p.watchdogState[name][wr.RouteKey()] = true
		}
		p.mu.Unlock()
		routesLogger().Debug("watchdog marked routes for announce (disconnected)", "peer", p.settings.Address, "watchdog", name, "count", len(routes))
		return nil
	}

	addr := p.settings.Address.String()
	announced := 0

	for i := range routes {
		wr := &routes[i]
		routeKey := wr.RouteKey()

		// Read state inside lock to avoid race with stale captured reference
		p.mu.RLock()
		isAnnounced := p.watchdogState[name][routeKey]
		p.mu.RUnlock()

		if isAnnounced {
			continue // Already announced
		}

		// Send the route - resolve next-hop from RouteNextHop policy
		nextHop, nhErr := p.resolveNextHop(wr.NextHop, routeFamily(&wr.StaticRoute))
		if nhErr != nil {
			routesLogger().Debug("watchdog next-hop resolution failed", "peer", addr, "watchdog", name, "error", nhErr)
			continue
		}
		// RFC 7911: Get ADD-PATH encoding state
		addPath := p.addPathFor(routeFamily(&wr.StaticRoute))
		update := buildStaticRouteUpdateNew(&wr.StaticRoute, nextHop, p.settings.LinkLocal, p.settings.LocalAS, p.settings.IsIBGP(), p.asn4(), addPath, p.sendCtx)
		if err := p.SendUpdate(update); err != nil {
			return err
		}

		// Update state
		p.mu.Lock()
		p.watchdogState[name][routeKey] = true
		p.mu.Unlock()

		routesLogger().Debug("route sent", "peer", addr, "prefix", routeKey, "nextHop", wr.NextHop.String())
		announced++
	}

	if announced > 0 {
		routesLogger().Debug("watchdog announced routes", "peer", addr, "watchdog", name, "count", announced)
	}
	return nil
}

// WithdrawWatchdog withdraws all routes in the named watchdog group that are currently announced.
// Routes are moved from announced (+) to withdrawn (-) state.
// State is updated even when disconnected, so routes will be in correct state on reconnect.
// Returns ErrWatchdogNotFound if the watchdog group doesn't exist.
func (p *Peer) WithdrawWatchdog(name string) error {
	p.mu.Lock()
	session := p.session
	routes, exists := p.settings.WatchdogGroups[name]

	if !exists {
		p.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrWatchdogNotFound, name)
	}

	// Ensure watchdogState is initialized for this group
	if p.watchdogState == nil {
		p.watchdogState = make(map[string]map[string]bool)
	}
	if p.watchdogState[name] == nil {
		p.watchdogState[name] = make(map[string]bool)
		// Initialize state for all routes in group
		for i := range routes {
			wr := &routes[i]
			p.watchdogState[name][wr.RouteKey()] = !wr.InitiallyWithdrawn
		}
	}
	p.mu.Unlock()

	// If not connected, just update state (will NOT be sent on reconnect)
	if session == nil {
		p.mu.Lock()
		for i := range routes {
			wr := &routes[i]
			p.watchdogState[name][wr.RouteKey()] = false
		}
		p.mu.Unlock()
		routesLogger().Debug("watchdog marked routes for withdraw (disconnected)", "peer", p.settings.Address, "watchdog", name, "count", len(routes))
		return nil
	}

	addr := p.settings.Address.String()
	withdrawn := 0

	for i := range routes {
		wr := &routes[i]
		routeKey := wr.RouteKey()

		// Read state inside lock to avoid race with stale captured reference
		p.mu.RLock()
		isAnnounced := p.watchdogState[name][routeKey]
		p.mu.RUnlock()

		if !isAnnounced {
			continue // Already withdrawn
		}

		// Build withdrawal - handles VPN, IPv4 unicast, IPv6 unicast correctly
		// RFC 7911: Get ADD-PATH encoding state
		addPath := p.addPathFor(routeFamily(&wr.StaticRoute))
		attrBuf := getBuildBuf()
		update := buildStaticRouteWithdraw(attrBuf, &wr.StaticRoute, addPath)
		sendErr := p.SendUpdate(update)
		putBuildBuf(attrBuf)
		if sendErr != nil {
			return sendErr
		}

		// Update state
		p.mu.Lock()
		p.watchdogState[name][routeKey] = false
		p.mu.Unlock()

		routesLogger().Debug("watchdog withdrew route", "peer", addr, "watchdog", name, "route", routeKey)
		withdrawn++
	}

	if withdrawn > 0 {
		routesLogger().Debug("watchdog withdrew routes", "peer", addr, "watchdog", name, "count", withdrawn)
	}
	return nil
}

// PauseReading pauses reading from this peer's session.
// If no active session exists, this is a no-op.
func (p *Peer) PauseReading() {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return
	}

	session.Pause()
	peerLogger().Warn("read paused", "peer", p.settings.Address)
}

// ResumeReading resumes reading from this peer's session.
// If no active session exists, this is a no-op.
func (p *Peer) ResumeReading() {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return
	}

	session.Resume()
	peerLogger().Warn("read resumed", "peer", p.settings.Address)
}

// IsReadPaused reports whether this peer's session read loop is paused.
// Returns false if no active session exists.
func (p *Peer) IsReadPaused() bool {
	p.mu.RLock()
	session := p.session
	p.mu.RUnlock()

	if session == nil {
		return false
	}

	return session.IsPaused()
}
