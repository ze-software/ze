// Design: docs/architecture/wire/attributes.md -- pinned IANA endpoint classification.
package ipregistry

import (
	"net/netip"
	"strings"
	"testing"
)

// TestLookupPinnedRegistry checks independent address expectations against the
// canonical XML, including both sides of the more-specific prefix boundaries.
func TestLookupPinnedRegistry(t *testing.T) {
	for _, tc := range []struct {
		address string
		want    Result
	}{
		{"10.0.0.1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"172.16.0.1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"192.168.0.1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"100.64.0.1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"198.18.0.1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"fd00::1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"100::1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"192.0.0.0", Result{MatchListed, ValueTrue, ValueTrue}},
		{"192.0.0.7", Result{MatchListed, ValueTrue, ValueTrue}},
		{"192.0.0.8", Result{MatchListed, ValueFalse, ValueFalse}},
		{"192.0.0.9", Result{MatchListed, ValueTrue, ValueTrue}},
		{"192.0.0.10", Result{MatchListed, ValueTrue, ValueTrue}},
		{"192.0.0.11", Result{MatchListed, ValueFalse, ValueFalse}},
		{"192.0.0.170", Result{MatchListed, ValueFalse, ValueFalse}},
		{"192.0.0.171", Result{MatchListed, ValueFalse, ValueFalse}},
		{"192.0.2.1", Result{MatchListed, ValueFalse, ValueFalse}},
		{"198.51.100.1", Result{MatchListed, ValueFalse, ValueFalse}},
		{"203.0.113.1", Result{MatchListed, ValueFalse, ValueFalse}},
		{"169.254.1.1", Result{MatchListed, ValueTrue, ValueFalse}},
		{"255.255.255.255", Result{MatchListed, ValueTrue, ValueFalse}},
		{"2001::1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"2001:1::1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"2001:1::2", Result{MatchListed, ValueTrue, ValueTrue}},
		{"2001:1::3", Result{MatchListed, ValueTrue, ValueTrue}},
		{"2001:1::4", Result{MatchListed, ValueFalse, ValueFalse}},
		{"2001:2::1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"2001:2:1::1", Result{MatchListed, ValueFalse, ValueFalse}},
		{"2001:10::1", Result{MatchListed, ValueUnspecified, ValueUnspecified}},
		{"2001:20::1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"2001:30::1", Result{MatchListed, ValueTrue, ValueTrue}},
		{"192.88.99.1", Result{MatchListed, ValueUnspecified, ValueUnspecified}},
		{"192.88.99.2", Result{MatchListed, ValueTrue, ValueTrue}},
		{"::ffff:10.0.0.1", Result{MatchListed, ValueFalse, ValueFalse}},
		{"100:0:0:1::1", Result{MatchListed, ValueFalse, ValueFalse}},
		{"2001:db8::1", Result{MatchListed, ValueFalse, ValueFalse}},
		{"3fff:fff:ffff:ffff:ffff:ffff:ffff:ffff", Result{MatchListed, ValueFalse, ValueFalse}},
		{"fe80::1", Result{MatchListed, ValueTrue, ValueFalse}},
		{"8.8.8.8", Result{MatchUnlisted, ValueUnspecified, ValueUnspecified}},
		{"3fff:1000::1", Result{MatchUnlisted, ValueUnspecified, ValueUnspecified}},
	} {
		t.Run(tc.address, func(t *testing.T) {
			if got := Lookup(netip.MustParseAddr(tc.address)); got != tc.want {
				t.Fatalf("Lookup = %+v, want %+v", got, tc.want)
			}
		})
	}
	if got := Lookup(netip.Addr{}); got.Match != MatchInvalid {
		t.Fatalf("invalid address classified as %+v", got)
	}
	if got := Lookup(netip.MustParseAddr("fe80::1%eth0")); got.Match != MatchInvalid {
		t.Fatalf("zoned address classified as %+v", got)
	}
	var absent *Registry
	if got := absent.Lookup(netip.MustParseAddr("10.0.0.1")); got.Match != MatchUnspecified {
		t.Fatalf("nil registry classified as %+v", got)
	}
	var empty Registry
	if got := empty.Lookup(netip.MustParseAddr("10.0.0.1")); got.Match != MatchUnspecified {
		t.Fatalf("uninitialized registry classified as %+v", got)
	}
}

const registryFixture = `<registry xmlns="http://www.iana.org/assignments" id="iana-ipv4-special-registry">
<updated>2025-10-09</updated><registry id="iana-ipv4-special-registry-1">
<record><address>192.0.0.0/24 <xref type="note" data="2"/></address><destination>False <xref type="note" data="1"/></destination><forwardable>False</forwardable></record>
<record><address>192.0.0.9/32, 192.0.0.10/32</address><destination>True</destination><forwardable>True</forwardable></record>
<record><address>192.0.0.11/32</address><destination/><forwardable>N/A</forwardable></record>
<record><address>192.0.0.12/32</address><destination>False</destination><forwardable>True</forwardable></record>
</registry></registry>`

// TestParseRegistry preserves blank/N/A overrides, annotations, split prefixes
// and an independently false destination even if forwardable is true.
func TestParseRegistry(t *testing.T) {
	registry, err := Parse([]byte(registryFixture), 32)
	if err != nil {
		t.Fatal(err)
	}
	if registry.Len() != 5 || registry.Updated() != "2025-10-09" {
		t.Fatalf("metadata: entries=%d updated=%s", registry.Len(), registry.Updated())
	}
	for _, tc := range []struct {
		address string
		want    Result
	}{
		{"192.0.0.8", Result{MatchListed, ValueFalse, ValueFalse}},
		{"192.0.0.9", Result{MatchListed, ValueTrue, ValueTrue}},
		{"192.0.0.10", Result{MatchListed, ValueTrue, ValueTrue}},
		{"192.0.0.11", Result{MatchListed, ValueUnspecified, ValueNotApplicable}},
		{"192.0.0.12", Result{MatchListed, ValueFalse, ValueTrue}},
		{"192.0.1.1", Result{MatchUnlisted, ValueUnspecified, ValueUnspecified}},
		{"::ffff:192.0.0.9", Result{MatchInvalid, ValueUnspecified, ValueUnspecified}},
	} {
		if got := registry.Lookup(netip.MustParseAddr(tc.address)); got != tc.want {
			t.Errorf("%s: %+v, want %+v", tc.address, got, tc.want)
		}
	}
}

// TestParseRegistryRejectsBrokenData prevents failed or changed upstream schema
// from becoming a silently permissive shipped table.
func TestParseRegistryRejectsBrokenData(t *testing.T) {
	for name, data := range map[string]string{
		"empty":             "",
		"not-xml":           "upstream unavailable",
		"no-records":        `<registry xmlns="http://www.iana.org/assignments" id="iana-ipv4-special-registry"><updated>2025-10-09</updated><registry id="iana-ipv4-special-registry-1"/></registry>`,
		"wrong-root":        strings.Replace(registryFixture, `id="iana-ipv4-special-registry"`, `id="other"`, 1),
		"wrong-table":       strings.Replace(registryFixture, `id="iana-ipv4-special-registry-1"`, `id="other"`, 1),
		"wrong-namespace":   strings.Replace(registryFixture, "http://www.iana.org/assignments", "https://example.com", 1),
		"missing-date":      strings.Replace(registryFixture, "<updated>2025-10-09</updated>", "", 1),
		"invalid-date":      strings.Replace(registryFixture, "2025-10-09", "2025-99-99", 1),
		"missing-field":     strings.Replace(registryFixture, "<destination/>", "", 1),
		"unknown-value":     strings.Replace(registryFixture, "<destination/>", "<destination>perhaps</destination>", 1),
		"wrong-family":      strings.Replace(registryFixture, "192.0.0.0/24", "2001:db8::/32", 1),
		"host-bits":         strings.Replace(registryFixture, "192.0.0.0/24", "192.0.0.1/24", 1),
		"bad-prefix":        strings.Replace(registryFixture, "192.0.0.0/24", "192.0.0.0/33", 1),
		"duplicate":         strings.Replace(registryFixture, "192.0.0.12/32", "192.0.0.11/32", 1),
		"oversized":         strings.Repeat(" ", SizeMax+1),
		"trailing-document": registryFixture + "<registry/>",
		"trailing-text":     registryFixture + "unexpected",
	} {
		t.Run(name, func(t *testing.T) {
			if registry, err := Parse([]byte(data), 32); err == nil || registry != nil {
				t.Fatalf("accepted invalid dataset: registry=%v err=%v", registry, err)
			}
		})
	}
	if registry, err := Parse([]byte(registryFixture), 64); err == nil || registry != nil {
		t.Fatalf("accepted unsupported address width: registry=%v err=%v", registry, err)
	}
}

// TestLookupAllocations measures only immutable lookup, never address parsing.
func TestLookupAllocations(t *testing.T) {
	for _, address := range []string{"192.0.0.9", "2001:2::1", "::ffff:10.0.0.1", "8.8.8.8"} {
		ip := netip.MustParseAddr(address)
		var got Result
		if allocations := testing.AllocsPerRun(100, func() { got = Lookup(ip) }); allocations != 0 {
			t.Fatalf("%s: %v lookup allocations", address, allocations)
		}
		if got.Match == MatchUnspecified {
			t.Fatal("lookup did not return a classification")
		}
	}
}
