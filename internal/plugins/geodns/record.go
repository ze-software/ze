// Design: docs/architecture/dns/geodns.md -- geodns record model (A/AAAA/SRV)

package geodns

import "net/netip"

// recordKind distinguishes the record types geodns serves.
type recordKind uint8

const (
	kindA recordKind = iota
	kindAAAA
	kindSRV
)

// dnsRecord is one answer record. For A/AAAA, Addr is set. For SRV, the SRV
// fields (Priority, Weight, Port, Target) are set. TTL is the record's
// time-to-live in seconds.
type dnsRecord struct {
	Kind     recordKind
	TTL      uint32
	Addr     netip.Addr
	Priority uint16
	Weight   uint16
	Port     uint16
	Target   string
}

// addrRecord builds an A or AAAA record from an address. An IPv4 address yields
// an A record; anything else yields AAAA, mirroring the reference daemon's
// per-address detection (a ":" in the address means IPv6).
func addrRecord(ttl uint32, addr netip.Addr) dnsRecord {
	k := kindAAAA
	if addr.Is4() {
		k = kindA
	}
	return dnsRecord{Kind: k, TTL: ttl, Addr: addr}
}

// srvRecord builds an SRV record (priority, weight, port, target per RFC 2782).
func srvRecord(ttl uint32, priority, weight, port uint16, target string) dnsRecord {
	return dnsRecord{Kind: kindSRV, TTL: ttl, Priority: priority, Weight: weight, Port: port, Target: target}
}
