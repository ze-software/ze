// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the cross-peer inner-selector guard, transport mode
// Related: inner_selectors.go -- classifyInnerPair, which makes a transport pair operator-fixed
// Related: rekey.go -- respondChildRekey, which judges a rekey in the retired Child SA's mode
// VALIDATES: a host-to-host transport-mode peer beside a full-tunnel peer whose operator
// configured 0.0.0.0/0 is served when the two sit at distinct policy-priority values and
// refused at equal ones, in both arrival orders; and the peer's rekey of a transport-mode
// Child SA after an IKE SA rekey is judged in transport mode, so it stays ordered.
// PREVENTS: the operator-ordered exemption (RFC 4301 Section 4.4.1) being unreachable for
// an ordinary transport peer, whose remote half is its configured remote address rather
// than a configured traffic selector.

package engine

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// orderedTransportPeers returns full-tunnel peer a (remote 198.18.0.7, configured
// traffic selector 0.0.0.0/0 to 0.0.0.0/0) at priorityA and transport-mode peer b
// (remote 198.18.0.9, no traffic selector) at priorityB, sharing reg.
func orderedTransportPeers(t *testing.T, reg *innerSelectorRegistry, priorityA, priorityB uint32) (a, b *SA) {
	t.Helper()
	a = innerPeerSA(t, reg, "a", "198.18.0.7", selectorPolicy(t, "0.0.0.0/0", "0.0.0.0/0", 0, 0))
	a.PeerCfg.PolicyPriority = priorityA
	b = transportPeerSA(t, reg, "b")
	b.PeerCfg.PolicyPriority = priorityB
	return a, b
}

// fullTunnelTS is peer a's proposal: everything on both sides.
var fullTunnelTS = []string{"0.0.0.0/0"}

// TestRFC3948OrderedTransportBesideFullTunnelServed: transport-mode peer b at
// policy-priority 1000 and full-tunnel peer a at 2000. b's remote half 198.18.0.9/32 is
// b's configured remote address and a's remote half 0.0.0.0/0 is a's configured selector,
// so the operator wrote both and the priorities order them. Method: both orders, each
// peer through narrowChildSelectors and createFirstChildSA; both must be served and the
// registry must hold both.
func TestRFC3948OrderedTransportBesideFullTunnelServed(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 positive -- transport-mode peer b with remote address 198.18.0.9 and no traffic selector at policy-priority 1000 (TSi 198.18.0.9/32, TSr 192.0.2.1/32) and tunnel peer a configured with traffic selector 0.0.0.0/0 to 0.0.0.0/0 at policy-priority 2000 proposing exactly it are both served in either arrival order, the registry holding both Child SAs.
	for _, transportFirst := range []bool{true, false} {
		name := "tunnel first"
		if transportFirst {
			name = "transport first"
		}
		t.Run(name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			a, b := orderedTransportPeers(t, reg, 2000, 1000)
			dpA, dpB := &mockDP{}, &mockDP{}
			if transportFirst {
				mustInnerChild(t, b, dpB, transportTSi, transportTSr)
				mustInnerChild(t, a, dpA, fullTunnelTS, fullTunnelTS)
			} else {
				mustInnerChild(t, a, dpA, fullTunnelTS, fullTunnelTS)
				mustInnerChild(t, b, dpB, transportTSi, transportTSr)
			}
			if got := reg.holdings(); got != 2 {
				t.Fatalf("registry holds %d Child SAs, want 2 (a and b)", got)
			}
		})
	}
}

// TestRFC3948OrderedTransportEqualPriorityRefused: the same two peers at one
// policy-priority, 1000. Both pairs are operator-fixed, but nothing orders them, so the
// second is refused, and the refusal states when the priorities would order them.
// Method: both orders; the second must be answered TS_UNACCEPTABLE with nothing on its
// dataplane and the registry holding the first alone.
func TestRFC3948OrderedTransportEqualPriorityRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- transport-mode peer b with remote address 198.18.0.9 and tunnel peer a configured with traffic selector 0.0.0.0/0 to 0.0.0.0/0, both at policy-priority 1000: whichever proposes second is answered TS_UNACCEPTABLE with no state or policy on its dataplane, the registry holding the first alone.
	for _, transportFirst := range []bool{true, false} {
		name := "tunnel first"
		if transportFirst {
			name = "transport first"
		}
		t.Run(name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			a, b := orderedTransportPeers(t, reg, 1000, 1000)
			first, second := a, b
			firstTS := [2][]string{fullTunnelTS, fullTunnelTS}
			secondTS := [2][]string{transportTSi, transportTSr}
			if transportFirst {
				first, second = b, a
				firstTS, secondTS = secondTS, firstTS
			}
			mustInnerChild(t, first, &mockDP{}, firstTS[0], firstTS[1])
			dp := &mockDP{}
			_, err := innerAuthChild(t, second, dp, secondTS[0], secondTS[1])
			mustInnerRefused(t, err, "peer "+second.PeerName)
			mustNamePeersAndFix(t, err, "a")
			want := "the priorities order them only when the tunnel is exactly a traffic selector peer " +
				"a's operator configured, both halves, and the transport-mode connection is exactly " +
				"local 192.0.2.1/32 remote 198.18.0.9/32 for any protocol and any port"
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal %q does not state when ordering exempts (%q)", err, want)
			}
			if len(dp.sas) != 0 || len(dp.policies) != 0 {
				t.Fatalf("peer %s's dataplane got %d states and %d policies, want none",
					second.PeerName, len(dp.sas), len(dp.policies))
			}
			if got := reg.holdings(); got != 1 {
				t.Fatalf("registry holds %d Child SAs, want 1 (peer %s)", got, first.PeerName)
			}
		})
	}
}

