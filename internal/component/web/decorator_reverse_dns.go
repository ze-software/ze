// Design: docs/architecture/web-interface.md -- reverse DNS (PTR) name resolution
// Overview: render.go -- Template rendering resolves decorators at render time
// Related: decorator.go -- Decorator interface and registry

package web

import (
	"net/netip"
	"strings"
)

// ptrResolver resolves PTR (reverse DNS) records for an IP address.
// Matches the signature of dns.Resolver.ResolvePTR.
type ptrResolver func(address string) ([]string, error)

// reverseDNSDecorator resolves an IP-address leaf value to its reverse-DNS
// (PTR) hostname, e.g. "192.0.2.1" -> "host.example.com".
type reverseDNSDecorator struct {
	resolve ptrResolver
}

// newReverseDNSDecorator creates a reverse-DNS decorator with the given PTR resolver.
func newReverseDNSDecorator(resolve ptrResolver) *reverseDNSDecorator {
	return &reverseDNSDecorator{resolve: resolve}
}

func (d *reverseDNSDecorator) Name() string { return "reverse-dns" }

// Decorate returns the PTR hostname for an IP-address value.
// Returns empty string (not error) on any failure -- graceful degradation.
func (d *reverseDNSDecorator) Decorate(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	// Only attempt a lookup for a syntactically valid IP address, so a leaf whose
	// value is not an address (e.g. a 'dynamic'/'auto' union member) never hits DNS.
	if _, err := netip.ParseAddr(value); err != nil {
		return "", nil //nolint:nilerr // graceful degradation: non-IP input is not an error
	}

	records, err := d.resolve(value)
	if err != nil {
		return "", nil //nolint:nilerr // graceful degradation: DNS failure is not a decorator error
	}
	if len(records) == 0 {
		return "", nil
	}

	// PTR records are FQDNs with a trailing dot; strip it for display.
	return strings.TrimSuffix(records[0], "."), nil
}

// NewReverseDNSDecoratorFromResolver creates a reverse-DNS decorator using a
// resolver that exposes a ResolvePTR method (e.g. *dns.Resolver).
func NewReverseDNSDecoratorFromResolver(resolver interface {
	ResolvePTR(string) ([]string, error)
}) Decorator {
	return newReverseDNSDecorator(resolver.ResolvePTR)
}
