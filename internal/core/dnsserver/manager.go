// Design: docs/architecture/dns/server-harness.md -- generic DNS listener lifecycle
// shared by two or more authoritative-only DNS plugins.
//
// Package dnsserver holds the listener lifecycle, EDNS0/client-IP resolution,
// and authoritative-answer shaping that any authoritative-only DNS plugin
// needs, so a second plugin never has to import a sibling plugin
// (ai/rules/plugins.md:133) or hand-copy a security-sensitive handler.
package dnsserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// drainTimeout bounds how long Stop waits for in-flight handlers to finish,
// so a wedged handler cannot block a rebind or process exit.
const drainTimeout = 5 * time.Second

// unappliedSig is a signature no real endpoint set can ever produce
// (endpointSig only ever returns "disabled" or a comma-joined host:port
// list). Apply uses it to mark "nothing is known to be correctly bound" so
// the next Apply call is never short-circuited by the same-signature no-op
// fast path -- covers both a fresh Manager and any Manager that just lost a
// listener (full bind failure, or an unexpected post-bind crash).
const unappliedSig = "\x00unset"

// Endpoint is one UDP+TCP bind target.
type Endpoint struct {
	IP   netip.Addr
	Port uint16
}

// Options configures a Manager's listener behavior.
type Options struct {
	// Freebind, when true, sets IP_FREEBIND (Linux only) on each bound socket
	// so it can bind an address not yet present on any local interface (e.g.
	// an anycast VIP that appears after the process starts). Default false:
	// existing callers keep today's bind-to-configured-address-only behavior.
	Freebind bool

	// OnListenerChange, if set, is called after each bind/unbind with the
	// protocol ("udp"/"tcp"), the listen address, and whether the listener is
	// now up. The harness never owns metrics; a consumer wires its own gauge
	// through this callback.
	OnListenerChange func(proto, addr string, up bool)

	// TLSMaterialResolver resolves a certificate NAME into serving PEM material
	// (leaf plus any intermediates, and the private key). It exists because
	// internal/core may not import internal/component: the PKI store lives in
	// the pki component, so a consumer plugin injects pki.ServerTLSMaterial here
	// and this package stays free of that dependency.
	//
	// nil means this consumer supports no store references. A SecureConfig that
	// names one anyway is then an error, never a silent fallback.
	TLSMaterialResolver func(name string) (certPEM, keyPEM []byte, err error)
}

// Manager owns the bound UDP+TCP listeners for a dns.Handler. The handler is
// expected to be state-independent (reading its own snapshot per request), so
// only an endpoint-set change triggers a rebind.
//
// servers/boundAddrs are written only by the Apply/Stop-calling goroutine
// (bind, Stop); mu additionally protects applied/generation, which the
// background serve goroutines also touch when an unexpected listener crash
// is detected.
type Manager struct {
	log     *slog.Logger
	handler dns.Handler
	opts    Options

	servers          []*dns.Server
	httpServers      []*http.Server
	boundAddrs       []string
	boundSecureAddrs []secureAddr

	// selfSigned caches the ephemeral self-signed certificate used when no
	// operator cert/key files are configured, so a config reload does not
	// regenerate it (a fresh cert would change the listener signature and churn
	// a rebind on every no-op reconcile). Written only by the Apply-calling
	// goroutine.
	selfSigned *tls.Config

	mu         sync.Mutex
	applied    string
	generation int
}

// secureAddr records a bound DoT/DoH listener so Stop can fire the matching
// OnListenerChange down-edge with the right proto label ("dot"/"doh").
type secureAddr struct {
	proto string
	addr  string
}

// New creates a Manager serving handler for every endpoint Apply binds.
func New(log *slog.Logger, handler dns.Handler, opts Options) *Manager {
	return &Manager{log: log, handler: handler, opts: opts, applied: unappliedSig}
}

// endpointSig is the signature of the desired bound-listener set; Apply
// rebinds only when it changes.
func endpointSig(enabled bool, endpoints []Endpoint) string {
	if !enabled {
		return "disabled"
	}
	eps := make([]string, len(endpoints))
	for i, e := range endpoints {
		eps[i] = net.JoinHostPort(e.IP.String(), strconv.Itoa(int(e.Port)))
	}
	sort.Strings(eps)
	return strings.Join(eps, ",")
}

// Apply reconciles the bound listeners with the desired endpoint set. A pure
// host-data change (same endpoints) is a no-op -- the handler is expected to
// pick up new state itself; an endpoint-set change stops and rebinds.
//
// The signature only sticks on a fully successful bind. A transient failure
// (port momentarily taken, anycast address not yet present) must not wedge
// the manager into treating an unchanged, still-failed endpoint set as
// already applied -- the caller's next identical Apply (a periodic
// reconcile, or an operator re-commit, including reverting straight back to
// a previously-good endpoint set after an intervening failed Apply) has to
// actually retry the bind, not silently no-op with zero listeners up.
func (m *Manager) Apply(enabled bool, endpoints []Endpoint) error {
	return m.ApplyListeners(enabled, Listeners{Plain: endpoints})
}

func (m *Manager) appliedSig() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.applied
}

func (m *Manager) setApplied(sig string) {
	m.mu.Lock()
	m.applied = sig
	m.mu.Unlock()
}

