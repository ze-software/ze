// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- exact traffic selectors
// Related: ts_narrow.go -- request narrowing, answer validation and TS encoding
// Related: child.go -- negotiated selector projection onto the kernel policy
// RFC 7296 Section 3.13.1 -- see rfc/short/rfc7296.md.

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// protocolSelectorPayloads creates the raw TS payloads a peer supplies. It
// deliberately bypasses the outbound encoder so invalid input reaches receive.
func protocolSelectorPayloads(t *testing.T, proto uint8, port uint16) (*wire.PayloadTS, *wire.PayloadTS) {
	t.Helper()
	local, remote := "10.1.0.0/24", "10.2.0.0/24"
	if proto == 58 || proto == 135 {
		local, remote = "2001:db8:1::/64", "2001:db8:2::/64"
	}
	return &wire.PayloadTS{
		TSPayloadType:    wire.PayloadTypeTSi,
		TrafficSelectors: []wire.TrafficSelector{wireSel(t, local, port, port, proto)},
	}, &wire.PayloadTS{
		TSPayloadType:    wire.PayloadTypeTSr,
		TrafficSelectors: []wire.TrafficSelector{wireSel(t, remote, 0, 65535, proto)},
	}
}

// TestTSProtocolRequestsRefuseUndefinedPorts drives the actual responder
// entry point, including a protocol-specific policy that could otherwise hide
// an illegal protocol-zero port during intersection.
func TestTSProtocolRequestsRefuseUndefinedPorts(t *testing.T) {
	for _, proto := range []uint8{0, 47, 50, 51, 89, 112, 255} {
		tsi, tsr := protocolSelectorPayloads(t, proto, 443)
		for _, configured := range []bool{false, true} {
			sa := testSA()
			if configured {
				sa.PeerCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{{
					Number: "1", Protocol: 6,
					LocalPrefix: mustNet(t, "10.2.0.0/24"), RemotePrefix: mustNet(t, "10.1.0.0/24"),
					LocalPort: ipsec.AnyPort(), RemotePort: ipsec.AnyPort(),
				}}
			}
			if err := narrowChildSelectors(sa, tsi, tsr, nil); err == nil {
				t.Errorf("protocol %d configured=%v: responder accepted a specific port", proto, configured)
			}
			if len(sa.NegotiatedPairs) != 0 {
				t.Errorf("protocol %d configured=%v: refused request published %v", proto, configured, sa.NegotiatedPairs)
			}
		}
	}
}

// TestTSProtocolAnswersRefuseUndefinedPorts drives the actual initiator answer
// entry point. A responder must never make an invalid port become installed.
func TestTSProtocolAnswersRefuseUndefinedPorts(t *testing.T) {
	for _, proto := range []uint8{0, 47, 50, 51, 89, 112, 255} {
		tsi, tsr := protocolSelectorPayloads(t, proto, 443)
		sa := testSA()
		if err := recordInitiatorSelectors(sa, tsi, tsr, nil); err == nil {
			t.Errorf("protocol %d: initiator accepted an answered specific port", proto)
		}
		if len(sa.NegotiatedPairs) != 0 {
			t.Errorf("protocol %d: refused answer published %v", proto, sa.NegotiatedPairs)
		}
	}
}

// TestTSProtocolDefinedSelectorsSurviveNegotiation is the accepted-input
// discriminator: both receive entry points and the encoder retain valid fields.
func TestTSProtocolDefinedSelectorsSurviveNegotiation(t *testing.T) {
	for _, tc := range []struct {
		proto uint8
		port  uint16
	}{
		{6, 443}, {17, 443}, {132, 443}, {136, 443},
		{1, 0x0800}, {58, 0x8000},
	} {
		tsi, tsr := protocolSelectorPayloads(t, tc.proto, tc.port)
		responder := testSA()
		if err := narrowChildSelectors(responder, tsi, tsr, nil); err != nil {
			t.Fatalf("protocol %d: responder refused a defined selector: %v", tc.proto, err)
		}
		if len(responder.NegotiatedPairs) != 1 {
			t.Fatalf("protocol %d: responder published %d pairs, want one", tc.proto, len(responder.NegotiatedPairs))
		}
		encodedI, encodedR := pairsToWire(responder.NegotiatedPairs)
		if encodedI == nil || encodedR == nil {
			t.Fatalf("protocol %d: no encoded negotiated selector", tc.proto)
		}
		if got := encodedI.TrafficSelectors[0]; got.StartPort != tc.port || got.EndPort != tc.port {
			t.Errorf("protocol %d: encoded %d-%d, want %d-%d", tc.proto, got.StartPort, got.EndPort, tc.port, tc.port)
		}
		initiator := testSA()
		if err := recordInitiatorSelectors(initiator, encodedI, encodedR, nil); err != nil {
			t.Errorf("protocol %d: initiator refused a defined selector: %v", tc.proto, err)
		}
		if len(initiator.NegotiatedPairs) != 1 {
			t.Errorf("protocol %d: initiator published %d pairs, want one", tc.proto, len(initiator.NegotiatedPairs))
		}
	}
}

