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

// The four addresses of the RFC 7296 Section 2.23.1 figure, which every fixture in this
// file uses:
//
//	+------+        +------+            +------+         +------+
//	|Client| IP1    | NAT  | IPN1  IPN2 | NAT  |     IP2 |Server|
//	|node  |<------>|  A   |<---------->|  B   |<------->|      |
//	+------+        +------+            +------+         +------+
//
// The client knows IP1 and IPN2. The server knows IP2 and IPN1. Neither knows the other's
// pre-NAT address, and that is the whole reason the substitution exists.
const (
	natClientReal   = "10.1.0.1"    // IP1, the client's own address
	natClientPublic = "203.0.113.1" // IPN1, the client as the server sees it
	natServerPublic = "203.0.113.2" // IPN2, the server as the client sees it
	natServerReal   = "10.2.0.1"    // IP2, the server's own address
)

// natClientPeer is the transport-mode peer configuration on the CLIENT, which dials the
// outer address of NAT B.
func natClientPeer() ipsec.SiteToSitePeer {
	return ipsec.SiteToSitePeer{
		Name:           "client",
		ConnectionType: ipsec.ConnectionInitiate,
		LocalAddress:   natClientReal,
		RemoteAddress:  natServerPublic,
		Mode:           dataplane.ModeTransport,
	}
}

// natServerPeer is the transport-mode peer configuration on the SERVER. Its
// remote-address is the client's POST-NAT address, because matchResponderPeer
// (register.go) accepts an unsolicited IKE_SA_INIT only from a source equal to it.
func natServerPeer() ipsec.SiteToSitePeer {
	return ipsec.SiteToSitePeer{
		Name:           "server",
		ConnectionType: ipsec.ConnectionRespond,
		LocalAddress:   natServerReal,
		RemoteAddress:  natClientPublic,
		Mode:           dataplane.ModeTransport,
	}
}

// natHostPolicy is one traffic-selector row naming a single host on each side, which is
// what an operator writes for a transport-mode peer (docs/guide/ipsec.md).
func natHostPolicy(t *testing.T, local, remote string) []ipsec.TrafficSelectorPolicy {
	t.Helper()
	return []ipsec.TrafficSelectorPolicy{{
		Number:       "1",
		LocalPrefix:  mustNet(t, local+"/32"),
		LocalPort:    ipsec.AnyPort(),
		RemotePrefix: mustNet(t, remote+"/32"),
		RemotePort:   ipsec.AnyPort(),
	}}
}

// natResponderSA builds a server-role SA with the NAT verdict the arguments name.
func natResponderSA(peerBehindNAT, behindNAT bool) *SA {
	sa := &SA{PeerCfg: natServerPeer(), IsInitiator: false}
	sa.PeerRequestedTransport = true
	sa.UseTransportMode = true
	sa.NATDetected = peerBehindNAT || behindNAT
	sa.PeerBehindNAT = peerBehindNAT
	sa.BehindNAT = behindNAT
	return sa
}

// natInitiatorSA builds a client-role SA with the NAT verdict the arguments name, and the
// proposal transportSelectorPairs would have recorded for it.
func natInitiatorSA(behindNAT, peerBehindNAT bool) *SA {
	sa := &SA{PeerCfg: natClientPeer(), IsInitiator: true}
	sa.UseTransportMode = true
	sa.NATDetected = behindNAT || peerBehindNAT
	sa.BehindNAT = behindNAT
	sa.PeerBehindNAT = peerBehindNAT
	sa.ProposedChildPairs = transportSelectorPairs(sa, nil)
	return sa
}

// pairText renders one negotiated pair as "TSi <-> TSr" for an assertion message.
func pairText(sa *SA) string {
	if sa.NegotiatedTSi == nil || sa.NegotiatedTSr == nil {
		return "<none>"
	}
	return sa.NegotiatedTSi.String() + " <-> " + sa.NegotiatedTSr.String()
}

