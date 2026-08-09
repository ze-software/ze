// rfc-test-change-approved: 2026-07-31 owner standing approval for
// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only. Every tag in this file is
// NEW; the edits that build it never relax an existing proof.

package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// tsPayload builds a TS payload the way a peer would send it.
func tsPayload(t *testing.T, payloadType uint8, cidrs ...string) *wire.PayloadTS {
	t.Helper()
	sels := make([]wire.TrafficSelector, 0, len(cidrs))
	for _, c := range cidrs {
		sels = append(sels, wireSel(t, c, 0, 65535, 0))
	}
	return &wire.PayloadTS{TSPayloadType: payloadType, TrafficSelectors: sels}
}

// transportPeer is the fixture peer: transport mode, one local and one remote address.
// RFC 7296 Section 2.23.1 pins a transport-mode proposal to exactly this pair.
func transportPeer() ipsec.SiteToSitePeer {
	return ipsec.SiteToSitePeer{
		Name:          "t",
		LocalAddress:  "10.0.0.1",
		RemoteAddress: "10.0.0.2",
		Mode:          dataplane.ModeTransport,
	}
}

// TestTSUnacceptableIsSentWhenNothingIsAcceptable drives RFC 7296 Section 2.9's first
// bullet through the production entry point both responder producers call.
//
// VALIDATES: a proposal disjoint from policy makes the responder refuse with the
// TS_UNACCEPTABLE notify type.
// PREVENTS: the responder answering a disjoint proposal at all, and answering it with the
// generic NO_PROPOSAL_CHOSEN, which tells the peer nothing about what it got wrong.
func TestTSUnacceptableIsSentWhenNothingIsAcceptable(t *testing.T) {
	policyPeer := ipsec.SiteToSitePeer{
		Name: "p",
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{{
			Number:       "1",
			LocalPrefix:  mustNet(t, "10.0.0.0/8"),
			LocalPort:    ipsec.AnyPort(),
			RemotePrefix: mustNet(t, "10.0.0.0/8"),
			RemotePort:   ipsec.AnyPort(),
		}},
	}

	sa := &SA{PeerCfg: policyPeer, IsInitiator: false}
	// The peer proposes 192.168/16, which the policy does not cover at all.
	err := narrowChildSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, "192.168.0.0/16"),
		tsPayload(t, wire.PayloadTypeTSr, "192.168.1.0/24"),
		nil)

	// RFC requirement: RFC7296-2.9-1 positive -- "If the responder's policy does not allow
	// it to accept any part of the proposed Traffic Selectors, it responds with a
	// TS_UNACCEPTABLE Notify message" (RFC 7296 S2.9, rfc/full/rfc7296.txt:2426-2428), and
	// the responder never widens (rfc/full/rfc7296.txt:2393-2395). narrowChildSelectors
	// refuses, and notifyForRefusal maps that refusal to the named notify type.
	if err == nil {
		t.Fatal("a proposal disjoint from policy was accepted; the responder would answer with selectors it never agreed to")
	}
	if got := notifyForRefusal(err); got != wire.NotifyTSUnacceptable {
		t.Errorf("notify for a refused traffic selector = %d (%s), want TS_UNACCEPTABLE (%d)",
			got, wire.NotifyTypeName(got), wire.NotifyTSUnacceptable)
	}
	if sa.NegotiatedPairs != nil {
		t.Error("a refused proposal still recorded negotiated selectors; a Child SA could be installed from them")
	}

	// RFC requirement: RFC7296-2.9-1 negative -- the discriminator. An OVERLAPPING
	// proposal is ACCEPTED and draws no TS_UNACCEPTABLE, so the refusal above is a
	// decision about that proposal rather than an unconditional answer.
	ok := &SA{PeerCfg: policyPeer, IsInitiator: false}
	if err := narrowChildSelectors(ok,
		tsPayload(t, wire.PayloadTypeTSi, "10.1.0.0/16"),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.0.0/16"),
		nil); err != nil {
		t.Fatalf("an overlapping proposal was refused (%v); TS_UNACCEPTABLE is unconditional", err)
	}
	if ok.NegotiatedTSi.String() != "10.1.0.0/16" {
		t.Errorf("accepted TSi = %v, want the narrowed 10.1.0.0/16", ok.NegotiatedTSi)
	}
}

