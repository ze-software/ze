package engine

import (
	"errors"
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
	// Two configured selectors differing only in local port: Section 2.23.1 permits that.
	// They share the remote selector, so TSi {179, 180} with TSr {any} is every
	// combination the two entries name, which RFC 7296 Section 3.13 needs of one TS
	// payload pair ("a packet matches a given TSi/TSr if it matches at least one of the
	// individual selectors in TSi, and at least one of the individual selectors in TSr").
	sa.PeerCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{
		{Number: "1", LocalPrefix: mustNet(t, "10.0.0.1/32"), RemotePrefix: mustNet(t, "10.0.0.2/32"),
			LocalPort: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 179}, RemotePort: ipsec.AnyPort(), Protocol: 6},
		{Number: "2", LocalPrefix: mustNet(t, "10.0.0.1/32"), RemotePrefix: mustNet(t, "10.0.0.2/32"),
			LocalPort: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 180}, RemotePort: ipsec.AnyPort(), Protocol: 6},
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
	wide.UseTransportMode = decideResponderTransportMode(wide, true) == modeTransport
	if !wide.UseTransportMode {
		t.Fatal("setup: a transport-mode request to a transport-mode peer was declined; the refusal below would test tunnel narrowing")
	}
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
// transportRequestAuth runs IKE_SA_INIT, then sends an IKE_AUTH request carrying
// USE_TRANSPORT_MODE (the initiator is configured for transport, so buildAuthRequest adds
// it) through the production responder, handleAuthRequest, whose peer is configured for
// respMode. It returns the established responder SA and the decrypted response.
func transportRequestAuth(t *testing.T, respMode uint8) (*SA, []wire.PayloadEntry) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "transport-psk")
	iniPeer.Mode = dataplane.ModeTransport
	respPeer.Mode = respMode

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)
	if !notifyIn(mbDecrypt(t, resp, ini.LastSentMsg), wire.NotifyUseTransportMode) {
		t.Fatal("setup: the IKE_AUTH request carries no USE_TRANSPORT_MODE; the responder would decide on no request")
	}

	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	if resp.State != StateEstablished {
		t.Fatalf("responder state = %v, want established", resp.State)
	}
	return resp, mbDecrypt(t, ini, resp.LastSentMsg)
}