// TestResponderSubstitutesTransportSelectorsBeforeNarrowing covers AC-1 and AC-2.
//
// VALIDATES: the responder replaces the TSi address with the address it OBSERVES the peer
// at when the peer is behind a NAT, replaces the TSr address with its own local address
// when it is itself behind one, and does both independently. The last case proves the
// substitution happens BEFORE the policy match, because the configured policy names only
// the substituted pair.
// PREVENTS: the responder narrowing a proposal that carries the client's pre-NAT address
// against a policy naming the address it observes, finding no intersection, and answering
// TS_UNACCEPTABLE to every conforming transport-mode client behind a NAT.
func TestResponderSubstitutesTransportSelectorsBeforeNarrowing(t *testing.T) {
	// The client always proposes IP1 <-> IPN2, which is what its own stack sees.
	proposalTSi := func() *wire.PayloadTS { return tsPayload(t, wire.PayloadTypeTSi, natClientReal+"/32") }
	proposalTSr := func() *wire.PayloadTS { return tsPayload(t, wire.PayloadTypeTSr, natServerPublic+"/32") }

	cases := []struct {
		name                     string
		peerBehindNAT, behindNAT bool
		wantTSi, wantTSr         string
	}{{
		// AC-1: only NAT A is present, so TSi moves to the observed remote address and
		// TSr is left exactly as the client sent it.
		name:          "client behind a NAT",
		peerBehindNAT: true,
		wantTSi:       natClientPublic + "/32", wantTSr: natServerPublic + "/32",
	}, {
		// AC-2: only NAT B is present, so TSr moves to this node's own local address.
		name:      "server behind a NAT",
		behindNAT: true,
		wantTSi:   natClientReal + "/32", wantTSr: natServerReal + "/32",
	}, {
		// AC-1 and AC-2 together: the two-NAT figure the section draws.
		name:          "both behind a NAT",
		peerBehindNAT: true, behindNAT: true,
		wantTSi: natClientPublic + "/32", wantTSr: natServerReal + "/32",
	}, {
		// The discriminator: with no NAT the substitution is not merely the identity by
		// arithmetic, it does not run at all, so a NAT-free transport exchange is
		// untouched.
		name:    "no NAT on the path",
		wantTSi: natClientReal + "/32", wantTSr: natServerPublic + "/32",
	}}

	for _, c := range cases {
		t.Run(c.name+"/unconfigured policy", func(t *testing.T) {
			sa := natResponderSA(c.peerBehindNAT, c.behindNAT)
			if err := narrowChildSelectors(sa, proposalTSi(), proposalTSr(), nil); err != nil {
				t.Fatalf("narrowChildSelectors: %v", err)
			}
			if sa.NegotiatedTSi.String() != c.wantTSi || sa.NegotiatedTSr.String() != c.wantTSr {
				t.Errorf("narrowed pair = %s, want %s <-> %s", pairText(sa), c.wantTSi, c.wantTSr)
			}

			// The answer put ON THE WIRE carries the substituted addresses, which is what
			// the client reads. A substitution that changed only the SA's record would
			// leave the two ends programming different traffic.
			wireTSi, wireTSr := pairsToWire(sa.NegotiatedPairs)
			if wireTSi == nil || wireTSr == nil {
				t.Fatal("the narrowed answer produced no TS payloads")
			}
			if got := net.IP(wireTSi.TrafficSelectors[0].StartAddress).String(); got != c.wantTSi[:len(c.wantTSi)-3] {
				t.Errorf("answered TSi address = %s, want %s", got, c.wantTSi)
			}
			if got := net.IP(wireTSr.TrafficSelectors[0].StartAddress).String(); got != c.wantTSr[:len(c.wantTSr)-3] {
				t.Errorf("answered TSr address = %s, want %s", got, c.wantTSr)
			}
		})
	}

	// The SPD lookup runs on the SUBSTITUTED pair. The operator's policy names the two
	// addresses this node's own stack sees, IPN1 and IP2, and nothing in it mentions the
	// client's pre-NAT IP1. Narrowing the raw proposal against it finds no intersection.
	configured := natResponderSA(true, true)
	configured.PeerCfg.TrafficSelectors = natHostPolicy(t, natServerReal, natClientPublic)
	if err := narrowChildSelectors(configured, proposalTSi(), proposalTSr(), nil); err != nil {
		t.Fatalf("a policy naming the observed pair refused the proposal (%v); the SPD lookup ran on the pre-NAT addresses", err)
	}
	if configured.NegotiatedTSi.String() != natClientPublic+"/32" || configured.NegotiatedTSr.String() != natServerReal+"/32" {
		t.Errorf("narrowed pair against a configured policy = %s, want %s/32 <-> %s/32",
			pairText(configured), natClientPublic, natServerReal)
	}

	// The discriminator for the policy case: the same policy, with the NAT verdict absent,
	// still refuses. That proves the acceptance above came from the substitution rather
	// than from a policy that would have accepted anything.
	unsubstituted := natResponderSA(false, false)
	unsubstituted.PeerCfg.TrafficSelectors = natHostPolicy(t, natServerReal, natClientPublic)
	if err := narrowChildSelectors(unsubstituted, proposalTSi(), proposalTSr(), nil); err == nil {
		t.Errorf("a pre-NAT proposal was accepted against a policy naming only the observed pair, narrowed to %s",
			pairText(unsubstituted))
	}
}

