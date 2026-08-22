package engine

import (
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// zeRekeyFixture builds a live Child SA and the pending rekey ze sends to replace it.
//
// The configured policy is 10.1.0.0/24 (this node) <-> 10.2.0.0/24 (the peer), so that is
// what proposeChildTSPayloads puts in the rekey request. floorLocal and floorRemote are the
// two halves of the SCOPE IN USE that IKE_AUTH left behind, and a half narrower than the
// policy is the responder's narrowing of ze's own IKE_AUTH proposal. The gap between the
// two is what makes an answer distinguishable from the retired pair's selectors.
//
// ze is the IKE SA initiator and the rekey initiator, so this node's side is TSi in both
// exchanges.
func zeRekeyFixture(t *testing.T, floorLocal, floorRemote string) (*SA, *ChildSA, *pendingRekey, *mockDP, *slog.Logger) {
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
	// What IKE_AUTH left behind: ze proposed the policy and the responder answered
	// floorLocal for this node's half.
	sa.NegotiatedPairs = policyPairs(sa.PeerCfg, sa.IsInitiator)
	sa.NegotiatedPairs[0].I.Net = mustNet(t, floorLocal)
	sa.NegotiatedPairs[0].R.Net = mustNet(t, floorRemote)
	sa.NegotiatedTSi = sa.NegotiatedPairs[0].I.Net
	sa.NegotiatedTSr = sa.NegotiatedPairs[0].R.Net

	dp := &mockDP{}
	log := slogutil.DiscardLogger()
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	if !old.SelectorsLocalIsTSi {
		t.Fatal("the fixture must store this node's side as TSi; ze initiated the IKE SA " +
			"that negotiated these selectors")
	}
	_, pending, err := initiateChildRekey(sa, old)
	if err != nil {
		t.Fatalf("initiateChildRekey: %v", err)
	}
	if want := []string{"10.1.0.0/24 <-> 10.2.0.0/24"}; !slices.Equal(scopeText(sa.ProposedChildPairs), want) {
		t.Fatalf("ze proposed %v, want %v; the fixture must propose the whole policy or the "+
			"answers below are outside it for the wrong reason", scopeText(sa.ProposedChildPairs), want)
	}
	return sa, old, pending, dp, log
}

// peerRekeyAnswer builds the CREATE_CHILD_SA response a peer sends to ze's rekey. ze sent
// Ni, so this node's side is TSi and the peer's side is TSr.
func peerRekeyAnswer(t *testing.T, answeredTSi, answeredTSr string) []wire.PayloadEntry {
	t.Helper()
	return []wire.PayloadEntry{
		{Payload: espSAPayload(0x0A0B0C0D)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(4)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, answeredTSi)},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, answeredTSr)},
	}
}

// installedInitiatorScope returns a Child SA's installed selector set in the orientation of
// a ze-initiated exchange, where this node's side is TSi. It compares like with like
// against the TS payloads such a response carries.
func installedInitiatorScope(child *ChildSA) []tsPair {
	if child.SelectorsLocalIsTSi {
		return child.Selectors
	}
	return swapPairs(child.Selectors)
}

// installedPolicyScope renders the prefixes the KERNEL policy carries.
//
// childPolicyParams (child.go) builds SPParams.Src and SPParams.Dst from TSLocal and
// TSRemote, never from Selectors. A fix that moved only the selector set would leave the
// SPD exactly where the defect left it, so both are asserted.
func installedPolicyScope(child *ChildSA) string {
	return child.TSLocal.String() + " <-> " + child.TSRemote.String()
}

