// Design: docs/architecture/resolve.md -- Team Cymru ASN name resolution
// Related: ../dns/resolver.go -- DNS resolver used for TXT queries
//
// Package cymru resolves AS numbers to organization names via Team Cymru DNS.
// Query format: TXT AS<asn>.asn.cymru.com.
// Response format: "ASN | CC | RIR | Date | LABEL - Org Name, CC"
//
// All errors return empty string (graceful degradation). Callers never see
// resolution failures -- this is intentional for display-time decoration.
//
// Uses the shared resolve/cache for 1h TTL caching. DNS keeps its own
// TTL-from-response cache underneath.
package cymru

import (
	"context"
	"math"
	"net"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/cache"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// defaultCacheTTL is the cache duration for Cymru results.
const defaultCacheTTL = time.Hour

// TXTResolver is a function that resolves TXT records for a DNS name.
// Matches the signature pattern of dns.Resolver.ResolveTXT with context.
type TXTResolver func(ctx context.Context, name string) ([]string, error)

// CymruResolver resolves AS numbers to organization names via Team Cymru DNS.
// Safe for concurrent use.
type CymruResolver struct {
	resolveTXT TXTResolver
	cache      *cache.Cache[string]
}

// New creates a Cymru resolver. The resolveTXT function performs the actual
// DNS TXT query. If c is nil, a default 1h TTL cache is created.
func New(resolveTXT TXTResolver, c *cache.Cache[string]) *CymruResolver {
	if c == nil {
		c = cache.New[string](defaultCacheTTL)
	}
	return &CymruResolver{
		resolveTXT: resolveTXT,
		cache:      c,
	}
}

// LookupASNName returns the organization name for the given ASN.
// Returns ("", nil) on any failure -- graceful degradation, never error.
func (r *CymruResolver) LookupASNName(ctx context.Context, asn uint32) (string, error) {
	key := "asn:" + textbuf.StringUint32(asn)

	// Check cache first.
	if name, ok := r.cache.Get(key); ok {
		return name, nil
	}

	query := "AS" + textbuf.StringUint32(asn) + ".asn.cymru.com."

	records, err := r.resolveTXT(ctx, query)
	if err != nil {
		return "", nil //nolint:nilerr // graceful degradation: DNS failure is not a caller error
	}

	if len(records) == 0 {
		return "", nil
	}

	name, ok := parseASNName(records[0])
	if !ok {
		return "", nil
	}

	r.cache.Set(key, name)

	return name, nil
}

// Origin holds the result of an IP-to-ASN origin lookup.
type Origin struct {
	ASN    uint32
	Prefix string
	Name   string
}

// LookupOrigin returns the origin ASN, prefix, and name for an IP address.
// Uses Team Cymru DNS: <reversed-IP>.origin.asn.cymru.com (TXT).
// Response format: "ASN | prefix | CC | RIR | Date"
// Returns (zero Origin, nil) on any failure -- graceful degradation.
func (r *CymruResolver) LookupOrigin(ctx context.Context, ip string) (Origin, error) {
	key := "origin:" + ip

	if name, ok := r.cache.Get(key); ok {
		return parseOriginCached(name), nil
	}

	query := buildOriginQuery(ip)
	if query == "" {
		return Origin{}, nil
	}

	records, err := r.resolveTXT(ctx, query)
	if err != nil || len(records) == 0 {
		return Origin{}, nil //nolint:nilerr // graceful degradation
	}

	o := parseOriginResponse(records[0])
	if o.ASN == 0 {
		return Origin{}, nil
	}

	// Resolve ASN name via the existing method.
	name, _ := r.LookupASNName(ctx, o.ASN)
	o.Name = name

	// Cache as "ASN|prefix|name" for compact storage.
	var b textbuf.Buffer
	b.Uint32(o.ASN).Byte('|').Str(o.Prefix).Byte('|').Str(o.Name)
	r.cache.Set(key, b.String())

	return o, nil
}

func buildOriginQuery(ip string) string {
	if strings.Contains(ip, ":") {
		return buildOrigin6Query(ip)
	}
	return buildOrigin4Query(ip)
}

func buildOrigin4Query(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ""
	}
	var b textbuf.Buffer
	b.Str(parts[3]).Byte('.').Str(parts[2]).Byte('.').Str(parts[1]).Byte('.').Str(parts[0])
	b.Str(".origin.asn.cymru.com.")
	return b.String()
}

func buildOrigin6Query(ip string) string {
	// Expand IPv6 to full form, then reverse nibbles.
	addr := net.ParseIP(ip)
	if addr == nil {
		return ""
	}
	addr = addr.To16()
	if addr == nil {
		return ""
	}
	var b textbuf.Buffer
	b.Grow(128)
	for i := len(addr) - 1; i >= 0; i-- {
		lo := addr[i] & 0x0f
		hi := addr[i] >> 4
		b.Byte("0123456789abcdef"[lo]).Byte('.')
		b.Byte("0123456789abcdef"[hi])
		if i > 0 {
			b.Byte('.')
		}
	}
	b.Str(".origin6.asn.cymru.com.")
	return b.String()
}

// parseOriginResponse parses a Cymru origin TXT response.
// Format: "ASN | prefix | CC | RIR | Date".
func parseOriginResponse(txt string) Origin {
	parts := strings.Split(txt, " | ")
	if len(parts) < 2 {
		return Origin{}
	}
	asn := parseUint32(strings.TrimSpace(parts[0]))
	prefix := strings.TrimSpace(parts[1])
	return Origin{ASN: asn, Prefix: prefix}
}

func parseUint32(s string) uint32 {
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + uint64(c-'0')
		if n > math.MaxUint32 {
			return 0
		}
	}
	return uint32(n)
}

func parseOriginCached(s string) Origin {
	parts := strings.SplitN(s, "|", 3)
	if len(parts) < 3 {
		return Origin{}
	}
	return Origin{
		ASN:    parseUint32(parts[0]),
		Prefix: parts[1],
		Name:   parts[2],
	}
}

// parseASNName extracts the organization name from a Team Cymru TXT response.
// Format: "ASN | CC | RIR | Date | LABEL - Org Name, CC"
// Returns the org name portion (after " - " and before ", CC" if present),
// or the full label if no dash separator exists.
func parseASNName(txt string) (string, bool) {
	if txt == "" {
		return "", false
	}

	parts := strings.Split(txt, " | ")
	if len(parts) < 5 {
		return "", false
	}

	label := strings.TrimSpace(parts[4])
	if label == "" {
		return "", false
	}

	// Try to extract "Org Name" from "LABEL - Org Name, CC" format.
	if _, after, found := strings.Cut(label, " - "); found {
		// Strip trailing ", CC" (country code suffix).
		if commaIdx := strings.LastIndex(after, ", "); commaIdx >= 0 {
			return after[:commaIdx], true
		}

		return after, true
	}

	// No dash separator -- return full label (e.g., "CLOUDFLARENET").
	return label, true
}