// TestInitiatorAdoptsSubstitutedTransportSelectorsBehindNAT covers AC-3 and AC-4.
//
// VALIDATES: the client converts the responder's answer back into its own address space
// before the answer is checked or installed, so a conforming responder's answer is
// accepted and the installed pair is the one the client's stack will see.
// PREVENTS: the defect this spec exists for -- the client refused a conforming answer with
// errTSWidened and deleted the SA, because the responder answered in ITS address space and
// the ceiling is in the client's.
func TestInitiatorAdoptsSubstitutedTransportSelectorsBehindNAT(t *testing.T) {
	cases := []struct {
		name                     string
		behindNAT, peerBehindNAT bool
		answerTSi, answerTSr     string
		wantTSi, wantTSr         string
	}{{
		// AC-3: the client is behind NAT A. The responder answered with the address it
		// observes the client at, and the client puts its own back.
		name:      "client behind a NAT",
		behindNAT: true,
		answerTSi: natClientPublic, answerTSr: natServerPublic,
		wantTSi: natClientReal + "/32", wantTSr: natServerPublic + "/32",
	}, {
		// AC-4: the server is behind NAT B. The responder answered with its own address,
		// and the client puts the address it dials back.
		name:          "server behind a NAT",
		peerBehindNAT: true,
		answerTSi:     natClientReal, answerTSr: natServerReal,
		wantTSi: natClientReal + "/32", wantTSr: natServerPublic + "/32",
	}, {
		// AC-3 and AC-4 together.
		name:      "both behind a NAT",
		behindNAT: true, peerBehindNAT: true,
		answerTSi: natClientPublic, answerTSr: natServerReal,
		wantTSi: natClientReal + "/32", wantTSr: natServerPublic + "/32",
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sa := natInitiatorSA(c.behindNAT, c.peerBehindNAT)
			err := recordInitiatorSelectors(sa,
				tsPayload(t, wire.PayloadTypeTSi, c.answerTSi+"/32"),
				tsPayload(t, wire.PayloadTypeTSr, c.answerTSr+"/32"), nil)
			if err != nil {
				t.Fatalf("a conforming responder answer was refused: %v", err)
			}
			if sa.NegotiatedTSi.String() != c.wantTSi || sa.NegotiatedTSr.String() != c.wantTSr {
				t.Errorf("installed pair = %s, want %s <-> %s", pairText(sa), c.wantTSi, c.wantTSr)
			}
		})
	}

	// The discriminator: the SAME answer, with the NAT verdict absent, is refused. Without
	// it the acceptances above could come from a ceiling that permits anything.
	noNAT := natInitiatorSA(false, false)
	if err := recordInitiatorSelectors(noNAT,
		tsPayload(t, wire.PayloadTypeTSi, natClientPublic+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, natServerReal+"/32"), nil); err == nil {
		t.Errorf("an answer outside the proposal was adopted with no NAT detected, as %s", pairText(noNAT))
	}
}

