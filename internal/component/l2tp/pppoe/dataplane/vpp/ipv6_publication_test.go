// Design: docs/architecture/l2tp/bng-5-pppoe.md -- acknowledged IPv6 carrier
// Related: ipv6_session.go -- negotiated pair publication guard
package vpp

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

// The common owner's post-ACK Connect publishes the pair. Merely staging its
// IPv6CP addresses must not expose the network-control carrier.
func TestVPPIPv6CarrierWaitsForPublishedPair(t *testing.T) {
	owner := ownershipSession(t, &ownershipWire{})
	if err := owner.AddAddressP2P(netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.OpenIPv6Carrier(context.Background()); err == nil {
		t.Fatal("staged pair opened an IPv6 carrier before ACK")
	}
	if err := owner.Connect(); err != nil {
		t.Fatal(err)
	}
	carrier, err := owner.OpenIPv6Carrier(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
}

// A lost native reply leaves a cleanup obligation, not permission to open the
// IPv6 carrier; the pair must await confirmed publication.
func TestVPPIPv6CarrierRefusesUnconfirmedPair(t *testing.T) {
	want := errors.New("peer-route reply lost")
	owner := ownershipSession(t, &ownershipWire{addReadError: want})
	if err := owner.AddAddressP2P(netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")); err != nil {
		t.Fatal(err)
	}
	if err := owner.Connect(); !errors.Is(err, want) {
		t.Fatalf("publication error = %v", err)
	}
	if _, err := owner.OpenIPv6Carrier(context.Background()); err == nil {
		t.Fatal("unconfirmed pair opened an IPv6 carrier")
	}
}
