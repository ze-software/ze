// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the cross-peer inner-selector guard, transport mode
// Related: inner_selectors.go -- the registry under test
// Related: ts_narrow.go -- narrowChildSelectors, whose filter step is under test
// VALIDATES: a tunnel another peer negotiates around a transport-mode peer's address, or
// a transport-mode connection inside another peer's tunnel, is refused in either arrival
// order, at narrowing and at install; two transport-mode connections are left to the
// dataplane; and an operator-fixed pair at a distinct priority does not make a pair the
// peer chose legal.
// PREVENTS: one packet matching two SAs of two peers, the confusion RFC 3948 Section 5.1
// names, through the transport-mode Child SAs the registry used to skip.

package engine

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// transportPeerSA is innerPeerSA for a peer this node runs transport mode with: the
// peer at 198.18.0.9, this node at 192.0.2.1.
func transportPeerSA(t *testing.T, reg *innerSelectorRegistry, name string) *SA {
	t.Helper()
	sa := innerPeerSA(t, reg, name, "198.18.0.9")
	sa.PeerCfg.Mode = dataplane.ModeTransport
	sa.UseTransportMode = true
	return sa
}

// transportTSi and transportTSr are the transport peer's proposal: its own address and
// this node's, one host each (RFC 7296 Section 2.23.1).
var (
	transportTSi = []string{"198.18.0.9/32"}
	transportTSr = []string{"192.0.2.1/32"}
)

// mustNamePeersAndFix asserts the refusal names both peers and the selector fix on the
// tunnel peer.
func mustNamePeersAndFix(t *testing.T, err error, tunnelPeer string) {
	t.Helper()
	for _, want := range []string{
		"peer a ", "peer b ", "transport-mode connection",
		"set vpn ipsec site-to-site peer " + tunnelPeer + " traffic-selector <n> remote prefix <prefix>",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
	}
}

// TestRFC3948TunnelAroundTransportPeerRefused replays the audit probe in both arrival
// orders. Peer b runs transport mode from 198.18.0.9 (TSi 198.18.0.9/32, TSr
// 192.0.2.1/32); unbound tunnel peer a proposes TSi 198.18.0.9/32, TSr 0.0.0.0/0. Both
// outbound policies match a packet from 192.0.2.1 to 198.18.0.9 at one priority.
// Method: the first peer negotiates and installs through narrowChildSelectors and
// createFirstChildSA; the second runs the same steps and must be refused with
// TS_UNACCEPTABLE, its dataplane untouched, the registry holding the first alone.
func TestRFC3948TunnelAroundTransportPeerRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- with transport-mode peer b at 198.18.0.9 served (TSi 198.18.0.9/32, TSr 192.0.2.1/32), unbound tunnel peer a proposing TSi 198.18.0.9/32, TSr 0.0.0.0/0 is answered TS_UNACCEPTABLE naming both peers and a's selector fix; in the reverse order, with a's tunnel served, b's transport-mode proposal is answered TS_UNACCEPTABLE; each time the refused peer's dataplane receives no state or policy and the registry holds the first peer alone.
	for _, c := range []struct {
		name           string
		transportFirst bool
	}{{"transport first", true}, {"tunnel first", false}} {
		t.Run(c.name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			dpFirst, dpSecond := &mockDP{}, &mockDP{}
			tunnel := innerPeerSA(t, reg, "a", "198.18.0.7")
			transport := transportPeerSA(t, reg, "b")
			first, second := transport, tunnel
			firstTSi, firstTSr := transportTSi, transportTSr
			secondTSi, secondTSr := []string{"198.18.0.9/32"}, []string{"0.0.0.0/0"}
			if !c.transportFirst {
				first, second = tunnel, transport
				firstTSi, firstTSr, secondTSi, secondTSr = secondTSi, secondTSr, firstTSi, firstTSr
			}
			mustInnerChild(t, first, dpFirst, firstTSi, firstTSr)
			_, err := innerAuthChild(t, second, dpSecond, secondTSi, secondTSr)
			mustInnerRefused(t, err, "peer "+second.PeerName)
			mustNamePeersAndFix(t, err, "a")
			if len(dpSecond.sas) != 0 || len(dpSecond.policies) != 0 {
				t.Fatalf("peer %s's dataplane got %d states and %d policies, want none",
					second.PeerName, len(dpSecond.sas), len(dpSecond.policies))
			}
			if got := policiesNaming(dpFirst, "198.18.0.9/32"); got != 2 {
				t.Fatalf("peer %s holds %d policies naming 198.18.0.9/32, want 2 (in and out)", first.PeerName, got)
			}
			if got := reg.holdings(); got != 1 {
				t.Fatalf("registry holds %d Child SAs, want 1 (peer %s)", got, first.PeerName)
			}
		})
	}
}

