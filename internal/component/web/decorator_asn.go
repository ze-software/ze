// Design: docs/architecture/web-interface.md -- ASN name resolution via Team Cymru DNS
// Overview: render.go -- Template rendering resolves decorators at render time
// Related: decorator.go -- Decorator interface and registry

package web

import (
	"fmt"
	"strconv"
	"strings"
)

// txtResolver is a function that resolves TXT records for a DNS name.
// Matches the signature of dns.Resolver.ResolveTXT.
type txtResolver func(name string) ([]string, error)

// asnNameDecorator resolves AS numbers to organization names via Team Cymru DNS.
// Query format: TXT ASxxxxx.asn.cymru.com.
// Response format: "ASN | CC | RIR | Date | Description".
type asnNameDecorator struct {
	resolve txtResolver
}

// newASNNameDecorator creates an ASN name decorator with the given TXT resolver.
func newASNNameDecorator(resolve txtResolver) *asnNameDecorator {
	return &asnNameDecorator{resolve: resolve}
}

func (d *asnNameDecorator) Name() string { return "asn-name" }

// Decorate returns the organization name for the given ASN value.
// Returns empty string (not error) on any failure -- graceful degradation.
func (d *asnNameDecorator) Decorate(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	// Validate ASN is numeric and in range 0-4294967295.
	asn, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return "", nil //nolint:nilerr // graceful degradation: non-numeric input is not an error
	}

	const maxASN = 4294967295
	if asn > maxASN {
		return "", nil
	}

	query := fmt.Sprintf("AS%d.asn.cymru.com.", asn)

	records, err := d.resolve(query)
	if err != nil {
		return "", nil //nolint:nilerr // graceful degradation: DNS failure is not a decorator error
	}

	if len(records) == 0 {
		return "", nil
	}

	name, ok := parseASNName(records[0])
	if !ok {
		return "", nil
	}

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

// NewASNNameDecoratorFromResolver creates an ASN name decorator using a DNS resolver.
// The resolver parameter must have a ResolveTXT method.
func NewASNNameDecoratorFromResolver(resolver interface {
	ResolveTXT(string) ([]string, error)
}) Decorator {
	return newASNNameDecorator(resolver.ResolveTXT)
}
