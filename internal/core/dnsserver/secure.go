// Design: docs/architecture/dns/secure-transports.md -- optional DNS-over-TLS
// (RFC 7858) and DNS-over-HTTPS (RFC 8484) listeners on the shared harness.
// RFC: rfc/short/rfc7858.md -- DoT transport; rfc/short/rfc8484.md -- DoH transport
//
// DoT and DoH reuse the SAME dns.Handler as the cleartext UDP/TCP listeners, so
// answer policy (as112 allow-from, geodns client-IP selection, the
// authoritative-answer/recursion-refusal guard) is identical across all four
// transports. Only the wire transport differs: DoT is a TLS-wrapped TCP
// dns.Server (RFC 7766 length-prefixed framing, provided by miekg/dns); DoH is
// a std net/http server whose handler unpacks the application/dns-message body
// or ?dns= parameter and drives the same handler through an in-memory
// dns.ResponseWriter.

package dnsserver

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// DefaultDoTPort is the IANA "domain-s" port for DNS-over-TLS (RFC 7858 §3.1).
	DefaultDoTPort uint16 = 853
	// DefaultDoHPort is the HTTPS port DoH endpoints default to (RFC 8484).
	DefaultDoHPort uint16 = 443
	// DefaultDoHPath is the conventional DoH request path.
	DefaultDoHPath = "/dns-query"

	// dohContentType is the DoH media type for both request and response bodies
	// (RFC 8484 §4.1).
	dohContentType = "application/dns-message"
	// maxDoHBody bounds an accepted DoH query body to the maximum DNS message
	// size, so a hostile client cannot force an unbounded read (DoS guard).
	maxDoHBody = 65535
	// dohReadHeaderTimeout bounds the header read so a slow-loris client cannot
	// hold a DoH connection open indefinitely.
	dohReadHeaderTimeout = 10 * time.Second
)

// Listeners is the full desired listener set across every transport a single
// reconcile owns. Plain is the cleartext UDP+TCP set (unchanged behavior); DoT
// and DoH are optional and require TLSConfig. A zero DoHPath defaults to
// DefaultDoHPath.
type Listeners struct {
	Plain     []Endpoint
	DoT       []Endpoint
	DoH       []Endpoint
	TLSConfig *tls.Config
	DoHPath   string
}

// ErrPreviousListenersLost marks a refused strict apply whose previous
// listeners could not all be bound again: the running config is not fully
// serving. A consumer's reload installer passes it on as
// sdk.ErrRunningConfigLost, which takes the transaction's repair path.
var ErrPreviousListenersLost = errors.New("the running config is not fully serving")

// BindPolicy says what an apply does when a listener it asks for cannot be
// bound, or the TLS material of its secure listeners cannot be loaded. The
// zero value is not a policy: every caller names one.
type BindPolicy uint8

const (
	// BindPolicyUnspecified is the zero value, refused as a Ze defect.
	BindPolicyUnspecified BindPolicy = iota
	// BindLenient logs each failure and keeps whatever bound; the apply fails
	// only when nothing bound. It is the boot (Stage 2) policy: there is no
	// previous config to keep, and an address can appear after the daemon.
	BindLenient
	// BindStrict refuses the whole set when one listener cannot be bound or
	// the TLS material cannot be loaded, and binds the listeners of the
	// previous apply again. It is the reload policy: a commit that reports
	// success MUST be served, and a refused one MUST leave the running
	// config serving, or say with ErrPreviousListenersLost that it could not.
	BindStrict
)

