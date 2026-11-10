// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: ts_narrow.go -- programmableSelector and portMatchesEveryFragment, the refusal
// Related: child.go -- childPolicyPair, the second check before a policy is built
// RFC: rfc/short/rfc4301.md -- fragments on an SA with non-trivial selectors (Section 7.3)
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// fragmentSelectorCases are the selectors one side of a negotiation can carry, each with
// whether the kernel matches every fragment against it. Linux reads no port, ICMP type or
// ICMP code from a fragment, so the flow it looks a policy up with carries 0 for each
// (portMatchesEveryFragment): a single port 0 under TCP, and IKE value 0 under ICMP
// (type 0 in the high octet, code 0 in the low one), are that flow's values.
var fragmentSelectorCases = []struct {
	name          string
	proto         uint8
	port          ipsec.PortSelector
	everyFragment bool
}{
	{"TCP single port 0", 6, ipsec.PortSelector{Form: ipsec.PortSingle, Port: 0}, true},
	{"ICMP type 0 code 0", 1, ipsec.PortSelector{Form: ipsec.PortSingle, Port: 0}, true},
	{"TCP single port 179", 6, ipsec.PortSelector{Form: ipsec.PortSingle, Port: 179}, false},
	{"ICMP type 0 code 3", 1, ipsec.PortSelector{Form: ipsec.PortSingle, Port: 0x0003}, false},
	{"ICMP type 8 code 0", 1, ipsec.PortSelector{Form: ipsec.PortSingle, Port: 0x0800}, false},
	{"TCP any port", 6, ipsec.AnyPort(), false},
}

// VALIDATES: RFC4301-7.3-4 at the responder. A peer's proposal whose port is a single 0,
// or whose ICMP type and code are 0 and 0, is narrowed away: with nothing else proposed,
// narrowSelectors answers not-ok, which the caller sends as TS_UNACCEPTABLE. The same
// proposal with port 179, ICMP type 0 code 3, ICMP type 8 code 0 or any port is accepted
// and answered unchanged, so the refusal is about the value and not a blanket refusal
// of single ports or ICMP.
// PREVENTS: a Child SA negotiated with a selector every fragment matches. Linux looks up
// the forward policy of a decrypted packet before reassembly, so such an SA would accept
// the non-initial fragments Section 7.3 orders discarded, because Ze negotiates no
// NON_FIRST_FRAGMENTS_ALSO.
// Method: the proposal is read through wireToSelectors from the 0-0 wire pair a peer
// sends, then narrowed against no policy (the peer accepts whatever is proposed) and
// against an any-port policy for the same prefixes.
// RFC requirement: RFC4301-7.3-4 negative -- a proposed single port 0 under TCP and a proposed ICMP type 0 code 0 are narrowed away, so narrowSelectors answers not-ok for a proposal holding nothing else, while port 179, ICMP type 0 code 3, ICMP type 8 code 0 and any port are accepted with the TSi port unchanged.
func TestNarrowingRefusesASelectorEveryFragmentMatches(t *testing.T) {
	for _, tc := range fragmentSelectorCases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := tc.port.Wire()
			proposedI := wireToSelectors([]wire.TrafficSelector{wireSel(t, "10.1.0.0/16", start, end, tc.proto)})
			if len(proposedI) != 1 || proposedI[0].Port != tc.port {
				t.Fatalf("wireToSelectors(%d-%d) = %+v, want the one selector %+v; the case never reaches the narrowing",
					start, end, proposedI, tc.port)
			}
			proposedR := []tsSelector{selPort(t, "10.2.0.0/16", ipsec.AnyPort(), tc.proto)}
			policy := []tsPair{{
				I: selPort(t, "10.0.0.0/8", ipsec.AnyPort(), tc.proto),
				R: selPort(t, "10.0.0.0/8", ipsec.AnyPort(), tc.proto),
			}}
			for _, p := range []struct {
				name   string
				policy []tsPair
			}{{"unconfigured peer", nil}, {"any-port policy", policy}} {
				got, ok := narrowSelectors(proposedI, proposedR, p.policy, nil)
				if tc.everyFragment {
					if ok {
						t.Errorf("%s: narrowSelectors accepted %s and answered %+v; every fragment between the prefixes would match the SA",
							p.name, tc.name, got)
					}
					continue
				}
				if !ok {
					t.Errorf("%s: narrowSelectors refused %s, which no fragment matches; the refusal is not about the value", p.name, tc.name)
					continue
				}
				if len(got) != 1 || got[0].I.Port != tc.port {
					t.Errorf("%s: answer = %+v, want the one pair whose TSi port is %+v", p.name, got, tc.port)
				}
			}
		})
	}
}

