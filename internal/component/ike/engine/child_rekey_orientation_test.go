package engine

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"
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
//
// It is written to both fields createFirstChildSA writes from the IKE_AUTH role.
// SelectorsLocalIsTSi is the orientation selectorPort reads. LocalIsInitiator is the
// KEYMAT role. A fixture that left the orientation at its zero value runs both table
// cases in one orientation. No assertion then sees a wrong write of the flag.
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
		InboundSPI:          0x1111,
		OutboundSPI:         0x2222,
		LocalAddr:           net.ParseIP("172.28.0.2"),
		RemoteAddr:          net.ParseIP("172.28.0.3"),
		IfID:                7,
		ReqID:               7,
		TSLocal:             tsLocal,
		TSRemote:            tsRemote,
		Mode:                modeTunnel,
		Owner:               "peer-a",
		Selectors:           []tsPair{pair},
		SelectorsLocalIsTSi: localIsInitiator,
		LocalIsInitiator:    localIsInitiator,
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

// peerRekeyFixture builds a live Child SA and the IKE SA that carries it. The scope in
// use is 10.1.0.0/24 (this node) <-> 10.2.0.0/24 (the peer), and ze is the IKE SA
// INITIATOR, so IKE_AUTH left the stored selectors with this node's side as TSi.
//
// The peer initiates every rekey below, which is the case RFC 7296 Section 2.9.2 governs:
// the peer proposes, and this node is the responder that may not narrow below the scope
// in use.
func peerRekeyFixture(t *testing.T) (*SA, *ChildSA, *mockDP, *slog.Logger) {
	t.Helper()
	sa := testSAWithGCMKeys(t)
	sa.ESPGroup = testESPGroup()
	sa.PeerCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{{
		Number:       "1",
		LocalPrefix:  mustNet(t, "10.1.0.0/24"),
		LocalPort:    ipsec.AnyPort(),
		RemotePrefix: mustNet(t, "10.2.0.0/24"),
		RemotePort:   ipsec.AnyPort(),
	}}
	sa.NegotiatedPairs = policyPairs(sa.PeerCfg, sa.IsInitiator)
	sa.NegotiatedTSi = sa.NegotiatedPairs[0].I.Net
	sa.NegotiatedTSr = sa.NegotiatedPairs[0].R.Net

	dp := &mockDP{}
	log := slogutil.DiscardLogger()
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	return sa, old, dp, log
}

// peerRekeyRequest builds the CREATE_CHILD_SA rekey a peer sends. The peer sent Ni, so
// its own side is TSi.
func peerRekeyRequest(t *testing.T, proposedTSi, proposedTSr string) []wire.PayloadEntry {
	t.Helper()
	return []wire.PayloadEntry{
		{Payload: espSAPayload(0x01020304)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(3)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, proposedTSi)},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, proposedTSr)},
	}
}

// installedScope returns a Child SA's installed selector set in the orientation of a
// peer-initiated exchange, where the peer's side is TSi. It is the read that
// respondChildRekey performs to build its floor, so it compares like with like.
func installedScope(child *ChildSA) []tsPair {
	if child.SelectorsLocalIsTSi {
		return swapPairs(child.Selectors)
	}
	return child.Selectors
}

// scopeText renders a selector set as a sorted, comparable list.
//
// It is this file's OWN oracle and is written over fmt and slices, never over coversFloor
// or containsNet. A containment predicate that answered true for everything would leave
// an "answer equals install" assertion green while the two diverge, which is the defect
// under test.
func scopeText(pairs []tsPair) []string {
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, fmt.Sprintf("%s <-> %s", p.I.Net, p.R.Net))
	}
	slices.Sort(out)
	return out
}

