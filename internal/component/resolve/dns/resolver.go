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

// Status is what the answer says about the NAME, separated from what the
// transport says about the query. A caller that acts on an answer needs both:
// an error means Ze never learned anything, while a Status means Ze learned
// something and this is what it learned.
//
// Ze collapses the RFC 1035 Section 4.1.1 RCODE space to the four outcomes a
// caller acts on differently, because every remaining code is a server-side
// failure a retry can clear and they take one branch. The numeric RCODE stays
// in the resolver: a caller comparing against mdns.RcodeNameError would be
// reading the wire format of a dependency this package exists to hide.
//
// StatusUnspecified is the zero value and is never a real answer. It is what a
// return carrying an error holds, so a caller that reads the Status without
// checking the error gets a value no branch accepts rather than a plausible
// one (ai/rules/principles.md).
type Status uint8

const (
	// StatusUnspecified means no answer was obtained. The error says why.
	StatusUnspecified Status = iota
	// StatusSuccess is NOERROR: the answer is authoritative for the name, and
	// an EMPTY record list means the name holds no record of this type.
	StatusSuccess
	// StatusNameError is NXDOMAIN: the name does not exist. Authoritative.
	StatusNameError
	// StatusServerFailure is SERVFAIL, which a broken DNSSEC chain also
	// produces upstream. Transient: it says nothing about the name.
	StatusServerFailure
	// StatusRefused is REFUSED and every other non-success RCODE. Transient.
	StatusRefused
)

// Authoritative reports whether the status describes the NAME rather than the
// server. A caller may act on the record list only when this is true: on a
// transient status the list is empty because the server did not answer, not
// because the name holds nothing.
func (s Status) Authoritative() bool {
	return s == StatusSuccess || s == StatusNameError
}

// String names the status for a log line or an operator-facing field. It is
// never compared against: callers branch on the constants.
func (s Status) String() string {
	switch s {
	case StatusSuccess:
		return "NOERROR"
	case StatusNameError:
		return "NXDOMAIN"
	case StatusServerFailure:
		return "SERVFAIL"
	case StatusRefused:
		return "REFUSED"
	case StatusUnspecified:
		return "unspecified"
	}
	return "unspecified"
}

// ParseStatus reads back a spelling String produced. It is the return path for
// a status that crossed a process boundary, so the vocabulary is declared once
// here rather than a second time in the transport (ai/rules/principles.md).
//
// An unrecognized spelling answers StatusUnspecified, which no branch treats as
// an answer, rather than the nearest plausible value.
func ParseStatus(s string) Status {
	switch s {
	case "NOERROR":
		return StatusSuccess
	case "NXDOMAIN":
		return StatusNameError
	case "SERVFAIL":
		return StatusServerFailure
	case "REFUSED":
		return StatusRefused
	}
	return StatusUnspecified
}

// statusFromRcode maps an RFC 1035 Section 4.1.1 RCODE onto the outcome a
// caller branches on. Anything past REFUSED is a server-side condition that
// says nothing about the name, so it takes the REFUSED branch: keep what you
// had and try again.
func statusFromRcode(rcode int) Status {
	switch rcode {
	case mdns.RcodeSuccess:
		return StatusSuccess
	case mdns.RcodeNameError:
		return StatusNameError
	case mdns.RcodeServerFailure:
		return StatusServerFailure
	}
	return StatusRefused
}

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

	records, ttl, _, err := r.query(name, qtype)
	if err != nil {
		return nil, err
	}

	// Only cache non-empty results. NXDOMAIN returns empty records and is not cached.
	if len(records) > 0 {
		r.cache.put(name, qtype, records, ttl)
	}

	return records, nil
}

// ResolveWithTTL queries DNS and returns records, the TTL in seconds, and what
// the answer said about the name.
// On cache hit, returns the remaining TTL. On cache miss, returns the response TTL.
//
// The Status separates a name that does not exist from a server that could not
// answer, which the record list alone cannot: both arrive as an empty list. A
// caller that programs state from the answer MUST read it, because emptying
// that state on a SERVFAIL enforces a server outage rather than a fact about
// the name. On an error the Status is StatusUnspecified: nothing was learned.
//
// A cache hit is always StatusSuccess. Only a successful answer carrying
// records is cached (put is called for no other outcome), so a hit is by
// construction a name that resolved.
func (r *Resolver) ResolveWithTTL(name string, qtype uint16) ([]string, uint32, Status, error) {
	if records, ttl, ok := r.cache.getWithTTL(name, qtype); ok {
		return records, ttl, StatusSuccess, nil
	}

	records, ttl, status, err := r.query(name, qtype)
	if err != nil {
		return nil, 0, StatusUnspecified, err
	}

	if len(records) > 0 {
		r.cache.put(name, qtype, records, ttl)
	}

	return records, ttl, status, nil
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
// Returns records, minimum TTL from answers, what the answer said about the
// name, and any error. Every error return carries StatusUnspecified: an error
// means the exchange produced no answer, so there is nothing for a status to
// describe.
func (r *Resolver) query(name string, qtype uint16) ([]string, uint32, Status, error) {
	if r.server == "" {
		return nil, 0, StatusUnspecified, fmt.Errorf("dns query %s %s: no DNS server configured", name, mdns.TypeToString[qtype])
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
		return nil, 0, StatusUnspecified, fmt.Errorf("dns query %s %s: %w", name, mdns.TypeToString[qtype], err)
	}

	if resp == nil {
		return nil, 0, StatusUnspecified, fmt.Errorf("dns query %s %s: nil response", name, mdns.TypeToString[qtype])
	}

	if resp.Truncated {
		r.logger.Warn("truncated DNS response", "name", name, "type", mdns.TypeToString[qtype])
	}

	// DNSSEC policy: reject (strict) or log (permissive) a broken chain before
	// the generic rcode handling below turns a SERVFAIL into an empty result.
	if warn, reject := dnssecDecision(resp.Rcode, resp.AuthenticatedData, r.dnssec); reject != nil {
		return nil, 0, StatusUnspecified, fmt.Errorf("dns query %s %s: %w", name, mdns.TypeToString[qtype], reject)
	} else if warn != "" {
		r.logger.Warn(warn, "name", name, "type", mdns.TypeToString[qtype])
	}

	status := statusFromRcode(resp.Rcode)

	// NXDOMAIN and other non-error response codes return empty results, not
	// errors. The status is what tells them apart: a caller acting on the
	// answer reads it, and one that only wants records ignores it, which is
	// how Resolve keeps its own signature.
	if resp.Rcode != mdns.RcodeSuccess {
		return nil, 0, status, nil
	}

	records, ttl := extractRecords(resp)
	return records, ttl, status, nil
}

// extractRecords pulls string values and minimum TTL from DNS answer records.
// Returns TTL=0 when answers have TTL=0 (caller should not cache per RFC 1035).
//
// It cannot fail: a record type it does not recognize is skipped rather than
// refused, because a server is free to answer with more than was asked for.
// So it returns no error, and a caller has no failure branch to write.
func extractRecords(resp *mdns.Msg) ([]string, uint32) {
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

	return records, minTTL
}