// TestTSProtocolDCCPAndMalformedMHRefused refuses exact selectors whose
// kernel projection is unavailable or whose IKE representation is malformed.
func TestTSProtocolDCCPAndMalformedMHRefused(t *testing.T) {
	for _, tc := range []struct {
		proto uint8
		port  uint16
	}{{33, 443}, {135, 5}, {135, 0x0501}, {135, 0xffff}} {
		tsi, tsr := protocolSelectorPayloads(t, tc.proto, tc.port)
		if err := narrowChildSelectors(testSA(), tsi, tsr, nil); err == nil {
			t.Errorf("protocol %d port %#04x: request accepted", tc.proto, tc.port)
		}
		if err := recordInitiatorSelectors(testSA(), tsi, tsr, nil); err == nil {
			t.Errorf("protocol %d port %#04x: answer accepted", tc.proto, tc.port)
		}
	}
}

// TestTSProtocolMHSpecificTypeRefused drives both actual negotiation entries
// on either constrained side and family, preserving ANY as the usable control.
func TestTSProtocolMHSpecificTypeRefused(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, ipv6 := range []bool{false, true} {
			local, far := "10.1.0.0/24", "10.2.0.0/24"
			if ipv6 {
				local, far = "2001:db8:1::/64", "2001:db8:2::/64"
			}
			tsi := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi,
				TrafficSelectors: []wire.TrafficSelector{wireSel(t, local, 1280, 1280, 135)}}
			tsr := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr,
				TrafficSelectors: []wire.TrafficSelector{wireSel(t, far, 0, 65535, 135)}}
			if remote {
				tsi.TrafficSelectors[0].StartPort, tsi.TrafficSelectors[0].EndPort = 0, 65535
				tsr.TrafficSelectors[0].StartPort, tsr.TrafficSelectors[0].EndPort = 1280, 1280
			}
			responder, initiator := testSA(), testSA()
			if err := narrowChildSelectors(responder, tsi, tsr, nil); !errors.Is(err, errTSUnacceptable) {
				t.Errorf("remote=%v ipv6=%v: exact MH request error %v, want typed refusal", remote, ipv6, err)
			}
			if err := recordInitiatorSelectors(initiator, tsi, tsr, nil); !errors.Is(err, errTSUnusable) {
				t.Errorf("remote=%v ipv6=%v: exact MH answer error %v, want typed refusal", remote, ipv6, err)
			}
			if len(responder.NegotiatedPairs) != 0 || len(initiator.NegotiatedPairs) != 0 {
				t.Error("refused exact MH published negotiated pairs")
			}
			tsi.TrafficSelectors[0].StartPort, tsi.TrafficSelectors[0].EndPort = 0, 65535
			tsr.TrafficSelectors[0].StartPort, tsr.TrafficSelectors[0].EndPort = 0, 65535
			if err := narrowChildSelectors(testSA(), tsi, tsr, nil); err != nil {
				t.Errorf("remote=%v ipv6=%v: ANY MH request refused: %v", remote, ipv6, err)
			}
			if err := recordInitiatorSelectors(testSA(), tsi, tsr, nil); err != nil {
				t.Errorf("remote=%v ipv6=%v: ANY MH answer refused: %v", remote, ipv6, err)
			}
		}
	}
}