// VALIDATES: when ze INITIATES a Child SA rekey, the replacement is installed with the
// selectors the responder answered, whichever half of the pair the answer moved, and an
// answer outside what ze proposed is refused.
// PREVENTS: ze programming the retired pair's scope while the peer programs the answered
// one. Measured against strongSwan 5.9.14: charon logged "inbound CHILD_SA ze-child{2}
// established with ... TS 10.1.0.0/24 === 10.2.0.0/25" and programmed that, while ze's
// kernel kept 10.2.0.0/24 <-> 10.1.0.0/24. Traffic in the difference is protected at one
// end and dropped at the other, with no notification.
//
// RFC requirement: RFC7296-2.9-1 positive -- "TS payloads specify the selection criteria
// for packets that will be forwarded over the newly set up SA" (RFC 7296 S2.9,
// rfc/full/rfc7296.txt:2347-2348), and "IKEv2 allows the responder to choose a subset of
// the traffic proposed by the initiator" (rfc/full/rfc7296.txt:2381-2382). The answer is
// therefore the criteria of the SA ze just set up, so ze forwards over it what the answer
// named and not what the retired pair carried.
//
// RFC requirement: RFC7296-2.9-1 negative -- the discriminator. Narrowing is the only move
// Section 2.9 gives the responder, so an answer WIDER than the proposal is refused instead
// of adopted. Without this case the test would pass for a producer that installs whatever
// came back, which hands the peer the choice of the traffic ze protects.
func TestChildRekeyInitiatorInstallsTheAnsweredSelectors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		floorLocal  string
		floorRemote string
	}{
		{"the responder had narrowed this node's half", "10.1.0.0/25", "10.2.0.0/24"},
		{"the responder had narrowed the peer's half", "10.1.0.0/24", "10.2.0.0/25"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa, _, pending, dp, log := zeRekeyFixture(t, tc.floorLocal, tc.floorRemote)

			// The peer answers the whole proposal this time, which covers the scope in use
			// and stays inside what ze asked for. RFC 7296 Section 2.9 permits it, and it
			// differs from the retired pair's set, which is what makes the two visible.
			child, err := applyChildRekeyResponse(sa, pending, peerRekeyAnswer(t, "10.1.0.0/24", "10.2.0.0/24"), dp, log)
			if err != nil {
				t.Fatalf("applyChildRekeyResponse refused an answer inside the proposal: %v", err)
			}
			if child == nil {
				t.Fatal("no replacement Child SA was installed")
			}

			want := []string{"10.1.0.0/24 <-> 10.2.0.0/24"}
			if got := scopeText(installedInitiatorScope(child)); !slices.Equal(got, want) {
				t.Errorf("the replacement was installed with %v and the peer answered %v; the "+
					"two ends would program different traffic", got, want)
			}
			if got, wantPolicy := installedPolicyScope(child), "10.1.0.0/24 <-> 10.2.0.0/24"; got != wantPolicy {
				t.Errorf("the kernel policy selector is %s, want %s; childPolicyParams reads "+
					"TSLocal and TSRemote, so the SPD still carries the retired pair's scope",
					got, wantPolicy)
			}
		})
	}

	t.Run("an answer wider than the proposal is refused", func(t *testing.T) {
		sa, old, pending, dp, log := zeRekeyFixture(t, "10.1.0.0/24", "10.2.0.0/24")
		installedBefore := len(dp.sas)

		child, err := applyChildRekeyResponse(sa, pending, peerRekeyAnswer(t, "10.1.0.0/16", "10.2.0.0/24"), dp, log)
		if err == nil {
			t.Fatalf("applyChildRekeyResponse adopted an answer of %v, which ze never proposed",
				scopeText(installedInitiatorScope(child)))
		}
		if !errors.Is(err, errTSWidened) {
			t.Errorf("refusal = %v, want errTSWidened", err)
		}
		if child != nil {
			t.Error("a refused rekey response installed a replacement Child SA")
		}
		if len(dp.sas) != installedBefore {
			t.Errorf("a refused rekey response installed %d dataplane SAs", len(dp.sas)-installedBefore)
		}
		if got, want := installedPolicyScope(old), "10.1.0.0/24 <-> 10.2.0.0/24"; got != want {
			t.Errorf("the retired pair now carries %s, want %s; the refused answer moved the "+
				"live SA", got, want)
		}
	})
}

