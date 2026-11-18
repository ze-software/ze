//go:build integration && linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- original IPv6 carrier custody
// Related: native_ipv6_carrier_fixture_integration_linux_test.go -- real PPPoE/IPv6 wire
// These are additive current-API TDD roots, not a new kernel ABI or a native
// result. Each uses Access.Open and the admitted public data-plane interface.
package kernel

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"reflect"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestNativeIPv6CarrierOriginalTraffic proves all four physical traffic lanes,
// cancellation without loss of the final write path, repeated carrier cleanup,
// and a second original session surviving the first carrier's departure.
func TestNativeIPv6CarrierOriginalTraffic(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeV6Fixture(t)
	opened, endpoint := f.open(t, 801)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := carrier.ReadRS(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("original canceled RS=%v", err)
	}
	var buffer [128]byte
	if _, _, err := carrier.ReadDHCPv6(ctx, buffer[:]); !errors.Is(err, context.Canceled) {
		t.Fatalf("original canceled DHCPv6=%v", err)
	}
	nativeV6CancelReads(t, carrier)
	// A later fresh read must not inherit an earlier cancellation deadline.
	nativeV6Useful(t, carrier, endpoint, 2)
	if err := carrier.WriteRA(nativeV6RA(3), 255); err != nil {
		t.Fatalf("final RA after reader cancellation: %v", err)
	}
	endpoint.requireOutput(t, 58, 3, 0)
	survivor, survivorEndpoint := f.open(t, 802)
	survivorCarrier := nativeV6Acquire(t, survivor)
	nativeV6Useful(t, survivorCarrier, survivorEndpoint, 4)
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
	nativeV6Useful(t, survivorCarrier, survivorEndpoint, 5)
	if err := opened.Dataplane().Close(); err != nil {
		t.Fatal(err)
	}
	if err := opened.CloseTransport(); err != nil {
		t.Fatal(err)
	}
	late, err := opened.Dataplane().OpenIPv6Carrier(t.Context())
	if late != nil {
		t.Cleanup(func() {
			if err := late.Close(); err != nil {
				t.Error(err)
			}
		})
		t.Fatal("closed original session acquired another carrier")
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed original carrier acquisition=%v", err)
	}
	nativeV6Useful(t, survivorCarrier, survivorEndpoint, 6)
}

// A physical rename leaves the original useful. The independently useful
// old-name replacement cannot become the existing carrier's receive/write lane.
func TestNativeIPv6CarrierRenameAfterAcquire(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeV6Fixture(t)
	opened, endpoint := f.open(t, 803)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 10)
	replacement := f.rename(t, opened)
	replacement.useful(t, 11)
	nativeV6Useful(t, carrier, endpoint, 12)
	replacement.useful(t, 13)
}

// Acquisition belongs to the retained admitted owner even after its captured
// name has been given to another real device. No future constructor is used.
func TestNativeIPv6CarrierAcquireAfterRename(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeV6Fixture(t)
	opened, endpoint := f.open(t, 804)
	initial := nativeV6Acquire(t, opened)
	nativeV6Useful(t, initial, endpoint, 20)
	if err := initial.Close(); err != nil {
		t.Fatal(err)
	}
	replacement := f.rename(t, opened)
	replacement.useful(t, 21)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 22)
	replacement.useful(t, 23)
}

// Each stale-receive root first proves all original lanes, then gives the
// exact native index AND name to a new Ethernet device with its own listeners.
// A useful replacement request must not satisfy the retired original's read.
func TestNativeIPv6CarrierRetiredRS(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeV6Fixture(t)
	opened, endpoint := f.open(t, 805)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 30)
	replacement := f.retire(t, opened)
	replacement.useful(t, 31)
	replacement.endpoint.inject(t, nativeV6RS())
	replacement.receiveRS(t)
	ctx, cancel := context.WithTimeout(t.Context(), nativeV6Wait)
	defer cancel()
	if err := carrier.ReadRS(ctx); err == nil {
		t.Fatal("retired original accepted an RS from the exact-index/name replacement")
	}
	replacement.useful(t, 32)
}

