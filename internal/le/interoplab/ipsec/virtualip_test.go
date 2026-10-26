// VALIDATES: the virtual-IP checkers read one lease per side and refuse a missing,
// shared, pool-wide or wildcard answer.
// PREVENTS: a virtual-ip-assignment or virtual-ip-uniqueness run that passes on an
// answer naming no lease.
package ipsec

import (
	"net/netip"
	"strings"
	"testing"
)

// TestSwanVirtualIPReadsTheLocalLease drives the strongSwan reader of the virtual-IP
// scenarios with an answer shaped as swanctl prints it, and requires it to refuse an
// SA that carries no virtual IP, two SAs, and an address that does not parse: each is
// an answer that cannot name the lease, and reading it as one would pass vacuously.
func TestSwanVirtualIPReadsTheLocalLease(t *testing.T) {
	const answer = "ze: #1, ESTABLISHED, IKEv2, 0123456789abcdef_i fedcba9876543210_r*\n" +
		"  local  'testuser' @ 172.28.0.3[4500] [10.10.0.1]\n" +
		"  remote '172.28.0.3' @ 172.28.0.2[4500]\n" +
		"  ze-child: #1, reqid 1, INSTALLED, TUNNEL, ESP:AES_GCM_16-256\n" +
		"    local  10.10.0.1/32\n" +
		"    remote 10.20.0.1/32\n"
	lease, err := swanVirtualIP(answer)
	if err != nil {
		t.Fatalf("read the lease: %v", err)
	}
	if lease != netip.MustParseAddr("10.10.0.1") {
		t.Fatalf("read lease %s, want 10.10.0.1", lease)
	}
	for _, test := range []struct{ name, answer string }{
		{name: "no virtual IP", answer: strings.ReplaceAll(answer, " [10.10.0.1]\n", "\n")},
		{name: "two IKE SAs", answer: answer + answer},
		{name: "not an address", answer: strings.ReplaceAll(answer, "[10.10.0.1]", "[10.10.0.x]")},
		{name: "empty", answer: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := swanVirtualIP(test.answer); err == nil {
				t.Fatal("accepted an answer that names no single lease")
			}
		})
	}
}

// TestZeLeaseRequiresTheLeaseAsTheRemoteSelector drives the Ze side of the checker:
// the record must name exactly the lease strongSwan holds, and the Child SA's remote
// selector must be that lease alone. A pool-wide or wildcard remote selector is the
// RFC 3948 Section 5.1 substitution missing, and it must be refused.
func TestZeLeaseRequiresTheLeaseAsTheRemoteSelector(t *testing.T) {
	record := func(virtualIP []any, remote string) []map[string]any {
		return []map[string]any{{
			"peer-name":  "swan",
			"state":      zeStateEstablished,
			"virtual-ip": virtualIP,
			"child-sa":   map[string]any{"ts-local": "10.20.0.1/32", "ts-remote": remote},
		}}
	}
	want := netip.MustParseAddr("10.10.0.1")
	lease, err := zeLeaseOf(record([]any{"10.10.0.1"}, "10.10.0.1/32"), "swan")
	if err != nil {
		t.Fatalf("read the lease: %v", err)
	}
	if err := requireZeLease(lease, "swan", want); err != nil {
		t.Fatalf("refused a correct lease: %v", err)
	}
	for _, test := range []struct {
		name    string
		records []map[string]any
	}{
		{name: "pool-wide selector", records: record([]any{"10.10.0.1"}, "10.10.0.0/24")},
		{name: "wildcard selector", records: record([]any{"10.10.0.1"}, "0.0.0.0/0")},
		{name: "other lease", records: record([]any{"10.10.0.2"}, "10.10.0.2/32")},
		{name: "no lease", records: record([]any{}, "10.10.0.1/32")},
		{name: "no record", records: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			lease, err := zeLeaseOf(test.records, "swan")
			if err == nil {
				err = requireZeLease(lease, "swan", want)
			}
			if err == nil {
				t.Fatal("accepted an SA whose selector or lease is not the client's lease")
			}
		})
	}
}

// TestLeaseChecksRefuseSharedAndPoolWideAnswers drives the two set predicates: two
// clients holding one address, a pair missing the requested address, a lease outside
// the pool, and a kernel policy for the whole pool are each refused, and the correct
// answers pass.
func TestLeaseChecksRefuseSharedAndPoolWideAnswers(t *testing.T) {
	requested := netip.MustParseAddr(vipRequested)
	other := netip.MustParseAddr("10.1.2.1")
	if err := requireDistinctLeases([]netip.Addr{requested, other}, vipUniquePool); err != nil {
		t.Fatalf("refused two distinct leases: %v", err)
	}
	for _, test := range []struct {
		name   string
		leases []netip.Addr
	}{
		{name: "one address twice", leases: []netip.Addr{requested, requested}},
		{name: "requested address absent", leases: []netip.Addr{other, netip.MustParseAddr("10.1.2.4")}},
		{name: "outside the pool", leases: []netip.Addr{requested, netip.MustParseAddr("10.9.9.9")}},
		{name: "network address", leases: []netip.Addr{requested, netip.MustParseAddr("10.1.2.0")}},
		{name: "one lease", leases: []netip.Addr{requested}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := requireDistinctLeases(test.leases, vipUniquePool); err == nil {
				t.Fatal("accepted leases that are not two distinct addresses of the pool")
			}
		})
	}

	leasePairs := map[string]struct{}{
		policyPair("10.20.0.1/32", "10.1.2.3/32"): {},
		policyPair("10.20.0.1/32", "10.1.2.1/32"): {},
	}
	leases := []netip.Addr{requested, other}
	if err := requireLeasePolicies(leasePairs, leases, vipUniquePool); err != nil {
		t.Fatalf("refused the lease policies: %v", err)
	}
	poolPairs := map[string]struct{}{policyPair("10.20.0.1/32", vipUniquePool): {}}
	if err := requireLeasePolicies(poolPairs, leases, vipUniquePool); err == nil {
		t.Fatal("accepted a kernel that holds only the pool-wide policy")
	}
	poolPairs[policyPair("10.20.0.1/32", "10.1.2.3/32")] = struct{}{}
	poolPairs[policyPair("10.20.0.1/32", "10.1.2.1/32")] = struct{}{}
	if err := requireLeasePolicies(poolPairs, leases, vipUniquePool); err == nil {
		t.Fatal("accepted a kernel that holds a pool-wide policy beside the lease policies")
	}
}