// reconcile reconciles every bound listener (cleartext, DoT, DoH) with the
// desired set. A pure host-data change (same listener set + same certificate)
// is a no-op, the handler being expected to pick up new state itself; any
// change to the endpoint sets, the DoH path, or the serving certificate stops
// and rebinds. Rotating the certificate therefore forces a rebind because the
// signature folds in the leaf certificate fingerprint. policy decides what a
// listener that cannot be bound does (BindPolicy); every caller names it.
//
// The signature only sticks on a successful bind. A transient failure (port
// momentarily taken, anycast address not yet present) must not wedge the
// manager into treating an unchanged, still-failed set as already applied:
// the caller's next identical apply (a periodic reconcile, or an operator
// re-commit, including reverting straight back to a previously-good set after
// an intervening failed apply) has to actually retry the bind, not silently
// no-op with zero listeners up.
func (m *Manager) reconcile(enabled bool, l Listeners, policy BindPolicy) error {
	if policy == BindPolicyUnspecified {
		panic("BUG: dnsserver: reconcile called without a BindPolicy")
	}
	if l.DoHPath == "" {
		l.DoHPath = DefaultDoHPath
	}
	sig := listenersSig(enabled, l)
	if sig == m.appliedSig() {
		return nil
	}

	// DoT/DoH require certificate material. Without it, a lenient apply drops
	// the secure listeners (logged) rather than silently pretending to serve,
	// and the cleartext listeners still come up; a strict apply refuses
	// before it stops anything.
	secureCount := len(l.DoT) + len(l.DoH)
	if enabled && secureCount > 0 && l.TLSConfig == nil {
		if policy == BindStrict {
			return errors.New("dnsserver: reload refused, DoT/DoH configured without TLS material; " +
				"the running listeners are kept; configure a certificate, then commit again")
		}
		m.log.Error("dnsserver: DoT/DoH configured without TLS material; secure listeners not started",
			"dot", len(l.DoT), "doh", len(l.DoH))
		secureCount = 0
	}

	previous, previousEnabled, previousSig := m.running, m.runningEnabled, m.appliedSig()
	m.Stop()

	if !enabled {
		m.setRunning(sig, enabled, l)
		return nil
	}

	bound, failed := m.bindListeners(l)
	if policy == BindStrict && len(failed) > 0 {
		// Close what bound, then bind the previous apply's listeners again,
		// so the running config keeps serving whatever it served.
		m.Stop()
		return m.restorePrevious(previousEnabled, previous, errors.Join(failed...), previousSig)
	}
	for _, err := range failed {
		m.log.Error("dnsserver: listen failed", logKeyError, err)
	}

	desiredTotal := len(l.Plain) + secureCount
	boundTotal := len(m.servers) + len(m.httpServers)
	if desiredTotal > 0 && boundTotal == 0 {
		m.setApplied(unappliedSig)
		return fmt.Errorf("dnsserver: no listeners bound on %d endpoint(s)", desiredTotal)
	}
	m.setRunning(sig, enabled, bound)
	m.log.Info("dnsserver: listening",
		"plain", len(l.Plain), "dot", len(l.DoT), "doh", len(l.DoH))
	return nil
}

// restorePrevious binds the previous apply's listeners again after a strict
// apply refused with cause, and answers the refusal. previous holds only the
// listeners that served, so a listener the running config never bound is not
// counted lost. When every previous listener is bound again the running
// config serves as before and the error says so. When one is not, the error
// wraps ErrPreviousListenersLost and names each endpoint that is not serving
// and why; previous stays the set a later strict failure binds again, and the
// applied signature is cleared, so applying the previous config again binds
// it rather than being taken as a no-op.
func (m *Manager) restorePrevious(enabled bool, previous Listeners, cause error, previousSig string) error {
	if !enabled {
		m.setRunning(previousSig, enabled, previous)
		return fmt.Errorf("dnsserver: reload refused, %w; the running config serves no listener, "+
			"and none was started; free the address and port, or change the listener, then commit again", cause)
	}
	_, lost := m.bindListeners(previous)
	if len(lost) == 0 {
		m.setRunning(previousSig, enabled, previous)
		return fmt.Errorf("dnsserver: reload refused, %w; the running listeners are kept; "+
			"free the address and port, or change the listener, then commit again", cause)
	}
	m.running, m.runningEnabled = previous, enabled
	m.setApplied(unappliedSig)
	total := len(previous.Plain)
	if previous.TLSConfig != nil {
		total += len(previous.DoT) + len(previous.DoH)
	}
	return fmt.Errorf("dnsserver: reload refused, %w; %w: %d of the %d running listener(s) could not be bound "+
		"again, so they are not serving (%w)", cause, ErrPreviousListenersLost, len(lost), total, errors.Join(lost...))
}