// VALIDATES: an IKE_AUTH request carrying USE_TRANSPORT_MODE, answered by the production
// responder (handleAuthRequest), leaves UseTransportMode true and puts the notification
// in the response when the peer is configured for transport, and leaves it false with no
// notification when the peer is configured for tunnel.
// PREVENTS: the responder echoing USE_TRANSPORT_MODE unconditionally, which would make
// acceptance an echo rather than a decision, and the responder dropping or inverting the
// decision it stores for the Child SA its later rounds create.
func TestUseTransportModeNotifyIsEchoedOnlyWhenAccepted(t *testing.T) {
	// RFC requirement: RFC7296-1.3.1-1 positive -- "If the request is accepted, the
	// response MUST also include a notification of type USE_TRANSPORT_MODE" (RFC 7296
	// S1.3.1, rfc/full/rfc7296.txt:802-806). The request reaches handleAuthRequest, and
	// both the decision it stores and the decrypted response are asserted.
	accept, acceptResponse := transportRequestAuth(t, dataplane.ModeTransport)
	if !accept.UseTransportMode {
		t.Error("a transport-mode request to a transport-mode peer was declined; the Child SA would be installed in tunnel mode")
	}
	if !notifyIn(acceptResponse, wire.NotifyUseTransportMode) {
		t.Error("an accepted transport-mode request was answered without USE_TRANSPORT_MODE")
	}

	// RFC requirement: RFC7296-1.3.1-1 negative -- the discriminator. A peer configured for
	// TUNNEL mode DECLINES the request: no notification is echoed and the Child SA is
	// established in tunnel mode, which is the outcome S1.3.1 names for a decline. This
	// separates "accepted" from "echoed whatever the peer asked for".
	decline, declineResponse := transportRequestAuth(t, dataplane.ModeTunnel)
	if decline.UseTransportMode {
		t.Error("a transport-mode request was accepted for a tunnel-mode peer; acceptance is an echo rather than a decision")
	}
	if notifyIn(declineResponse, wire.NotifyUseTransportMode) {
		t.Error("a declined transport-mode request was answered with USE_TRANSPORT_MODE")
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
	mode, deleteSA := recordInitiatorTransportMode(sa, false)
	if !deleteSA {
		t.Error("a declined transport-mode request with transport-required set did not delete the SA")
	}
	if mode != modeTunnel {
		t.Errorf("a declined request answered mode %d, want tunnel (%d); the Child SA would be installed in the wrong mode", mode, modeTunnel)
	}

	// RFC requirement: RFC7296-1.3.1-2 negative -- the discriminator. WITHOUT
	// transport-required the same decline keeps the SA, in tunnel mode. This proves the
	// delete is conditional on the operator's statement rather than on the decline alone.
	optional := &SA{PeerCfg: transportPeer()}
	mode, deleteSA = recordInitiatorTransportMode(optional, false)
	if deleteSA {
		t.Error("a declined request deleted the SA with transport-required unset; every peer without transport mode would lose its tunnel")
	}
	if mode != modeTunnel {
		t.Errorf("a declined request answered mode %d, want tunnel (%d)", mode, modeTunnel)
	}

	// An ACCEPTED request keeps the SA and answers transport mode.
	accepted := &SA{PeerCfg: required}
	mode, deleteSA = recordInitiatorTransportMode(accepted, true)
	if deleteSA {
		t.Error("an accepted transport-mode request deleted the SA")
	}
	if mode != modeTransport {
		t.Errorf("an accepted request answered mode %d, want transport (%d)", mode, modeTransport)
	}

	// A tunnel-mode peer is unaffected by a response that carries no notification.
	tunnel := &SA{PeerCfg: ipsec.SiteToSitePeer{Name: "t", Mode: dataplane.ModeTunnel, TransportRequired: false}}
	mode, deleteSA = recordInitiatorTransportMode(tunnel, false)
	if deleteSA {
		t.Error("a tunnel-mode peer was deleted for a response with no USE_TRANSPORT_MODE")
	}
	if mode != modeTunnel {
		t.Errorf("a tunnel-mode peer answered mode %d, want tunnel (%d)", mode, modeTunnel)
	}

	// The decision is a value of the exchange: it writes no IKE SA flag a later exchange
	// could read as the mode of an installed Child SA.
	for name, s := range map[string]*SA{"required": sa, "optional": optional, "accepted": accepted, "tunnel": tunnel} {
		if s.UseTransportMode {
			t.Errorf("%s: recordInitiatorTransportMode wrote UseTransportMode; adoptAuthResponseNegotiation owns that IKE_AUTH carrier", name)
		}
	}

	// The same decisions through the IKE_AUTH entry point, adoptAuthResponseNegotiation,
	// with a valid one-pair answer: it keeps the decided mode on the SA (UseTransportMode)
	// for createFirstChildSA, and deletes the SA only on a decline with transport-required.
	quiet := slogutil.DiscardLogger()
	answerI := tsPayload(t, wire.PayloadTypeTSi, "10.0.0.1/32")
	answerR := tsPayload(t, wire.PayloadTypeTSr, "10.0.0.2/32")
	adopted := &SA{PeerCfg: required, IsInitiator: true}
	if ok, notify := adoptAuthResponseNegotiation(adopted, true, true, answerI, answerR, quiet); !ok {
		t.Fatalf("an accepted transport-mode answer with one host pair was refused (notify %d)", notify)
	}
	if !adopted.UseTransportMode {
		t.Error("an accepted transport-mode answer left UseTransportMode false; createFirstChildSA would install tunnel mode")
	}
	keptTunnel := &SA{PeerCfg: transportPeer(), IsInitiator: true}
	if ok, notify := adoptAuthResponseNegotiation(keptTunnel, false, true, answerI, answerR, quiet); !ok {
		t.Fatalf("a declined answer with transport-required unset deleted the SA (notify %d)", notify)
	}
	if keptTunnel.UseTransportMode {
		t.Error("a declined answer left UseTransportMode true; createFirstChildSA would install transport mode the responder refused")
	}
	deleted := &SA{PeerCfg: required, IsInitiator: true}
	if ok, notify := adoptAuthResponseNegotiation(deleted, false, true, answerI, answerR, quiet); ok || notify != 0 {
		t.Errorf("a declined answer with transport-required set = (%v, %d), want (false, 0): delete the SA with no error notification", ok, notify)
	}
	if deleted.UseTransportMode {
		t.Error("a declined answer with transport-required set left UseTransportMode true")
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

	// A disjoint proposal is refused on the EAP path too. The refusal is kept for the
	// final IKE_AUTH to answer beside AUTH (RFC 7296 Section 2.21.2), so the test reads
	// the kept refusal: this SA holds no keys and dies later for that reason alone.
	bad := &SA{PeerName: "p", PeerCfg: policyPeer}
	ps.startResponderEAP(bad, 1, nil,
		tsPayload(t, wire.PayloadTypeTSi, "192.168.0.0/16"),
		tsPayload(t, wire.PayloadTypeTSr, "192.168.1.0/24"),
		nil, nil, slogutil.DiscardLogger())
	if !errors.Is(bad.eapChildRefusal, errTSUnacceptable) {
		t.Errorf("EAP refusal = %v after an unacceptable proposal, want errTSUnacceptable; the EAP path does not narrow", bad.eapChildRefusal)
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
// VALIDATES: a rekeyed Child SA installs the mode its rekey exchange negotiated (the
// caller computes it: respondChildRekey from the request's notify and the retired mode,
// applyChildRekeyResponse from the retired mode), and keeps the negotiated selector set.
// PREVENTS: a transport-mode tunnel silently becoming tunnel-mode at its first rekey, the
// constructor installing the retired mode over the one the exchange negotiated, and the
// NEXT rekey losing the RFC 7296 Section 2.9.2 floor because the scope in use was not
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

	// The rekey negotiated no selector of its own, so the replacement carries the scope in
	// use forward and the NEXT rekey still has a floor.
	fresh := newRekeyedChild(old, 3, 4, nil, true, nil, modeTransport)
	if fresh.Mode != modeTransport {
		t.Errorf("rekeyed child mode = %d, want transport (%d); a transport tunnel would become a tunnel-mode one at its first rekey",
			fresh.Mode, modeTransport)
	}
	// A rekey that negotiated tunnel (RFC 7296 Section 1.3.1: no USE_TRANSPORT_MODE in the
	// request) installs tunnel even though the retired SA ran transport.
	tunneled := newRekeyedChild(old, 5, 6, nil, true, nil, modeTunnel)
	if tunneled.Mode != modeTunnel {
		t.Errorf("rekeyed child mode = %d, want the negotiated tunnel (%d); the constructor installed the retired mode",
			tunneled.Mode, modeTunnel)
	}
	if len(fresh.Selectors) != 1 {
		t.Fatalf("rekeyed child carried %d selectors, want 1; the next rekey would have no RFC 7296 S2.9.2 floor", len(fresh.Selectors))
	}
	if fresh.Selectors[0].I.Net.String() != "10.1.0.0/16" {
		t.Errorf("rekeyed child TSi selector = %v, want the scope in use 10.1.0.0/16", fresh.Selectors[0].I.Net)
	}
}

// TestResponderNarrowsBothSelectorsToPolicy drives RFC 7296 Section 2.9's narrowing
// through narrowChildSelectors, the entry point both responder producers call, and
// reads the result back from the response the responder would send.
//
// VALIDATES: a proposal wider than policy on BOTH TSi and TSr is narrowed on both
// sides to exactly the policy prefixes, and the IKE_AUTH response carries those
// narrowed selectors.
// PREVENTS: a responder that narrows only one side, echoes the wider proposal, or
// answers with its policy when the proposal is already inside it, which would widen
// what the initiator asked for.
func TestResponderNarrowsBothSelectorsToPolicy(t *testing.T) {
	policyPeer := ipsec.SiteToSitePeer{
		Name: "p",
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{{
			Number:       "1",
			LocalPrefix:  mustNet(t, "10.20.0.0/16"),
			LocalPort:    ipsec.AnyPort(),
			RemotePrefix: mustNet(t, "10.10.0.0/16"),
			RemotePort:   ipsec.AnyPort(),
		}},
	}

	// RFC requirement: RFC7296-2.9-1 positive -- "When the responder chooses a subset
	// of the traffic proposed by the initiator, it narrows the Traffic Selectors to
	// some subset of the initiator's proposal (provided the set does not become the
	// null set)" (RFC 7296 Section 2.9, rfc/full/rfc7296.txt:2393-2395). The initiator
	// proposes 10.0.0.0/8 on both sides. The responder's policy is 10.10.0.0/16 for
	// the initiator's side (TSi) and 10.20.0.0/16 for its own (TSr), a strict subset
	// of the proposal on each side, so both selectors MUST come back narrowed to it.
	sa := &SA{PeerCfg: policyPeer, IsInitiator: false}
	sa.ESPGroup = testESPGroup()
	sa.ChildProposalNum = 1
	if err := narrowChildSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, "10.0.0.0/8"),
		tsPayload(t, wire.PayloadTypeTSr, "10.0.0.0/8"),
		nil); err != nil {
		t.Fatalf("a proposal that contains the policy was refused (%v); the responder must narrow it", err)
	}
	if got := sa.NegotiatedTSi.String(); got != "10.10.0.0/16" {
		t.Errorf("negotiated TSi = %s, want 10.10.0.0/16 narrowed from the proposed 10.0.0.0/8", got)
	}
	if got := sa.NegotiatedTSr.String(); got != "10.20.0.0/16" {
		t.Errorf("negotiated TSr = %s, want 10.20.0.0/16 narrowed from the proposed 10.0.0.0/8", got)
	}
	_, _, respTSi, respTSr, err := buildChildSAResponsePayloads(sa)
	if err != nil {
		t.Fatalf("buildChildSAResponsePayloads: %v", err)
	}
	assertOneSelector(t, "response TSi", respTSi, "10.10.0.0/16")
	assertOneSelector(t, "response TSr", respTSr, "10.20.0.0/16")

	// RFC requirement: RFC7296-2.9-1 negative -- the result is a subset of the
	// INITIATOR'S PROPOSAL, not the policy itself. A proposal already inside policy on
	// both sides comes back unchanged; answering with the policy prefixes would widen it.
	inside := &SA{PeerCfg: policyPeer, IsInitiator: false}
	if err := narrowChildSelectors(inside,
		tsPayload(t, wire.PayloadTypeTSi, "10.10.1.0/24"),
		tsPayload(t, wire.PayloadTypeTSr, "10.20.2.0/24"),
		nil); err != nil {
		t.Fatalf("a proposal inside policy was refused: %v", err)
	}
	if got := inside.NegotiatedTSi.String(); got != "10.10.1.0/24" {
		t.Errorf("negotiated TSi = %s, want the proposed 10.10.1.0/24; anything wider widens the proposal", got)
	}
	if got := inside.NegotiatedTSr.String(); got != "10.20.2.0/24" {
		t.Errorf("negotiated TSr = %s, want the proposed 10.20.2.0/24; anything wider widens the proposal", got)
	}
}

// assertOneSelector requires a TS payload carrying exactly one selector that spans
// the whole of want, with every port.
func assertOneSelector(t *testing.T, what string, payload *wire.PayloadTS, want string) {
	t.Helper()
	if payload == nil {
		t.Fatalf("%s: no TS payload", what)
	}
	if len(payload.TrafficSelectors) != 1 {
		t.Fatalf("%s carried %d selectors, want 1", what, len(payload.TrafficSelectors))
	}
	got := payload.TrafficSelectors[0]
	wantSel := wireSel(t, want, 0, 65535, 0)
	if !bytesEqual(got.StartAddress, wantSel.StartAddress) || !bytesEqual(got.EndAddress, wantSel.EndAddress) {
		t.Errorf("%s = %v-%v, want %s (%v-%v)", what,
			net.IP(got.StartAddress), net.IP(got.EndAddress), want,
			net.IP(wantSel.StartAddress), net.IP(wantSel.EndAddress))
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
