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
	"fmt"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/resolve/cache"
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
	key := fmt.Sprintf("asn:%d", asn)

	// Check cache first.
	if name, ok := r.cache.Get(key); ok {
		return name, nil
	}

	query := fmt.Sprintf("AS%d.asn.cymru.com.", asn)

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