// VALIDATES: ze refuses a rekey answer that names a scope narrower than the SA it is
// replacing, whichever half of the pair the peer shrank, and the refusal names both
// selector sets. The SAME fixture accepts an answer that covers the scope in use.
// PREVENTS: ze installing a replacement Child SA the RFC forbids, which drops traffic the
// retired SA carried while nothing reports it.
//
// RFC requirement: RFC7296-2.9.2-1 positive -- "Thus, the new SA MUST NOT have narrower
// selectors than the original" (RFC 7296 S2.9.2, rfc/full/rfc7296.txt:2539-2540). The
// obligation binds the new SA, so it binds the end that INSTALLS one as much as the end
// that answers. ze declines to create it.
//
// RFC requirement: RFC7296-2.9.2-2 positive -- "The responder MUST NOT narrow down the
// Traffic Selectors narrower than the scope currently in use" (RFC 7296 S2.9.2,
// rfc/full/rfc7296.txt:2551-2552). A peer that answers below that scope has broken the
// MUST NOT, and RFC 7296 Section 2.21.3 tells the initiator not to answer a bad response
// with an INFORMATIONAL error, so ze abandons the exchange and keeps the SA in use.
//
// RFC requirement: RFC7296-2.9.2-1 negative -- the discriminator. The last case answers the
// scope in use and is installed. A refusal of every case would be a rekey path that is
// broken rather than a floor that is enforced.
//
// RFC requirement: RFC7296-2.9.2-2 negative -- the same discriminator for the responder's
// half of the obligation. An answer that keeps the scope in use has narrowed nothing below
// it, so ze must install it.
func TestChildRekeyAnswerBelowTheScopeInUseIsRefused(t *testing.T) {
	// Each answered half below is INSIDE the proposal, so it is a legal narrowing of it. It
	// covers no pair of the scope in use, which is what makes it illegal for a REKEY. Both
	// halves are driven: a floor test that reads one of them leaves the other unguarded.
	for _, tc := range []struct {
		name        string
		answeredTSi string
		answeredTSr string
		dropped     string
	}{
		{"the peer narrowed this node's half", "10.1.0.128/25", "10.2.0.0/24", "10.1.0.128/25"},
		{"the peer narrowed its own half", "10.1.0.0/24", "10.2.0.128/25", "10.2.0.128/25"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa, old, pending, dp, log := zeRekeyFixture(t, "10.1.0.0/24", "10.2.0.0/24")
			installedBefore := len(dp.sas)

			child, err := applyChildRekeyResponse(sa, pending, peerRekeyAnswer(t, tc.answeredTSi, tc.answeredTSr), dp, log)
			if err == nil {
				t.Fatalf("applyChildRekeyResponse installed %v for an SA whose scope in use is "+
					"10.1.0.0/24 <-> 10.2.0.0/24", scopeText(installedInitiatorScope(child)))
			}
			if !errors.Is(err, errTSUnacceptable) {
				t.Errorf("refusal = %v, want errTSUnacceptable", err)
			}
			if child != nil {
				t.Error("a refused rekey response installed a replacement Child SA")
			}
			if len(dp.sas) != installedBefore {
				t.Errorf("a refused rekey response installed %d dataplane SAs", len(dp.sas)-installedBefore)
			}
			for _, want := range []string{"10.1.0.0/24 <-> 10.2.0.0/24", tc.dropped} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal %q does not name %s; an operator cannot see which "+
						"prefix the peer dropped", err, want)
				}
			}
			if got, want := installedPolicyScope(old), "10.1.0.0/24 <-> 10.2.0.0/24"; got != want {
				t.Errorf("the retired pair now carries %s, want %s", got, want)
			}
		})
	}

	t.Run("an answer covering the scope in use is installed", func(t *testing.T) {
		sa, _, pending, dp, log := zeRekeyFixture(t, "10.1.0.0/24", "10.2.0.0/24")

		child, err := applyChildRekeyResponse(sa, pending, peerRekeyAnswer(t, "10.1.0.0/24", "10.2.0.0/24"), dp, log)
		if err != nil {
			t.Fatalf("applyChildRekeyResponse refused an answer that covers the scope in use: %v", err)
		}
		if child == nil {
			t.Fatal("no replacement Child SA was installed")
		}
		want := []string{"10.1.0.0/24 <-> 10.2.0.0/24"}
		if got := scopeText(installedInitiatorScope(child)); !slices.Equal(got, want) {
			t.Errorf("the replacement was installed with %v, want %v", got, want)
		}
	})
}

// VALIDATES: a CREATE_CHILD_SA rekey response that carries no traffic selectors is refused
// and installs nothing, while the same exchange carrying TSi and TSr is installed.
// PREVENTS: ze keeping the retired pair's selectors for a replacement whose scope the peer
// never stated. The peer programs its SPD from the SA it built, ze programs its own from an
// SA that no longer exists, and neither end reports the difference.
//
// RFC requirement: RFC7296-2.9-1 negative -- "TS payloads specify the selection criteria
// for packets that will be forwarded over the newly set up SA" (RFC 7296 S2.9,
// rfc/full/rfc7296.txt:2347-2348). RFC 7296 Section 1.3.3 puts TSi and TSr in the rekey
// RESPONSE for that reason (rfc/full/rfc7296.txt:918-919). A response that omits them
// states no criteria for the SA it just created, so there is nothing to install.
//
// RFC requirement: RFC7296-2.9-1 positive -- the discriminator. The same fixture, given a
// response that carries both payloads, installs the scope they name. A refusal of both
// would be a rekey path that is broken rather than a mandatory payload that is checked.
func TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused(t *testing.T) {
	t.Run("a response carrying no traffic selectors is refused", func(t *testing.T) {
		sa, _, pending, dp, log := zeRekeyFixture(t, "10.1.0.0/24", "10.2.0.0/24")
		installedBefore := len(dp.sas)

		inner := []wire.PayloadEntry{
			{Payload: espSAPayload(0x0A0B0C0D)},
			{Payload: &wire.PayloadNonce{NonceData: testNonce(4)}},
		}
		child, err := applyChildRekeyResponse(sa, pending, inner, dp, log)
		if err == nil {
			t.Fatalf("applyChildRekeyResponse installed %v for a response that named no scope",
				scopeText(installedInitiatorScope(child)))
		}
		if child != nil {
			t.Error("a refused rekey response installed a replacement Child SA")
		}
		if len(dp.sas) != installedBefore {
			t.Errorf("a refused rekey response installed %d dataplane SAs", len(dp.sas)-installedBefore)
		}
	})

	t.Run("a response carrying TSi and TSr is installed", func(t *testing.T) {
		sa, _, pending, dp, log := zeRekeyFixture(t, "10.1.0.0/24", "10.2.0.0/24")

		child, err := applyChildRekeyResponse(sa, pending, peerRekeyAnswer(t, "10.1.0.0/24", "10.2.0.0/24"), dp, log)
		if err != nil {
			t.Fatalf("applyChildRekeyResponse refused a complete response: %v", err)
		}
		if child == nil {
			t.Fatal("no replacement Child SA was installed")
		}
	})
}
