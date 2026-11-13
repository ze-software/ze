// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact traffic selectors
// Related: traffic_selector.go -- config validation before a proposal is encoded
// RFC 7296 Section 3.13.1 -- see rfc/short/rfc7296.md.

package ipsec

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/opererr"
)

// protocolSelectorPeer builds one configured selector, changing only the side
// under test. The other side has ANY ports, so it cannot explain a refusal.
func protocolSelectorPeer(t *testing.T, proto uint8, port PortSelector, remote bool) SiteToSitePeer {
	t.Helper()
	localPrefix, remotePrefix := "10.1.0.0/24", "10.2.0.0/24"
	if proto == 58 || proto == 135 {
		localPrefix, remotePrefix = "2001:db8:1::/64", "2001:db8:2::/64"
	}
	selector := TrafficSelectorPolicy{
		Number: "1", Protocol: proto,
		LocalPrefix: cidr(t, localPrefix), RemotePrefix: cidr(t, remotePrefix),
		LocalPort: AnyPort(), RemotePort: AnyPort(),
	}
	if remote {
		selector.RemotePort = port
	} else {
		selector.LocalPort = port
	}
	return SiteToSitePeer{
		Name: "selector-peer", Mode: dataplane.ModeTunnel,
		TrafficSelectors: []TrafficSelectorPolicy{selector},
	}
}

// TestTrafficSelectorPortlessProtocolsRefused drives the complete typed config
// validator with a specific port on either side under protocols with no ports.
// A matching ANY-port control must still pass for every protocol.
func TestTrafficSelectorPortlessProtocolsRefused(t *testing.T) {
	for _, proto := range []uint8{0, 47, 50, 51, 89, 112, 255} {
		for _, remote := range []bool{false, true} {
			peer := protocolSelectorPeer(t, proto, PortSelector{Form: PortSingle, Port: 443}, remote)
			err := cfgWith(peer).ValidateTrafficSelectors()
			if !errors.Is(err, ErrTrafficSelectorPolicy) {
				t.Errorf("protocol %d remote=%v: error %v, want ErrTrafficSelectorPolicy", proto, remote, err)
			}
			peer = protocolSelectorPeer(t, proto, AnyPort(), remote)
			if err := cfgWith(peer).ValidateTrafficSelectors(); err != nil {
				t.Errorf("protocol %d remote=%v: ANY ports refused: %v", proto, remote, err)
			}
		}
	}
}

// TestTrafficSelectorDefinedSelectorsStayProgrammable prevents a refusal that
// loses valid transport ports or programmable ICMP selectors.
func TestTrafficSelectorDefinedSelectorsStayProgrammable(t *testing.T) {
	for _, tc := range []struct {
		proto uint8
		port  uint16
	}{
		{6, 443}, {17, 443}, {132, 443}, {136, 443},
		{1, 0x0800}, {58, 0x8000},
	} {
		for _, remote := range []bool{false, true} {
			peer := protocolSelectorPeer(t, tc.proto, PortSelector{Form: PortSingle, Port: tc.port}, remote)
			if err := cfgWith(peer).ValidateTrafficSelectors(); err != nil {
				t.Errorf("protocol %d port %#04x remote=%v refused: %v", tc.proto, tc.port, remote, err)
			}
		}
	}
}

// TestTrafficSelectorDCCPSpecificPortRefused distinguishes a protocol that
// defines ports from one whose ports Linux XFRM can select. ANY remains usable.
func TestTrafficSelectorDCCPSpecificPortRefused(t *testing.T) {
	peer := protocolSelectorPeer(t, 33, PortSelector{Form: PortSingle, Port: 443}, false)
	err := cfgWith(peer).ValidateTrafficSelectors()
	if !errors.Is(err, ErrTrafficSelectorPolicy) {
		t.Fatalf("specific DCCP port error %v, want ErrTrafficSelectorPolicy", err)
	}
	if !strings.Contains(err.Error(), "DCCP") {
		t.Errorf("refusal %q does not identify the unsupported DCCP selector", err)
	}
	peer = protocolSelectorPeer(t, 33, AnyPort(), false)
	if err := cfgWith(peer).ValidateTrafficSelectors(); err != nil {
		t.Errorf("DCCP with ANY ports refused: %v", err)
	}
}

// TestTrafficSelectorMHRequiresWireTypeEncoding rejects nonzero low octets:
// the configuration's number is the IKE selector, not the bare kernel type.
func TestTrafficSelectorMHRequiresWireTypeEncoding(t *testing.T) {
	for _, port := range []uint16{5, 0x0501, 0xffff} {
		peer := protocolSelectorPeer(t, 135, PortSelector{Form: PortSingle, Port: port}, false)
		if err := cfgWith(peer).ValidateTrafficSelectors(); !errors.Is(err, ErrTrafficSelectorPolicy) {
			t.Errorf("MH selector %#04x error %v, want ErrTrafficSelectorPolicy", port, err)
		}
	}
}

// TestTrafficSelectorMHSpecificTypeRefused distinguishes RFC-valid wire
// encoding from the absent exact receive classifier, on each side and family.
func TestTrafficSelectorMHSpecificTypeRefused(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, ipv6 := range []bool{false, true} {
			port := PortSelector{Form: PortSingle, Port: 0x0500}
			if !port.ValidForProtocol(MobilityHeaderProtocol) {
				t.Fatal("RFC-valid high-octet MH Type5 was confused with an invalid encoding")
			}
			peer := protocolSelectorPeer(t, MobilityHeaderProtocol, port, remote)
			if !ipv6 {
				peer.TrafficSelectors[0].LocalPrefix = cidr(t, "10.1.0.0/24")
				peer.TrafficSelectors[0].RemotePrefix = cidr(t, "10.2.0.0/24")
			}
			err := cfgWith(peer).ValidateTrafficSelectors()
			if !errors.Is(err, ErrTrafficSelectorPolicy) {
				t.Fatalf("remote=%v ipv6=%v: exact MH error %v, want typed selector refusal", remote, ipv6, err)
			}
			if _, ok := errors.AsType[*opererr.Error](err); !ok {
				t.Errorf("exact MH error %T has no operator repair", err)
			}
			if !strings.Contains(err.Error(), "Mobility Header") {
				t.Errorf("exact MH refusal does not name the unsupported field: %v", err)
			}
			peer.TrafficSelectors[0].LocalPort = AnyPort()
			peer.TrafficSelectors[0].RemotePort = AnyPort()
			if err := cfgWith(peer).ValidateTrafficSelectors(); err != nil {
				t.Errorf("remote=%v ipv6=%v: ANY MH was refused: %v", remote, ipv6, err)
			}
		}
	}
}

// TestSPDDefinedPortProtocols checks the other caller of the protocol-port
// classification: Linux can select UDP-Lite ports, but cannot select DCCP ports.
func TestSPDDefinedPortProtocols(t *testing.T) {
	for _, tc := range []struct {
		proto     uint8
		wantError bool
	}{{6, false}, {17, false}, {132, false}, {136, false}, {33, true}} {
		p := SPDPolicy{
			Name: "ports", Action: dataplane.SPActionDiscard, Direction: SPDDirOut,
			Order: 1000, Protocol: tc.proto,
			LocalPrefix: cidr(t, "10.1.0.0/24"), RemotePrefix: cidr(t, "10.2.0.0/24"),
			LocalPort: PortSelector{Form: PortSingle, Port: 443}, RemotePort: AnyPort(),
		}
		err := p.validate()
		if (err != nil) != tc.wantError {
			t.Errorf("protocol %d: error %v, wantError %v", tc.proto, err, tc.wantError)
		}
	}
}
