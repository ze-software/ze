// Design: docs/architecture/dns/as112.md -- static AS112 zone table, SOA/NS/TXT synthesis
// RFC: rfc/short/rfc7534.md -- zone list, SOA-only content, RFC-mandated SOA timers (finding M1)
// RFC: rfc/short/rfc7535.md -- EMPTY.AS112.ARPA (identical shape to the Direct-Delegation zones)
// RFC: rfc/short/rfc1035.md -- NODATA (NOERROR + SOA in Authority) vs NXDOMAIN

package as112

import (
	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// RFC 7534 Section 3.5: "$TTL 1W" / "1W ; refresh" / "1M ; retry" / "1W ; expire"
// / "1W ; negative caching TTL" -- verbatim from the db.dd-empty and db.dr-empty
// example zone files (rfc/full/rfc7534.txt lines 685-696, 717-723). BIND zone-file
// time units: W=604800s, M=60s (minutes, NOT months -- there is no month unit in
// BIND zone-file syntax). These four constants plus zoneTTL are the exact values
// every served zone's SOA and NS records use; TestSOA_RFCMandatedParameters pins them.
const (
	soaRefresh uint32 = 604800 // 1W
	soaRetry   uint32 = 60     // 1M (1 minute)
	soaExpire  uint32 = 604800 // 1W
	soaMinTTL  uint32 = 604800 // 1W ("negative caching TTL" == RFC 2308 minimum)
	zoneTTL    uint32 = 604800 // 1W ($TTL, used for NS/TXT record TTLs too)
)

// RFC 7534 Section 3.5 / rfc/full/rfc7534.txt db.dd-empty: the Direct-Delegation
// zones' SOA MNAME is the canonical AS112 dynamic-update target, and RNAME is a
// real, monitored mailbox -- not a per-operator placeholder (the RFC's later
// db.hostname.as112.{net,arpa} examples use "server.example.net"/"admin.example.net"
// as illustrative placeholders for operators to substitute; this plugin instead
// reuses the same canonical values as the delegated zones for HOSTNAME.AS112.NET/ARPA
// too, since one anycast node serves both under the same identity -- see Design Insights).
const (
	directDelegationMName = "prisoner.iana.org."
	directDelegationRName = "hostmaster.root-servers.org."
	dnameRedirectionMName = "blackhole.as112.arpa."
	dnameRedirectionRName = "noc.dns.icann.org."
)

// Direct-Delegation NS records (rfc/full/rfc7534.txt db.dd-empty, lines 692-693)
// and the DNAME-Redirection NS record (db.dr-empty, line 725).
var (
	directDelegationNS = []string{"blackhole-1.iana.org.", "blackhole-2.iana.org."}
	dnameRedirectionNS = []string{"blackhole.as112.arpa."}
)

// zoneKind selects which SOA MNAME/RNAME/NS set a zone uses.
type zoneKind int

const (
	kindDirectDelegation zoneKind = iota
	kindDNAMERedirection
)

// zoneEntry is one statically-served zone: its FQDN and which canonical
// MNAME/RNAME/NS set applies.
type zoneEntry struct {
	Name string
	Kind zoneKind
}

// directDelegationReverseZones is the fixed list of RFC 1918 / link-local
// reverse zones (RFC 7534 Section 2.2): 10.IN-ADDR.ARPA; 16.172.IN-ADDR.ARPA
// .. 31.172.IN-ADDR.ARPA (16 zones); 168.192.IN-ADDR.ARPA; 254.169.IN-ADDR.ARPA
// -- 19 zones total.
func directDelegationReverseZones() []string {
	zones := make([]string, 0, 19)
	zones = append(zones, "10.in-addr.arpa.")
	var tb textbuf.Buffer
	for n := 16; n <= 31; n++ {
		zones = append(zones, tb.Reset().Str(itoa(n)).Str(".172.in-addr.arpa.").String())
	}
	zones = append(zones, "168.192.in-addr.arpa.", "254.169.in-addr.arpa.")
	return zones
}

// itoa converts 0..99 to decimal without importing strconv for a two-digit range.
func itoa(n int) string {
	var tb textbuf.Buffer
	if n < 10 {
		return tb.Byte(byte('0' + n)).String()
	}
	return tb.Byte(byte('0' + n/10)).Byte(byte('0' + n%10)).String()
}

// emptyAS112Arpa is the DNAME-Redirection sink zone (RFC 7535 Section 2).
const emptyAS112Arpa = "empty.as112.arpa."

// hostnameNetZone and hostnameArpaZone carry node-identification TXT data
// (RFC 7534 Section 3.5).
const (
	hostnameNetZone  = "hostname.as112.net."
	hostnameArpaZone = "hostname.as112.arpa."
)

// allServedZones is the fixed 22-zone table, built once at package init
// rather than on every query: matchZone/zoneKindFor run on the DNS hot path
// (an AS112 node's purpose is absorbing high-volume misdirected reverse-DNS
// traffic), and the table's contents never change at runtime.
var allServedZones = buildServedZones()

// servedZones returns every zone this plugin answers authoritatively for,
// each tagged with its SOA/NS kind. Callers must treat the result as
// read-only; it is the shared package-level table, not a copy.
func servedZones() []zoneEntry {
	return allServedZones
}

func buildServedZones() []zoneEntry {
	zones := make([]zoneEntry, 0, 22)
	for _, z := range directDelegationReverseZones() {
		zones = append(zones, zoneEntry{Name: z, Kind: kindDirectDelegation})
	}
	zones = append(zones,
		zoneEntry{Name: emptyAS112Arpa, Kind: kindDNAMERedirection},
		zoneEntry{Name: hostnameNetZone, Kind: kindDirectDelegation},
		zoneEntry{Name: hostnameArpaZone, Kind: kindDNAMERedirection},
	)
	return zones
}

// matchZone returns the served zone that is a suffix of (or equal to) name,
// or ("", false) if name is outside every served zone.
func matchZone(name string) (string, bool) {
	n := dns.Fqdn(name)
	for _, z := range servedZones() {
		if equalOrSubdomain(n, z.Name) {
			return z.Name, true
		}
	}
	return "", false
}

// equalOrSubdomain reports whether name is zone itself or a name within it.
// Uses dns.IsSubDomain for label-boundary-aware comparison: a raw string
// suffix match would incorrectly treat a sibling name like
// "evil10.in-addr.arpa." as inside zone "10.in-addr.arpa." merely because
// the characters match, when the actual DNS label sequence does not nest.
func equalOrSubdomain(name, zone string) bool {
	if strEqualFold(name, zone) {
		return true
	}
	return dns.IsSubDomain(zone, name)
}

func strEqualFold(a, b string) bool {
	return dns.CanonicalName(a) == dns.CanonicalName(b)
}

// zoneKindFor returns the SOA/NS kind for zone (must be a result of matchZone).
func zoneKindFor(zone string) zoneKind {
	for _, z := range servedZones() {
		if strEqualFold(z.Name, zone) {
			return z.Kind
		}
	}
	return kindDirectDelegation
}

// buildSOA synthesizes the zone's SOA record with the RFC-mandated fixed
// timers and canonical MNAME/RNAME for its kind (finding M1). serial is the
// current published generation's serial (see state.go).
func buildSOA(zone string, kind zoneKind, serial uint32) *dns.SOA {
	mname, rname := directDelegationMName, directDelegationRName
	if kind == kindDNAMERedirection {
		mname, rname = dnameRedirectionMName, dnameRedirectionRName
	}
	return &dns.SOA{
		Hdr:     dns.RR_Header{Name: zone, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: zoneTTL},
		Ns:      mname,
		Mbox:    rname,
		Serial:  serial,
		Refresh: soaRefresh,
		Retry:   soaRetry,
		Expire:  soaExpire,
		Minttl:  soaMinTTL,
	}
}

// nsNamesFor returns the canonical NS name list for kind.
func nsNamesFor(kind zoneKind) []string {
	if kind == kindDNAMERedirection {
		return dnameRedirectionNS
	}
	return directDelegationNS
}

// appendNS adds the zone's canonical NS records to msg.Answer (for an NS
// query) or msg.Ns (Authority, alongside SOA for negative answers).
func appendNS(msg *dns.Msg, zone string, kind zoneKind, authority bool) {
	for _, ns := range nsNamesFor(kind) {
		rr := &dns.NS{Hdr: dns.RR_Header{Name: zone, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: zoneTTL}, Ns: ns}
		if authority {
			msg.Ns = append(msg.Ns, rr)
		} else {
			msg.Answer = append(msg.Answer, rr)
		}
	}
}

// hostnameTXTStrings builds the TXT record string set for the HOSTNAME.AS112.*
// zones (RFC 7534 Section 3.5 db.hostname.as112.net/arpa examples): the
// operator-configured hostname (if set), the facility/location string (if
// set, combined per the RFC's "Name of Facility or similar", "City, Country"
// two-string TXT), and the fixed informational URL string.
func hostnameTXTStrings(hostname, facility, location string) []string {
	var strs []string
	if hostname != "" {
		strs = append(strs, hostname)
	}
	if facility != "" || location != "" {
		var tb textbuf.Buffer
		strs = append(strs, tb.Str(facility).Str(", ").Str(location).String())
	}
	strs = append(strs, "See http://www.as112.net/ for more information.")
	return strs
}

// buildHostnameTXT synthesizes the single TXT record for a HOSTNAME.AS112.*
// query. Content is never empty: at minimum the fixed informational string
// is served, matching the RFC example's minimum content.
func buildHostnameTXT(zone, hostname, facility, location string) *dns.TXT {
	return &dns.TXT{
		Hdr: dns.RR_Header{Name: zone, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: zoneTTL},
		Txt: hostnameTXTStrings(hostname, facility, location),
	}
}

// isHostnameZone reports whether zone is one of the two node-identification
// zones (TXT-bearing), as opposed to a plain SOA/NS-only reverse or
// DNAME-redirection zone.
func isHostnameZone(zone string) bool {
	return strEqualFold(zone, hostnameNetZone) || strEqualFold(zone, hostnameArpaZone)
}

// answerQuestions fills msg for each question in r, using the zone table and
// the given serial/hostname-TXT identity. A name outside every served zone
// leaves found=false (caller sets NXDOMAIN); an in-zone name with no
// matching record type is NOERROR with the zone SOA in Authority (RFC 1035
// NODATA), never NXDOMAIN.
func answerQuestions(msg, r *dns.Msg, serial uint32, hostname, facility, location string) {
	found := false
	for _, q := range r.Question {
		zone, ok := matchZone(q.Name)
		if !ok {
			continue
		}
		found = true
		kind := zoneKindFor(zone)

		switch q.Qtype {
		case dns.TypeSOA:
			if strEqualFold(q.Name, zone) {
				msg.Answer = append(msg.Answer, buildSOA(zone, kind, serial))
				appendNS(msg, zone, kind, true)
			} else {
				msg.Ns = append(msg.Ns, buildSOA(zone, kind, serial))
			}
		case dns.TypeNS:
			if strEqualFold(q.Name, zone) {
				appendNS(msg, zone, kind, false)
			} else {
				msg.Ns = append(msg.Ns, buildSOA(zone, kind, serial))
			}
		case dns.TypeTXT:
			if isHostnameZone(zone) && strEqualFold(q.Name, zone) {
				msg.Answer = append(msg.Answer, buildHostnameTXT(zone, hostname, facility, location))
			} else {
				msg.Ns = append(msg.Ns, buildSOA(zone, kind, serial))
			}
		default:
			// RFC 7534 Section 3.5: "There should be no other resource records
			// included in this zone." Any other query type in a served zone is
			// NODATA, never a synthesized answer.
			msg.Ns = append(msg.Ns, buildSOA(zone, kind, serial))
		}
	}
	if !found {
		msg.SetRcode(r, dns.RcodeNameError)
	}
}