// bind opens a UDP and a TCP listener on ep (best-effort: the caller logs and
// continues if one endpoint fails so the rest still serve).
//
// Waits for both spawned listener goroutines to reach dns.Server's genuine
// "started" state (via NotifyStartedFunc) before returning. Without this,
// a caller that calls Stop() immediately after bind() returns (e.g. a rapid
// Apply->Apply sequence) can race ActivateAndServe: dns.Server.
// ShutdownContext returns immediately with "server not started", WITHOUT
// closing the underlying socket, whenever srv.started is still false --
// which it is until ActivateAndServe's goroutine actually gets scheduled.
// The listener then proceeds to serve indefinitely on a socket the Manager
// has already forgotten (m.servers cleared by Stop), permanently occupying
// the port. NotifyStartedFunc fires unconditionally, early inside
// serveUDP/serveTCP, strictly after ActivateAndServe sets srv.started=true
// (miekg/dns server.go), so waiting on it closes the race window exactly.
func (m *Manager) bind(ep, addr string) error {
	lc := listenConfig(m.opts.Freebind)
	pc, err := lc.ListenPacket(context.Background(), "udp", ep)
	if err != nil {
		return fmt.Errorf("udp: %w", err)
	}
	udpStarted := make(chan struct{})
	udp := &dns.Server{PacketConn: pc, Handler: m.handler, NotifyStartedFunc: func() { close(udpStarted) }}
	ln, err := lc.Listen(context.Background(), "tcp", ep)
	if err != nil {
		if cerr := pc.Close(); cerr != nil {
			m.log.Debug("dnsserver: closing udp after tcp bind failure", "endpoint", ep, "error", cerr)
		}
		return fmt.Errorf("tcp: %w", err)
	}
	tcpStarted := make(chan struct{})
	tcp := &dns.Server{Listener: ln, Handler: m.handler, NotifyStartedFunc: func() { close(tcpStarted) }}
	m.servers = append(m.servers, udp, tcp)
	m.boundAddrs = append(m.boundAddrs, addr)
	// Fire the up-edge here, right after the sockets are BOUND (ListenPacket/
	// Listen above) and before the serve goroutines start. A serving-gated
	// consumer (as112's BGP announce, RFC 7534 Section 3.3) thus sees "up" a
	// moment before ActivateAndServe's userspace read loop begins -- but this is
	// benign, not an advertise-before-serve blackhole: the kernel has the socket
	// bound and buffers any query arriving in that sub-millisecond window until
	// the loop drains it. Deliberately NOT deferred until after <-udpStarted/
	// <-tcpStarted below: doing so would let an instant post-start crash fire the
	// down-edge (from serve's monitor) BEFORE this up-edge, leaving the consumer's
	// serving tracker stuck "up". Ordering up-before-serve keeps up strictly
	// before any possible down.
	if m.opts.OnListenerChange != nil {
		m.opts.OnListenerChange("udp", addr, true)
		m.opts.OnListenerChange("tcp", addr, true)
	}
	m.mu.Lock()
	gen := m.generation
	m.mu.Unlock()
	go m.serve(udp, ep, addr, "udp", gen)
	go m.serve(tcp, ep, addr, "tcp", gen)
	<-udpStarted
	<-tcpStarted
	return nil
}

// serve runs one listener's accept loop until it exits. gen is the Manager's
// generation snapshot at bind time: Stop increments generation before
// touching any listener, so if the generation is still current when
// ActivateAndServe returns, no deliberate Stop has happened since this
// listener was bound and the exit is an unexpected crash (socket error, an
// anycast address withdrawn under it, ...) rather than a graceful shutdown.
//
// An unexpected crash must surface: silently leaving the listener-up gauge
// at "true" and the Manager's applied signature unchanged would make a dead
// listener look healthy forever, and a later Apply call with the same
// (still-desired, unchanged) endpoint set would short-circuit as a no-op
// instead of actually rebinding.
//
// Known benign race: a genuine crash whose ActivateAndServe return races a
// concurrent Stop/Apply's generation++ can be misclassified as graceful
// (crashed==false here), losing this function's Error log line. This does
// not lose state: Stop's own loop unconditionally calls OnListenerChange
// false for every bound address it tears down and clears m.servers, so the
// listener-up gauge and m.applied end up correct regardless of which path
// ran. The concurrent Stop/Apply was already replacing this listener, so
// the crash is moot once it completes -- only a diagnostic log line is
// lost, not correctness.
func (m *Manager) serve(s *dns.Server, ep, addr, proto string, gen int) {
	err := s.ActivateAndServe()

	m.mu.Lock()
	crashed := m.generation == gen
	if crashed {
		m.applied = unappliedSig
	}
	m.mu.Unlock()

	if !crashed {
		m.log.Debug("dnsserver: listener stopped", "endpoint", ep, "proto", proto, "error", err)
		return
	}
	m.log.Error("dnsserver: listener crashed unexpectedly", "endpoint", ep, "proto", proto, "error", err)
	if m.opts.OnListenerChange != nil {
		m.opts.OnListenerChange(proto, addr, false)
	}
}

// Stop drains and closes every bound listener with a bounded timeout.
func (m *Manager) Stop() {
	m.mu.Lock()
	m.generation++
	m.mu.Unlock()

	for _, s := range m.servers {
		ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		if err := s.ShutdownContext(ctx); err != nil {
			m.log.Debug("dnsserver: listener shutdown", "error", err)
		}
		cancel()
	}
	for _, hs := range m.httpServers {
		ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		if err := hs.Shutdown(ctx); err != nil {
			m.log.Debug("dnsserver: doh listener shutdown", "error", err)
		}
		cancel()
	}
	if m.opts.OnListenerChange != nil {
		for _, addr := range m.boundAddrs {
			m.opts.OnListenerChange("udp", addr, false)
			m.opts.OnListenerChange("tcp", addr, false)
		}
		for _, addr := range m.boundSecureAddrs {
			m.opts.OnListenerChange(addr.proto, addr.addr, false)
		}
	}
	m.boundAddrs = nil
	m.boundSecureAddrs = nil
	m.servers = nil
	m.httpServers = nil
}
