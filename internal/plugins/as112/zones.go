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
// the given serial/hostname-TXT identity, and picks the reply's RCODE from what
// the served zones say about the names asked for.
//
// Every zone this node serves holds its records at the apex and nowhere else:
// RFC 7534 Section 3.5's zone files put an SOA and an NS set at @ in
// db.dd-empty and db.dr-empty, and an SOA, an NS set and the TXT strings at @
// in db.hostname.as112.net and db.hostname.as112.arpa. Nothing below the apex
// is a node of the tree, so the three answers below follow from where the query
// name sits.
//
//   - A name under no served zone draws RCODE 5. RFC 1035 Section 4.1.1:
//     "Refused - The name server refuses to perform the specified operation for
//     policy reasons." This node is delegated 22 zones and holds no data about
//     anything else, so it makes no claim about the name and the harness clears
//     AA for the reply (dnsserver's shapeAuthoritative).
//   - A name inside a served zone but below its apex draws RCODE 3. RFC 1035
//     Section 4.1.1: "Name Error - Meaningful only for responses from an
//     authoritative name server, this code signifies that the domain name
//     referenced in the query does not exist." Sinking the reverse-DNS query
//     for a private address is exactly the statement that the name does not
//     exist, which is what RFC 7534 Section 3.5 has these empty zones make.
//   - The apex itself exists, so a query type it holds no record for draws
//     NOERROR with an empty Answer.
//
// The last two carry the zone SOA in the Authority section, which RFC 2308
// Section 3 requires of an authoritative server "when reporting an NXDOMAIN or
// indicating that no data of the requested type exists", so that a resolver can
// cache the negative answer for the 1W the SOA's MINIMUM field gives it. That
// caching is the point of an AS112 node: RFC 7535 Section 6 counts on it, "The
// negative caching [RFC2308] of the CNAME target follows the parameters defined
// in the target zone, EMPTY.AS112.ARPA."
//
// The RCODE is assigned directly rather than through Msg.SetRcode, which calls
// SetReply and would drop every question after the first from a multi-question
// reply.
func answerQuestions(msg, r *dns.Msg, serial uint32, hostname, facility, location string) {
	served, missing := false, false
	for _, q := range r.Question {
		zone, ok := matchZone(q.Name)
		if !ok {
			continue
		}
		served = true
		kind := zoneKindFor(zone)
		if !strEqualFold(q.Name, zone) {
			missing = true
			msg.Ns = append(msg.Ns, buildSOA(zone, kind, serial))
			continue
		}

		switch q.Qtype {
		case dns.TypeSOA:
			msg.Answer = append(msg.Answer, buildSOA(zone, kind, serial))
			appendNS(msg, zone, kind, true)
		case dns.TypeNS:
			appendNS(msg, zone, kind, false)
		case dns.TypeTXT:
			if isHostnameZone(zone) {
				msg.Answer = append(msg.Answer, buildHostnameTXT(zone, hostname, facility, location))
			} else {
				msg.Ns = append(msg.Ns, buildSOA(zone, kind, serial))
			}
		default:
			// RFC 7534 Section 3.5: "There should be no other resource records
			// included in this zone." Any other query type at the apex of a
			// served zone is NODATA, never a synthesized answer.
			msg.Ns = append(msg.Ns, buildSOA(zone, kind, serial))
		}
	}
	switch {
	case !served:
		msg.Rcode = dns.RcodeRefused
	case missing:
		msg.Rcode = dns.RcodeNameError
	}
}