// TestTransportModeUsesExactlyOneAddress drives all three RFC 7296 Section 2.23.1 MUSTs.
//
// VALIDATES: a transport-mode proposal carries exactly one IP address in every TSi and
// TSr entry, TSi matches the IKE SA source and TSr the IKE SA destination, and several
// selectors sharing that one address are still permitted.
// PREVENTS: a transport-mode proposal carrying a range, naming an address the IKE SA does
// not run on, or being rejected merely for carrying more than one selector.
func TestTransportModeUsesExactlyOneAddress(t *testing.T) {
	sa := &SA{PeerCfg: transportPeer(), IsInitiator: true}
	// Two configured selectors differing only in port: Section 2.23.1 permits that.
	sa.PeerCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{
		{Number: "1", LocalPrefix: mustNet(t, "10.0.0.1/32"), RemotePrefix: mustNet(t, "10.0.0.2/32"),
			LocalPort: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 179}, RemotePort: ipsec.AnyPort(), Protocol: 6},
		{Number: "2", LocalPrefix: mustNet(t, "10.0.0.1/32"), RemotePrefix: mustNet(t, "10.0.0.2/32"),
			LocalPort: ipsec.AnyPort(), RemotePort: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 179}, Protocol: 6},
	}

	tsi, tsr := proposeChildTSPayloads(sa)
	if tsi == nil || tsr == nil {
		t.Fatal("transport-mode proposal produced no TS payloads")
	}
	if len(tsi.TrafficSelectors) == 0 || len(tsr.TrafficSelectors) == 0 {
		t.Fatal("transport-mode proposal produced empty selector lists; the sweeps below would assert nothing")
	}

	// RFC requirement: RFC7296-2.23.1-1 positive -- "For transport mode, it MUST use
	// exactly one IP address in the TSi and TSr payloads" (RFC 7296 S2.23.1,
	// rfc/full/rfc7296.txt:3712-3714). Every entry of both payloads spans one address.
	for _, side := range []struct {
		name string
		p    *wire.PayloadTS
	}{{"TSi", tsi}, {"TSr", tsr}} {
		for i, s := range side.p.TrafficSelectors {
			if !bytesEqual(s.StartAddress, s.EndAddress) {
				t.Errorf("%s[%d] spans %v-%v, want exactly one IP address", side.name, i, s.StartAddress, s.EndAddress)
			}
		}
	}
	// RFC requirement: RFC7296-2.23.1-1 negative -- the discriminator. TWO selectors are
	// ACCEPTED, because S2.23.1 allows "It can have multiple Traffic Selectors if it has,
	// for example, multiple port ranges that it wants to negotiate"
	// (rfc/full/rfc7296.txt:3715-3718) provided each carries the single address. The
	// constraint is one ADDRESS, never one SELECTOR, so a blanket refusal of multiple
	// selectors would be wrong and this assertion catches it.
	if len(tsi.TrafficSelectors) != 2 {
		t.Errorf("transport-mode TSi carried %d selectors, want 2; multiple selectors sharing one address are permitted (rfc/full/rfc7296.txt:3715-3718)",
			len(tsi.TrafficSelectors))
	}

	// RFC requirement: RFC7296-2.23.1-2 positive -- "The TSi entries MUST have exactly one
	// IP address, and that MUST match the source address of the IKE SA" (RFC 7296 S2.23.1,
	// rfc/full/rfc7296.txt:3819-3820).
	wantSrc := net.ParseIP("10.0.0.1").To4()
	for i, s := range tsi.TrafficSelectors {
		if !bytesEqual(s.StartAddress, wantSrc) {
			t.Errorf("TSi[%d] address = %v, want the IKE SA source 10.0.0.1", i, net.IP(s.StartAddress))
		}
	}

	// RFC requirement: RFC7296-2.23.1-3 positive -- "The TSr entries MUST have exactly one
	// IP address, and that MUST match the destination address of the IKE SA" (RFC 7296
	// S2.23.1, rfc/full/rfc7296.txt:3822-3823).
	wantDst := net.ParseIP("10.0.0.2").To4()
	for i, s := range tsr.TrafficSelectors {
		if !bytesEqual(s.StartAddress, wantDst) {
			t.Errorf("TSr[%d] address = %v, want the IKE SA destination 10.0.0.2", i, net.IP(s.StartAddress))
		}
	}

	// The discriminator for -2 and -3: a peer configured with selectors naming OTHER
	// addresses still proposes the IKE SA's own pair, so the addresses come from the SA
	// rather than being echoed from config.
	strayCfg := transportPeer()
	strayCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{{
		Number: "1", LocalPrefix: mustNet(t, "192.168.5.5/32"), RemotePrefix: mustNet(t, "192.168.6.6/32"),
		LocalPort: ipsec.AnyPort(), RemotePort: ipsec.AnyPort(),
	}}
	strayTSi, strayTSr := proposeChildTSPayloads(&SA{PeerCfg: strayCfg, IsInitiator: true})

	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only. This ADDS the two missing
	// negative polarities; it relaxes nothing.
	//
	// RFC requirement: RFC7296-2.23.1-2 negative -- the discriminator. A configured TSi
	// address that DIFFERS from the IKE SA source never reaches the wire. Without this the
	// positive above could pass on a fixture that merely happened to agree with config.
	if bytesEqual(strayTSi.TrafficSelectors[0].StartAddress, net.ParseIP("192.168.5.5").To4()) {
		t.Error("TSi carried the configured 192.168.5.5, which is not the IKE SA source; S2.23.1 requires the two to match")
	}
	// RFC requirement: RFC7296-2.23.1-3 negative -- the same discriminator for TSr: a
	// configured remote address differing from the IKE SA destination never reaches the wire.
	if bytesEqual(strayTSr.TrafficSelectors[0].StartAddress, net.ParseIP("192.168.6.6").To4()) {
		t.Error("TSr carried the configured 192.168.6.6, which is not the IKE SA destination; S2.23.1 requires the two to match")
	}
	if !bytesEqual(strayTSi.TrafficSelectors[0].StartAddress, wantSrc) {
		t.Errorf("TSi took its address from config (%v) instead of the IKE SA source",
			net.IP(strayTSi.TrafficSelectors[0].StartAddress))
	}
	if !bytesEqual(strayTSr.TrafficSelectors[0].StartAddress, wantDst) {
		t.Errorf("TSr took its address from config (%v) instead of the IKE SA destination",
			net.IP(strayTSr.TrafficSelectors[0].StartAddress))
	}

	// A responder narrowing under transport mode refuses a proposal wider than a host.
	wide := &SA{PeerCfg: transportPeer(), IsInitiator: false}
	wide.PeerRequestedTransport = true
	if err := narrowChildSelectors(wide,
		tsPayload(t, wire.PayloadTypeTSi, "10.1.0.0/16"),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.0.0/16"),
		nil); err == nil {
		t.Error("a transport-mode exchange accepted a /16 selector; RFC 7296 S2.23.1 requires exactly one IP address")
	}
}

