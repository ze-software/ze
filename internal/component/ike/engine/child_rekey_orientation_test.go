package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// asymmetricPortChild builds an installed Child SA whose negotiated selector carries a
// DIFFERENT port on each side, which is what makes an orientation error observable.
//
// localIsInitiator says which exchange produced the selector set: when it is true this
// node sent TSi, so the I half of Selectors is this node's side.
func asymmetricPortChild(t *testing.T, localIsInitiator bool) *ChildSA {
	t.Helper()
	_, tsLocal, err := net.ParseCIDR("10.1.0.0/24")
	if err != nil {
		t.Fatalf("parsing the local traffic selector: %v", err)
	}
	_, tsRemote, err := net.ParseCIDR("10.2.0.0/24")
	if err != nil {
		t.Fatalf("parsing the remote traffic selector: %v", err)
	}
	local := tsSelector{Net: tsLocal, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 500}, Proto: 17}
	remote := tsSelector{Net: tsRemote, Port: ipsec.PortSelector{Form: ipsec.PortSingle, Port: 4500}, Proto: 17}
	pair := tsPair{I: local, R: remote}
	if !localIsInitiator {
		pair = tsPair{I: remote, R: local}
	}
	return &ChildSA{
		InboundSPI:       0x1111,
		OutboundSPI:      0x2222,
		LocalAddr:        net.ParseIP("172.28.0.2"),
		RemoteAddr:       net.ParseIP("172.28.0.3"),
		IfID:             7,
		ReqID:            7,
		TSLocal:          tsLocal,
		TSRemote:         tsRemote,
		Mode:             modeTunnel,
		Owner:            "peer-a",
		Selectors:        []tsPair{pair},
		LocalIsInitiator: localIsInitiator,
	}
}

// VALIDATES: a rekey whose exchange role differs from the retired pair's leaves the
// POLICY selector where it was. The replacement's local port stays the local port.
// PREVENTS: a role-flipping rekey installing a port-swapped policy, so the kernel
// protects the peer's port as if it were ours and drops the traffic the tunnel exists
// for. It also makes samePolicySelector answer false for two pairs that share one
// policy, so retiring the superseded pair strips the live pair's policy.
//
// NOT TAGGED with an RFC requirement id, deliberately. What it pins is the ORIENTATION
// of an installed SPD selector, and the nearest checklist rows are about what the
// responder puts on the WIRE (RFC 7296 Section 2.9.2) or about the SPD selector set RFC
// 4301 Section 4.4.1.1 requires an implementation to support. This test exercises
// neither producer, and a tag naming a requirement the assertion does not drive is worth
// less than no tag (ai/rules/evidence.md).
func TestRekeyKeepsThePolicyOrientationOfTheRetiredPair(t *testing.T) {
	for _, tc := range []struct {
		name              string
		oldLocalInitiator bool
		rekeyInitiator    bool
	}{
		{"peer rekeys a ze-initiated child", true, false},
		{"ze rekeys a peer-initiated child", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := asymmetricPortChild(t, tc.oldLocalInitiator)
			replacement := newRekeyedChild(old, 0x3333, 0x4444, &crypto.ChildSAKeys{}, tc.rekeyInitiator)

			for _, side := range []struct {
				name  string
				local bool
			}{{"local", true}, {"remote", false}} {
				want := selectorPort(old, side.local)
				got := selectorPort(replacement, side.local)
				if got != want {
					t.Errorf("the replacement's %s port is %+v, want %+v; the rekey moved a "+
						"selector that did not move", side.name, got, want)
				}
			}

			if !samePolicySelector(old, replacement) {
				t.Error("samePolicySelector says the replacement answers to a DIFFERENT policy " +
					"than the pair it replaces; retiring the old pair will strip the live policy")
			}

			for _, dir := range []dataplane.SADir{dataplane.SADirOut, dataplane.SADirIn} {
				oldSP, newSP := childPolicyParams(old, dir), childPolicyParams(replacement, dir)
				if oldSP.SrcPort != newSP.SrcPort || oldSP.DstPort != newSP.DstPort {
					t.Errorf("dir %v: the replacement installs ports src=%+v dst=%+v where the "+
						"retired pair installed src=%+v dst=%+v",
						dir, newSP.SrcPort, newSP.DstPort, oldSP.SrcPort, oldSP.DstPort)
				}
			}
		})
	}
}

// VALIDATES: ze answers a peer-initiated Child SA rekey against the orientation of THAT
// exchange. The peer sent Ni, so the peer's TSi is the peer's own side, whichever end
// initiated the IKE SA.
// PREVENTS: ze refusing every peer-initiated Child SA rekey with TS_UNACCEPTABLE on a
// tunnel ze itself initiated, because the configured policy and the rekey floor are both
// oriented by the IKE SA role while the request is oriented by the exchange role. The
// tunnel then dies at the peer's first rekey timer.
//
// RFC requirement: RFC7296-2.9-2 positive -- "If the responder's policy allows it to
// accept the first selector of TSi and TSr, then the responder MUST narrow the Traffic
// Selectors to a subset that includes the initiator's first choices" (S2.9). The policy
// here allows exactly the pair the peer proposes, so the answer must include it.
func TestPeerInitiatedRekeyIsNarrowedInTheExchangeOrientation(t *testing.T) {
	sa := testSAWithGCMKeys(t)
	sa.ESPGroup = testESPGroup()
	if !sa.IsInitiator {
		t.Fatal("the fixture must be the IKE SA INITIATOR; the whole point is that the " +
			"exchange role and the IKE SA role disagree")
	}
	sa.PeerCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{{
		Number:       "1",
		LocalPrefix:  mustNet(t, "10.1.0.0/24"),
		LocalPort:    ipsec.AnyPort(),
		RemotePrefix: mustNet(t, "10.2.0.0/24"),
		RemotePort:   ipsec.AnyPort(),
	}}
	// What IKE_AUTH left behind: ze was the initiator there, so TSi is ze's side.
	sa.NegotiatedPairs = policyPairs(sa.PeerCfg, sa.IsInitiator)
	sa.NegotiatedTSi = sa.NegotiatedPairs[0].I.Net
	sa.NegotiatedTSr = sa.NegotiatedPairs[0].R.Net

	dp := &mockDP{}
	log := slogutil.DiscardLogger()
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	// The peer initiates the rekey, so ITS side is TSi.
	inner := []wire.PayloadEntry{
		{Payload: espSAPayload(0x01020304)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(3)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, "10.2.0.0/24")},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, "10.1.0.0/24")},
	}
	_, child, err := respondChildRekey(sa, inner, old, 5, dp, log)
	if err != nil {
		t.Fatalf("respondChildRekey refused a rekey the configured policy allows: %v", err)
	}
	if child == nil {
		t.Fatal("no replacement Child SA was installed")
	}
	if got := sa.NegotiatedTSi.String(); got != "10.2.0.0/24" {
		t.Errorf("answered TSi = %s, want 10.2.0.0/24 (the rekey initiator's own side)", got)
	}
	if got := sa.NegotiatedTSr.String(); got != "10.1.0.0/24" {
		t.Errorf("answered TSr = %s, want 10.1.0.0/24 (this node's side)", got)
	}
}
