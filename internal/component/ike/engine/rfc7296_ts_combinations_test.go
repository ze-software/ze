// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- TSi and TSr are read as every combination of their entries
// RFC: rfc/short/rfc4301.md -- RFC4301-4.4.2-2
// RFC: rfc/short/rfc7296.md -- RFC7296-2.9-1

package engine

import (
	"errors"
	"net"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// RFC 7296 Section 3.13: "There is no requirement that TSi and TSr contain the same number
// of individual Traffic Selectors. Thus, they are interpreted as follows: a packet matches
// a given TSi/TSr if it matches at least one of the individual selectors in TSi, and at
// least one of the individual selectors in TSr." Every test in this file reads a TS
// payload pair that way: as each TSi entry with each TSr entry, never entry i with entry i.

const (
	comboA = "10.1.0.0/16"
	comboB = "10.4.0.0/16"
	comboC = "10.2.0.0/16"
	comboD = "10.3.0.0/16"
)

func anySel(t *testing.T, cidr string) tsSelector {
	t.Helper()
	return tsSelector{Net: mustNet(t, cidr), Port: ipsec.AnyPort()}
}

func comboPolicy(t *testing.T, number, local, remote string) ipsec.TrafficSelectorPolicy {
	t.Helper()
	return ipsec.TrafficSelectorPolicy{
		Number: number, LocalPrefix: mustNet(t, local), LocalPort: ipsec.AnyPort(),
		RemotePrefix: mustNet(t, remote), RemotePort: ipsec.AnyPort(),
	}
}

// wireCombinations renders every TSi-entry/TSr-entry combination of an answer as
// "TSi-start-end <-> TSr-start-end", which is the traffic RFC 7296 Section 3.13 says the
// two payloads carry. It reads the wire bytes, not the pairs the code recorded.
func wireCombinations(tsi, tsr *wire.PayloadTS) []string {
	var out []string
	for _, i := range tsi.TrafficSelectors {
		for _, r := range tsr.TrafficSelectors {
			out = append(out, net.IP(i.StartAddress).String()+"-"+net.IP(i.EndAddress).String()+
				" <-> "+net.IP(r.StartAddress).String()+"-"+net.IP(r.EndAddress).String())
		}
	}
	slices.Sort(out)
	return out
}

// TestRFC4301InitiatorInstallsEveryAnsweredCombination drives recordInitiatorSelectors, the
// producer of NegotiatedPairs on every initiator path.
//
// VALIDATES: an answer of one TSi selector and two TSr selectors is recorded as both
// combinations, and an answer whose combinations reach past the proposal is refused.
// PREVENTS: the positional reading that paired TSi[0] with TSr[0] only, which installed
// A-C of the answer TSi {A}, TSr {C, D} and dropped A-D (the strongSwan shape of one
// local subnet and two remote ones), and accepted A-D and B-C it never proposed.
func TestRFC4301InitiatorInstallsEveryAnsweredCombination(t *testing.T) {
	// RFC requirement: RFC4301-4.4.2-2 positive -- an initiator answered with one TSi selector and two TSr selectors records one negotiated pair for each of the two combinations, so the SAD entry is populated with every value the answer negotiated.
	sa := &SA{IsInitiator: true, ProposedChildPairs: []tsPair{
		{I: anySel(t, comboA), R: anySel(t, comboC)},
		{I: anySel(t, comboA), R: anySel(t, comboD)},
	}}
	if err := recordInitiatorSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, comboA),
		tsPayload(t, wire.PayloadTypeTSr, comboC, comboD), nil); err != nil {
		t.Fatalf("the answer TSi {A}, TSr {C, D} to the proposal of exactly those pairs was refused: %v", err)
	}
	want := comboA + " <-> " + comboC + ", " + comboA + " <-> " + comboD
	if got := pairsText(sa.NegotiatedPairs); got != want {
		t.Errorf("negotiated pairs = %q, want %q: every TSi entry pairs with every TSr entry (RFC 7296 Section 3.13)", got, want)
	}

	// RFC requirement: RFC4301-4.4.2-2 negative -- an initiator that proposed only the pairs (A, C) and (B, D) and is answered TSi {A, B}, TSr {C, D} refuses the answer and records no negotiated pair, because the answer also negotiates A-D and B-C, which no SAD entry of the proposal can hold.
	split := &SA{IsInitiator: true, ProposedChildPairs: []tsPair{
		{I: anySel(t, comboA), R: anySel(t, comboC)},
		{I: anySel(t, comboB), R: anySel(t, comboD)},
	}}
	err := recordInitiatorSelectors(split,
		tsPayload(t, wire.PayloadTypeTSi, comboA, comboB),
		tsPayload(t, wire.PayloadTypeTSr, comboC, comboD), nil)
	if !errors.Is(err, errTSWidened) {
		t.Errorf("answer negotiating A-D and B-C past a proposal of A-C and B-D: err = %v, want errTSWidened", err)
	}
	if split.NegotiatedPairs != nil {
		t.Errorf("a refused answer recorded %s", pairsText(split.NegotiatedPairs))
	}
}

