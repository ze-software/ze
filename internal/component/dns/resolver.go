// Design: (none -- new component, predates documentation)
// Related: cache.go -- in-memory cache for DNS query results

package dns

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	mdns "github.com/miekg/dns"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// ResolverConfig holds DNS resolver configuration from YANG.
type ResolverConfig struct {
	Server    string // DNS server address (e.g., "8.8.8.8:53"). Empty uses system default.
	Timeout   uint16 // Query timeout in seconds.
	CacheSize uint32 // Max cached entries. 0 disables caching.
	CacheTTL  uint32 // Max cache TTL in seconds. 0 means use response TTL only.
}

// Resolver provides DNS query services to Ze components.
// Safe for concurrent use. Caller MUST call Close when done.
type Resolver struct {
	client *mdns.Client
	server string
	cache  *cache
	logger *slog.Logger
}

// NewResolver creates a DNS resolver with the given configuration.
// Caller MUST call Close when done to release resources.
func NewResolver(cfg ResolverConfig) *Resolver {
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	server := cfg.Server
	if server != "" {
		// Ensure server has a port.
		if _, _, err := net.SplitHostPort(server); err != nil {
			server = net.JoinHostPort(server, "53")
		}
	} else {
		// Resolve system default DNS server once at construction.
		server = resolveSystemDNS()
	}

	return &Resolver{
		client: &mdns.Client{
			Net:     "udp",
			Timeout: timeout,
		},
		server: server,
		cache:  newCache(cfg.CacheSize, cfg.CacheTTL),
		logger: slogutil.Logger("dns"),
	}
}

// resolveSystemDNS reads the system DNS server from /etc/resolv.conf.
// Falls back to 8.8.8.8:53 (Google Public DNS) if the file is missing or empty,
// so DNS resolution always works out of the box.
func resolveSystemDNS() string {
	config, err := mdns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil || len(config.Servers) == 0 {
		return "8.8.8.8:53"
	}
	return net.JoinHostPort(config.Servers[0], config.Port)
}

// Close releases resolver resources.
func (r *Resolver) Close() {
	// Currently no persistent connections to close.
	// Present for API contract: NewResolver documents "MUST call Close".
}

// Resolve queries DNS for records of the given type.
// Returns the string representation of each answer record.
func (r *Resolver) Resolve(name string, qtype uint16) ([]string, error) {
	// Check cache first.
	if records, ok := r.cache.get(name, qtype); ok {
		r.logger.Debug("cache hit", "name", name, "type", mdns.TypeToString[qtype])
		return records, nil
	}

	records, ttl, err := r.query(name, qtype)
	if err != nil {
		return nil, err
	}

	// Only cache non-empty results. NXDOMAIN returns empty records and is not cached.
	if len(records) > 0 {
		r.cache.put(name, qtype, records, ttl)
	}

	return records, nil
}

// ResolveTXT queries for TXT records.
func (r *Resolver) ResolveTXT(name string) ([]string, error) {
	return r.Resolve(name, mdns.TypeTXT)
}

// ResolveA queries for A (IPv4) records.
func (r *Resolver) ResolveA(name string) ([]string, error) {
	return r.Resolve(name, mdns.TypeA)
}

// ResolveAAAA queries for AAAA (IPv6) records.
func (r *Resolver) ResolveAAAA(name string) ([]string, error) {
	return r.Resolve(name, mdns.TypeAAAA)
}

// ResolvePTR queries for PTR (reverse DNS) records.
// The address parameter is an IP address; it is automatically converted to
// the in-addr.arpa or ip6.arpa format.
func (r *Resolver) ResolvePTR(address string) ([]string, error) {
	arpa, err := mdns.ReverseAddr(address)
	if err != nil {
		return nil, fmt.Errorf("reverse addr %q: %w", address, err)
	}
	return r.Resolve(arpa, mdns.TypePTR)
}

// query sends a DNS query and extracts answer records.
// Returns records, minimum TTL from answers, and any error.
func (r *Resolver) query(name string, qtype uint16) ([]string, uint32, error) {
	fqdn := mdns.Fqdn(name)

	m := new(mdns.Msg)
	m.SetQuestion(fqdn, qtype)
	m.RecursionDesired = true

	resp, _, err := r.client.Exchange(m, r.server)
	if err != nil {
		return nil, 0, fmt.Errorf("dns query %s %s: %w", name, mdns.TypeToString[qtype], err)
	}

	if resp == nil {
		return nil, 0, fmt.Errorf("dns query %s %s: nil response", name, mdns.TypeToString[qtype])
	}

	// NXDOMAIN and other non-error response codes return empty results, not errors.
	if resp.Rcode != mdns.RcodeSuccess {
		return nil, 0, nil
	}

	return extractRecords(resp)
}

// extractRecords pulls string values and minimum TTL from DNS answer records.
// Returns TTL=0 when answers have TTL=0 (caller should not cache per RFC 1035).
func extractRecords(resp *mdns.Msg) ([]string, uint32, error) {
	var records []string
	var minTTL uint32
	hasAnswers := false

	for _, rr := range resp.Answer {
		hasAnswers = true
		hdr := rr.Header()
		if minTTL == 0 || hdr.Ttl < minTTL {
			minTTL = hdr.Ttl
		}

		switch v := rr.(type) {
		case *mdns.A:
			records = append(records, v.A.String())
		case *mdns.AAAA:
			records = append(records, v.AAAA.String())
		case *mdns.TXT:
			records = append(records, strings.Join(v.Txt, ""))
		case *mdns.PTR:
			records = append(records, v.Ptr)
		case *mdns.CNAME:
			records = append(records, v.Target)
		case *mdns.MX:
			records = append(records, v.Mx)
		case *mdns.NS:
			records = append(records, v.Ns)
		case *mdns.SRV:
			records = append(records, fmt.Sprintf("%s:%d", v.Target, v.Port))
		}
	}

	// Only apply a default TTL when there were no answers at all.
	// When answers have TTL=0, the server explicitly says "do not cache."
	if !hasAnswers && minTTL == 0 {
		minTTL = 300
	}

	return records, minTTL, nil
}
