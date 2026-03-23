// Design: (none -- predates documentation)
//
// Package irr provides a client for querying IRR (Internet Routing Registry)
// databases via the RPSL whois protocol (RFC 2622). Used to generate prefix
// filter lists per peer based on their registered AS-SET.
//
// The client performs two operations:
//  1. AS-SET expansion: resolve an AS-SET name to a set of origin ASNs
//  2. Prefix lookup: fetch announced prefixes for an AS-SET, per address family
//
// Both operations use the RPSL whois protocol (TCP port 43) with RIPE/RADB
// query commands. Results are returned as parsed prefix lists ready for
// filter configuration.
//
// No caching is provided; callers manage their own cache and staleness.
package irr

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
	"time"
)

const (
	defaultPort    = "43"
	defaultTimeout = 10 * time.Second
	maxResponse    = 4 << 20 // 4 MB max response (large AS-SETs can be big)
)

// PrefixList holds the resolved prefixes for both address families.
type PrefixList struct {
	IPv4 []netip.Prefix
	IPv6 []netip.Prefix
}

// Empty reports whether both families have zero prefixes.
func (p PrefixList) Empty() bool {
	return len(p.IPv4) == 0 && len(p.IPv6) == 0
}

// IRR queries an IRR database via the RPSL whois protocol.
type IRR struct {
	server  string // host:port of the IRR whois server
	timeout time.Duration
}

// NewIRR creates an IRR client for the given whois server.
// The server string may be "host", "host:port", or empty for the default
// (whois.radb.net:43).
func NewIRR(server string) *IRR {
	if server == "" {
		server = "whois.radb.net"
	}
	if _, _, err := net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, defaultPort)
	}
	return &IRR{
		server:  server,
		timeout: defaultTimeout,
	}
}

// ResolveASSet expands an AS-SET name into a set of origin ASNs.
// Handles recursive AS-SET references (AS-SET containing other AS-SETs).
// Uses the RPSL "!i" command for AS-SET member expansion.
func (c *IRR) ResolveASSet(ctx context.Context, asSet string) ([]uint32, error) {
	seen := make(map[string]bool)
	result := make(map[uint32]bool)
	if err := c.resolveASSetRecursive(ctx, asSet, seen, result); err != nil {
		return nil, err
	}

	asns := make([]uint32, 0, len(result))
	for asn := range result {
		asns = append(asns, asn)
	}
	slices.Sort(asns)
	return asns, nil
}

func (c *IRR) resolveASSetRecursive(ctx context.Context, asSet string, seen map[string]bool, result map[uint32]bool) error {
	upper := strings.ToUpper(asSet)
	if seen[upper] {
		return nil // cycle detection
	}
	seen[upper] = true

	response, err := c.query(ctx, fmt.Sprintf("!i%s", asSet))
	if err != nil {
		return fmt.Errorf("irr: resolve AS-SET %s: %w", asSet, err)
	}

	for line := range strings.SplitSeq(strings.TrimSpace(response), "\n") {
		for word := range strings.FieldsSeq(line) {
			if word == "C" || word == "D" {
				continue // end markers
			}
			// Skip the leading "A<count>" answer marker (e.g., "A3" = 3 results).
			if isAnswerMarker(word) {
				continue
			}

			// Try to parse as ASN (with or without "AS" prefix).
			if asn, ok := parseASN(word); ok {
				result[asn] = true
				continue
			}

			// Nested AS-SET reference.
			if strings.HasPrefix(strings.ToUpper(word), "AS-") || strings.Contains(word, ":") {
				if err := c.resolveASSetRecursive(ctx, word, seen, result); err != nil {
					return err
				}
				continue
			}
		}
	}

	return nil
}

// LookupPrefixes fetches the announced prefixes for an AS-SET from the IRR.
// Uses the RPSL "!a4" and "!a6" commands for IPv4 and IPv6 prefix queries.
// Prefixes are aggregated (collapsed) and sorted.
func (c *IRR) LookupPrefixes(ctx context.Context, asSet string) (PrefixList, error) {
	ipv4, err := c.lookupFamilyPrefixes(ctx, asSet, 4)
	if err != nil {
		return PrefixList{}, err
	}

	ipv6, err := c.lookupFamilyPrefixes(ctx, asSet, 6)
	if err != nil {
		return PrefixList{}, err
	}

	return PrefixList{
		IPv4: aggregateAndSort(ipv4),
		IPv6: aggregateAndSort(ipv6),
	}, nil
}