// bindListeners binds every listener of l. It answers the listeners that
// bound (l with only those endpoints) and one error for each that could not
// be bound, naming its transport and endpoint. Each failure leaves the other
// listeners bound; the caller decides what a failure means (BindPolicy).
func (m *Manager) bindListeners(l Listeners) (Listeners, []error) {
	bound := Listeners{TLSConfig: l.TLSConfig, DoHPath: l.DoHPath}
	var failed []error
	for _, e := range l.Plain {
		ep := net.JoinHostPort(e.IP.String(), strconv.Itoa(int(e.Port)))
		if err := m.bind(ep, netip.AddrPortFrom(e.IP, e.Port)); err != nil {
			failed = append(failed, fmt.Errorf("cannot listen on %s: %w", ep, err))
			continue
		}
		bound.Plain = append(bound.Plain, e)
	}
	if l.TLSConfig == nil {
		return bound, failed
	}
	for _, e := range l.DoT {
		ep := net.JoinHostPort(e.IP.String(), strconv.Itoa(int(e.Port)))
		if err := m.bindDoT(ep, netip.AddrPortFrom(e.IP, e.Port), l.TLSConfig); err != nil {
			failed = append(failed, fmt.Errorf("cannot listen for DoT on %s: %w", ep, err))
			continue
		}
		bound.DoT = append(bound.DoT, e)
	}
	for _, e := range l.DoH {
		ep := net.JoinHostPort(e.IP.String(), strconv.Itoa(int(e.Port)))
		if err := m.bindDoH(ep, netip.AddrPortFrom(e.IP, e.Port), l.TLSConfig, l.DoHPath); err != nil {
			failed = append(failed, fmt.Errorf("cannot listen for DoH on %s: %w", ep, err))
			continue
		}
		bound.DoH = append(bound.DoH, e)
	}
	return bound, failed
}

// SecureConfig is the parsed DoT/DoH configuration a consumer plugin fills from
// its YANG leaves. CertFile/KeyFile are the operator PEM material shared by both
// transports; empty means fall back to a self-signed certificate.
type SecureConfig struct {
	DoTEnabled bool
	DoTPort    uint16
	DoHEnabled bool
	DoHPort    uint16
	DoHPath    string
	CertFile   string
	KeyFile    string
	// Certificate names an entry in the PKI store, resolved through the
	// Manager's injected TLSMaterialResolver. Mutually exclusive with
	// CertFile/KeyFile. Empty means "no store reference": the file pair or the
	// self-signed fallback applies, exactly as before this field existed.
	Certificate string
}

// DefaultSecureConfig returns a SecureConfig with the DoT/DoH transports
// disabled and the default ports/path (853 / 443 / /dns-query). Consumers seed
// this then overlay ParseSecureLeaves, so the Go defaults mirror the YANG ones.
func DefaultSecureConfig() SecureConfig {
	return SecureConfig{
		DoTPort: DefaultDoTPort,
		DoHPort: DefaultDoHPort,
		DoHPath: DefaultDoHPath,
	}
}

// ParseSecureLeaves overlays the "tls" and "doh" containers of a plugin's config
// node (the map at service.<plugin>) onto sc. The tls container carries DoT's
// enable + listen-port and the cert material shared with DoH; the doh container
// carries DoH's enable + listen-port + path. A zero or non-numeric port is
// rejected (defensive mirror of the YANG zt:port range 1..65535).
func ParseSecureLeaves(node map[string]any, sc *SecureConfig, svc string) error {
	if tlsM, ok := node["tls"].(map[string]any); ok {
		if v, ok := tlsM["enabled"].(string); ok {
			sc.DoTEnabled = v == "true"
		}
		if v, ok := tlsM["listen-port"].(string); ok {
			p, err := strconv.ParseUint(v, 10, 16)
			if err != nil || p == 0 {
				return fmt.Errorf("%s: tls listen-port %q invalid (expected 1..65535)", svc, v)
			}
			sc.DoTPort = uint16(p)
		}
		if v, ok := tlsM["cert-file"].(string); ok {
			sc.CertFile = v
		}
		if v, ok := tlsM["key-file"].(string); ok {
			sc.KeyFile = v
		}
		if v, ok := tlsM["certificate"].(string); ok {
			sc.Certificate = v
		}
		// Two sources of TLS material in one container is a configuration
		// error, not a precedence question: whichever one lost would be
		// silently ignored, and the operator would have no way to see which.
		if sc.Certificate != "" && (sc.CertFile != "" || sc.KeyFile != "") {
			return fmt.Errorf("%s: tls certificate %q and cert-file/key-file are mutually exclusive (use one source of TLS material)", svc, sc.Certificate)
		}
	}
	if dohM, ok := node["doh"].(map[string]any); ok {
		if v, ok := dohM["enabled"].(string); ok {
			sc.DoHEnabled = v == "true"
		}
		if v, ok := dohM["listen-port"].(string); ok {
			p, err := strconv.ParseUint(v, 10, 16)
			if err != nil || p == 0 {
				return fmt.Errorf("%s: doh listen-port %q invalid (expected 1..65535)", svc, v)
			}
			sc.DoHPort = uint16(p)
		}
		if v, ok := dohM["path"].(string); ok && v != "" {
			sc.DoHPath = v
		}
	}
	return nil
}

