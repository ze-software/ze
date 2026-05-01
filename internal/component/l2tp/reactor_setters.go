// Design: docs/architecture/l2tp.md -- reactor setters used by Reload
// Related: reactor.go -- owns the fields these setters mutate
// Related: subsystem_reload.go -- sole caller

package l2tp

import (
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// setSharedSecret updates the per-reactor tunnel-default shared secret.
// Only affects new tunnels (the FSM reads the secret at SCCRQ time).
// Caller MUST NOT hold tunnelsMu; this method acquires it because
// params.Defaults is consulted from the reactor goroutine.
func (r *L2TPReactor) setSharedSecret(secret string) {
	r.tunnelsMu.Lock()
	r.params.Defaults.SharedSecret = secret
	r.tunnelsMu.Unlock()
}

// setHelloInterval updates the per-reactor hello interval. New tunnels
// schedule their first HELLO based on this value; live tunnels keep
// their originally-scheduled interval.
func (r *L2TPReactor) setHelloInterval(d time.Duration) {
	r.tunnelsMu.Lock()
	r.params.HelloInterval = d
	r.tunnelsMu.Unlock()
}

// setMaxTunnels updates the per-reactor tunnel admission cap. Affects
// the next SCCRQ admission check; existing tunnels are untouched.
func (r *L2TPReactor) setMaxTunnels(n uint16) {
	r.tunnelsMu.Lock()
	r.params.MaxTunnels = n
	r.tunnelsMu.Unlock()
}

// setMaxSessions updates the per-reactor session admission cap. Affects
// ICRQ/OCRQ admission on new sessions; existing sessions are untouched.
func (r *L2TPReactor) setMaxSessions(n uint16) {
	r.tunnelsMu.Lock()
	r.params.MaxSessions = n
	r.tunnelsMu.Unlock()
}

// setPPPAuthMethod updates the Auth-Protocol advertised to new PPP
// sessions. Live sessions keep their already-negotiated method.
func (r *L2TPReactor) setPPPAuthMethod(m ppp.AuthMethod) {
	r.tunnelsMu.Lock()
	r.params.AuthMethod = m
	r.tunnelsMu.Unlock()
}

// setPPPAuthRequired updates whether new PPP sessions may proceed after
// LCP opens with no negotiated Auth-Protocol.
func (r *L2TPReactor) setPPPAuthRequired(required bool) {
	r.tunnelsMu.Lock()
	r.params.AuthRequired = required
	r.tunnelsMu.Unlock()
}

// SetRouteObserver installs a RouteObserver for this reactor. MUST be
// called before Start(); the goroutine creation barrier synchronizes
// the write here with reads in the run loop. Passing nil is a no-op
// (leaves the existing observer unchanged).
func (r *L2TPReactor) SetRouteObserver(obs RouteObserver) {
	if obs == nil {
		return
	}
	r.routeObserver = obs
}

// SetEventBus installs the EventBus for emitting (l2tp, session-down)
// events. MUST be called before Start().
func (r *L2TPReactor) SetEventBus(bus ze.EventBus) {
	r.eventBus = bus
}