// answeredScope decrypts a CREATE_CHILD_SA response and returns the selector set its TSi
// and TSr payloads announce.
//
// The response is sealed with this node's SEND keys, so it is read back through a peer's
// view of the same IKE SA: one SKKeys, the opposite role. That is what the far end does
// with it, and reading the wire is the point -- sa.NegotiatedPairs is the INPUT to the
// payload builder, not evidence that the builder used it.
func answeredScope(t *testing.T, sa *SA, raw []byte) []tsPair {
	t.Helper()
	peerView := *sa
	peerView.IsInitiator = !sa.IsInitiator

	var msg wire.Message
	if err := msg.ReadFrom(raw); err != nil {
		t.Fatalf("the rekey response does not parse: %v", err)
	}
	var sk *wire.PayloadSK
	for _, pe := range msg.Payloads {
		if s, ok := pe.Payload.(*wire.PayloadSK); ok {
			sk = s
		}
	}
	if sk == nil {
		t.Fatal("the rekey response carries no SK payload")
	}
	plain, err := decryptSKPayload(&peerView, raw, sk)
	if err != nil {
		t.Fatalf("decrypting the rekey response: %v", err)
	}
	inner, err := wire.ParsePayloadChain(plain, sk.InnerNextPayload)
	if err != nil {
		t.Fatalf("parsing the rekey response payloads: %v", err)
	}
	var tsi, tsr *wire.PayloadTS
	for _, pe := range inner {
		ts, ok := pe.Payload.(*wire.PayloadTS)
		if !ok {
			continue
		}
		switch ts.TSPayloadType {
		case wire.PayloadTypeTSi:
			tsi = ts
		case wire.PayloadTypeTSr:
			tsr = ts
		}
	}
	if tsi == nil || tsr == nil {
		t.Fatalf("the rekey response announces TSi=%v TSr=%v, want both", tsi, tsr)
	}
	iSels := wireToSelectors(tsi.TrafficSelectors)
	rSels := wireToSelectors(tsr.TrafficSelectors)
	if len(iSels) == 0 || len(rSels) == 0 {
		t.Fatalf("the announced payloads decode to %d TSi and %d TSr selectors", len(iSels), len(rSels))
	}
	pairs := make([]tsPair, 0, len(iSels))
	for i := range iSels {
		pairs = append(pairs, tsPair{I: iSels[i], R: rSels[min(i, len(rSels)-1)]})
	}
	return pairs
}

// VALIDATES: the TS payloads a Child SA rekey response announces name the selector set
// the replacement Child SA was installed with, whether the peer proposes the scope in use
// or a superset of it, and whichever orientation the retired pair stored.
// PREVENTS: ze announcing one scope on the wire and programming another in the kernel.
// The peer then builds its SPD from the answer, ze builds its own from the retired pair,
// and traffic inside the difference is protected at one end and dropped at the other with
// no notification.
//
// RFC requirement: RFC7296-2.9.2-1 positive -- "Thus, the new SA MUST NOT have narrower
// selectors than the original" (RFC 7296 S2.9.2, rfc/full/rfc7296.txt:2539-2540). The
// announced set is the original's set on every case here, including the one where the
// peer proposes a /16 and could be answered with more than the SA carries.
func TestRekeyAnswerMatchesTheInstalledSelectors(t *testing.T) {
	for _, tc := range []struct {
		name             string
		storedLocalIsTSi bool
		proposedTSi      string
		proposedTSr      string
	}{
		{"the peer proposes the scope in use", true, "10.2.0.0/24", "10.1.0.0/24"},
		{"the peer proposes a superset of it", true, "10.2.0.0/16", "10.1.0.0/16"},
		{"the retired pair was negotiated in the other orientation", false, "10.2.0.0/24", "10.1.0.0/24"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa, old, dp, log := peerRekeyFixture(t)
			if !old.SelectorsLocalIsTSi {
				t.Fatal("the fixture must store this node's side as TSi; the third case " +
					"turns it over and would otherwise test the first case twice")
			}
			if !tc.storedLocalIsTSi {
				// A tunnel the PEER established: IKE_AUTH stored the peer's side as TSi.
				old.Selectors = swapPairs(old.Selectors)
				old.SelectorsLocalIsTSi = false
			}

			resp, child, err := respondChildRekey(sa, peerRekeyRequest(t, tc.proposedTSi, tc.proposedTSr), old, 5, dp, log)
			if err != nil {
				t.Fatalf("respondChildRekey refused a rekey that covers the scope in use: %v", err)
			}
			if child == nil {
				t.Fatal("no replacement Child SA was installed")
			}

			announced := scopeText(answeredScope(t, sa, resp))
			installed := scopeText(installedScope(child))
			if !slices.Equal(announced, installed) {
				t.Errorf("the response announces %v and the replacement was installed with %v; "+
					"the peer would program one scope and ze the other", announced, installed)
			}
			if want := []string{"10.2.0.0/24 <-> 10.1.0.0/24"}; !slices.Equal(announced, want) {
				t.Errorf("the response announces %v, want %v (the scope in use, neither widened "+
					"to the proposal nor narrowed below the SA)", announced, want)
			}
		})
	}
}