func (c *IRR) lookupFamilyPrefixes(ctx context.Context, asSet string, family int) ([]netip.Prefix, error) {
	response, err := c.query(ctx, fmt.Sprintf("!a%d%s", family, asSet))
	if err != nil {
		return nil, fmt.Errorf("irr: lookup prefixes %s (IPv%d): %w", asSet, family, err)
	}

	var prefixes []netip.Prefix

	for line := range strings.SplitSeq(strings.TrimSpace(response), "\n") {
		for word := range strings.FieldsSeq(line) {
			if word == "C" || word == "D" {
				continue // end markers
			}
			// Skip the leading "A<count>" answer line.
			if word != "" && word[0] == 'A' {
				continue
			}

			p, parseErr := netip.ParsePrefix(word)
			if parseErr != nil {
				continue // skip unparseable entries
			}
			prefixes = append(prefixes, p.Masked()) // normalize
		}
	}

	return prefixes, nil
}

// query sends an RPSL query to the IRR server and returns the raw response.
// Each query opens a new TCP connection (whois protocol is one-shot).
func (c *IRR) query(ctx context.Context, command string) (string, error) {
	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.server)
	if err != nil {
		return "", fmt.Errorf("irr: connect %s: %w", c.server, err)
	}
	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(c.timeout))
	}

	if _, err := fmt.Fprintf(conn, "%s\n", command); err != nil {
		return "", fmt.Errorf("irr: send query: %w", err)
	}

	buf := make([]byte, maxResponse)
	var total int
	for {
		n, readErr := conn.Read(buf[total:])
		total += n
		if readErr != nil {
			break
		}
		if total >= maxResponse {
			break
		}
	}

	return string(buf[:total]), nil
}

// isAnswerMarker reports whether s is an RPSL answer marker like "A3" or "A125".
// These appear at the start of whois responses to indicate the result count.
func isAnswerMarker(s string) bool {
	if len(s) < 2 || s[0] != 'A' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseASN parses an ASN string in formats: "AS65001", "65001".
// Returns the ASN number and true on success.
func parseASN(s string) (uint32, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	// Strip "AS" prefix (e.g., "AS65001" or "as65001").
	num := s
	if strings.HasPrefix(strings.ToUpper(s), "AS") {
		num = s[2:]
	}

	if num == "" {
		return 0, false
	}

	// Parse as uint32.
	var asn uint32
	for _, c := range num {
		if c < '0' || c > '9' {
			return 0, false
		}
		asn = asn*10 + uint32(c-'0')
	}

	if asn == 0 {
		return 0, false
	}

	return asn, true
}

// aggregateAndSort collapses overlapping prefixes and sorts the result.
// This is the Go equivalent of Python's ipaddress.collapse_addresses().
func aggregateAndSort(prefixes []netip.Prefix) []netip.Prefix {
	if len(prefixes) == 0 {
		return nil
	}

	// Deduplicate first.
	seen := make(map[netip.Prefix]bool, len(prefixes))
	unique := make([]netip.Prefix, 0, len(prefixes))
	for _, p := range prefixes {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}

	// Sort by address, then by prefix length (shorter first).
	slices.SortFunc(unique, func(a, b netip.Prefix) int {
		if c := a.Addr().Compare(b.Addr()); c != 0 {
			return c
		}
		return cmp.Compare(a.Bits(), b.Bits())
	})

	// Remove prefixes that are covered by a shorter (broader) prefix.
	result := make([]netip.Prefix, 0, len(unique))
	for _, p := range unique {
		covered := false
		for _, existing := range result {
			if existing.Contains(p.Addr()) && existing.Bits() <= p.Bits() {
				covered = true
				break
			}
		}
		if !covered {
			result = append(result, p)
		}
	}

	return result
}