// TestRFC3948TunnelAroundTransportPeerInstallBackstop: the same two peers both passed
// narrowing before either installed, so the install alone decides. Method: each SA's
// answer is set as narrowing records it (NegotiatedPairs), then createFirstChildSA runs
// for both, in both orders; the second must be refused before its dataplane sees a call.
func TestRFC3948TunnelAroundTransportPeerInstallBackstop(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- transport-mode peer b (198.18.0.9/32 to 192.0.2.1/32) and unbound tunnel peer a (198.18.0.9/32 to 0.0.0.0/0) both past narrowing before either installed: whichever installs second through createFirstChildSA is answered TS_UNACCEPTABLE and its dataplane receives no state or policy, in both arrival orders.
	pair := func(i, r string) []tsPair {
		return []tsPair{{I: tsSelector{Net: mustNet(t, i)}, R: tsSelector{Net: mustNet(t, r)}}}
	}
	for _, c := range []struct {
		name           string
		transportFirst bool
	}{{"transport first", true}, {"tunnel first", false}} {
		t.Run(c.name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			tunnel := innerPeerSA(t, reg, "a", "198.18.0.7")
			tunnel.NegotiatedPairs = pair("198.18.0.9/32", "0.0.0.0/0")
			transport := transportPeerSA(t, reg, "b")
			transport.NegotiatedPairs = pair("198.18.0.9/32", "192.0.2.1/32")
			order := []*SA{transport, tunnel}
			if !c.transportFirst {
				order = []*SA{tunnel, transport}
			}
			log := slogutil.DiscardLogger()
			for i, sa := range order {
				dp := &mockDP{}
				_, err := createFirstChildSA(sa, sa.ESPGroup, sa.PeerCfg.LocalAddress, sa.PeerCfg.RemoteAddress, 0, dp, log)
				if i == 0 {
					if err != nil {
						t.Fatalf("peer %s install: %v", sa.PeerName, err)
					}
					continue
				}
				mustInnerRefused(t, err, "peer "+sa.PeerName+" install")
				if len(dp.sas) != 0 || len(dp.policies) != 0 {
					t.Fatalf("peer %s's dataplane got %d states and %d policies, want none",
						sa.PeerName, len(dp.sas), len(dp.policies))
				}
			}
			if got := reg.holdings(); got != 1 {
				t.Fatalf("registry holds %d Child SAs, want 1", got)
			}
		})
	}
}

// TestRFC3948NarrowedByPeerDistinctPriorityRefused: both peers are configured with the
// remote prefix 10.0.0.0/8, at policy-priority 1000 and 2000, and each narrows to
// 10.1.2.3/32. The distinct priorities would order two operator-fixed pairs, so only the
// classification refuses here: a narrowed remote half is the peer's choice.
func TestRFC3948NarrowedByPeerDistinctPriorityRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- two tunnel peers each configured with remote prefix 10.0.0.0/8, at policy-priority 1000 and 2000, that both narrow their proposal to 10.1.2.3/32: the second is answered TS_UNACCEPTABLE, because a narrowed remote half is the peer's choice and distinct priorities order only operator-fixed selectors.
	reg := newInnerSelectorRegistry()
	wide := selectorPolicy(t, "10.0.0.0/8", "0.0.0.0/0", 0, 0)
	a := innerPeerSA(t, reg, "a", "198.18.0.7", wide)
	a.PeerCfg.PolicyPriority = 1000
	mustInnerChild(t, a, &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	b := innerPeerSA(t, reg, "b", "198.18.0.9", wide)
	b.PeerCfg.PolicyPriority = 2000
	_, err := innerAuthChild(t, b, &mockDP{}, []string{"10.1.2.3/32"}, []string{"0.0.0.0/0"})
	mustInnerRefused(t, err, "peer b")
}

// TestRFC3948PeerChosenAgainstFixedDistinctPriorityRefused: peer b's operator
// configured remote 10.1.2.0/24 at policy-priority 1000 and b proposes exactly that;
// unbound peer a at 2000 proposes 10.1.2.3/32. The priorities differ, but a's pair is
// its own choice, so a, arriving second, is refused: the pair being judged is the
// peer-chosen one.
func TestRFC3948PeerChosenAgainstFixedDistinctPriorityRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- with tunnel peer b configured with remote prefix 10.1.2.0/24 at policy-priority 1000 and holding exactly it, unbound tunnel peer a at policy-priority 2000 proposing 10.1.2.3/32 is answered TS_UNACCEPTABLE with no state or policy on its dataplane, because a pair the peer chose is not operator-ordered whatever the priorities.
	fixedThenChosen(t, true)
}

// TestRFC3948FixedAgainstPeerChosenDistinctPriorityRefused is the reverse order: unbound
// peer a at 2000 holds 10.1.2.3/32, its own choice, and peer b, configured with remote
// 10.1.2.0/24 at 1000 and proposing exactly it, is refused: the pair already held is the
// peer-chosen one.
func TestRFC3948FixedAgainstPeerChosenDistinctPriorityRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- with unbound tunnel peer a at policy-priority 2000 holding 10.1.2.3/32, tunnel peer b configured with remote prefix 10.1.2.0/24 at policy-priority 1000 and proposing exactly it is answered TS_UNACCEPTABLE with no state or policy on its dataplane, because a held pair the peer chose is not operator-ordered whatever the priorities.
	fixedThenChosen(t, false)
}