// TestTransportSelectorRoundTripAcrossTwoNATs runs the responder producer and feeds its
// wire answer straight into the client's adoption path.
//
// VALIDATES: the two substitutions are inverses of each other over the RFC's own
// two-NAT figure, so the client installs the pair its stack sees and the server installs
// the pair its stack sees.
// PREVENTS: the half fix R-4 names. Each producer tested alone can be self-consistent and
// still disagree with the other, and this is the only test that would go red for that.
func TestTransportSelectorRoundTripAcrossTwoNATs(t *testing.T) {
	client := natInitiatorSA(true, true)
	client.PeerCfg.TrafficSelectors = natHostPolicy(t, natClientReal, natServerPublic)
	proposalTSi, proposalTSr := proposeChildTSPayloads(client)
	if proposalTSi == nil || proposalTSr == nil {
		t.Fatal("the client proposed no traffic selectors")
	}

	server := natResponderSA(true, true)
	server.PeerCfg.TrafficSelectors = natHostPolicy(t, natServerReal, natClientPublic)
	if err := narrowChildSelectors(server, proposalTSi, proposalTSr, nil); err != nil {
		t.Fatalf("the server refused the client's proposal: %v", err)
	}
	if server.NegotiatedTSi.String() != natClientPublic+"/32" || server.NegotiatedTSr.String() != natServerReal+"/32" {
		t.Fatalf("server installed %s, want %s/32 <-> %s/32", pairText(server), natClientPublic, natServerReal)
	}

	answerTSi, answerTSr := pairsToWire(server.NegotiatedPairs)
	if err := recordInitiatorSelectors(client, answerTSi, answerTSr, nil); err != nil {
		t.Fatalf("the client refused the server's conforming answer: %v", err)
	}
	if client.NegotiatedTSi.String() != natClientReal+"/32" || client.NegotiatedTSr.String() != natServerPublic+"/32" {
		t.Errorf("client installed %s, want %s/32 <-> %s/32", pairText(client), natClientReal, natServerPublic)
	}
}

// TestSubstitutionStoresTheOriginalSelectorAddresses covers AC-5.
//
// VALIDATES: both roles record the addresses that arrived on the wire, before the
// substitution replaces them, and the record survives the Child SA install.
// PREVENTS: losing the "real source and destination address" RFC 7296 Section 2.23.1
// requires be kept for the [UDPENCAPS] checksum fixup and for undoing the substitution.
func TestSubstitutionStoresTheOriginalSelectorAddresses(t *testing.T) {
	server := natResponderSA(true, true)
	if err := narrowChildSelectors(server,
		tsPayload(t, wire.PayloadTypeTSi, natClientReal+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, natServerPublic+"/32"), nil); err != nil {
		t.Fatalf("narrowChildSelectors: %v", err)
	}
	if server.OriginalTSiAddr.String() != natClientReal || server.OriginalTSrAddr.String() != natServerPublic {
		t.Errorf("responder stored originals %v / %v, want %s / %s",
			server.OriginalTSiAddr, server.OriginalTSrAddr, natClientReal, natServerPublic)
	}
	// The stored value is the address as RECEIVED, so it must differ from the substituted
	// one. Storing after the substitution would keep the wrong pair and pass a laxer test.
	if server.OriginalTSiAddr.String() == server.NegotiatedTSi.IP.String() {
		t.Error("the responder stored the SUBSTITUTED TSi address; the original is lost")
	}

	client := natInitiatorSA(true, true)
	if err := recordInitiatorSelectors(client,
		tsPayload(t, wire.PayloadTypeTSi, natClientPublic+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, natServerReal+"/32"), nil); err != nil {
		t.Fatalf("recordInitiatorSelectors: %v", err)
	}
	if client.OriginalTSiAddr.String() != natClientPublic || client.OriginalTSrAddr.String() != natServerReal {
		t.Errorf("initiator stored originals %v / %v, want %s / %s",
			client.OriginalTSiAddr, client.OriginalTSrAddr, natClientPublic, natServerReal)
	}
	if client.OriginalTSrAddr.String() == client.NegotiatedTSr.IP.String() {
		t.Error("the initiator stored the SUBSTITUTED TSr address; the original is lost")
	}
}