// TestUseTransportModeNotifyIsEchoedOnlyWhenAccepted drives RFC 7296 Section 1.3.1's
// first MUST.
//
// VALIDATES: a request Ze accepts yields UseTransportMode true (the response then carries
// the notification), and a request Ze declines yields false and a tunnel-mode Child SA.
// PREVENTS: the responder echoing USE_TRANSPORT_MODE unconditionally, which would make
// acceptance an echo rather than a decision.
func TestUseTransportModeNotifyIsEchoedOnlyWhenAccepted(t *testing.T) {
	// RFC requirement: RFC7296-1.3.1-1 positive -- "If the request is accepted, the
	// response MUST also include a notification of type USE_TRANSPORT_MODE" (RFC 7296
	// S1.3.1, rfc/full/rfc7296.txt:802-806). Acceptance sets the flag buildAuthResponse
	// reads to append that notification.
	accept := &SA{PeerCfg: transportPeer()}
	accept.PeerRequestedTransport = true
	decideResponderTransportMode(accept)
	if !accept.UseTransportMode {
		t.Error("a transport-mode request to a transport-mode peer was declined; the response would omit USE_TRANSPORT_MODE")
	}

	// RFC requirement: RFC7296-1.3.1-1 negative -- the discriminator. A peer configured for
	// TUNNEL mode DECLINES the request: no notification is echoed and the Child SA is
	// established in tunnel mode, which is the outcome S1.3.1 names for a decline. This
	// separates "accepted" from "echoed whatever the peer asked for".
	decline := &SA{PeerCfg: ipsec.SiteToSitePeer{Name: "d", Mode: dataplane.ModeTunnel}}
	decline.PeerRequestedTransport = true
	decideResponderTransportMode(decline)
	if decline.UseTransportMode {
		t.Error("a transport-mode request was accepted for a tunnel-mode peer; acceptance is an echo rather than a decision")
	}

	// A Child SA built from the declining SA is installed in tunnel mode.
	child := &ChildSA{Mode: modeTunnel}
	if decline.UseTransportMode {
		child.Mode = modeTransport
	}
	if child.Mode != modeTunnel {
		t.Errorf("declined Child SA mode = %d, want tunnel (%d)", child.Mode, modeTunnel)
	}
}

