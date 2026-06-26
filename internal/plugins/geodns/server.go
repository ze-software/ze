// Design: plan/learned/993-geodns-2-server.md -- geodns DNS server (listener, EDNS0, answer synthesis)
// RFC: rfc/short/rfc7871.md -- EDNS0 client subnet; rfc/short/rfc1035.md -- DNS messages, SOA, NS

package geodns

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

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// drainTimeout bounds how long we wait for in-flight handlers to finish when a
// listener is shut down, so a wedged handler cannot block reload or exit.
const drainTimeout = 5 * time.Second

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

// clientIP resolves the client IP used for source selection, per mode.
// RFC 7871: when present, the EDNS0 client-subnet option's network is the
// client view; otherwise (per mode) the packet source is used.
func clientIP(r *dns.Msg, packetSrc netip.Addr, mode string) (netip.Addr, bool) {
	if mode != "packet" {
		if opt := r.IsEdns0(); opt != nil {
			for _, o := range opt.Option {
				if ecs, ok := o.(*dns.EDNS0_SUBNET); ok {
					if a, ok := netip.AddrFromSlice(ecs.Address); ok {
						return a.Unmap(), true
					}
				}
			}
		}
		if mode == "edns0" {
			return netip.Addr{}, false
		}
	}
	if packetSrc.IsValid() {
		return packetSrc.Unmap(), true
	}
	return netip.Addr{}, false
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
	setName, ok := st.matcher.lookup(client)
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

func remoteAddr(w dns.ResponseWriter) netip.Addr {
	host, _, err := net.SplitHostPort(w.RemoteAddr().String())
	if err != nil {
		return netip.Addr{}
	}
	a, _ := netip.ParseAddr(host)
	return a
}

// handleQuery is the single mux handler. It reads the current resolver snapshot
// per request (so reload swaps answers without rebinding), recovers any panic so
// one bad query cannot take the daemon down, and never recurses.
func handleQuery(w dns.ResponseWriter, r *dns.Msg) {
	start := time.Now()
	log := loggerPtr.Load()
	m := gmetrics()

	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true
	msg.Compress = false

	zoneLabel, qLabel := "none", "NONE"
	if len(r.Question) > 0 {
		qLabel = qtypeLabel(r.Question[0].Qtype)
	}

	defer func() {
		if rec := recover(); rec != nil {
			log.Error("geodns: recovered panic handling query", "panic", rec)
		}
		m.responseTotal.With(zoneLabel, qLabel, dns.RcodeToString[msg.Rcode]).Inc()
		m.latency.Observe(float64(time.Since(start).Microseconds()) / 1000.0)
	}()

	st := loadState()
	if st == nil || !st.cfg.Enabled {
		msg.SetRcode(r, dns.RcodeRefused)
		m.requestTotal.With(zoneLabel, qLabel).Inc()
		_ = w.WriteMsg(msg)
		return
	}

	if len(r.Question) > 0 {
		if z := matchZone(r.Question[0].Name, st.cfg.Zones); z != "" {
			zoneLabel = z
		}
	}
	m.requestTotal.With(zoneLabel, qLabel).Inc()

	client, ok := clientIP(r, remoteAddr(w), st.cfg.ClientIPSource)
	if !ok {
		// edns0-only mode with no client-subnet: answer nothing (NOERROR, empty).
		_ = w.WriteMsg(msg)
		return
	}

	answerQuestions(msg, r, st, client)
	_ = w.WriteMsg(msg)
}

// serverManager owns the bound UDP+TCP listeners. The mux and its handler are
// state-independent (the handler reads loadState per request), so only an
// endpoint change (the listener set / enabled) triggers a rebind.
type serverManager struct {
	log        *slog.Logger
	mux        *dns.ServeMux
	servers    []*dns.Server
	boundAddrs []string
	applied    string
}

func newServerManager(log *slog.Logger) *serverManager {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", handleQuery)
	return &serverManager{log: log, mux: mux, applied: "\x00unset"}
}

// endpointSig is the signature of the bound-listener set; reload rebinds only
// when it changes.
func endpointSig(cfg geodnsConfig) string {
	if !cfg.Enabled {
		return "disabled"
	}
	eps := make([]string, len(cfg.Listeners))
	for i, l := range cfg.Listeners {
		eps[i] = net.JoinHostPort(l.IP.String(), strconv.Itoa(int(l.Port)))
	}
	sort.Strings(eps)
	return strings.Join(eps, ",")
}

// apply reconciles the listeners with cfg. A pure host-data change is a no-op
// (the handler picks up the new snapshot); an endpoint change stops and rebinds.
func (m *serverManager) apply(cfg geodnsConfig) error {
	sig := endpointSig(cfg)
	if sig == m.applied {
		return nil
	}
	m.stopAll()
	m.applied = sig
	if !cfg.Enabled {
		return nil
	}
	for _, l := range cfg.Listeners {
		ep := net.JoinHostPort(l.IP.String(), strconv.Itoa(int(l.Port)))
		if err := m.bind(ep, l.IP.String()); err != nil {
			m.log.Error("geodns: listen failed", "endpoint", ep, "error", err)
			continue
		}
	}
	if len(m.servers) == 0 && len(cfg.Listeners) > 0 {
		return fmt.Errorf("geodns: no listeners bound on %d endpoint(s)", len(cfg.Listeners))
	}
	m.log.Info("geodns: listening", "endpoints", len(m.servers)/2)
	return nil
}

// bind opens a UDP and a TCP listener on ep (best-effort: the caller logs and
// continues if one endpoint fails so the rest still serve).
func (m *serverManager) bind(ep, addr string) error {
	var lc net.ListenConfig
	pc, err := lc.ListenPacket(context.Background(), "udp", ep)
	if err != nil {
		return fmt.Errorf("udp: %w", err)
	}
	udp := &dns.Server{PacketConn: pc, Handler: m.mux}
	ln, err := lc.Listen(context.Background(), "tcp", ep)
	if err != nil {
		if cerr := pc.Close(); cerr != nil {
			m.log.Debug("geodns: closing udp after tcp bind failure", "endpoint", ep, "error", cerr)
		}
		return fmt.Errorf("tcp: %w", err)
	}
	tcp := &dns.Server{Listener: ln, Handler: m.mux}
	m.servers = append(m.servers, udp, tcp)
	m.boundAddrs = append(m.boundAddrs, addr)
	gm := gmetrics()
	gm.listenerUp.With("udp", addr).Set(1)
	gm.listenerUp.With("tcp", addr).Set(1)
	go m.serve(udp, ep, "udp")
	go m.serve(tcp, ep, "tcp")
	return nil
}

func (m *serverManager) serve(s *dns.Server, ep, proto string) {
	if err := s.ActivateAndServe(); err != nil {
		m.log.Debug("geodns: server stopped", "endpoint", ep, "proto", proto, "error", err)
	}
}

// stopAll drains and closes every bound listener with a bounded timeout.
func (m *serverManager) stopAll() {
	for _, s := range m.servers {
		ctx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		if err := s.ShutdownContext(ctx); err != nil {
			m.log.Debug("geodns: listener shutdown", "error", err)
		}
		cancel()
	}
	gm := gmetrics()
	for _, addr := range m.boundAddrs {
		gm.listenerUp.With("udp", addr).Set(0)
		gm.listenerUp.With("tcp", addr).Set(0)
	}
	m.boundAddrs = nil
	m.servers = nil
}
