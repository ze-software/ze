// Design: (none -- new component, predates documentation)
// Related: cache.go -- in-memory cache for DNS query results
// RFC: rfc/short/rfc4035.md -- DNSSEC stub-resolver handling (EDNS0 DO bit, the
// upstream AD bit, and SERVFAIL on a broken chain)

package dns

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	mdns "github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ResolverConfig holds DNS resolver configuration from YANG.
type ResolverConfig struct {
	Server         string // DNS server address (e.g., "8.8.8.8:53"). Empty uses system default.
	ResolvConfPath string // Path to resolv.conf (empty uses /etc/resolv.conf).
	Timeout        uint16 // Query timeout in seconds.
	CacheSize      uint32 // Max cached entries. 0 disables caching.
	CacheTTL       uint32 // Max cache TTL in seconds. 0 means use response TTL only.
	// DNSSECValidation controls upstream-answer DNSSEC handling (RFC 4035 stub
	// model): "off" (default) leaves behavior unchanged; "permissive" and
	// "strict" set the EDNS0 DO bit and rely on a validating upstream (CD=0) to
	// SERVFAIL a broken chain. "strict" rejects such answers as an error;
	// "permissive" logs and returns the (empty) result. Empty means off.
	DNSSECValidation string
}

// DNSSEC validation modes.
const (
	dnssecOff        = "off"
	dnssecPermissive = "permissive"
	dnssecStrict     = "strict"
)

// Resolver provides DNS query services to Ze components.
// Safe for concurrent use. Caller MUST call Close when done.
type Resolver struct {
	client *mdns.Client
	server string
	cache  *cache
	logger *slog.Logger
	dnssec string // one of dnssecOff/dnssecPermissive/dnssecStrict
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
		resolvPath := cfg.ResolvConfPath
		if resolvPath == "" {
			resolvPath = "/etc/resolv.conf"
		}
		server = resolveSystemDNS(resolvPath)
	}

	dnssec := cfg.DNSSECValidation
	if dnssec == "" {
		dnssec = dnssecOff
	}

	return &Resolver{
		client: &mdns.Client{
			Net:     "udp",
			Timeout: timeout,
		},
		server: server,
		cache:  newCache(cfg.CacheSize, cfg.CacheTTL),
		logger: slogutil.Logger("dns"),
		dnssec: dnssec,
	}
}

// dnssecDecision decides how a resolver in mode should treat a response with the
// given rcode and AuthenticatedData bit. It returns a non-nil error to reject
// the answer (strict mode, broken chain), or a non-empty warn string to log
// (permissive mode), or neither. The stub model (RFC 4035): a validating
// upstream returns SERVFAIL for a broken chain (CD=0), so SERVFAIL under
// validation is the failure signal. A NOERROR answer is accepted whether it is
// secure (AD=1) or insecure/unsigned (AD=0) -- rejecting AD=0 would break every
// unsigned zone.
func dnssecDecision(rcode int, _ bool, mode string) (warn string, reject error) {
	if mode == "" || mode == dnssecOff {
		return "", nil
	}
	if rcode == mdns.RcodeServerFailure {
		switch mode {
		case dnssecStrict:
			return "", fmt.Errorf("dnssec validation failed: upstream returned SERVFAIL (broken chain or unreachable)")
		case dnssecPermissive:
			return "dnssec: upstream SERVFAIL under validation (possible broken chain), returning empty result", nil
		}
	}
	return "", nil
}

// resolveSystemDNS reads the system DNS server from the configured resolv.conf path.
// Falls back to /etc/resolv.conf when the configured path has no servers (e.g.,
// gokrazy default /tmp/resolv.conf absent before DHCP, or on macOS dev machines).
// Returns an empty server when neither path yields a nameserver.
func resolveSystemDNS(resolvConfPath string) string {
	config, err := mdns.ClientConfigFromFile(resolvConfPath)
	if err == nil && len(config.Servers) > 0 {
		return net.JoinHostPort(config.Servers[0], config.Port)
	}
	if resolvConfPath != "/etc/resolv.conf" {
		config, err = mdns.ClientConfigFromFile("/etc/resolv.conf")
		if err == nil && len(config.Servers) > 0 {
			return net.JoinHostPort(config.Servers[0], config.Port)
		}
	}
	return ""
}

