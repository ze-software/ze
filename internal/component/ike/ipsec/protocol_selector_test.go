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

// TestMHTypeSelectorValidForProtocol checks the one producer of the MH Type
// encoding rule directly. RFC 4301 Section 4.4.1.1 places the 8-bit MH Type in
// the most significant eight bits of the 16-bit port selector, so a value whose
// low octet is not zero names no Type. 0x0580 sets only the top bit of the low
// octet, so a mask narrower than 0xff cannot pass it; ANY and OPAQUE stay valid.
func TestMHTypeSelectorValidForProtocol(t *testing.T) {
	for _, tc := range []struct {
		port  PortSelector
		valid bool
	}{
		{AnyPort(), true},
		{PortSelector{Form: PortOpaque}, true},
		{PortSelector{Form: PortSingle, Port: 0x0500}, true},
		{PortSelector{Form: PortSingle, Port: 0xff00}, true},
		{PortSelector{Form: PortSingle, Port: 0x0005}, false},
		{PortSelector{Form: PortSingle, Port: 0x0501}, false},
		{PortSelector{Form: PortSingle, Port: 0x0580}, false},
		{PortSelector{Form: PortSingle, Port: 0xffff}, false},
	} {
		if got := tc.port.ValidForProtocol(MobilityHeaderProtocol); got != tc.valid {
			t.Errorf("MH selector %+v: valid %v, want %v", tc.port, got, tc.valid)
		}
	}
}

// TestTrafficSelectorMHMalformedTypeNamesTheEncoding drives the config
// validator with an MH selector whose low octet is set, on either side. Every
// exact MH Type is refused anyway, so the error class cannot tell the two
// refusals apart: this test asserts the encoding refusal, whose reason names
// the high octet, and not the classifier refusal an RFC-valid Type gets.
func TestTrafficSelectorMHMalformedTypeNamesTheEncoding(t *testing.T) {
	for _, port := range []uint16{0x0005, 0x0501, 0x0580, 0xffff} {
		for _, remote := range []bool{false, true} {
			selector := PortSelector{Form: PortSingle, Port: port}
			peer := protocolSelectorPeer(t, MobilityHeaderProtocol, selector, remote)
			err := cfgWith(peer).ValidateTrafficSelectors()
			if !errors.Is(err, ErrTrafficSelectorPolicy) {
				t.Fatalf("MH selector %#04x remote=%v: error %v, want ErrTrafficSelectorPolicy", port, remote, err)
			}
			if !strings.Contains(err.Error(), "high octet") {
				t.Errorf("MH selector %#04x remote=%v: refusal %q does not name the high-octet encoding", port, remote, err)
			}
			if strings.Contains(err.Error(), "names a specific IPv6 Mobility Header Type") {
				t.Errorf("MH selector %#04x remote=%v: malformed value refused as a valid Type: %q", port, remote, err)
			}
		}
	}
}

// TestSPDPolicyMHTypeRefused checks the operator SPD list's answer for an MH
// Type. The list has no MH Type field and protocolCarriesPorts leaves 135 out,
// so a port under protocol 135 is refused on either side, the RFC-valid Type
// encoding 0x0500 included. The ANY entry is the accepted control.
func TestSPDPolicyMHTypeRefused(t *testing.T) {
	for _, remote := range []bool{false, true} {
		p := SPDPolicy{
			Name: "mh", Action: dataplane.SPActionDiscard, Direction: SPDDirOut,
			Order: 1000, Protocol: MobilityHeaderProtocol,
			LocalPrefix: cidr(t, "2001:db8:1::/64"), RemotePrefix: cidr(t, "2001:db8:2::/64"),
			LocalPort: AnyPort(), RemotePort: AnyPort(),
		}
		if err := p.validate(); err != nil {
			t.Fatalf("remote=%v: ANY MH entry refused: %v", remote, err)
		}
		typed := PortSelector{Form: PortSingle, Port: 0x0500}
		if remote {
			p.RemotePort = typed
		} else {
			p.LocalPort = typed
		}
		if err := p.validate(); !errors.Is(err, ErrSPDPolicy) {
			t.Errorf("remote=%v: MH Type entry error %v, want ErrSPDPolicy", remote, err)
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

// TestSPDPortRefusalsAreOperatorErrors pins the shape of the two port refusals
// SPDPolicy.validate answers: each is an operator error that keeps ErrSPDPolicy, the
// port-needs-protocol refusal names every protocol whose ports Ze can select (UDP-Lite
// included, as protocolCarriesPorts accepts it), and both fixes name the entry the
// same way, unquoted, as the operator types it. Method: a port under ICMP (1) and a
// port under DCCP (33) through validate, reading Problem, Why and Fix.
func TestSPDPortRefusalsAreOperatorErrors(t *testing.T) {
	refusal := func(proto uint8) *opererr.Error {
		t.Helper()
		p := SPDPolicy{
			Name: "ports", Action: dataplane.SPActionDiscard, Direction: SPDDirOut,
			Order: 1000, Protocol: proto,
			LocalPrefix: cidr(t, "10.1.0.0/24"), RemotePrefix: cidr(t, "10.2.0.0/24"),
			LocalPort: PortSelector{Form: PortSingle, Port: 443}, RemotePort: AnyPort(),
		}
		err := p.validate()
		if !errors.Is(err, ErrSPDPolicy) {
			t.Fatalf("protocol %d: error %v, want ErrSPDPolicy", proto, err)
		}
		var oe *opererr.Error
		if !errors.As(err, &oe) {
			t.Fatalf("protocol %d: error %v is not an operator error", proto, err)
		}
		return oe
	}

	icmp := refusal(1)
	if !strings.Contains(icmp.Problem(), "a port selector needs a protocol that carries ports, and protocol is 1") {
		t.Errorf("ICMP problem = %q, want the port-needs-protocol refusal", icmp.Problem())
	}
	for _, named := range []string{"6 (TCP)", "17 (UDP)", "132 (SCTP)", "136 (UDP-Lite)"} {
		if !strings.Contains(icmp.Why(), named) {
			t.Errorf("ICMP why = %q, want it to name %s", icmp.Why(), named)
		}
	}
	if want := "set vpn ipsec policy ports protocol <6|17|132|136>"; icmp.Fix() != want {
		t.Errorf("ICMP fix = %q, want %q", icmp.Fix(), want)
	}

	dccp := refusal(protoDCCP)
	if want := "set vpn ipsec policy ports local port any; set vpn ipsec policy ports remote port any"; dccp.Fix() != want {
		t.Errorf("DCCP fix = %q, want %q", dccp.Fix(), want)
	}
}
