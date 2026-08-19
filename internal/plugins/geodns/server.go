// Design: docs/architecture/dns/geodns.md -- geodns DNS server (listener, EDNS0, answer synthesis)
// Design: docs/architecture/dns/server-harness.md -- listener lifecycle, client-IP and
// authoritative-answer shaping moved to internal/core/dnsserver; this file
// keeps only geodns's answer policy and its thin harness wiring.
// RFC: rfc/short/rfc7871.md -- EDNS0 client subnet; rfc/short/rfc1035.md -- DNS messages, SOA, NS

package geodns

import (
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"

	zepki "github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/dnsserver"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// computeSerial produces the 32-bit SOA serial for a config generation.
//   - auto-epoch: max(Unix seconds, prev+1) — strictly increases at any rate,
//     fits uint32 to ~2106 (RFC 1982 arithmetic handles the eventual wrap).
//   - auto-datetime: YYYYMMDDnn (RFC 1912); capped at 100 revisions/day because
//     the 8-digit date prefix leaves only 2 counter digits within uint32.
//   - fixed: the configured serial leaf verbatim.
func computeSerial(soa soaConfig, prevSerial uint32, now time.Time) uint32 {
	switch soa.SerialMode {
	case "fixed":
		return soa.Serial
	case "auto-datetime":
		y, mo, d := now.Date()
		base := uint32(y*10000+int(mo)*100+d) * 100
		if prevSerial >= base && prevSerial < base+100 {
			nn := min(prevSerial-base+1, 99)
			return base + nn
		}
		return base
	default: // auto-epoch
		s := uint32(now.Unix())
		if s <= prevSerial {
			s = prevSerial + 1
		}
		return s
	}
}

// nsID returns the 1-based nameserver index when queried is ns<n>.<zone> within
// the configured nameserver count, else 0.
func nsID(queried, zone string, ns []netip.Addr) int {
	name := strings.ToLower(queried)
	if !strings.HasPrefix(name, "ns") || len(name) != len(zone)+4 || !strings.HasSuffix(name, zone) {
		return 0
	}
	n, err := strconv.Atoi(string(name[2]))
	if err != nil || n < 1 || n > len(ns) {
		return 0
	}
	return n
}

// inZone reports whether name is zone itself or a name inside it.
//
// The comparison runs over DNS labels, never over characters: the name
// "evilexample.com." ends with every character of the zone "example.com."
// while its label sequence does not nest inside it. A character suffix match
// therefore places a name Ze serves no zone for inside one it does, and the
// answer that follows claims authority over somebody else's namespace.
func inZone(name, zone string) bool {
	n := fqdn(name)
	return n == zone || dns.IsSubDomain(zone, n)
}

// matchZone returns the longest configured zone that contains name.
func matchZone(name string, zones []string) string {
	best := ""
	for _, z := range zones {
		if z != "" && inZone(name, z) && len(z) > len(best) {
			best = z
		}
	}
	return best
}

func equalName(a, zone string) bool { return strings.EqualFold(fqdn(a), zone) }

// nameExists reports whether zone owns the name, whatever record types the
// query asks for and whichever host set this client draws from. It answers the
// question RFC 1035 Section 4.1.1 puts behind RCODE 3, "the domain name
// referenced in the query does not exist", and it is deliberately independent
// of the client: existence is a property of the zone, so a name configured for
// one source prefix and not another still EXISTS for every client. That case is
// no data of this type, not a name error.
//
// Three things give a name existence: the zone apex (which owns the SOA and the
// NS set), a synthesized ns<n>.<zone> glue name, and any name st.names carries
// (each configured host, plus every interior node above one).
func nameExists(st *resolverState, zone, name string) bool {
	if equalName(name, zone) {
		return true
	}
	if nsID(fqdn(name), zone, st.cfg.Nameservers) > 0 {
		return true
	}
	_, ok := st.names[fqdn(name)]
	return ok
}

func resolveHost(st *resolverState, client netip.Addr, name string) []dnsRecord {
	setName, ok := st.matcher.Lookup(client)
	if !ok {
		return nil
	}
	hs := st.cfg.HostSets[setName]
	if hs == nil {
		return nil
	}
	return hs.Hosts[fqdn(name)]
}

func typeMatches(qtype uint16, kind recordKind) bool {
	switch kind {
	case kindA:
		return qtype == dns.TypeA
	case kindAAAA:
		return qtype == dns.TypeAAAA
	case kindSRV:
		return qtype == dns.TypeSRV
	default:
		return false
	}
}

// hdr builds an RR header. The TTL reaches the wire as configured.
//
// It is NOT floored against the zone's SOA MINIMUM. RFC 1035 Section 3.3.13
// asks for that floor, and RFC 2308 (Standards Track, "Updates: 1034, 1035")
// Section 4 withdraws it: "Despite being the original defined meaning, the
// first of these, the minimum TTL value of all RRs in a zone, has never in
// practice been used and is hereby deprecated." MINIMUM's one remaining meaning
// is the negative-caching TTL, which buildSOA already carries in the SOA it
// puts in the Authority section.
func hdr(name string, rrtype uint16, ttl uint32) dns.RR_Header {
	return dns.RR_Header{Name: name, Rrtype: rrtype, Class: dns.ClassINET, Ttl: ttl}
}

func recordRR(name string, rec dnsRecord) dns.RR {
	switch rec.Kind {
	case kindAAAA:
		return &dns.AAAA{Hdr: hdr(name, dns.TypeAAAA, rec.TTL), AAAA: rec.Addr.AsSlice()}
	case kindSRV:
		return &dns.SRV{Hdr: hdr(name, dns.TypeSRV, rec.TTL), Priority: rec.Priority, Weight: rec.Weight, Port: rec.Port, Target: rec.Target}
	default:
		return &dns.A{Hdr: hdr(name, dns.TypeA, rec.TTL), A: rec.Addr.AsSlice()}
	}
}

// rname builds the SOA RNAME. A bare label (no dot) is taken as a mailbox local
// part under the zone (hostmaster -> hostmaster.<zone>); a dotted value is used
// as a full domain name.
func rname(contact, zone string) string {
	if contact == "" {
		contact = "hostmaster"
	}
	if strings.Contains(contact, ".") {
		return fqdn(contact)
	}
	var tb textbuf.Buffer
	return tb.Str(contact).Byte('.').Str(zone).String()
}

func buildSOA(st *resolverState, zone string) *dns.SOA {
	mname := st.cfg.SOA.MName
	if mname == "" {
		var tb textbuf.Buffer
		mname = tb.Str("ns1.").Str(zone).String()
	}
	return &dns.SOA{
		Hdr:     hdr(zone, dns.TypeSOA, st.cfg.SOA.Minimum),
		Ns:      mname,
		Mbox:    rname(st.cfg.SOA.Contact, zone),
		Serial:  st.serial,
		Refresh: st.cfg.SOA.Refresh,
		Retry:   st.cfg.SOA.Retry,
		Expire:  st.cfg.SOA.Expire,
		Minttl:  st.cfg.SOA.Minimum,
	}
}

// appendNS adds the synthesized NS records (and their A glue). For an NS query
// the records go in the Answer; for SOA/negative they go in the Authority.
func appendNS(msg *dns.Msg, st *resolverState, zone string, authority bool) {
	var tb textbuf.Buffer
	for i, ip := range st.cfg.Nameservers {
		nsName := tb.Reset().Str("ns").Int(int64(i + 1)).Byte('.').Str(zone).String()
		ns := &dns.NS{Hdr: hdr(zone, dns.TypeNS, st.cfg.DefaultTTL), Ns: nsName}
		if authority {
			msg.Ns = append(msg.Ns, ns)
		} else {
			msg.Answer = append(msg.Answer, ns)
		}
		msg.Extra = append(msg.Extra, &dns.A{Hdr: hdr(nsName, dns.TypeA, st.cfg.DefaultTTL), A: ip.AsSlice()})
	}
}

// answerQuestions fills msg for each question, and picks the reply's RCODE from
// what the served zones say about the names asked for. Three answers are
// possible, and the split between them is the whole policy:
//
//   - A name under no configured zone draws Refused. RFC 1035 Section 4.1.1
//     defines RCODE 5 as "Refused - The name server refuses to perform the
//     specified operation for policy reasons", and Ze serving no zone for the
//     name is exactly that. The harness clears AA for it (dnsserver's
//     shapeAuthoritative), so the reply makes no authority claim either.
//   - A name inside a configured zone that the zone does not own draws RCODE 3.
//     RFC 1035 Section 4.1.1: "Name Error - Meaningful only for responses from
//     an authoritative name server, this code signifies that the domain name
//     referenced in the query does not exist."
//   - A name the zone owns, with no record of the type asked for, draws NOERROR
//     with an empty Answer. This covers the case that makes geodns geodns: a
//     host configured in one host set and not another EXISTS for every client,
//     so a client whose host set has no record for it gets no data of this
//     type, never a name error.
//
// The last two both carry the zone SOA in the Authority section, which RFC 2308
// Section 3 requires of an authoritative server "when reporting an NXDOMAIN or
// indicating that no data of the requested type exists", so that a resolver can
// cache the negative answer.
//
// A/AAAA/SRV/ANY resolve per the client source. The RCODE is assigned directly
// rather than through Msg.SetRcode, which calls SetReply and would drop every
// question after the first from a multi-question reply.
func answerQuestions(msg, r *dns.Msg, st *resolverState, client netip.Addr) {
	served, missing := false, false
	for _, q := range r.Question {
		zone := matchZone(q.Name, st.cfg.Zones)
		if zone == "" {
			continue
		}
		served = true
		if !nameExists(st, zone, q.Name) {
			missing = true
			msg.Ns = append(msg.Ns, buildSOA(st, zone))
			continue
		}
		switch q.Qtype {
		case dns.TypeA, dns.TypeAAAA, dns.TypeSRV, dns.TypeANY:
			if idx := nsID(q.Name, zone, st.cfg.Nameservers); idx > 0 {
				msg.Answer = append(msg.Answer, &dns.A{Hdr: hdr(q.Name, dns.TypeA, st.cfg.DefaultTTL), A: st.cfg.Nameservers[idx-1].AsSlice()})
				continue
			}
			added := false
			for _, rec := range resolveHost(st, client, q.Name) {
				if q.Qtype == dns.TypeANY || typeMatches(q.Qtype, rec.Kind) {
					msg.Answer = append(msg.Answer, recordRR(q.Name, rec))
					added = true
				}
			}
			if !added {
				msg.Ns = append(msg.Ns, buildSOA(st, zone))
			}
		case dns.TypeSOA:
			if equalName(q.Name, zone) {
				msg.Answer = append(msg.Answer, buildSOA(st, zone))
				appendNS(msg, st, zone, true)
			} else {
				msg.Ns = append(msg.Ns, buildSOA(st, zone))
			}
		case dns.TypeNS:
			if equalName(q.Name, zone) {
				appendNS(msg, st, zone, false)
			} else {
				msg.Ns = append(msg.Ns, buildSOA(st, zone))
			}
		default:
			msg.Ns = append(msg.Ns, buildSOA(st, zone))
		}
	}
	switch {
	case !served:
		msg.Rcode = dns.RcodeRefused
	case missing:
		msg.Rcode = dns.RcodeNameError
	}
}

// answerQuery is geodns's dnsserver.AnswerFunc. The harness has already shaped
// msg (SetReply/Authoritative/Compress/no-recursion, RFC 1035) and owns the
// single wire write; geodns supplies the per-request state snapshot, metrics,
// the enabled check, client-IP resolution (RFC 7871), and answer policy, then
// returns whether the harness should send msg. p is the read-only transport
// peer; the packet source is resolved lazily (dnsserver.RemoteAddr) only when
// the answer actually depends on it, so a refused query costs nothing for it.
func answerQuery(msg, r *dns.Msg, p dnsserver.Peer) bool {
	start := time.Now()
	m := gmetrics()

	zoneLabel, qLabel := "none", "NONE"
	if len(r.Question) > 0 {
		qLabel = qtypeLabel(r.Question[0].Qtype)
	}

	defer func() {
		m.responseTotal.With(zoneLabel, qLabel, dns.RcodeToString[msg.Rcode]).Inc()
		m.latency.Observe(float64(time.Since(start).Microseconds()) / 1000.0)
	}()

	st := loadState()
	if st == nil || !st.cfg.Enabled {
		msg.SetRcode(r, dns.RcodeRefused)
		m.requestTotal.With(zoneLabel, qLabel).Inc()
		return true
	}

	if len(r.Question) > 0 {
		if z := matchZone(r.Question[0].Name, st.cfg.Zones); z != "" {
			zoneLabel = z
		}
	}
	m.requestTotal.With(zoneLabel, qLabel).Inc()

	// In edns0-only mode a query carrying no client-subnet resolves to no client
	// at all, and the zero Addr is passed on rather than short-circuited. Which
	// zone owns a name is not a property of the client, so the answer policy owes
	// the same three response codes here as anywhere: a name under no served zone
	// is refused, a name a served zone does not own is a name error, and a name it
	// owns is a no-data answer, because no source prefix matches the zero Addr and
	// so no host set is selected. Returning early instead answered every name,
	// in-zone or not, with an empty NOERROR the harness then stamped AA on.
	client, _ := dnsserver.ClientIP(r, dnsserver.RemoteAddr(p), st.cfg.ClientIPSource)

	answerQuestions(msg, r, st, client)
	return true
}

// onPanic logs a query that panicked mid-answer; the harness has already
// recovered it and dropped the (unwritten) reply.
func onPanic(rec any) {
	loggerPtr.Load().Error("geodns: recovered panic handling query", "panic", rec)
}

// onListenerChange publishes bind/unbind transitions to geodns's own
// listenerUp gauge, whose label values are this plugin's listen addresses.
// The harness owns only the metric no consumer can see, the write-failure
// counter of internal/core/dnsserver/metrics.go.
func onListenerChange(proto, addr string, up bool) {
	v := 0.0
	if up {
		v = 1
	}
	gmetrics().listenerUp.With(proto, addr).Set(v)
}

// geodnsServer is a thin adapter over dnsserver.Manager, keeping geodns's own
// apply/stopAll call shape (geodnsConfig in, no dnsserver types leaking to
// callers) so register.go and the existing test suite are unaffected by the
// harness's endpoint-agnostic Apply/Stop signatures.
type geodnsServer struct {
	mgr *dnsserver.Manager
}

// newServerManager builds the geodns DNS server on top of the shared harness:
// the harness owns the listener lifecycle, client-IP resolution, and the
// authoritative-answer/recursion-refusal guard; geodns supplies only
// answerQuery (metrics + policy) and its own listener-up gauge.
func newServerManager(log *slog.Logger) *geodnsServer {
	handler := dnsserver.Authoritative(log, answerQuery, onPanic)
	return &geodnsServer{mgr: dnsserver.New(log, handler, dnsserver.Options{
		OnListenerChange: onListenerChange,
		// The PKI store lives in the hub process. Injecting the resolver here
		// (rather than letting core dnsserver reach for it) keeps the
		// core-must-not-import-component tier rule intact, and makes an
		// out-of-process instance fail loudly: its store is empty, so a
		// configured `tls { certificate }` errors and the secure listeners do
		// not start, instead of quietly serving a self-signed certificate.
		TLSMaterialResolver: zepki.ServerTLSMaterial,
	})}
}

// apply reconciles the bound listeners with cfg: the cleartext UDP+TCP listener
// set plus any DoT (RFC 7858) / DoH (RFC 8484) listeners on the same IPs sharing
// the tls cert material. A pure host-data change is a no-op (answerQuery reads
// the new snapshot via loadState); a listener-set or certificate change stops
// and rebinds (dnsserver's listener-signature check).
func (s *geodnsServer) apply(cfg geodnsConfig) error {
	plain := make([]dnsserver.Endpoint, len(cfg.Listeners))
	for i, l := range cfg.Listeners {
		plain[i] = dnsserver.Endpoint{IP: l.IP, Port: l.Port}
	}
	return s.mgr.ApplyWithSecure(cfg.Enabled, plain, cfg.Secure, loggerPtr.Load())
}

func (s *geodnsServer) stopAll() { s.mgr.Stop() }