// TestTSWireAnyPortsForEveryProtocol checks both fields for all protocol
// values: undefined ports and the separate "all ports are allowed" clause.
// RFC requirement: RFC7296-3.13.1-1 positive -- selectorsToWire emits Start Port zero for ANY ports under every IP protocol value, covering protocols with undefined ports and all-ports selectors under defined-port protocols.
// RFC requirement: RFC7296-3.13.1-2 positive -- selectorsToWire emits End Port 65535 for ANY ports under every IP protocol value, covering protocols with undefined ports and all-ports selectors under defined-port protocols.
func TestTSWireAnyPortsForEveryProtocol(t *testing.T) {
	for number := range 256 {
		proto := uint8(number)
		prefix := "10.1.0.0/24"
		if proto == 58 || proto == 135 {
			prefix = "2001:db8:1::/64"
		}
		payload := selectorsToWire([]tsSelector{selPort(t, prefix, ipsec.AnyPort(), proto)}, wire.PayloadTypeTSi)
		if payload == nil {
			t.Fatalf("protocol %d: ANY-port selector was not emitted", proto)
		}
		if len(payload.TrafficSelectors) != 1 {
			t.Fatalf("protocol %d: encoded %d selectors, want one", proto, len(payload.TrafficSelectors))
		}
		got := payload.TrafficSelectors[0]
		if got.StartPort != 0 || got.EndPort != 65535 {
			t.Errorf("protocol %d: encoded %d-%d, want 0-65535", proto, got.StartPort, got.EndPort)
		}
	}
}

// TestTSWireRefusesUndefinedPortValues tests the encoder boundary itself;
// request/config checks cannot conceal a missing final wire guard.
// RFC requirement: RFC7296-3.13.1-1 negative -- selectorsToWire refuses a selector with a specific or OPAQUE port under portless protocols 0, GRE, ESP, AH, OSPF, VRRP and an unknown protocol, so no nonzero Start Port is emitted for one.
// RFC requirement: RFC7296-3.13.1-2 negative -- selectorsToWire refuses a selector with a specific or OPAQUE port under portless protocols 0, GRE, ESP, AH, OSPF, VRRP and an unknown protocol, so no specific End Port is emitted for one.
func TestTSWireRefusesUndefinedPortValues(t *testing.T) {
	for _, proto := range []uint8{0, 47, 50, 51, 89, 112, 255} {
		for _, port := range []ipsec.PortSelector{{Form: ipsec.PortSingle, Port: 443}, {Form: ipsec.PortOpaque}} {
			payload := selectorsToWire([]tsSelector{selPort(t, "10.1.0.0/24", port, proto)}, wire.PayloadTypeTSi)
			if payload != nil {
				t.Errorf("protocol %d port %+v: encoder emitted %+v, want refusal", proto, port, payload.TrafficSelectors)
			}
		}
	}
}

// TestChildMHSelectorProjectsTypeOnce calibrates the raw projection and wire
// value only. Negotiation and Child installation refuse this unprogrammable
// exact selector; this lower-boundary check claims no packet-delivery support.
func TestChildMHSelectorProjectsTypeOnce(t *testing.T) {
	pair := tsPair{
		I: selPort(t, "2001:db8:1::/64", ipsec.PortSelector{Form: ipsec.PortSingle, Port: 0x0500}, 135),
		R: selPort(t, "2001:db8:2::/64", ipsec.PortSelector{Form: ipsec.PortSingle, Port: 0x0700}, 135),
	}
	child := rfc4301ChildWith(&pair)
	projected := policyPair{local: pair.I.Net, remote: pair.R.Net, upperProto: 135,
		localPort: dataplane.ExactPortMatch(0x0500), remotePort: dataplane.ExactPortMatch(0x0700)}
	for _, tc := range []struct {
		dir       dataplane.SADir
		typeValue uint16
	}{{dataplane.SADirOut, 5}, {dataplane.SADirIn, 7}, {dataplane.SADirFwd, 7}} {
		got := childPairPolicy(child, tc.dir, projected)
		if got.SrcPort != dataplane.ExactPortMatch(tc.typeValue) {
			t.Errorf("direction %d: source selector %+v, want exact MH type %d", tc.dir, got.SrcPort, tc.typeValue)
		}
		if !got.DstPort.IsAny() {
			t.Errorf("direction %d: destination selector %+v, want ANY (MH has no second field)", tc.dir, got.DstPort)
		}
	}
	tsi, tsr := pairsToWire([]tsPair{pair})
	if tsi == nil || tsr == nil {
		t.Fatal("the MH pair has no wire representation")
	}
	if got := tsi.TrafficSelectors[0].StartPort; got != 0x0500 {
		t.Errorf("TSi port %#04x, want unchanged high-octet type 0x0500", got)
	}
	if got := tsr.TrafficSelectors[0].StartPort; got != 0x0700 {
		t.Errorf("TSr port %#04x, want unchanged high-octet type 0x0700", got)
	}
}