// ApplyWithSecure reconciles the plain listeners plus any DoT/DoH listeners
// described by sc. The secure listeners bind the SAME IP set as the plain
// endpoints (deduplicated) but on their own ports, so a consumer only passes its
// cleartext endpoints and the secure config. Certificate material is loaded once
// (self-signed) or per-apply (operator files, so rotation is picked up); a TLS
// load failure under BindLenient disables only the secure listeners, never the
// cleartext ones. Under BindStrict it refuses the apply before any listener
// stops, so the running set keeps serving, unless the secure listeners would
// not change (same sc, same addresses as the last accepted apply): the reload
// then keeps the material those listeners already serve, logs the load
// failure, and goes on, so a reload of host data alone is not refused for a
// certificate file that left the disk.
func (m *Manager) ApplyWithSecure(enabled bool, plain []Endpoint, sc SecureConfig, policy BindPolicy, log *slog.Logger) error {
	l := Listeners{Plain: plain, DoHPath: sc.DoHPath}
	ips := uniqueIPs(plain)
	sans := make([]string, len(ips))
	for i, ip := range ips {
		sans[i] = ip.String()
	}
	inputs := secureInputs{enabled: enabled, sc: sc, sans: strings.Join(sans, ",")}
	if enabled && (sc.DoTEnabled || sc.DoHEnabled) {
		tlsCfg, err := m.buildSecureTLS(sc, sans, log)
		if err != nil {
			kept, kerr := m.keptSecure(inputs, policy, err, log)
			if kerr != nil {
				return kerr
			}
			tlsCfg = kept.tls
		}
		if tlsCfg != nil {
			l.TLSConfig = tlsCfg
			if sc.DoTEnabled {
				l.DoT = endpointsOnPort(ips, sc.DoTPort)
			}
			if sc.DoHEnabled {
				l.DoH = endpointsOnPort(ips, sc.DoHPort)
			}
		}
	}
	if err := m.reconcile(enabled, l, policy); err != nil {
		return err
	}
	m.secureRunning = &secureApplied{inputs: inputs, tls: l.TLSConfig}
	return nil
}

// keptSecure answers a TLS material load failure (loadErr) with the secure
// side the apply goes on with. A lenient apply logs it and starts no secure
// listener (a zero secureApplied). A strict apply refuses it unless inputs
// equal those of the last accepted apply and that apply holds TLS material,
// whose secure side it then keeps, and logs why. An accepted apply that holds
// none (a lenient boot whose own load failed) serves no DoT/DoH listener, so
// keeping it would accept a commit whose DoT/DoH is configured and not served.
func (m *Manager) keptSecure(inputs secureInputs, policy BindPolicy, loadErr error, log *slog.Logger) (secureApplied, error) {
	if policy != BindStrict {
		log.Error("dnsserver: TLS material load failed; DoT/DoH not started", logKeyError, loadErr)
		return secureApplied{}, nil
	}
	running := m.secureRunning
	if running == nil {
		return secureApplied{}, secureLoadRefusal(loadErr)
	}
	if running.inputs != inputs {
		return secureApplied{}, secureLoadRefusal(loadErr)
	}
	if running.tls == nil {
		return secureApplied{}, secureLoadRefusal(loadErr)
	}
	log.Warn("dnsserver: TLS material cannot be loaded again; this reload does not change the DoT/DoH "+
		"listeners, so they keep the material they serve; restore the certificate before the next "+
		"change to them", logKeyError, loadErr)
	return *running, nil
}

// secureLoadRefusal is the error of a strict apply refused because the TLS
// material of its secure listeners cannot be loaded.
func secureLoadRefusal(err error) error {
	return fmt.Errorf("dnsserver: reload refused, the DoT/DoH TLS material cannot be loaded: %w; "+
		"the running listeners are kept; name a certificate the PKI store holds "+
		"(ze show pki certificates) or a readable cert-file and key-file, then commit again", err)
}