// TestSubstitutionStillRefusesAWidenedAnswer covers AC-6.
//
// VALIDATES: the ceiling is unchanged. An answered half that the substitution does NOT
// cover, and a port outside the proposal, are both still refused with errTSWidened.
// PREVENTS: R-1, the selector-confusion hole. The substitution replaces an address with
// one this node observed; it must not become a licence to install whatever the peer named.
func TestSubstitutionStillRefusesAWidenedAnswer(t *testing.T) {
	// Only the CLIENT is behind a NAT, so the substitution touches TSi alone and TSr is
	// left exactly as the responder answered it. A responder naming a third party there is
	// choosing the traffic Ze protects.
	sa := natInitiatorSA(true, false)
	err := recordInitiatorSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, natClientPublic+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, "198.51.100.9/32"), nil)
	if err == nil {
		t.Fatalf("a responder-chosen TSr address was installed, as %s", pairText(sa))
	}
	if !errors.Is(err, errTSWidened) {
		t.Errorf("refusal = %v, want errTSWidened", err)
	}

	// The same answer through the production adoption path: the SA is torn down and the
	// peer is owed TS_UNACCEPTABLE, exactly as it was before the substitution existed.
	viaEntry := natInitiatorSA(true, false)
	ok, notify := adoptAuthResponseNegotiation(viaEntry, true,
		tsPayload(t, wire.PayloadTypeTSi, natClientPublic+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, "198.51.100.9/32"), slogutil.DiscardLogger())
	if ok {
		t.Errorf("the IKE_AUTH adoption path accepted a responder-chosen TSr, as %s", pairText(viaEntry))
	}
	if notify != wire.NotifyTSUnacceptable {
		t.Errorf("notify for the refusal = %d (%s), want TS_UNACCEPTABLE", notify, wire.NotifyTypeName(notify))
	}

	// The port and the protocol survive the substitution, so the ceiling still constrains
	// them even when both addresses are replaced. A responder that answers a port the
	// client never proposed is refused.
	both := natInitiatorSA(true, true)
	both.PeerCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{{
		Number:       "1",
		LocalPrefix:  mustNet(t, natClientReal+"/32"),
		LocalPort:    ipsec.PortSelector{Form: ipsec.PortSingle, Port: 179},
		RemotePrefix: mustNet(t, natServerPublic+"/32"),
		RemotePort:   ipsec.AnyPort(),
		Protocol:     6,
	}}
	both.ProposedChildPairs = transportSelectorPairs(both, policyPairs(both.PeerCfg, true))
	widePort := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, natClientPublic+"/32", 0, 65535, 6),
	}}
	if err := recordInitiatorSelectors(both, widePort,
		tsPayload(t, wire.PayloadTypeTSr, natServerReal+"/32"), nil); err == nil {
		t.Errorf("an any-port answer to a single-port proposal was installed, as %s", pairText(both))
	}

	// The discriminator: the conforming answer to the same proposal is ACCEPTED, so the
	// refusals above are decisions about those answers rather than a blanket refusal.
	good := natInitiatorSA(true, false)
	if err := recordInitiatorSelectors(good,
		tsPayload(t, wire.PayloadTypeTSi, natClientPublic+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, natServerPublic+"/32"), nil); err != nil {
		t.Fatalf("a conforming answer was refused: %v", err)
	}
}