// VALIDATES: a peer-initiated Child SA rekey whose proposal covers no pair of the scope
// in use is refused with TS_UNACCEPTABLE, and the refusal names both selector sets so an
// operator can see which prefix the peer dropped. The SAME fixture accepts a proposal that
// covers the scope in use.
// PREVENTS: ze answering the intersection, which is the responder narrowing below the
// scope in use, and installing the retired pair's wider set at the same time.
//
// RFC requirement: RFC7296-2.9.2-2 positive -- "The responder MUST NOT narrow down the
// Traffic Selectors narrower than the scope currently in use" (RFC 7296 S2.9.2,
// rfc/full/rfc7296.txt:2551-2552). A proposal covering no pair of that scope leaves no
// legal narrowing, because Section 2.9 refuses an answer wider than the proposal, so the
// exchange draws the TS_UNACCEPTABLE that Section 2.9 names.
//
// RFC requirement: RFC7296-2.9.2-2 negative -- the discriminator. The second case differs
// only in the proposed TSi, covers the scope in use, and is answered and installed. A
// refusal of both cases would be a rekey path that is broken rather than a floor that is
// enforced.
func TestRekeyProposalBelowTheFloorIsRefused(t *testing.T) {
	t.Run("a proposal below the scope in use is refused", func(t *testing.T) {
		sa, old, dp, log := peerRekeyFixture(t)
		before := scopeText(sa.NegotiatedPairs)
		installedBefore := len(dp.sas)

		// 10.2.0.128/25 is INSIDE the peer's half of the scope in use, so it intersects
		// the configured policy and narrows to a non-empty set. It covers no pair of the
		// scope in use, which is what makes that non-empty set illegal.
		resp, child, err := respondChildRekey(sa, peerRekeyRequest(t, "10.2.0.128/25", "10.1.0.0/24"), old, 5, dp, log)
		if err == nil {
			t.Fatalf("respondChildRekey answered a proposal that covers no pair of the scope "+
				"in use, announcing %v", scopeText(answeredScope(t, sa, resp)))
		}
		if !errors.Is(err, errTSUnacceptable) {
			t.Errorf("refusal = %v, want errTSUnacceptable", err)
		}
		if got := notifyForRefusal(err); got != wire.NotifyTSUnacceptable {
			t.Errorf("notify = %d, want TS_UNACCEPTABLE (%d)", got, wire.NotifyTSUnacceptable)
		}
		if child != nil {
			t.Error("a refused rekey installed a replacement Child SA")
		}
		if len(dp.sas) != installedBefore {
			t.Errorf("a refused rekey installed %d dataplane SAs", len(dp.sas)-installedBefore)
		}
		for _, want := range []string{"10.2.0.0/24", "10.2.0.128/25"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal %q does not name %s; an operator cannot see which "+
					"prefix the peer dropped", err, want)
			}
		}
		if after := scopeText(sa.NegotiatedPairs); !slices.Equal(after, before) {
			t.Errorf("the refused rekey left the negotiated scope at %v, was %v; the next "+
				"response would announce the refused set", after, before)
		}
	})

	t.Run("a proposal covering the scope in use is answered", func(t *testing.T) {
		sa, old, dp, log := peerRekeyFixture(t)
		resp, child, err := respondChildRekey(sa, peerRekeyRequest(t, "10.2.0.0/24", "10.1.0.0/24"), old, 5, dp, log)
		if err != nil {
			t.Fatalf("respondChildRekey refused a proposal that covers the scope in use: %v", err)
		}
		if child == nil {
			t.Fatal("no replacement Child SA was installed")
		}
		announced := scopeText(answeredScope(t, sa, resp))
		if want := []string{"10.2.0.0/24 <-> 10.1.0.0/24"}; !slices.Equal(announced, want) {
			t.Errorf("the response announces %v, want %v", announced, want)
		}
	})
}
