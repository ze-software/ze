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
	"slices"
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
// list). An apply uses it to mark "nothing is known to be correctly bound" so
// the next apply is never short-circuited by the same-signature no-op
// fast path -- covers both a fresh Manager and any Manager that just lost a
// listener (full bind failure, or an unexpected post-bind crash).
const unappliedSig = "\x00unset"

// logKeyError is the structured-log attribute that carries an error value.
// Every log line in this package names the error with the same key, so an
// operator greps one word.
const logKeyError = "error"

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
	// now up. The listener gauge stays with the consumer, whose listen
	// addresses are its label values, and this callback is how it gets one.
	// The harness owns only the metric a consumer cannot see, which today is
	// the write-failure counter in metrics.go.
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

	// UDPSize is the read buffer, in octets, of each UDP listener: a datagram
	// longer than it is cut short and dropped as malformed. Zero keeps the
	// miekg default of 512 (dns.MinMsgSize), which the authoritative services
	// use today. A recursive service passes RecursiveUDPSize, so a client's
	// EDNS query of up to that size is read whole.
	UDPSize int
}

// Manager owns the bound UDP+TCP listeners for a dns.Handler. The handler is
// expected to be state-independent (reading its own snapshot per request), so
// only an endpoint-set change triggers a rebind.
//
// servers/boundAddrs are written only by the ApplyWithSecure/Stop-calling goroutine
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
	// a rebind on every no-op reconcile). Written only by the
	// ApplyWithSecure-calling goroutine.
	selfSigned *tls.Config

	// running and runningEnabled are the listeners that serve: those of the
	// last completed apply that bound, which a strict apply that fails binds
	// again. Written only by the ApplyWithSecure-calling goroutine.
	running        Listeners
	runningEnabled bool
	// secureRunning is the secure side of the last accepted ApplyWithSecure,
	// nil before one. A strict apply whose TLS material cannot be loaded keeps
	// it when its inputs are unchanged. Written only by the
	// ApplyWithSecure-calling goroutine.
	secureRunning *secureApplied

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

// New creates a Manager serving handler for every endpoint ApplyWithSecure binds.
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
	slices.Sort(eps)
	return strings.Join(eps, ",")
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

// setRunning records a completed apply: its signature, and the listener set a
// later strict apply that fails binds again.
func (m *Manager) setRunning(sig string, enabled bool, l Listeners) {
	m.running, m.runningEnabled = l, enabled
	m.setApplied(sig)
}

// bind opens a UDP and a TCP listener on ep (best-effort: the caller logs and
// continues if one endpoint fails so the rest still serve).
//
// Waits for both spawned listener goroutines to reach dns.Server's genuine
// "started" state (via NotifyStartedFunc) before returning. Without this,
// a caller that calls Stop() immediately after bind() returns (e.g. a rapid
// apply->apply sequence) can race ActivateAndServe: dns.Server.
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
	udp := &dns.Server{PacketConn: pc, Handler: m.handler, UDPSize: m.opts.UDPSize, NotifyStartedFunc: func() { close(udpStarted) }}
	ln, err := lc.Listen(context.Background(), "tcp", ep)
	if err != nil {
		if cerr := pc.Close(); cerr != nil {
			m.log.Debug("dnsserver: closing udp after tcp bind failure", "endpoint", ep, logKeyError, cerr)
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
// listener look healthy forever, and a later apply with the same
// (still-desired, unchanged) endpoint set would short-circuit as a no-op
// instead of actually rebinding.
//
// Known benign race: a genuine crash whose ActivateAndServe return races a
// concurrent Stop/apply's generation++ can be misclassified as graceful
// (crashed==false here), losing this function's Error log line. This does
// not lose state: Stop's own loop unconditionally calls OnListenerChange
// false for every bound address it tears down and clears m.servers, so the
// listener-up gauge and m.applied end up correct regardless of which path
// ran. The concurrent Stop/apply was already replacing this listener, so
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
		m.log.Debug("dnsserver: listener stopped", "endpoint", ep, "proto", proto, logKeyError, err)
		return
	}
	m.log.Error("dnsserver: listener crashed unexpectedly", "endpoint", ep, "proto", proto, logKeyError, err)
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
			m.log.Debug("dnsserver: listener shutdown", logKeyError, err)
		}
		cancel()
	}
	for _, hs := range m.httpServers {
		ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		if err := hs.Shutdown(ctx); err != nil {
			m.log.Debug("dnsserver: doh listener shutdown", logKeyError, err)
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