// TestOrderedTransportChildRekeyAfterIKERekey: b's transport-mode Child SA is ordered
// beside a's full tunnel, then b's IKE SA is rekeyed, which leaves UseTransportMode
// unset on the replacement SA, and b's peer rekeys the Child SA. The replacement
// installs the retired Child SA's mode, so it must be judged a transport-mode pair and
// stay ordered. Method: the IKE SA rekey's effect is set on b (UseTransportMode false,
// as respondIKERekey builds the new SA without it), then respondChildRekey answers the
// peer's USE_TRANSPORT_MODE request for TSi 198.18.0.9/32, TSr 192.0.2.1/32; it must
// answer, and the replacement must run in transport mode.
func TestOrderedTransportChildRekeyAfterIKERekey(t *testing.T) {
	reg := newInnerSelectorRegistry()
	a, b := orderedTransportPeers(t, reg, 2000, 1000)
	mustInnerChild(t, a, &mockDP{}, fullTunnelTS, fullTunnelTS)
	dpB := &mockDP{}
	old := mustInnerChild(t, b, dpB, transportTSi, transportTSr)
	if old.Mode != modeTransport {
		t.Fatalf("b's Child SA runs mode %d, want transport (%d)", old.Mode, modeTransport)
	}

	b.UseTransportMode = false
	inner := []wire.PayloadEntry{
		{Payload: espSAPayload(0x01020304)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(3)}},
		{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyUseTransportMode}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, transportTSi...)},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, transportTSr...)},
	}
	_, next, err := respondChildRekey(b, inner, old, 5, dpB, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("respondChildRekey of b's transport-mode Child SA after an IKE SA rekey: %v", err)
	}
	if next.Mode != modeTransport {
		t.Fatalf("replacement runs mode %d, want transport (%d)", next.Mode, modeTransport)
	}
}

// TestRFC3948OrderedTransportNarrowedToPortRefused: transport-mode peer b at
// policy-priority 1000 beside full-tunnel peer a at 2000, as in the Served test, but b
// proposes its own address for TCP port 80 only (TSi 198.18.0.9/32 protocol 6 port 80,
// TSr 192.0.2.1/32 protocol 6). b's operator wrote the address, not the protocol or the
// port, so that remote half is b's choice and the priorities do not order it. Method:
// both orders, each peer through narrowChildSelectors and createFirstChildSA; the second
// must be answered TS_UNACCEPTABLE with nothing on its dataplane and the registry
// holding the first alone.
func TestRFC3948OrderedTransportNarrowedToPortRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- transport-mode peer b with remote address 198.18.0.9 and no traffic selector at policy-priority 1000 proposing TSi 198.18.0.9/32 protocol 6 port 80, TSr 192.0.2.1/32 protocol 6, and tunnel peer a configured with traffic selector 0.0.0.0/0 to 0.0.0.0/0 at policy-priority 2000 proposing exactly it: whichever proposes second is answered TS_UNACCEPTABLE with no state or policy on its dataplane, the registry holding the first alone, because a remote half narrowed to one protocol and port at the configured remote address is the peer's choice.
	for _, transportFirst := range []bool{true, false} {
		name := "tunnel first"
		if transportFirst {
			name = "transport first"
		}
		t.Run(name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			a, b := orderedTransportPeers(t, reg, 2000, 1000)
			first, second := a, b
			if transportFirst {
				first, second = b, a
			}
			if _, err := orderedTransportChild(t, first, &mockDP{}); err != nil {
				t.Fatalf("peer %s alone: %v", first.PeerName, err)
			}
			dp := &mockDP{}
			_, err := orderedTransportChild(t, second, dp)
			mustInnerRefused(t, err, "peer "+second.PeerName)
			mustNamePeersAndFix(t, err, "a")
			if len(dp.sas) != 0 || len(dp.policies) != 0 {
				t.Fatalf("peer %s's dataplane got %d states and %d policies, want none",
					second.PeerName, len(dp.sas), len(dp.policies))
			}
			if got := reg.holdings(); got != 1 {
				t.Fatalf("registry holds %d Child SAs, want 1 (peer %s)", got, first.PeerName)
			}
		})
	}
}

// orderedTransportChild runs innerAuthChild's two steps for one peer of
// orderedTransportPeers: peer a proposes its full tunnel, peer b its own address for
// TCP port 80 only.
func orderedTransportChild(t *testing.T, sa *SA, dp *mockDP) (*ChildSA, error) {
	t.Helper()
	if sa.PeerName == "a" {
		return innerAuthChild(t, sa, dp, fullTunnelTS, fullTunnelTS)
	}
	tsi := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, "198.18.0.9/32", 80, 80, 6),
	}}
	tsr := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, "192.0.2.1/32", 0, 65535, 6),
	}}
	if err := narrowChildSelectors(sa, tsi, tsr, nil); err != nil {
		return nil, err
	}
	ifID, err := resolveIfID(&sa.PeerCfg)
	if err != nil {
		t.Fatalf("resolveIfID: %v", err)
	}
	return createFirstChildSA(sa, sa.ESPGroup, sa.PeerCfg.LocalAddress, sa.PeerCfg.RemoteAddress, ifID, dp,
		slogutil.DiscardLogger())
}