// TestTransportRequiredDeletesTheSAOnDecline drives RFC 7296 Section 1.3.1's second MUST.
//
// VALIDATES: with transport-required set, a response lacking USE_TRANSPORT_MODE tears the
// SA down; without it, the same response keeps a working tunnel-mode SA.
// PREVENTS: a silent downgrade to tunnel mode when the operator said tunnel is
// unacceptable, and an unconditional teardown that would break every peer whose
// implementation lacks transport mode.
func TestTransportRequiredDeletesTheSAOnDecline(t *testing.T) {
	// RFC requirement: RFC7296-1.3.1-2 positive -- "If the responder declines the request,
	// the Child SA will be established in tunnel mode. If this is unacceptable to the
	// initiator, the initiator MUST delete the SA" (RFC 7296 S1.3.1,
	// rfc/full/rfc7296.txt:806-809). transport-required is the operator saying it is
	// unacceptable, and the response carrying no notification is the decline.
	required := transportPeer()
	required.TransportRequired = true
	sa := &SA{PeerCfg: required}
	if !recordInitiatorTransportMode(sa, false) {
		t.Error("a declined transport-mode request with transport-required set did not delete the SA")
	}
	if sa.UseTransportMode {
		t.Error("a declined request still recorded transport mode; the Child SA would be installed in the wrong mode")
	}

	// RFC requirement: RFC7296-1.3.1-2 negative -- the discriminator. WITHOUT
	// transport-required the same decline keeps the SA, in tunnel mode. This proves the
	// delete is conditional on the operator's statement rather than on the decline alone.
	optional := &SA{PeerCfg: transportPeer()}
	if recordInitiatorTransportMode(optional, false) {
		t.Error("a declined request deleted the SA with transport-required unset; every peer without transport mode would lose its tunnel")
	}
	if optional.UseTransportMode {
		t.Error("a declined request recorded transport mode")
	}

	// An ACCEPTED request keeps the SA and records transport mode.
	accepted := &SA{PeerCfg: required}
	if recordInitiatorTransportMode(accepted, true) {
		t.Error("an accepted transport-mode request deleted the SA")
	}
	if !accepted.UseTransportMode {
		t.Error("an accepted request did not record transport mode")
	}

	// A tunnel-mode peer is unaffected by a response that carries no notification.
	tunnel := &SA{PeerCfg: ipsec.SiteToSitePeer{Name: "t", Mode: dataplane.ModeTunnel, TransportRequired: false}}
	if recordInitiatorTransportMode(tunnel, false) {
		t.Error("a tunnel-mode peer was deleted for a response with no USE_TRANSPORT_MODE")
	}
}

// TestTransportModeInstallCarriesNoTunnelEndpoints proves the dataplane half of transport
// mode: the mode reaches every installed state and policy, and a transport-mode policy
// carries no tunnel endpoints.
//
// VALIDATES: a transport-mode Child SA installs four dataplane objects, all carrying
// ModeTransport, with the two policies carrying neither tunnel endpoint.
// PREVENTS: the mode reaching only the policies and not the states, and the install
// failing because tunnelEndpoints refuses leftover endpoints.
func TestTransportModeInstallCarriesNoTunnelEndpoints(t *testing.T) {
	sa := testSA()
	sa.IsInitiator = true
	sa.PeerCfg.Mode = dataplane.ModeTransport
	sa.UseTransportMode = true
	sa.NegotiatedTSi = mustNet(t, "10.0.0.1/32")
	sa.NegotiatedTSr = mustNet(t, "10.0.0.2/32")

	dp := &mockDP{}
	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 7, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	defer child.Clear()

	if child.Mode != modeTransport {
		t.Fatalf("child mode = %d, want transport (%d)", child.Mode, modeTransport)
	}
	if len(dp.sas) != 2 {
		t.Fatalf("installed SAs = %d, want 2; the sweep below would assert nothing", len(dp.sas))
	}
	if len(dp.policies) != 2 {
		t.Fatalf("installed policies = %d, want 2; the sweep below would assert nothing", len(dp.policies))
	}
	for i, s := range dp.sas {
		if s.Mode != modeTransport {
			t.Errorf("SA[%d] mode = %d, want transport (%d)", i, s.Mode, modeTransport)
		}
	}
	for i, p := range dp.policies {
		if p.Mode != modeTransport {
			t.Errorf("policy[%d] mode = %d, want transport (%d)", i, p.Mode, modeTransport)
		}
		// dataplane.tunnelEndpoints REFUSES a non-tunnel policy carrying endpoints, so
		// leaving them set would fail the install rather than be ignored.
		if len(p.TunnelSrc) != 0 || len(p.TunnelDst) != 0 {
			t.Errorf("transport policy[%d] carries tunnel endpoints src=%v dst=%v; RFC 4301 S4.4.1.2 leaves them unused and the dataplane refuses them",
				i, p.TunnelSrc, p.TunnelDst)
		}
	}
}