// TestRFC7296ResponderAnswersOnlyCombinationsItChose drives narrowChildSelectors and
// pairsToWire, the responder's narrowing and the payloads it answers with.
//
// VALIDATES: a policy accepting (A, C) and (B, D) of the proposal TSi {A, B}, TSr {C, D}
// is answered with payloads whose every combination is a pair the policy accepts, and
// the pairs recorded for install are exactly those combinations. A policy accepting all
// four combinations is answered with all four.
// PREVENTS: the answer TSi {A, B}, TSr {C, D}, which hands back the whole proposal and so
// negotiates A-D and B-C, while the responder installed only A-C and B-D.
func TestRFC7296ResponderAnswersOnlyCombinationsItChose(t *testing.T) {
	split := ipsec.SiteToSitePeer{Name: "p", TrafficSelectors: []ipsec.TrafficSelectorPolicy{
		comboPolicy(t, "1", comboC, comboA),
		comboPolicy(t, "2", comboD, comboB),
	}}
	sa := &SA{PeerCfg: split}
	if err := narrowChildSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, comboA, comboB),
		tsPayload(t, wire.PayloadTypeTSr, comboC, comboD), nil); err != nil {
		t.Fatalf("a proposal the policy partly accepts was refused: %v", err)
	}
	tsi, tsr := pairsToWire(sa.NegotiatedPairs)
	if tsi == nil || tsr == nil {
		t.Fatalf("the narrowed set %s has no TS payload pair", pairsText(sa.NegotiatedPairs))
	}

	// RFC requirement: RFC7296-2.9-1 positive -- a responder whose policy accepts only (A, C) and (B, D) of the proposal TSi {A, B}, TSr {C, D} answers TS payloads every combination of which is a pair the policy accepts, so the answer narrows the traffic to a subset it chose instead of handing back the whole proposal.
	allowed := []string{
		"10.1.0.0-10.1.255.255 <-> 10.2.0.0-10.2.255.255",
		"10.4.0.0-10.4.255.255 <-> 10.3.0.0-10.3.255.255",
	}
	combos := wireCombinations(tsi, tsr)
	for _, c := range combos {
		if !slices.Contains(allowed, c) {
			t.Errorf("the answer negotiates %s, which the policy does not accept (answer combinations %v)", c, combos)
		}
	}
	if got := pairsText(sa.NegotiatedPairs); got != comboA+" <-> "+comboC {
		t.Errorf("recorded pairs = %q, want the first choice %s <-> %s alone", got, comboA, comboC)
	}

	// RFC requirement: RFC7296-2.9-1 negative -- a responder whose policy accepts all four combinations answers the same proposal with all four, so the narrowing above is a decision about that policy rather than an unconditional cut.
	full := ipsec.SiteToSitePeer{Name: "p", TrafficSelectors: []ipsec.TrafficSelectorPolicy{
		comboPolicy(t, "1", comboC, comboA),
		comboPolicy(t, "2", comboD, comboA),
		comboPolicy(t, "3", comboC, comboB),
		comboPolicy(t, "4", comboD, comboB),
	}}
	whole := &SA{PeerCfg: full}
	if err := narrowChildSelectors(whole,
		tsPayload(t, wire.PayloadTypeTSi, comboA, comboB),
		tsPayload(t, wire.PayloadTypeTSr, comboC, comboD), nil); err != nil {
		t.Fatalf("a proposal the policy wholly accepts was refused: %v", err)
	}
	if got := len(whole.NegotiatedPairs); got != 4 {
		t.Errorf("recorded %d pairs (%s), want all 4 combinations", got, pairsText(whole.NegotiatedPairs))
	}
}