// TestTunnelModeSelectorsUnchangedWithNATDetected covers AC-7, both arms of the gate.
//
// VALIDATES: a tunnel-mode exchange with a NAT detected, and a transport-mode exchange
// with no NAT, each negotiate exactly the selectors they did before the substitution
// existed.
// PREVENTS: R-2, the substitution leaking into tunnel mode and replacing prefixes an
// operator configured with a pair of host addresses.
func TestTunnelModeSelectorsUnchangedWithNATDetected(t *testing.T) {
	// A tunnel-mode responder with both NAT flags set. Nothing asked for transport mode,
	// so nothing is substituted and the /16 prefixes survive.
	tunnelCfg := ipsec.SiteToSitePeer{
		Name:          "tunnel",
		LocalAddress:  natServerReal,
		RemoteAddress: natClientPublic,
		Mode:          dataplane.ModeTunnel,
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{{
			Number:       "1",
			LocalPrefix:  mustNet(t, "10.20.0.0/16"),
			LocalPort:    ipsec.AnyPort(),
			RemotePrefix: mustNet(t, "10.10.0.0/16"),
			RemotePort:   ipsec.AnyPort(),
		}},
	}
	tunnel := &SA{PeerCfg: tunnelCfg, IsInitiator: false}
	tunnel.NATDetected = true
	tunnel.BehindNAT = true
	tunnel.PeerBehindNAT = true
	if err := narrowChildSelectors(tunnel,
		tsPayload(t, wire.PayloadTypeTSi, "10.10.0.0/16"),
		tsPayload(t, wire.PayloadTypeTSr, "10.20.0.0/16"), nil); err != nil {
		t.Fatalf("a tunnel-mode proposal was refused: %v", err)
	}
	if tunnel.NegotiatedTSi.String() != "10.10.0.0/16" || tunnel.NegotiatedTSr.String() != "10.20.0.0/16" {
		t.Errorf("tunnel-mode pair = %s, want 10.10.0.0/16 <-> 10.20.0.0/16; the substitution leaked into tunnel mode",
			pairText(tunnel))
	}
	if tunnel.OriginalTSiAddr != nil || tunnel.OriginalTSrAddr != nil {
		t.Errorf("a tunnel-mode exchange stored transport-mode originals %v / %v",
			tunnel.OriginalTSiAddr, tunnel.OriginalTSrAddr)
	}

	// A transport-mode responder with NO NAT detected answers the pair the peer proposed,
	// exactly as it did before.
	plain := natResponderSA(false, false)
	if err := narrowChildSelectors(plain,
		tsPayload(t, wire.PayloadTypeTSi, natClientReal+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, natServerPublic+"/32"), nil); err != nil {
		t.Fatalf("a NAT-free transport proposal was refused: %v", err)
	}
	if plain.NegotiatedTSi.String() != natClientReal+"/32" || plain.NegotiatedTSr.String() != natServerPublic+"/32" {
		t.Errorf("NAT-free transport pair = %s, want %s/32 <-> %s/32; the substitution ran with no NAT verdict",
			pairText(plain), natClientReal, natServerPublic)
	}
}

// TestChildRekeyFloorComparedInSubstitutedSpace covers AC-8.
//
// VALIDATES: a Child SA rekey across the NAT compares the RFC 7296 Section 2.9.2 floor
// against the SUBSTITUTED answer, on both roles, so the scope in use and the new answer
// are in one address space.
// PREVENTS: R-3 -- a floor recorded in post-substitution addresses compared against a
// pre-substitution answer covers nothing, and every rekey would be refused one lifetime
// after the tunnel came up.
func TestChildRekeyFloorComparedInSubstitutedSpace(t *testing.T) {
	// The scope in use is what the first exchange negotiated, which is already
	// substituted: the client's own view is IP1 <-> IPN2.
	clientFloor := []tsPair{{
		I: tsSelector{Net: mustNet(t, natClientReal+"/32"), Port: ipsec.AnyPort()},
		R: tsSelector{Net: mustNet(t, natServerPublic+"/32"), Port: ipsec.AnyPort()},
	}}
	client := natInitiatorSA(true, true)
	if err := recordInitiatorSelectors(client,
		tsPayload(t, wire.PayloadTypeTSi, natClientPublic+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, natServerReal+"/32"), clientFloor); err != nil {
		t.Fatalf("the rekey answer was refused against a floor in the same space: %v", err)
	}

	// The responder's scope in use is its own view, IPN1 <-> IP2.
	serverFloor := []tsPair{{
		I: tsSelector{Net: mustNet(t, natClientPublic+"/32"), Port: ipsec.AnyPort()},
		R: tsSelector{Net: mustNet(t, natServerReal+"/32"), Port: ipsec.AnyPort()},
	}}
	server := natResponderSA(true, true)
	if err := narrowChildSelectors(server,
		tsPayload(t, wire.PayloadTypeTSi, natClientReal+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, natServerPublic+"/32"), serverFloor); err != nil {
		t.Fatalf("the rekey request was refused against a floor in the same space: %v", err)
	}
	if server.NegotiatedTSi.String() != natClientPublic+"/32" || server.NegotiatedTSr.String() != natServerReal+"/32" {
		t.Errorf("rekey answer = %s, want the scope in use %s/32 <-> %s/32",
			pairText(server), natClientPublic, natServerReal)
	}

	// The discriminator: a floor in the PRE-substitution space is refused, which is the
	// failure R-3 describes. It proves the two acceptances above turn on the space the
	// floor is in rather than on the floor being ignored.
	stale := natResponderSA(true, true)
	if err := narrowChildSelectors(stale,
		tsPayload(t, wire.PayloadTypeTSi, natClientReal+"/32"),
		tsPayload(t, wire.PayloadTypeTSr, natServerPublic+"/32"), clientFloor); err == nil {
		t.Errorf("a floor in the pre-substitution space was covered by a substituted answer %s; the floor check compares nothing",
			pairText(stale))
	}
}