// CacheStats returns a snapshot of DNS cache counters.
func (r *Resolver) CacheStats() CacheStats {
	return r.cache.Stats()
}

// CacheClear removes all entries and resets all counters.
func (r *Resolver) CacheClear() {
	r.cache.Clear()
}

// CacheDelete removes a single entry by name and record type.
// Returns true if the entry existed and was removed.
func (r *Resolver) CacheDelete(name string, qtype uint16) bool {
	return r.cache.Delete(name, qtype)
}

// CacheDeleteByName removes all entries matching the given name regardless of type.
// Returns the number of entries removed.
func (r *Resolver) CacheDeleteByName(name string) int {
	return r.cache.deleteByName(name)
}

// CacheResetStats zeros all counters without removing cached entries.
func (r *Resolver) CacheResetStats() {
	r.cache.resetStats()
}

// CacheEntries returns a snapshot of all cached entries with remaining TTL
// and human-readable type names.
func (r *Resolver) CacheEntries() []CacheEntryInfo {
	entries := r.cache.Entries()
	for i := range entries {
		if name, ok := mdns.TypeToString[entries[i].Type]; ok {
			entries[i].TypeName = name
		}
	}
	return entries
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

// ResolveWithTTL queries DNS and returns records plus the TTL in seconds.
// On cache hit, returns the remaining TTL. On cache miss, returns the response TTL.
func (r *Resolver) ResolveWithTTL(name string, qtype uint16) ([]string, uint32, error) {
	if records, ttl, ok := r.cache.getWithTTL(name, qtype); ok {
		return records, ttl, nil
	}

	records, ttl, err := r.query(name, qtype)
	if err != nil {
		return nil, 0, err
	}

	if len(records) > 0 {
		r.cache.put(name, qtype, records, ttl)
	}

	return records, ttl, nil
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
	if r.server == "" {
		return nil, 0, fmt.Errorf("dns query %s %s: no DNS server configured", name, mdns.TypeToString[qtype])
	}

	fqdn := mdns.Fqdn(name)

	validating := r.dnssec != "" && r.dnssec != dnssecOff

	m := new(mdns.Msg)
	m.SetQuestion(fqdn, qtype)
	m.RecursionDesired = true
	// Set the EDNS0 DO (DNSSEC OK) bit only when validation is enabled, so a
	// validating upstream signs / validates and reports SERVFAIL on a broken
	// chain (CD stays 0). Off mode keeps today's non-DNSSEC query exactly.
	m.SetEdns0(4096, validating)

	resp, _, err := r.client.Exchange(m, r.server)
	if err != nil {
		return nil, 0, fmt.Errorf("dns query %s %s: %w", name, mdns.TypeToString[qtype], err)
	}

	if resp == nil {
		return nil, 0, fmt.Errorf("dns query %s %s: nil response", name, mdns.TypeToString[qtype])
	}

	if resp.Truncated {
		r.logger.Warn("truncated DNS response", "name", name, "type", mdns.TypeToString[qtype])
	}

	// DNSSEC policy: reject (strict) or log (permissive) a broken chain before
	// the generic rcode handling below turns a SERVFAIL into an empty result.
	if warn, reject := dnssecDecision(resp.Rcode, resp.AuthenticatedData, r.dnssec); reject != nil {
		return nil, 0, fmt.Errorf("dns query %s %s: %w", name, mdns.TypeToString[qtype], reject)
	} else if warn != "" {
		r.logger.Warn(warn, "name", name, "type", mdns.TypeToString[qtype])
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
			records = append(records, textbuf.Join(v.Txt, ""))
		case *mdns.PTR:
			records = append(records, v.Ptr)
		case *mdns.CNAME:
			records = append(records, v.Target)
		case *mdns.MX:
			records = append(records, v.Mx)
		case *mdns.NS:
			records = append(records, v.Ns)
		case *mdns.SRV:
			var bSrv textbuf.Buffer
			records = append(records, bSrv.Reset().Str(v.Target).Byte(':').Uint16(v.Port).String())
		}
	}

	// Only apply a default TTL when there were no answers at all.
	// When answers have TTL=0, the server explicitly says "do not cache."
	if !hasAnswers && minTTL == 0 {
		minTTL = 300
	}

	return records, minTTL, nil
}