// secureInputs is everything the secure listeners of one apply are built
// from: whether the apply serves, the secure config, and the addresses the
// certificate names. Comparable, so two applies with equal inputs build the
// same secure listeners.
type secureInputs struct {
	enabled bool
	sc      SecureConfig
	sans    string
}

// secureApplied is the secure side of the last accepted ApplyWithSecure: its
// inputs and the TLS material its listeners serve (nil when none started).
type secureApplied struct {
	inputs secureInputs
	tls    *tls.Config
}

// buildSecureTLS returns the tls.Config for the secure listeners. Operator PEM
// files are re-read every apply (so a rotated cert is picked up, and an unchanged
// cert yields the same fingerprint hence no rebind). The self-signed fallback is
// generated once and cached, so a config reload does not churn a rebind.
func (m *Manager) buildSecureTLS(sc SecureConfig, sans []string, log *slog.Logger) (*tls.Config, error) {
	if sc.Certificate != "" {
		// Resolved per apply, like the operator file pair below it, so rotating
		// the store entry's material is picked up and the listener signature's
		// certificate fingerprint forces the rebind.
		if m.opts.TLSMaterialResolver == nil {
			return nil, fmt.Errorf("dnsserver: tls certificate %q configured but this consumer injected no TLS material resolver", sc.Certificate)
		}
		certPEM, keyPEM, err := m.opts.TLSMaterialResolver(sc.Certificate)
		if err != nil {
			return nil, fmt.Errorf("dnsserver: tls certificate %q: %w", sc.Certificate, err)
		}
		cfg, err := selfcert.NewTLSConfig(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("dnsserver: tls certificate %q material: %w", sc.Certificate, err)
		}
		return cfg, nil
	}
	if sc.CertFile != "" || sc.KeyFile != "" {
		return LoadTLSMaterial(sc.CertFile, sc.KeyFile, sans, log)
	}
	if m.selfSigned != nil {
		return m.selfSigned, nil
	}
	cfg, err := LoadTLSMaterial("", "", sans, log)
	if err != nil {
		return nil, err
	}
	m.selfSigned = cfg
	return cfg, nil
}

// uniqueIPs returns the distinct IPs across endpoints, preserving first-seen
// order.
func uniqueIPs(endpoints []Endpoint) []netip.Addr {
	seen := make(map[netip.Addr]struct{}, len(endpoints))
	out := make([]netip.Addr, 0, len(endpoints))
	for _, e := range endpoints {
		if _, ok := seen[e.IP]; ok {
			continue
		}
		seen[e.IP] = struct{}{}
		out = append(out, e.IP)
	}
	return out
}

// endpointsOnPort pairs each IP with port.
func endpointsOnPort(ips []netip.Addr, port uint16) []Endpoint {
	eps := make([]Endpoint, len(ips))
	for i, ip := range ips {
		eps[i] = Endpoint{IP: ip, Port: port}
	}
	return eps
}

// listenersSig is the signature of the full desired listener set. It folds in
// every transport's endpoints, the DoH path, and a fingerprint of the serving
// certificate so a rotated cert (same endpoints) still forces a rebind.
func listenersSig(enabled bool, l Listeners) string {
	if !enabled {
		return "disabled"
	}
	var b textbuf.Buffer
	b.Str("plain=").Str(endpointSig(true, l.Plain))
	b.Str(";dot=").Str(endpointSig(true, l.DoT))
	b.Str(";doh=").Str(endpointSig(true, l.DoH))
	b.Str(";dohpath=").Str(l.DoHPath)
	b.Str(";cert=").Str(tlsFingerprint(l.TLSConfig))
	return b.String()
}

// tlsFingerprint returns a stable hex fingerprint of the leaf certificate so the
// listener signature changes when the operator rotates the cert. Empty when no
// certificate is present.
func tlsFingerprint(cfg *tls.Config) string {
	if cfg == nil || len(cfg.Certificates) == 0 || len(cfg.Certificates[0].Certificate) == 0 {
		return ""
	}
	sum := sha256.Sum256(cfg.Certificates[0].Certificate[0])
	return hex.EncodeToString(sum[:])
}

