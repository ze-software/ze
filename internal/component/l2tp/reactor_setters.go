// Design: docs/guide/l2tp.md -- reactor setters used by Reload
// Related: reactor.go -- owns the fields these setters mutate
// Related: subsystem_reload.go -- sole caller

package l2tp

import (
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/pkg/ze"
)

// setSharedSecret updates the per-reactor tunnel-default shared secret.
// Only affects new tunnels (the FSM reads the secret at SCCRQ time).
// Caller MUST NOT hold tunnelsMu; this method acquires it because
// params.Defaults is consulted from the reactor goroutine.
func (r *l2tpReactor) setSharedSecret(secret string) {
	r.tunnelsMu.Lock()
	r.params.Defaults.SharedSecret = secret
	r.tunnelsMu.Unlock()
}

// setHelloInterval updates the per-reactor hello interval. New tunnels
// schedule their first HELLO based on this value; live tunnels keep
// their originally-scheduled interval.
func (r *l2tpReactor) setHelloInterval(d time.Duration) {
	r.tunnelsMu.Lock()
	r.params.HelloInterval = d
	r.tunnelsMu.Unlock()
}

// setHelloRetries updates the per-reactor dead-peer detection threshold:
// the number of consecutive unanswered HELLO keepalive intervals tolerated
// before an Established tunnel's peer is declared dead. Effective detection
// time is HelloRetries * HelloInterval. Zero disables dead-peer detection.
// Read by handleTick on the reactor goroutine; mutated here under tunnelsMu.
func (r *l2tpReactor) setHelloRetries(n uint8) {
	r.tunnelsMu.Lock()
	r.params.HelloRetries = n
	r.tunnelsMu.Unlock()
}

// setMaxTunnels updates the per-reactor tunnel admission cap. Affects
// the next SCCRQ admission check; existing tunnels are untouched.
func (r *l2tpReactor) setMaxTunnels(n uint16) {
	r.tunnelsMu.Lock()
	r.params.MaxTunnels = n
	r.tunnelsMu.Unlock()
}

// setMaxSessions updates the per-reactor session admission cap. Affects
// ICRQ/OCRQ admission on new sessions; existing sessions are untouched.
func (r *l2tpReactor) setMaxSessions(n uint16) {
	r.tunnelsMu.Lock()
	r.params.MaxSessions = n
	r.tunnelsMu.Unlock()
}

// setPPPAuthMethod updates the Auth-Protocol advertised to new PPP
// sessions. Live sessions keep their already-negotiated method.
func (r *l2tpReactor) setPPPAuthMethod(m ppp.AuthMethod) {
	r.tunnelsMu.Lock()
	r.params.AuthMethod = m
	r.tunnelsMu.Unlock()
}

// setPPPAuthRequired updates whether new PPP sessions may proceed after
// LCP opens with no negotiated Auth-Protocol.
func (r *l2tpReactor) setPPPAuthRequired(required bool) {
	r.tunnelsMu.Lock()
	r.params.AuthRequired = required
	r.tunnelsMu.Unlock()
}

func (r *l2tpReactor) setAuthTimeout(d time.Duration) {
	r.tunnelsMu.Lock()
	r.params.AuthTimeout = d
	r.tunnelsMu.Unlock()
}

func (r *l2tpReactor) setReauthInterval(d time.Duration) {
	r.tunnelsMu.Lock()
	r.params.ReauthInterval = d
	r.tunnelsMu.Unlock()
}

func (r *l2tpReactor) setEnableIPCP(enabled bool) {
	r.tunnelsMu.Lock()
	r.params.EnableIPCP = enabled
	r.tunnelsMu.Unlock()
}

func (r *l2tpReactor) setEnableIPv6CP(enabled bool) {
	r.tunnelsMu.Lock()
	r.params.EnableIPv6CP = enabled
	r.tunnelsMu.Unlock()
}

func (r *l2tpReactor) setNCPTimeout(d time.Duration) {
	r.tunnelsMu.Lock()
	r.params.NCPTimeout = d
	r.tunnelsMu.Unlock()
}

// setRouteObserver installs a RouteObserver for this reactor. MUST be
// called before Start(); the goroutine creation barrier synchronizes
// the write here with reads in the run loop. Passing nil is a no-op
// (leaves the existing observer unchanged).
func (r *l2tpReactor) setRouteObserver(obs RouteObserver) {
	if obs == nil {
		return
	}
	r.routeObserver = obs
}

// SetEventBus installs the EventBus for emitting (l2tp, session-down)
// events. MUST be called before Start().
func (r *l2tpReactor) SetEventBus(bus ze.EventBus) {
	r.eventBus = bus
}
