// Design: docs/architecture/l2tp/bng-5-pppoe.md -- bounded session cleanup
// Related: session_routes.go -- exact route ownership
package vpp

import (
	"errors"
	"io"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// Retrying a cleanup whose owner never acquired the pair must be harmless and
// must leave another pair's native path untouched.
func TestVPPRemoveAbsentAddressPreservesOwnedPair(t *testing.T) {
	wire := &ownershipWire{}
	owner := openedOwnershipSession(t, wire)
	if err := owner.RemoveAddress(netip.MustParsePrefix("192.0.2.99/32")); err != nil {
		t.Fatal(err)
	}
	if wire.pathDeletes != 0 {
		t.Fatal("absent address cleanup deleted a native path")
	}
	if wire.paths != 1 {
		t.Fatalf("unrelated native paths = %d", wire.paths)
	}
	if len(owner.addresses) != 1 {
		t.Fatal("absent address cleanup changed the owned pair")
	}
}

// A valid route absent from this owner must not issue a native delete, since
// another session may own the same prefix through a different path.
func TestVPPRemoveAbsentRoutePreservesOwnedPath(t *testing.T) {
	wire := &ownershipWire{}
	owner := openedOwnershipSession(t, wire)
	if err := owner.RemoveRoute(netip.MustParsePrefix("198.51.100.0/24"), netip.Addr{}, 0, rtproto.Iface); err != nil {
		t.Fatal(err)
	}
	if wire.pathDeletes != 0 {
		t.Fatal("absent route cleanup sent a native delete")
	}
	if wire.paths != 1 {
		t.Fatalf("unrelated native paths = %d", wire.paths)
	}
}

// Idempotent absence applies only to valid input on a live owner; it cannot
// conceal an invalid prefix or revive a closed session.
func TestVPPAbsentCleanupRejectsInvalidOwnerUse(t *testing.T) {
	owner := ownershipSession(t, &ownershipWire{})
	if err := owner.RemoveAddress(netip.Prefix{}); err == nil {
		t.Fatal("invalid address cleanup accepted")
	}
	if err := owner.RemoveAddress(netip.MustParsePrefix("::ffff:192.0.2.1/128")); err == nil {
		t.Fatal("mapped IPv4 address cleanup accepted")
	}
	if err := owner.RemoveRoute(netip.Prefix{}, netip.Addr{}, 0, rtproto.Iface); err == nil {
		t.Fatal("invalid route cleanup accepted")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := owner.RemoveAddress(netip.MustParsePrefix("192.0.2.99/32")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed address cleanup = %v", err)
	}
	if err := owner.RemoveRoute(netip.MustParsePrefix("198.51.100.0/24"), netip.Addr{}, 0, rtproto.Iface); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed route cleanup = %v", err)
	}
}
