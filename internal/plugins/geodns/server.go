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

// matchZone returns the longest configured zone that is a suffix of name.
func matchZone(name string, zones []string) string {
	n := fqdn(name)
	best := ""
	for _, z := range zones {
		if z != "" && strings.HasSuffix(n, z) && len(z) > len(best) {
			best = z
		}
	}
	return best
}

func equalName(a, zone string) bool { return strings.EqualFold(fqdn(a), zone) }

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

// answerQuestions fills msg for each question. A/AAAA/SRV/ANY resolve per the
// client source; a miss is a NOERROR negative answer with the SOA in Authority.
// A name outside every configured zone leaves found=false, yielding NXDOMAIN.
func answerQuestions(msg, r *dns.Msg, st *resolverState, client netip.Addr) {
	found := false
	for _, q := range r.Question {
		zone := matchZone(q.Name, st.cfg.Zones)
		if zone == "" {
			continue
		}
		found = true
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
	if !found {
		msg.SetRcode(r, dns.RcodeNameError)
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

	client, ok := dnsserver.ClientIP(r, dnsserver.RemoteAddr(p), st.cfg.ClientIPSource)
	if !ok {
		// edns0-only mode with no client-subnet: answer nothing (NOERROR, empty).
		return true
	}

	answerQuestions(msg, r, st, client)
	return true
}

// onPanic logs a query that panicked mid-answer; the harness has already
// recovered it and dropped the (unwritten) reply.
func onPanic(rec any) {
	loggerPtr.Load().Error("geodns: recovered panic handling query", "panic", rec)
}

// onListenerChange publishes bind/unbind transitions to geodns's own
// listenerUp gauge; the harness never owns metrics.
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
	handler := dnsserver.Authoritative(answerQuery, onPanic)
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