// TestChildMHDirectInstallRefusedBeforeDataplaneWrite covers a raw Child
// object that bypassed the first-Child constructor as a rekey can do.
func TestChildMHDirectInstallRefusedBeforeDataplaneWrite(t *testing.T) {
	pair := tsPair{
		I: selPort(t, "2001:db8:1::/64", ipsec.AnyPort(), 135),
		R: selPort(t, "2001:db8:2::/64", ipsec.PortSelector{Form: ipsec.PortSingle, Port: 1280}, 135),
	}
	child := rfc4301ChildWith(&pair)
	child.Keys = &crypto.ChildSAKeys{EncryptKeyI: make([]byte, 32), EncryptKeyR: make([]byte, 32),
		IntegKeyI: make([]byte, 32), IntegKeyR: make([]byte, 32)}
	t.Cleanup(child.Clear)
	dp := &mockDP{}
	err := installChildSA(child, testESPGroup().Proposals[0], dp, slogutil.DiscardLogger())
	if !errors.Is(err, errTSUnacceptable) {
		t.Errorf("direct exact MH install error %v, want errTSUnacceptable", err)
	}
	if errors.Is(err, dataplane.ErrNotSupported) || child.ESPInstalled {
		t.Errorf("direct exact MH install published degraded/installed state: error=%v installed=%v", err, child.ESPInstalled)
	}
	if len(dp.sas) != 0 || len(dp.policies) != 0 || len(dp.removed) != 0 {
		t.Errorf("direct refused Child touched dataplane: %d SA installs, %d policies, %d removals", len(dp.sas), len(dp.policies), len(dp.removed))
	}
	if err == nil {
		removeChildSA(child, dp, slogutil.DiscardLogger())
	}
}

// TestChildMHSpecificTypeRefusedBeforeDataplaneWrite bypasses negotiation on
// purpose. A direct Child constructor must refuse the unsupported exact field
// before it can install states, install policies, or publish a degraded Child.
func TestChildMHSpecificTypeRefusedBeforeDataplaneWrite(t *testing.T) {
	for _, remote := range []bool{false, true} {
		sa := testSA()
		sa.IsInitiator = true
		pair := tsPair{
			I: selPort(t, "2001:db8:1::/64", ipsec.AnyPort(), ipsec.MobilityHeaderProtocol),
			R: selPort(t, "2001:db8:2::/64", ipsec.AnyPort(), ipsec.MobilityHeaderProtocol),
		}
		if remote {
			pair.R.Port = ipsec.PortSelector{Form: ipsec.PortSingle, Port: 0x0500}
		} else {
			pair.I.Port = ipsec.PortSelector{Form: ipsec.PortSingle, Port: 0x0500}
		}
		sa.NegotiatedPairs = []tsPair{pair}
		sa.NegotiatedTSi, sa.NegotiatedTSr = pair.I.Net, pair.R.Net
		dp := &mockDP{}
		child, err := createFirstChildSA(sa, testESPGroup(), "192.0.2.1", "192.0.2.2", 0, dp, slogutil.DiscardLogger())
		if child != nil {
			t.Errorf("remote=%v: unsupported exact MH returned a Child (installed=%v)", remote, child.ESPInstalled)
		}
		if !errors.Is(err, errTSUnacceptable) {
			t.Errorf("remote=%v: exact MH error %v, want errTSUnacceptable", remote, err)
		}
		if errors.Is(err, dataplane.ErrNotSupported) {
			t.Errorf("remote=%v: exact MH was reported as a platform without IPsec", remote)
		}
		if len(dp.sas) != 0 || len(dp.policies) != 0 || len(dp.removed) != 0 {
			t.Errorf("remote=%v: refused Child touched dataplane: %d SA installs, %d policies, %d removals", remote, len(dp.sas), len(dp.policies), len(dp.removed))
		}
		if child != nil {
			removeChildSA(child, dp, slogutil.DiscardLogger())
			child.Clear()
		}
	}
}
