// Design: docs/architecture/l2tp/bng-5-pppoe.md -- NCP acknowledgement barrier.
// Method: observe the real session owner against the native feature model.
package vpp

import (
	"net/netip"
	"testing"
)

// A late address installation stages its peer route but cannot grant data
// before the common PPP owner confirms its assignment and calls Connect.
func TestVPPAddressPairWaitsForPostACKConnect(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if wire.paths != 0 {
		t.Fatalf("native peer paths before ACK = %d", wire.paths)
	}
	if wire.enables != [2]int{} {
		t.Fatalf("address installation granted a family before ACK: %v", wire.enables)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if wire.enables != [2]int{1, 0} {
		t.Fatalf("post-ACK family enables = %v", wire.enables)
	}
}

// A refused route withdrawal remains a cleanup obligation, never a grantable
// address when the independent family later completes its assignment.
func TestVPPFailedAddressWithdrawalCannotReopenFamily(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	local := netip.MustParsePrefix("fe80::1/64")
	if err := owner.AddAddressP2P(local, netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	wire.deleteRouteRetval = -1
	if err := owner.RemoveAddress(local); err == nil {
		t.Fatal("native route refusal was hidden")
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if wire.references != [2]int{1, 0} {
		t.Fatalf("independent assignment reopened a retiring family: %v", wire.references)
	}
	if wire.paths != 2 {
		t.Fatalf("unconfirmed withdrawal lost its route obligation: %d paths", wire.paths)
	}
	if err := owner.Close(); err == nil {
		t.Fatal("unconfirmed route cleanup reported success")
	}
	if wire.sessionDeletes != 0 {
		t.Fatal("retiring route allowed native index reuse")
	}
}

// A refused gate close records withdrawal intent before mutation. A later
// reconciliation retries quarantine while preserving the other open family.
func TestVPPFailedAddressGateCloseRetainsWithdrawalIntent(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	local := netip.MustParsePrefix("fe80::1/64")
	if err := owner.AddAddressP2P(local, netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if err := owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.7/32")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	wire.disableRetval[1] = -1
	if err := owner.RemoveAddress(local); err == nil {
		t.Fatal("native gate refusal was hidden")
	}
	wire.disableRetval[1] = 0
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if wire.references != [2]int{1, 0} {
		t.Fatalf("reconciliation discarded withdrawal intent: %v", wire.references)
	}
	if wire.enables[0] != 1 {
		t.Fatalf("independent IPv4 gate was cycled: %d opens", wire.enables[0])
	}
}

// A retained failed withdrawal can be retried by its original local prefix;
// no replacement may reuse that prefix before the exact route is gone.
func TestVPPAddressWithdrawalRetryKeepsExactOwner(t *testing.T) {
	wire := &ownershipWire{}
	owner := ownershipSession(t, wire)
	local := netip.MustParsePrefix("fe80::1/64")
	peer := netip.MustParsePrefix("fe80::2/64")
	if err := owner.AddAddressP2P(local, peer); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	wire.deleteRouteRetval = -1
	if err := owner.RemoveAddress(local); err == nil {
		t.Fatal("native route refusal was hidden")
	}
	if err := owner.AddAddressP2P(local, netip.MustParsePrefix("fe80::3/64")); err == nil {
		t.Fatal("retiring local prefix accepted a replacement")
	}
	wire.deleteRouteRetval = 0
	if err := owner.RemoveAddress(local); err != nil {
		t.Fatal(err)
	}
	if wire.paths != 0 {
		t.Fatalf("retried original peer path remained: %d", wire.paths)
	}
	if err := owner.AddAddressP2P(local, netip.MustParsePrefix("fe80::3/64")); err != nil {
		t.Fatal(err)
	}
	if wire.references[1] != 0 {
		t.Fatal("replacement opened before its assignment ACK")
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	if wire.references[1] != 1 {
		t.Fatal("post-ACK replacement family remained closed")
	}
}
