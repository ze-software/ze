//go:build linux

// Design: docs/architecture/testing/interop.md -- form evidence is actual interface egress.
// Related: probe_linux.go -- AF_PACKET metadata boundary; frame_test.go -- wire bytes.
package main

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// TestLinkObservationRequiresOutgoingIPv4OnTheSelectedInterface refuses ingress and ARP.
func TestLinkObservationRequiresOutgoingIPv4OnTheSelectedInterface(t *testing.T) {
	for _, test := range []struct {
		name     string
		metadata unix.SockaddrLinklayer
		accepted bool
	}{
		{"outgoing", unix.SockaddrLinklayer{Ifindex: 7, Protocol: nl.Swap16(unix.ETH_P_IP), Pkttype: unix.PACKET_OUTGOING}, true},
		{"ingress", unix.SockaddrLinklayer{Ifindex: 7, Protocol: nl.Swap16(unix.ETH_P_IP), Pkttype: unix.PACKET_HOST}, false},
		{"another-interface", unix.SockaddrLinklayer{Ifindex: 8, Protocol: nl.Swap16(unix.ETH_P_IP), Pkttype: unix.PACKET_OUTGOING}, false},
		{"arp", unix.SockaddrLinklayer{Ifindex: 7, Protocol: nl.Swap16(unix.ETH_P_ARP), Pkttype: unix.PACKET_OUTGOING}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := linkObservation(packetFixture(formBare), &test.metadata, 7, fixtureKey())
			if !test.accepted {
				if !errors.Is(err, errUnrelatedPacket) {
					t.Fatalf("unrelated metadata = %v, want refusal", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.sequence != 42 {
				t.Fatal("selected actual outgoing ESP sequence was lost")
			}
		})
	}
}
