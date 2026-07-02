// Design: plan/learned/1033-as112-2-dns-server.md -- as112 DNS server (answer policy,
// allow-from enforcement, thin harness wiring)
// RFC: rfc/short/rfc1035.md -- DNS messages, SOA, NS
// RFC: rfc/short/rfc7534.md -- Section 3.5 authoritative-only, recursion disabled

package as112

import (
	"log/slog"
	"net/netip"
	"time"

	"github.com/miekg/dns"

	"codeberg.org/thomas-mangin/ze/internal/core/dnsserver"
)

// ownAnycastAddrs are the four fixed anycast host addresses this plugin
// binds (register.go's hostAddresses/serverEndpoints constants), computed
// once at package init.
var ownAnycastAddrs = [4]netip.Addr{
	netip.MustParseAddr(anycastV4DirectDelegationAddr),
	netip.MustParseAddr(anycastV4DNAMERedirectionAddr),
	netip.MustParseAddr(anycastV6DirectDelegationAddr),
	netip.MustParseAddr(anycastV6DNAMERedirectionAddr),
}

// isOnBox reports whether ip is the local host -- the H1/M4 carve-out (spec
// Task section, "CRITICAL interaction with H1/M4"): the healthcheck probe
// (finding M4) always queries from the local host, so it must never be
// blocked by allow-from, regardless of configuration. Finding H1 has the
// probe deliberately target a real anycast service address rather than
// loopback (a loopback-only probe would report UP even when the anycast
// path itself is unreachable), so a self-directed query may arrive with
// either 127.0.0.1/::1 or the destination anycast address itself as its
// source -- which one the kernel presents for a same-box query to an
// address bound on `lo` is routing/architecture-dependent and not something
// this plugin controls, so both cases must be recognized as on-box.
//
// KNOWN LIMITATION: unlike loopback (never valid as a UDP source over the
// wire, so any packet claiming to be from 127.0.0.0/8 or ::1 has already
// been dropped or is genuinely local), the four anycast addresses in
// ownAnycastAddrs are ordinary public, globally-known IPs. A remote sender
// can trivially forge one as its UDP source; this carve-out cannot
// distinguish that from a genuine local self-query. This does not enable
// exfiltration (the reply is sent to the real, global anycast address, not
// the spoofed sender), but it does mean an operator relying on `allow-from`
// for strict access control should be aware a forged-source query claiming
// to originate from the node's own anycast address bypasses it. Closing
// this gap fully would need interface-level source verification (e.g.
// reject a query claiming an on-box source unless it actually arrived via
// loopback), which this harness does not currently have.
func isOnBox(ip netip.Addr) bool {
	if ip.IsLoopback() {
		return true
	}
	for _, a := range ownAnycastAddrs {
		if ip == a {
			return true
		}
	}
	return false
}

// allowed reports whether client may be answered: always true when
// allow-from is empty (default public-sink behavior) or the client is
// on-box; otherwise true only if client matches a configured prefix.
func allowed(matcher *dnsserver.Matcher, client netip.Addr) bool {
	if matcher == nil {
		return true
	}
	if isOnBox(client) {
		return true
	}
	_, ok := matcher.Lookup(client)
	return ok
}

// answerQuery is as112's dnsserver.AnswerFunc. The harness has already
// shaped msg (SetReply/Authoritative/Compress/no-recursion, RFC 1035) and
// owns the single wire write; as112 supplies the enabled check, the
// allow-from access-list enforcement, and the static zone-answer policy,
// then returns whether the harness should send msg.
func answerQuery(msg, r *dns.Msg, p dnsserver.Peer) bool {
	start := time.Now()
	m := ametrics()

	// Metric labels reflect only r.Question[0]: answerQuestions below still
	// answers every question in a multi-question message (RFC 1035 permits
	// QDCOUNT > 1), this just avoids per-message multi-valued metric labels
	// for a shape essentially no real resolver sends (in practice DNS
	// clients send exactly one question per message).
	zoneLabel, qLabel := "none", "NONE"
	if len(r.Question) > 0 {
		qLabel = qtypeLabel(r.Question[0].Qtype)
	}

	st := loadState()
	if st == nil || !st.cfg.Enabled {
		msg.SetRcode(r, dns.RcodeRefused)
		m.requestTotal.With(zoneLabel, qLabel).Inc()
		m.responseTotal.With(zoneLabel, qLabel, dns.RcodeToString[msg.Rcode]).Inc()
		return true
	}

	if len(r.Question) > 0 {
		if z, ok := matchZone(r.Question[0].Name); ok {
			zoneLabel = z
		}
	}

	client := dnsserver.RemoteAddr(p)
	if !allowed(st.matcher, client) {
		// requestTotal counts every received request per its own registered
		// description, including ones dropped by allow-from -- a denied
		// query is still a request received, just not answered (mirrors the
		// disabled-service branch above, which counts both request and
		// response even though it also refuses to serve).
		m.requestTotal.With(zoneLabel, qLabel).Inc()
		m.deniedTotal.With("source-not-allowed").Inc()
		return false
	}
	m.requestTotal.With(zoneLabel, qLabel).Inc()

	answerQuestions(msg, r, st.serial, st.cfg.Hostname, st.cfg.Facility, st.cfg.Location)

	m.responseTotal.With(zoneLabel, qLabel, dns.RcodeToString[msg.Rcode]).Inc()
	m.latency.Observe(float64(time.Since(start).Microseconds()) / 1000.0)
	return true
}

// onPanic logs a query that panicked mid-answer; the harness has already
// recovered it and dropped the (unwritten) reply.
func onPanic(rec any) {
	loggerPtr.Load().Error("as112: recovered panic handling query", "panic", rec)
}

// onListenerChange publishes bind/unbind transitions to as112's own
// listenerUp gauge; the harness never owns metrics.
func onListenerChange(proto, addr string, up bool) {
	v := 0.0
	if up {
		v = 1
	}
	ametrics().listenerUp.With(proto, addr).Set(v)
}

// as112Server is a thin adapter over dnsserver.Manager, keeping as112's own
// apply/stopAll call shape so register.go is unaffected by the harness's
// endpoint-agnostic Apply/Stop signatures.
type as112Server struct {
	mgr *dnsserver.Manager
}

// newServerManager builds the as112 DNS server on top of the shared harness
// with Options{Freebind: true} (finding B2): the harness owns the listener
// lifecycle, client-IP resolution, and the authoritative-answer/
// recursion-refusal guard; as112 supplies only answerQuery (metrics +
// policy) and its own listener-up gauge.
func newServerManager(log *slog.Logger) *as112Server {
	handler := dnsserver.Authoritative(answerQuery, onPanic)
	return &as112Server{mgr: dnsserver.New(log, handler, dnsserver.Options{
		Freebind:         true,
		OnListenerChange: onListenerChange,
	})}
}

// apply reconciles the bound listeners with the given endpoints (the fixed
// anycast addresses filtered by address-family, plus loopback, per
// register.go). A pure host-data change is a no-op; an endpoint change stops
// and rebinds (dnsserver.Manager.Apply's endpoint-signature check).
func (s *as112Server) apply(enabled bool, endpoints []dnsserver.Endpoint) error {
	return s.mgr.Apply(enabled, endpoints)
}

func (s *as112Server) stopAll() { s.mgr.Stop() }