// bindDoT opens a TLS-wrapped TCP listener and serves DNS-over-TLS (RFC 7858) on
// it. DoT is TCP-only -- there is no UDP counterpart -- so unlike bind() this
// opens a single listener. It reuses m.serve (and its generation-based crash
// detection) because a TLS listener is an ordinary net.Listener to miekg/dns.
func (m *Manager) bindDoT(ep string, listen netip.AddrPort, tlsConfig *tls.Config) error {
	lc := ListenConfig(m.opts.Freebind)
	ln, err := lc.Listen(context.Background(), "tcp", ep)
	if err != nil {
		return fmt.Errorf("tcp: %w", err)
	}
	started := make(chan struct{})
	srv := &dns.Server{
		Listener:          tls.NewListener(ln, tlsConfig),
		Handler:           m.handler,
		MsgAcceptFunc:     m.opts.MsgAcceptFunc,
		NotifyStartedFunc: func() { close(started) },
	}
	m.servers = append(m.servers, srv)
	m.boundSecureAddrs = append(m.boundSecureAddrs, secureAddr{proto: "dot", addr: listen})
	if m.opts.OnListenerChange != nil {
		m.opts.OnListenerChange("dot", listen, true)
	}
	m.mu.Lock()
	gen := m.generation
	m.mu.Unlock()
	go m.serve(srv, ep, listen, "dot", gen)
	<-started
	return nil
}

// bindDoH opens a TLS-wrapped TCP listener and serves DNS-over-HTTPS (RFC 8484)
// on it, mounting the DoH handler at path. Binding happens before the serve
// goroutine starts, so there is no advertise-before-bind gap.
func (m *Manager) bindDoH(ep string, listen netip.AddrPort, tlsConfig *tls.Config, path string) error {
	lc := ListenConfig(m.opts.Freebind)
	ln, err := lc.Listen(context.Background(), "tcp", ep)
	if err != nil {
		return fmt.Errorf("tcp: %w", err)
	}
	tlsLn := tls.NewListener(ln, tlsConfig)
	mux := http.NewServeMux()
	mux.HandleFunc(path, m.dohHandler(tlsLn.Addr()))
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: dohReadHeaderTimeout}
	m.httpServers = append(m.httpServers, srv)
	m.boundSecureAddrs = append(m.boundSecureAddrs, secureAddr{proto: "doh", addr: listen})
	if m.opts.OnListenerChange != nil {
		m.opts.OnListenerChange("doh", listen, true)
	}
	m.mu.Lock()
	gen := m.generation
	m.mu.Unlock()
	go m.serveHTTP(srv, tlsLn, ep, listen, gen)
	return nil
}

// serveHTTP runs a DoH http.Server until it exits. Mirrors serve()'s
// generation-based crash detection: a Serve return whose generation is still
// current is an unexpected crash (the graceful path is Stop -> generation++ ->
// Shutdown, which makes this generation stale first), so it must surface and
// mark the manager unapplied. http.ErrServerClosed from a graceful Shutdown lands
// on the stale-generation (debug) path.
func (m *Manager) serveHTTP(srv *http.Server, ln net.Listener, ep string, listen netip.AddrPort, gen int) {
	err := srv.Serve(ln)

	m.mu.Lock()
	crashed := m.generation == gen
	if crashed {
		m.applied = unappliedSig
	}
	m.mu.Unlock()

	if !crashed {
		m.log.Debug("dnsserver: doh listener stopped", "endpoint", ep, logKeyError, err)
		return
	}
	m.log.Error("dnsserver: doh listener crashed unexpectedly", "endpoint", ep, logKeyError, err)
	if m.opts.OnListenerChange != nil {
		m.opts.OnListenerChange("doh", listen, false)
	}
}

// dohHandler returns the RFC 8484 request handler. localAddr is the bound TLS
// listener address, reported as the in-memory ResponseWriter's LocalAddr; the
// query's RemoteAddr is the HTTP peer, so source-based answer policy applies to
// DoH clients exactly as it does to UDP/TCP clients.
func (m *Manager) dohHandler(localAddr net.Addr) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		wire, code, emsg := dohRequestBody(req)
		if code != http.StatusOK {
			http.Error(w, emsg, code)
			return
		}

		reqMsg := new(dns.Msg)
		if err := reqMsg.Unpack(wire); err != nil {
			http.Error(w, "malformed DNS message", http.StatusBadRequest)
			return
		}

		rw := &dohResponseWriter{remote: httpRemoteAddr(req.RemoteAddr), local: localAddr}
		m.handler.ServeDNS(rw, reqMsg)
		if rw.msg == nil {
			// The handler declined to answer (e.g. as112 allow-from denied the
			// source). Cleartext DNS silently drops; DoH is request/response over
			// HTTP, so the honest mapping of "received but refused" is 403.
			http.Error(w, "query refused", http.StatusForbidden)
			return
		}
		out, err := rw.msg.Pack()
		if err != nil {
			http.Error(w, "failed to pack response", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", dohContentType)
		if ttl := minAnswerTTL(rw.msg); ttl > 0 {
			// RFC 8484 §5.1: cap HTTP freshness at the smallest record TTL.
			var tb textbuf.Buffer
			w.Header().Set("Cache-Control", tb.Str("max-age=").Uint(uint64(ttl)).String())
		}
		w.WriteHeader(http.StatusOK)
		if _, werr := w.Write(out); werr != nil {
			m.log.Debug("dnsserver: doh response write", logKeyError, werr)
		}
	}
}

