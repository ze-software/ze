// Design: plan/spec-dns-server-harness.md -- generic DNS listener lifecycle
// shared by two or more authoritative-only DNS plugins.
//
// Package dnsserver holds the listener lifecycle, EDNS0/client-IP resolution,
// and authoritative-answer shaping that any authoritative-only DNS plugin
// needs, so a second plugin never has to import a sibling plugin
// (ai/rules/plugin-design.md:133) or hand-copy a security-sensitive handler.
package dnsserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// drainTimeout bounds how long Stop waits for in-flight handlers to finish,
// so a wedged handler cannot block a rebind or process exit.
const drainTimeout = 5 * time.Second

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
}

// Manager owns the bound UDP+TCP listeners for a dns.Handler. The handler is
// expected to be state-independent (reading its own snapshot per request), so
// only an endpoint-set change triggers a rebind.
type Manager struct {
	log        *slog.Logger
	handler    dns.Handler
	opts       Options
	servers    []*dns.Server
	boundAddrs []string
	applied    string
}

// New creates a Manager serving handler for every endpoint Apply binds.
func New(log *slog.Logger, handler dns.Handler, opts Options) *Manager {
	return &Manager{log: log, handler: handler, opts: opts, applied: "\x00unset"}
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
func (m *Manager) Apply(enabled bool, endpoints []Endpoint) error {
	sig := endpointSig(enabled, endpoints)
	if sig == m.applied {
		return nil
	}
	m.Stop()
	m.applied = sig
	if !enabled {
		return nil
	}
	for _, e := range endpoints {
		ep := net.JoinHostPort(e.IP.String(), strconv.Itoa(int(e.Port)))
		if err := m.bind(ep, e.IP.String()); err != nil {
			m.log.Error("dnsserver: listen failed", "endpoint", ep, "error", err)
			continue
		}
	}
	if len(m.servers) == 0 && len(endpoints) > 0 {
		return fmt.Errorf("dnsserver: no listeners bound on %d endpoint(s)", len(endpoints))
	}
	m.log.Info("dnsserver: listening", "endpoints", len(m.servers)/2)
	return nil
}

// bind opens a UDP and a TCP listener on ep (best-effort: the caller logs and
// continues if one endpoint fails so the rest still serve).
func (m *Manager) bind(ep, addr string) error {
	lc := listenConfig(m.opts.Freebind)
	pc, err := lc.ListenPacket(context.Background(), "udp", ep)
	if err != nil {
		return fmt.Errorf("udp: %w", err)
	}
	udp := &dns.Server{PacketConn: pc, Handler: m.handler}
	ln, err := lc.Listen(context.Background(), "tcp", ep)
	if err != nil {
		if cerr := pc.Close(); cerr != nil {
			m.log.Debug("dnsserver: closing udp after tcp bind failure", "endpoint", ep, "error", cerr)
		}
		return fmt.Errorf("tcp: %w", err)
	}
	tcp := &dns.Server{Listener: ln, Handler: m.handler}
	m.servers = append(m.servers, udp, tcp)
	m.boundAddrs = append(m.boundAddrs, addr)
	if m.opts.OnListenerChange != nil {
		m.opts.OnListenerChange("udp", addr, true)
		m.opts.OnListenerChange("tcp", addr, true)
	}
	go m.serve(udp, ep, "udp")
	go m.serve(tcp, ep, "tcp")
	return nil
}

func (m *Manager) serve(s *dns.Server, ep, proto string) {
	if err := s.ActivateAndServe(); err != nil {
		m.log.Debug("dnsserver: server stopped", "endpoint", ep, "proto", proto, "error", err)
	}
}

// Stop drains and closes every bound listener with a bounded timeout.
func (m *Manager) Stop() {
	for _, s := range m.servers {
		ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		if err := s.ShutdownContext(ctx); err != nil {
			m.log.Debug("dnsserver: listener shutdown", "error", err)
		}
		cancel()
	}
	if m.opts.OnListenerChange != nil {
		for _, addr := range m.boundAddrs {
			m.opts.OnListenerChange("udp", addr, false)
			m.opts.OnListenerChange("tcp", addr, false)
		}
	}
	m.boundAddrs = nil
	m.servers = nil
}