// VALIDATES: RFC4301-7.3-3 at the initiator. A responder's answer whose port is a single
// 0, or whose ICMP type and code are 0 and 0, is refused rather than installed, while
// the same answer with port 179, ICMP type 0 code 3, ICMP type 8 code 0 or any port
// builds its pair.
// PREVENTS: the initiator installing an outbound policy every fragment matches. Linux
// looks up the outbound policy of a forwarded packet before reassembly, so a transit
// fragment would be sent into the SA, which Section 7.3 forbids without a negotiated
// NON_FIRST_FRAGMENTS_ALSO.
// RFC requirement: RFC4301-7.3-3 negative -- an answered single port 0 under TCP and an answered ICMP type 0 code 0 make answerPairs refuse the answer, while port 179, ICMP type 0 code 3, ICMP type 8 code 0 and any port each build the answered pair.
func TestAnswerRefusesASelectorEveryFragmentMatches(t *testing.T) {
	for _, tc := range fragmentSelectorCases {
		t.Run(tc.name, func(t *testing.T) {
			iSels := []tsSelector{selPort(t, "10.1.0.0/16", tc.port, tc.proto)}
			rSels := []tsSelector{selPort(t, "10.2.0.0/16", ipsec.AnyPort(), tc.proto)}
			pairs, err := answerPairs(iSels, rSels)
			if tc.everyFragment {
				if err == nil {
					t.Errorf("answerPairs accepted %s and built %+v; every forwarded fragment between the prefixes would be sent into the SA",
						tc.name, pairs)
				}
				return
			}
			if err != nil {
				t.Fatalf("answerPairs refused %s, which no fragment matches: %v", tc.name, err)
			}
			if len(pairs) != 1 || pairs[0].I.Port != tc.port {
				t.Errorf("answered pairs = %+v, want the one pair whose TSi port is %+v", pairs, tc.port)
			}
		})
	}
}

// VALIDATES: childPolicyPair, the step every installed Child SA policy is built through,
// builds no policy for a pair with a single port 0 or ICMP type 0 code 0 on either side,
// and builds one for the other cases.
// PREVENTS: a pair that reached the install without passing the negotiation's check
// (a rekey floor, a stored selector, a future producer) installing a selector every
// fragment matches; this is the second of the two checks.
func TestChildPolicyPairRefusesASelectorEveryFragmentMatches(t *testing.T) {
	child := &ChildSA{SelectorsLocalIsTSi: true}
	for _, tc := range fragmentSelectorCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, side := range []string{"local", "remote"} {
				pair := tsPair{
					I: selPort(t, "10.1.0.0/16", tc.port, tc.proto),
					R: selPort(t, "10.2.0.0/16", ipsec.AnyPort(), tc.proto),
				}
				if side == "remote" {
					pair.I, pair.R = selPort(t, "10.1.0.0/16", ipsec.AnyPort(), tc.proto), selPort(t, "10.2.0.0/16", tc.port, tc.proto)
				}
				_, ok := childPolicyPair(child, pair)
				if ok == tc.everyFragment {
					t.Errorf("%s side: childPolicyPair answered ok=%v for %s, want %v", side, ok, tc.name, !tc.everyFragment)
				}
			}
		})
	}
}
