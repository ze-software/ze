// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the cross-peer inner-selector guard
// Related: inner_selectors.go -- classifyInnerPair and innerSelectorRegistry.claim
// VALIDATES: a transport-mode pair at a host other than the peer's configured address is
// the peer's choice and is refused beside a configured tunnel at a distinct priority;
// and installs that all passed narrowing for one inner address, run at once, leave
// exactly one Child SA recorded; a tunnel peer's own host pair, and a configured
// selector the peer narrowed on its local half (port or prefix), are the peer's choice.
// PREVENTS: the configured-host clause admitting any transport pair, or admitting a
// tunnel; a configured-selector match that ignores the local port or the local prefix;
// and a claim whose check and record are not one critical section letting two peers
// hold one address.

package engine

import (
	"strconv"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC3948TransportPeerChosenHostBesideConfiguredTunnelRefused: transport-mode peer b
// (remote address 198.18.0.9, no traffic selector, policy-priority 1000) proposes TSi
// 10.1.2.3/32, an address that is not its configured one, and TSr 192.0.2.1/32; tunnel
// peer a holds its configured 10.1.2.0/24 at policy-priority 2000. b's pair is not the
// pair between b's configured addresses, so it is b's choice and the priorities do not
// order it. Method: both orders through narrowChildSelectors and createFirstChildSA; the
// second must be answered TS_UNACCEPTABLE with nothing on its dataplane.
func TestRFC3948TransportPeerChosenHostBesideConfiguredTunnelRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- transport-mode peer b with remote address 198.18.0.9, no traffic selector and policy-priority 1000 proposing TSi 10.1.2.3/32, TSr 192.0.2.1/32, and tunnel peer a configured with remote prefix 10.1.2.0/24 at policy-priority 2000 proposing exactly it: whichever proposes second is answered TS_UNACCEPTABLE with no state or policy on its dataplane, the registry holding the first alone.
	for _, transportFirst := range []bool{true, false} {
		name := "tunnel first"
		if transportFirst {
			name = "transport first"
		}
		t.Run(name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			a := innerPeerSA(t, reg, "a", "198.18.0.7", selectorPolicy(t, "10.1.2.0/24", "0.0.0.0/0", 0, 0))
			a.PeerCfg.PolicyPriority = 2000
			b := transportPeerSA(t, reg, "b")
			b.PeerCfg.PolicyPriority = 1000
			aTS := [2][]string{{"10.1.2.0/24"}, fullTunnelTS}
			bTS := [2][]string{{"10.1.2.3/32"}, transportTSr}
			first, second := a, b
			firstTS, secondTS := aTS, bTS
			if transportFirst {
				first, second = b, a
				firstTS, secondTS = bTS, aTS
			}
			mustInnerChild(t, first, &mockDP{}, firstTS[0], firstTS[1])
			dp := &mockDP{}
			_, err := innerAuthChild(t, second, dp, secondTS[0], secondTS[1])
			mustInnerRefused(t, err, "peer "+second.PeerName)
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

// TestRFC3948ConcurrentInstallsHoldOneInnerAddress: sixteen unbound tunnel peers have all
// passed narrowing for inner address 10.1.2.3/32 (TSr 0.0.0.0/0) and install at once.
// Method: each peer's negotiated pair is set as narrowing leaves it, then every peer
// calls createFirstChildSA from its own goroutine released by one channel close; over
// many rounds the registry must hold exactly one Child SA each time.
func TestRFC3948ConcurrentInstallsHoldOneInnerAddress(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- sixteen unbound tunnel peers that all passed narrowing for inner address 10.1.2.3/32 (TSr 0.0.0.0/0) install concurrently through createFirstChildSA: in every one of 100 rounds the registry holds exactly one Child SA.
	const peers = 16
	for round := range 100 {
		reg := newInnerSelectorRegistry()
		sas := make([]*SA, peers)
		for i := range sas {
			sas[i] = innerPeerSA(t, reg, "peer-"+strconv.Itoa(i), "198.18.0.7")
			sas[i].NegotiatedPairs = []tsPair{{
				I: tsSelector{Net: mustNet(t, "10.1.2.3/32")},
				R: tsSelector{Net: mustNet(t, "0.0.0.0/0")},
			}}
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range sas {
			wg.Add(1)
			go installAfter(&wg, start, sas[i])
		}
		close(start)
		wg.Wait()
		if got := reg.holdings(); got != 1 {
			t.Fatalf("round %d: registry holds %d Child SAs for one inner address, want 1", round, got)
		}
	}
}

// installAfter waits for start, then installs sa's first Child SA; a refusal is the
// expected answer for all but one peer, and the registry count is what the test reads.
func installAfter(wg *sync.WaitGroup, start <-chan struct{}, sa *SA) {
	defer wg.Done()
	<-start
	//nolint:errcheck // the outcome is read from the registry count, not from each peer's answer.
	createFirstChildSA(sa, sa.ESPGroup, sa.PeerCfg.LocalAddress, sa.PeerCfg.RemoteAddress, 0, &mockDP{},
		slogutil.DiscardLogger())
}

// proposal is one peer's TSi and TSr as they arrive in IKE_AUTH.
type proposal struct {
	tsi, tsr *wire.PayloadTS
}

// prefixProposal is a proposal of one prefix on each half, every protocol and port.
func prefixProposal(t *testing.T, tsi, tsr string) proposal {
	t.Helper()
	return proposal{tsi: tsPayload(t, wire.PayloadTypeTSi, tsi), tsr: tsPayload(t, wire.PayloadTypeTSr, tsr)}
}

// proposeChild runs the IKE_AUTH responder's two steps for p, as innerAuthChild does, for
// a proposal that may carry a protocol and ports.
func proposeChild(t *testing.T, sa *SA, dp *mockDP, p proposal) error {
	t.Helper()
	if err := narrowChildSelectors(sa, p.tsi, p.tsr, nil); err != nil {
		return err
	}
	ifID, err := resolveIfID(&sa.PeerCfg)
	if err != nil {
		t.Fatalf("resolveIfID: %v", err)
	}
	_, err = createFirstChildSA(sa, sa.ESPGroup, sa.PeerCfg.LocalAddress, sa.PeerCfg.RemoteAddress, ifID, dp,
		slogutil.DiscardLogger())
	return err
}

// mustSecondRefused proposes first's proposal, which must be served, then second's, which
// must be answered TS_UNACCEPTABLE with nothing on second's dataplane and the registry
// holding first's Child SA alone.
func mustSecondRefused(t *testing.T, reg *innerSelectorRegistry, first, second *SA, firstP, secondP proposal) {
	t.Helper()
	if err := proposeChild(t, first, &mockDP{}, firstP); err != nil {
		t.Fatalf("peer %s: %v", first.PeerName, err)
	}
	dp := &mockDP{}
	mustInnerRefused(t, proposeChild(t, second, dp, secondP), "peer "+second.PeerName)
	if len(dp.sas) != 0 || len(dp.policies) != 0 {
		t.Fatalf("peer %s's dataplane got %d states and %d policies, want none",
			second.PeerName, len(dp.sas), len(dp.policies))
	}
	if got := reg.holdings(); got != 1 {
		t.Fatalf("registry holds %d Child SAs, want 1 (peer %s)", got, first.PeerName)
	}
}

// TestRFC3948TunnelPeerOwnHostPairBesideFullTunnelRefused: tunnel peer c (remote address
// 198.18.0.11, no traffic selector, policy-priority 1000) proposes TSi 198.18.0.11/32,
// TSr 192.0.2.1/32, exactly the pair between its two configured addresses; peer a holds
// its configured 0.0.0.0/0 to 0.0.0.0/0 at policy-priority 2000. The configured-host
// clause names a transport-mode connection's addresses only: a tunnel with no selector
// configured no inner pair, so c's pair is its choice and the priorities do not order it.
// Method: both orders through narrowChildSelectors and createFirstChildSA; the second
// must be answered TS_UNACCEPTABLE with nothing on its dataplane.
func TestRFC3948TunnelPeerOwnHostPairBesideFullTunnelRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- tunnel peer c with remote address 198.18.0.11, no traffic selector and policy-priority 1000 proposing TSi 198.18.0.11/32, TSr 192.0.2.1/32, and tunnel peer a configured with remote and local prefix 0.0.0.0/0 at policy-priority 2000 proposing exactly it: whichever proposes second is answered TS_UNACCEPTABLE with no state or policy on its dataplane, the registry holding the first alone.
	for _, tunnelHostFirst := range []bool{true, false} {
		name := "full tunnel first"
		if tunnelHostFirst {
			name = "host pair first"
		}
		t.Run(name, func(t *testing.T) {
			reg := newInnerSelectorRegistry()
			a := innerPeerSA(t, reg, "a", "198.18.0.7", selectorPolicy(t, "0.0.0.0/0", "0.0.0.0/0", 0, 0))
			a.PeerCfg.PolicyPriority = 2000
			c := innerPeerSA(t, reg, "c", "198.18.0.11")
			c.PeerCfg.PolicyPriority = 1000
			aP := prefixProposal(t, "0.0.0.0/0", "0.0.0.0/0")
			cP := prefixProposal(t, "198.18.0.11/32", "192.0.2.1/32")
			if tunnelHostFirst {
				mustSecondRefused(t, reg, c, a, cP, aP)
				return
			}
			mustSecondRefused(t, reg, a, c, aP, cP)
		})
	}
}

// TestRFC3948ConfiguredSelectorNarrowedOnLocalPortRefused: peer b configured remote
// 10.1.2.0/24, local 0.0.0.0/0, protocol TCP, at policy-priority 1500 proposes TSi
// 10.1.2.0/24 TCP and TSr 0.0.0.0/0 TCP port 80; peer a holds its configured 10.1.0.0/16
// at policy-priority 2000. b's pair differs from its configured selector only by the
// local port, so it is b's choice. Method: a first, then b through narrowChildSelectors
// and createFirstChildSA; b must be answered TS_UNACCEPTABLE with nothing on its dataplane.
func TestRFC3948ConfiguredSelectorNarrowedOnLocalPortRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- tunnel peer b configured with remote prefix 10.1.2.0/24, local prefix 0.0.0.0/0 and protocol TCP at policy-priority 1500 proposing TSi 10.1.2.0/24 TCP, TSr 0.0.0.0/0 TCP port 80, after tunnel peer a configured with remote prefix 10.1.0.0/16 at policy-priority 2000 installed it: b is answered TS_UNACCEPTABLE with no state or policy on its dataplane, the registry holding a alone.
	reg := newInnerSelectorRegistry()
	a := innerPeerSA(t, reg, "a", "198.18.0.7", selectorPolicy(t, "10.1.0.0/16", "0.0.0.0/0", 0, 0))
	a.PeerCfg.PolicyPriority = 2000
	b := innerPeerSA(t, reg, "b", "198.18.0.9", selectorPolicy(t, "10.1.2.0/24", "0.0.0.0/0", 6, 0))
	b.PeerCfg.PolicyPriority = 1500
	bP := proposal{
		tsi: &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSi,
			TrafficSelectors: []wire.TrafficSelector{wireSel(t, "10.1.2.0/24", 0, 65535, 6)}},
		tsr: &wire.PayloadTS{TSPayloadType: wire.PayloadTypeTSr,
			TrafficSelectors: []wire.TrafficSelector{wireSel(t, "0.0.0.0/0", 80, 80, 6)}},
	}
	mustSecondRefused(t, reg, a, b, prefixProposal(t, "10.1.0.0/16", "0.0.0.0/0"), bP)
}

// TestRFC3948ConfiguredSelectorNarrowedOnLocalPrefixRefused: peer b configured remote
// 10.1.2.0/24, local 0.0.0.0/0, at policy-priority 1500 proposes TSi 10.1.2.0/24 and TSr
// 192.0.2.0/24; peer a holds its configured 10.1.0.0/16 at policy-priority 2000. b's pair
// differs from its configured selector only by the local prefix, so it is b's choice.
// Method: a first, then b through narrowChildSelectors and createFirstChildSA; b must be
// answered TS_UNACCEPTABLE with nothing on its dataplane.
func TestRFC3948ConfiguredSelectorNarrowedOnLocalPrefixRefused(t *testing.T) {
	// RFC requirement: RFC3948-5.1-1 negative -- tunnel peer b configured with remote prefix 10.1.2.0/24 and local prefix 0.0.0.0/0 at policy-priority 1500 proposing TSi 10.1.2.0/24, TSr 192.0.2.0/24, after tunnel peer a configured with remote prefix 10.1.0.0/16 at policy-priority 2000 installed it: b is answered TS_UNACCEPTABLE with no state or policy on its dataplane, the registry holding a alone.
	reg := newInnerSelectorRegistry()
	a := innerPeerSA(t, reg, "a", "198.18.0.7", selectorPolicy(t, "10.1.0.0/16", "0.0.0.0/0", 0, 0))
	a.PeerCfg.PolicyPriority = 2000
	b := innerPeerSA(t, reg, "b", "198.18.0.9", selectorPolicy(t, "10.1.2.0/24", "0.0.0.0/0", 0, 0))
	b.PeerCfg.PolicyPriority = 1500
	mustSecondRefused(t, reg, a, b, prefixProposal(t, "10.1.0.0/16", "0.0.0.0/0"),
		prefixProposal(t, "10.1.2.0/24", "192.0.2.0/24"))
}