// TestAuthResponsePayloadsCarryTheNarrowedSelectors closes the gap between "the narrowing
// engine is correct" and "the response actually carries its result".
//
// VALIDATES: buildChildSAResponsePayloads emits the selectors narrowSelectors produced,
// and refuses to build a response when nothing was negotiated.
// PREVENTS: the responder reverting to anyChildTSPayloads, which answered every exchange
// with 0.0.0.0-255.255.255.255 -- a strict superset of any narrower proposal, and the
// widening this package exists to remove.
func TestAuthResponsePayloadsCarryTheNarrowedSelectors(t *testing.T) {
	policyPeer := ipsec.SiteToSitePeer{
		Name: "p",
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{{
			Number:       "1",
			LocalPrefix:  mustNet(t, "10.0.0.0/8"),
			LocalPort:    ipsec.AnyPort(),
			RemotePrefix: mustNet(t, "10.0.0.0/8"),
			RemotePort:   ipsec.AnyPort(),
		}},
	}
	sa := &SA{PeerCfg: policyPeer, IsInitiator: false}
	sa.ESPGroup = testESPGroup()
	sa.ChildProposalNum = 1

	if err := narrowChildSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, "10.1.0.0/16"),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.0.0/16"),
		nil); err != nil {
		t.Fatalf("narrowChildSelectors: %v", err)
	}

	// RFC requirement: RFC7296-2.9-2 positive -- the RESPONSE carries the narrowed subset
	// that includes the initiator's first choices, not a wildcard (RFC 7296 S2.9,
	// rfc/full/rfc7296.txt:2434-2436). rfc/full/rfc7296.txt:2393-2395 forbids widening,
	// and a wildcard answer is a strict superset of this proposal.
	_, _, respTSi, respTSr, err := buildChildSAResponsePayloads(sa)
	if err != nil {
		t.Fatalf("buildChildSAResponsePayloads: %v", err)
	}
	if respTSi == nil || respTSr == nil {
		t.Fatal("response carried no TS payloads")
	}
	if len(respTSi.TrafficSelectors) != 1 || len(respTSr.TrafficSelectors) != 1 {
		t.Fatalf("response carried %d TSi and %d TSr selectors, want 1 each",
			len(respTSi.TrafficSelectors), len(respTSr.TrafficSelectors))
	}
	gotI := respTSi.TrafficSelectors[0]
	gotR := respTSr.TrafficSelectors[0]
	wantI := wireSel(t, "10.1.0.0/16", 0, 65535, 0)
	wantR := wireSel(t, "10.2.0.0/16", 0, 65535, 0)
	if !bytesEqual(gotI.StartAddress, wantI.StartAddress) || !bytesEqual(gotI.EndAddress, wantI.EndAddress) {
		t.Errorf("response TSi = %v-%v, want the narrowed 10.1.0.0/16 (%v-%v); a wildcard here widens the proposal",
			net.IP(gotI.StartAddress), net.IP(gotI.EndAddress), net.IP(wantI.StartAddress), net.IP(wantI.EndAddress))
	}
	if !bytesEqual(gotR.StartAddress, wantR.StartAddress) || !bytesEqual(gotR.EndAddress, wantR.EndAddress) {
		t.Errorf("response TSr = %v-%v, want the narrowed 10.2.0.0/16 (%v-%v)",
			net.IP(gotR.StartAddress), net.IP(gotR.EndAddress), net.IP(wantR.StartAddress), net.IP(wantR.EndAddress))
	}

	// The discriminator: with nothing negotiated, the builder REFUSES rather than
	// inventing a wildcard.
	empty := &SA{PeerCfg: policyPeer, ESPGroup: testESPGroup(), ChildProposalNum: 1}
	if _, _, _, _, err := buildChildSAResponsePayloads(empty); err == nil {
		t.Error("a response was built with no negotiated selectors; the builder invents traffic the peer never proposed")
	}
}