// dohRequestBody extracts the wire-format DNS query from a DoH request per RFC
// 8484 §4.1: GET carries it base64url-unpadded in ?dns=, POST carries it raw in
// the application/dns-message body. Returns (wire, 200, "") on success, else a
// non-200 HTTP status and message.
func dohRequestBody(req *http.Request) ([]byte, int, string) {
	switch req.Method {
	case http.MethodGet:
		enc := req.URL.Query().Get("dns")
		if enc == "" {
			return nil, http.StatusBadRequest, "missing dns parameter"
		}
		b, err := base64.RawURLEncoding.DecodeString(enc)
		if err != nil {
			return nil, http.StatusBadRequest, "invalid dns parameter"
		}
		if len(b) > maxDoHBody {
			return nil, http.StatusBadRequest, "query too large"
		}
		return b, http.StatusOK, ""
	case http.MethodPost:
		if ct := req.Header.Get("Content-Type"); ct != dohContentType {
			return nil, http.StatusUnsupportedMediaType, "unsupported content-type"
		}
		b, err := io.ReadAll(io.LimitReader(req.Body, maxDoHBody+1))
		if err != nil {
			return nil, http.StatusBadRequest, "read error"
		}
		if len(b) > maxDoHBody {
			return nil, http.StatusBadRequest, "query too large"
		}
		return b, http.StatusOK, ""
	default:
		return nil, http.StatusMethodNotAllowed, "method not allowed"
	}
}

// httpRemoteAddr parses an HTTP RemoteAddr ("host:port") into a *net.TCPAddr so
// dnsserver.RemoteAddr(Peer) resolves the DoH client IP identically to a TCP
// packet source.
func httpRemoteAddr(s string) net.Addr {
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		host = s
	}
	port, _ := strconv.Atoi(portStr)
	return &net.TCPAddr{IP: net.ParseIP(host), Port: port}
}

// minAnswerTTL returns the smallest TTL across the answer section, or 0 when
// there are no answers.
func minAnswerTTL(msg *dns.Msg) uint32 {
	var out uint32
	first := true
	for _, rr := range msg.Answer {
		ttl := rr.Header().Ttl
		if first || ttl < out {
			out, first = ttl, false
		}
	}
	return out
}

// dohResponseWriter is an in-memory dns.ResponseWriter that captures the reply
// the shared dns.Handler writes, so the DoH handler can pack it into the HTTP
// response body. RemoteAddr reports the HTTP peer so answer policy that keys on
// the client source works over DoH.
type dohResponseWriter struct {
	remote net.Addr
	local  net.Addr
	msg    *dns.Msg
}

func (w *dohResponseWriter) LocalAddr() net.Addr       { return w.local }
func (w *dohResponseWriter) RemoteAddr() net.Addr      { return w.remote }
func (w *dohResponseWriter) WriteMsg(m *dns.Msg) error { w.msg = m; return nil }

// Write accepts an already-packed message (the harness always uses WriteMsg, but
// the interface requires Write too); it unpacks so the captured message is
// consistent.
func (w *dohResponseWriter) Write(b []byte) (int, error) {
	msg := new(dns.Msg)
	if err := msg.Unpack(b); err != nil {
		return 0, err
	}
	w.msg = msg
	return len(b), nil
}

func (w *dohResponseWriter) Close() error        { return nil }
func (w *dohResponseWriter) TsigStatus() error   { return nil }
func (w *dohResponseWriter) TsigTimersOnly(bool) {}
func (w *dohResponseWriter) Hijack()             {}