func TestNativeIPv6CarrierRetiredDHCP(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeV6Fixture(t)
	opened, endpoint := f.open(t, 806)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 40)
	replacement := f.retire(t, opened)
	replacement.useful(t, 41)
	replacement.endpoint.inject(t, nativeV6Request(42))
	replacement.receiveDHCP(t, 42)
	ctx, cancel := context.WithTimeout(t.Context(), nativeV6Wait)
	defer cancel()
	var buffer [128]byte
	if n, peer, err := carrier.ReadDHCPv6(ctx, buffer[:]); err == nil {
		t.Fatalf("retired original accepted replacement DHCPv6 %x from %s", buffer[:n], peer)
	}
	replacement.useful(t, 43)
}

// A replacement's own marker closes the actual wire observation window.
// Success requires both an explicit stale-write refusal and no stale marker
// ahead of that useful replacement output, rather than an error alone.
func TestNativeIPv6CarrierRetiredRAWrite(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeV6Fixture(t)
	opened, endpoint := f.open(t, 807)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 50)
	replacement := f.retire(t, opened)
	replacement.useful(t, 51)
	staleErr := carrier.WriteRA(nativeV6RA(52), 255)
	replacement.writeRA(t, 53)
	replacement.endpoint.requireOutput(t, 58, 53, 52)
	if staleErr == nil {
		t.Fatal("retired original reported a successful RA write after actual index/name replacement")
	}
	replacement.useful(t, 54)
}

func TestNativeIPv6CarrierRetiredDHCPWrite(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeV6Fixture(t)
	opened, endpoint := f.open(t, 808)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 60)
	replacement := f.retire(t, opened)
	replacement.useful(t, 61)
	peer := netip.AddrPortFrom(netip.MustParseAddr("fe80::2"), 546)
	_, staleErr := carrier.WriteDHCPv6(nativeV6DHCP(7, 62), peer)
	replacement.writeDHCP(t, 63)
	replacement.endpoint.requireOutput(t, 17, 63, 62)
	if staleErr == nil {
		t.Fatal("retired original reported a successful DHCPv6 write after actual index/name replacement")
	}
	replacement.useful(t, 64)
}

func TestNativeIPv6CarrierMulticastCleanup(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeV6Fixture(t)
	opened, endpoint := f.open(t, 809)
	carrier := nativeV6Acquire(t, opened)
	nativeV6Useful(t, carrier, endpoint, 70)
	replacement := f.retire(t, opened)
	replacement.useful(t, 71)
	before := nativeV6GroupUsers(t, replacement.link.Attrs().Index)
	if err := carrier.Close(); err != nil {
		t.Fatalf("retired original multicast cleanup: %v", err)
	}
	after := nativeV6GroupUsers(t, replacement.link.Attrs().Index)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("retired socket cleanup changed replacement memberships: before=%v after=%v", before, after)
	}
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
	replacement.useful(t, 72)
}

// The retained session, not the calling OS thread's namespace, selects socket
// construction. The foreign same-name/index device is independently useful.
func TestNativeIPv6CarrierForeignCaller(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	f := newNativeV6Fixture(t)
	opened, endpoint := f.open(t, 810)
	initial := nativeV6Acquire(t, opened)
	nativeV6Useful(t, initial, endpoint, 80)
	if err := initial.Close(); err != nil {
		t.Fatal(err)
	}
	identity := opened.Dataplane().Identity()
	var carrier ppp.IPv6Carrier
	var foreign *nativeV6Replacement
	nativeV6Foreign(t, func(route *netlink.Handle) {
		foreign = nativeV6ReplacementNew(t, route, identity.InterfaceName, int(identity.NativeIndex))
		foreign.useful(t, 81)
		carrier = nativeV6Acquire(t, opened)
	})
	nativeV6Useful(t, carrier, endpoint, 82)
	foreign.useful(t, 83)
}