// TestEAPResponderNarrowsToo proves the SECOND responder producer narrows.
//
// VALIDATES: startResponderEAP refuses an unacceptable proposal instead of stashing it,
// and records the narrowed result for an acceptable one.
// PREVENTS: the EAP path keeping the old behavior while the direct path narrows, which
// no test of buildAuthResponse alone can catch.
func TestEAPResponderNarrowsToo(t *testing.T) {
	policyPeer := ipsec.SiteToSitePeer{
		Name: "p",
		Auth: ipsec.AuthConfig{Mode: ipsec.AuthEAPMSCHAPv2},
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{{
			Number:       "1",
			LocalPrefix:  mustNet(t, "10.0.0.0/8"),
			LocalPort:    ipsec.AnyPort(),
			RemotePrefix: mustNet(t, "10.0.0.0/8"),
			RemotePort:   ipsec.AnyPort(),
		}},
	}
	ps := &PeerSession{peerName: "p"}

	// A disjoint proposal kills the SA on the EAP path too.
	bad := &SA{PeerName: "p", PeerCfg: policyPeer}
	ps.startResponderEAP(bad, 1, nil,
		tsPayload(t, wire.PayloadTypeTSi, "192.168.0.0/16"),
		tsPayload(t, wire.PayloadTypeTSr, "192.168.1.0/24"),
		nil, nil, slogutil.DiscardLogger())
	if bad.State != StateDead {
		t.Errorf("EAP responder state = %v after an unacceptable proposal, want dead; the EAP path does not narrow", bad.State)
	}
	if bad.NegotiatedPairs != nil {
		t.Error("the EAP path stashed selectors it should have refused")
	}

	// An acceptable proposal is narrowed and recorded before the EAP method runs.
	good := &SA{PeerName: "p", PeerCfg: policyPeer}
	ps.startResponderEAP(good, 1, nil,
		tsPayload(t, wire.PayloadTypeTSi, "10.1.0.0/16"),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.0.0/16"),
		nil, nil, slogutil.DiscardLogger())
	if good.NegotiatedTSi == nil {
		t.Fatal("the EAP path recorded no selectors for an acceptable proposal")
	}
	if good.NegotiatedTSi.String() != "10.1.0.0/16" {
		t.Errorf("EAP path recorded TSi %v, want the narrowed 10.1.0.0/16", good.NegotiatedTSi)
	}
}

// TestRekeyedChildInheritsModeAndSelectors pins the two fields a replacement Child SA must
// carry forward. It has the same shape as the UDPEncap inheritance the same constructor
// already documents: createFirstChildSA is their only other writer, so a replacement that
// drops them silently changes what the tunnel does.
//
// VALIDATES: a rekeyed Child SA keeps the negotiated mode and the negotiated selector set.
// PREVENTS: a transport-mode tunnel silently becoming tunnel-mode at its first rekey, and
// the NEXT rekey losing the RFC 7296 Section 2.9.2 floor because the scope in use was not
// carried forward.
func TestRekeyedChildInheritsModeAndSelectors(t *testing.T) {
	old := &ChildSA{
		InboundSPI: 1, OutboundSPI: 2,
		LocalAddr: net.ParseIP("10.0.0.1"), RemoteAddr: net.ParseIP("10.0.0.2"),
		Mode: modeTransport,
		Selectors: []tsPair{{
			I: tsSelector{Net: mustNet(t, "10.1.0.0/16"), Port: ipsec.AnyPort()},
			R: tsSelector{Net: mustNet(t, "10.2.0.0/16"), Port: ipsec.AnyPort()},
		}},
	}

	fresh := newRekeyedChild(old, 3, 4, nil, true)
	if fresh.Mode != modeTransport {
		t.Errorf("rekeyed child mode = %d, want transport (%d); a transport tunnel would become a tunnel-mode one at its first rekey",
			fresh.Mode, modeTransport)
	}
	if len(fresh.Selectors) != 1 {
		t.Fatalf("rekeyed child carried %d selectors, want 1; the next rekey would have no RFC 7296 S2.9.2 floor", len(fresh.Selectors))
	}
	if fresh.Selectors[0].I.Net.String() != "10.1.0.0/16" {
		t.Errorf("rekeyed child TSi selector = %v, want the scope in use 10.1.0.0/16", fresh.Selectors[0].I.Net)
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