// TestUnconfiguredResponderAnswersEveryCombination checks the allow-everything branch.
//
// VALIDATES: with no policy, the proposal TSi {A}, TSr {C, D} is recorded as both
// combinations and answered with both TSr entries.
// PREVENTS: the positional reading that answered and installed A-C only.
func TestUnconfiguredResponderAnswersEveryCombination(t *testing.T) {
	sa := &SA{PeerCfg: ipsec.SiteToSitePeer{Name: "p"}}
	if err := narrowChildSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, comboA),
		tsPayload(t, wire.PayloadTypeTSr, comboC, comboD), nil); err != nil {
		t.Fatalf("an unconfigured responder refused a proposal: %v", err)
	}
	if got := len(sa.NegotiatedPairs); got != 2 {
		t.Fatalf("recorded %d pairs (%s), want both combinations", got, pairsText(sa.NegotiatedPairs))
	}
	_, tsr := pairsToWire(sa.NegotiatedPairs)
	if tsr == nil || len(tsr.TrafficSelectors) != 2 {
		t.Errorf("the answer TSr does not carry both proposed entries: %+v", tsr)
	}
}

// TestSelectorListThatIsNotEveryCombinationIsRefused checks the config-verify refusal and
// the payload builder that backs it.
//
// VALIDATES: a peer list (A, C), (B, D) is refused at verify with ErrTrafficSelectorPolicy,
// naming a missing combination, and has no TS payload pair; the list of all four
// combinations passes and has one.
// PREVENTS: proposing TSi {A, B}, TSr {C, D}, which offers A-D and B-C the operator never
// configured.
func TestSelectorListThatIsNotEveryCombinationIsRefused(t *testing.T) {
	split := &ipsec.IPsecConfig{Peers: map[string]ipsec.SiteToSitePeer{"p": {Name: "p",
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{
			comboPolicy(t, "1", comboA, comboC),
			comboPolicy(t, "2", comboB, comboD),
		}}}}
	err := validateSelectorCombinations(split)
	if !errors.Is(err, ipsec.ErrTrafficSelectorPolicy) {
		t.Fatalf("validateSelectorCombinations(A-C, B-D) = %v, want ErrTrafficSelectorPolicy", err)
	}
	if tsi, tsr := pairsToWire(policyPairs(split.Peers["p"], true)); tsi != nil || tsr != nil {
		t.Error("pairsToWire built payloads for A-C, B-D; they would offer A-D and B-C as well")
	}

	full := &ipsec.IPsecConfig{Peers: map[string]ipsec.SiteToSitePeer{"p": {Name: "p",
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{
			comboPolicy(t, "1", comboA, comboC),
			comboPolicy(t, "2", comboA, comboD),
		}}}}
	if err := validateSelectorCombinations(full); err != nil {
		t.Errorf("a list holding every combination was refused: %v", err)
	}
	tsi, tsr := pairsToWire(policyPairs(full.Peers["p"], true))
	if tsi == nil || tsr == nil {
		t.Fatal("pairsToWire built no payloads for A-C, A-D")
	}
	if len(tsi.TrafficSelectors) != 1 || len(tsr.TrafficSelectors) != 2 {
		t.Errorf("payloads carry %d TSi and %d TSr entries, want 1 and 2",
			len(tsi.TrafficSelectors), len(tsr.TrafficSelectors))
	}
}