// fixedThenChosen runs operator-fixed peer b (remote 10.1.2.0/24, priority 1000) and
// peer-chosen peer a (10.1.2.3/32, priority 2000) in the order fixedFirst names, and
// asserts the second is refused before its dataplane sees a call.
func fixedThenChosen(t *testing.T, fixedFirst bool) {
	t.Helper()
	reg := newInnerSelectorRegistry()
	fixed := innerPeerSA(t, reg, "b", "198.18.0.9", selectorPolicy(t, "10.1.2.0/24", "0.0.0.0/0", 0, 0))
	fixed.PeerCfg.PolicyPriority = 1000
	chosen := innerPeerSA(t, reg, "a", "198.18.0.7")
	chosen.PeerCfg.PolicyPriority = 2000
	type step struct {
		sa  *SA
		tsi []string
	}
	order := []step{{fixed, []string{"10.1.2.0/24"}}, {chosen, []string{"10.1.2.3/32"}}}
	if !fixedFirst {
		order[0], order[1] = order[1], order[0]
	}
	mustInnerChild(t, order[0].sa, &mockDP{}, order[0].tsi, []string{"0.0.0.0/0"})
	dp := &mockDP{}
	_, err := innerAuthChild(t, order[1].sa, dp, order[1].tsi, []string{"0.0.0.0/0"})
	mustInnerRefused(t, err, "peer "+order[1].sa.PeerName)
	if len(dp.sas) != 0 || len(dp.policies) != 0 {
		t.Fatalf("peer %s's dataplane got %d states and %d policies, want none",
			order[1].sa.PeerName, len(dp.sas), len(dp.policies))
	}
}

// TestRFC3948NarrowedProtocolDistinctPriorityRefused: peer a's operator configured
// remote 10.1.0.0/16 at policy-priority 2000 and a proposes exactly it. Peer b's operator
// configured remote 10.1.2.0/24 at 1500 for any protocol, and b narrows to TCP to any
// port. b's remote half differs from the configured one only in its protocol, so it is
// the peer's choice and b is refused, though the priorities differ.
func TestRFC3948NarrowedProtocolDistinctPriorityRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- with tunnel peer a holding its configured remote prefix 10.1.0.0/16 at policy-priority 2000, tunnel peer b configured with remote prefix 10.1.2.0/24 for any protocol at policy-priority 1500 that narrows its proposal to TCP to any port is answered TS_UNACCEPTABLE, because a remote half narrowed in protocol alone is the peer's choice.
	narrowedInsideConfigured(t, 0, 0, 65535)
}

// TestRFC3948NarrowedPortDistinctPriorityRefused is the port case: peer b's operator
// configured remote 10.1.2.0/24 for TCP to any port, and b narrows to TCP port 179, so
// its remote half differs from the configured one only in its port.
func TestRFC3948NarrowedPortDistinctPriorityRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- with tunnel peer a holding its configured remote prefix 10.1.0.0/16 at policy-priority 2000, tunnel peer b configured with remote prefix 10.1.2.0/24 for TCP to any port at policy-priority 1500 that narrows its proposal to TCP port 179 is answered TS_UNACCEPTABLE, because a remote half narrowed in port alone is the peer's choice.
	narrowedInsideConfigured(t, 6, 179, 179)
}

// narrowedInsideConfigured installs operator-fixed peer a (remote 10.1.0.0/16, priority
// 2000), then has peer b, configured with remote 10.1.2.0/24 for protocol proto (0 any)
// at priority 1500, propose TCP to the remote ports portLo to portHi, and asserts b's
// narrowing is refused.
func narrowedInsideConfigured(t *testing.T, proto uint8, portLo, portHi uint16) {
	t.Helper()
	reg := newInnerSelectorRegistry()
	a := innerPeerSA(t, reg, "a", "198.18.0.7", selectorPolicy(t, "10.1.0.0/16", "0.0.0.0/0", 0, 0))
	a.PeerCfg.PolicyPriority = 2000
	mustInnerChild(t, a, &mockDP{}, []string{"10.1.0.0/16"}, []string{"0.0.0.0/0"})

	b := innerPeerSA(t, reg, "b", "198.18.0.9", selectorPolicy(t, "10.1.2.0/24", "0.0.0.0/0", proto, 0))
	b.PeerCfg.PolicyPriority = 1500
	tsi := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, "10.1.2.0/24", portLo, portHi, 6),
	}}
	tsr := &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr, TrafficSelectors: []wire.TrafficSelector{
		wireSel(t, "0.0.0.0/0", 0, 65535, 6),
	}}
	mustInnerRefused(t, narrowChildSelectors(b, tsi, tsr, nil), "peer b")
}