// TestTransportProposalUsesObservedIKEAddresses pins the source of the proposal's two
// addresses.
//
// VALIDATES: transportSelectorPairs reads the IKE SA's observed pair, so an authenticated
// peer endpoint that differs from the configured remote-address is what reaches the wire,
// and each configured selector's port and protocol survive.
// PREVENTS: the stale claim the function's own comment made -- it said the addresses come
// from the IKE SA while the code read the config, which is only accidentally the same
// thing while nothing moves.
func TestTransportProposalUsesObservedIKEAddresses(t *testing.T) {
	sa := &SA{PeerCfg: natClientPeer(), IsInitiator: true}
	sa.PeerCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{{
		Number:       "1",
		LocalPrefix:  mustNet(t, "192.168.5.5/32"),
		LocalPort:    ipsec.PortSelector{Form: ipsec.PortSingle, Port: 179},
		RemotePrefix: mustNet(t, "192.168.6.6/32"),
		RemotePort:   ipsec.AnyPort(),
		Protocol:     6,
	}}

	pairs := transportSelectorPairs(sa, policyPairs(sa.PeerCfg, true))
	if len(pairs) != 1 {
		t.Fatalf("proposal carried %d pairs, want 1", len(pairs))
	}
	if pairs[0].I.Net.String() != natClientReal+"/32" || pairs[0].R.Net.String() != natServerPublic+"/32" {
		t.Errorf("proposal = %v <-> %v, want the observed pair %s/32 <-> %s/32",
			pairs[0].I.Net, pairs[0].R.Net, natClientReal, natServerPublic)
	}
	if pairs[0].I.Port.Port != 179 || pairs[0].I.Proto != 6 {
		t.Errorf("proposal TSi port/proto = %v/%d, want 179/6; the substitution replaced more than the address",
			pairs[0].I.Port, pairs[0].I.Proto)
	}

	// The peer answered from an address the configuration does not name. remoteUDPAddr
	// prefers that AUTHENTICATED observation, so the proposal follows it.
	moved := &SA{PeerCfg: natClientPeer(), IsInitiator: true}
	moved.peerEndpoint = &net.UDPAddr{IP: net.ParseIP("198.51.100.4"), Port: 4500}
	movedPairs := transportSelectorPairs(moved, nil)
	if len(movedPairs) != 1 {
		t.Fatalf("moved proposal carried %d pairs, want 1", len(movedPairs))
	}
	if movedPairs[0].R.Net.String() != "198.51.100.4/32" {
		t.Errorf("proposal TSr = %v, want the observed 198.51.100.4/32; the address came from config",
			movedPairs[0].R.Net)
	}
}
