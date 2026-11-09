// VALIDATES: the Extended Sequence Numbers transform is negotiated rather than ignored.
// The responder answers a value the offer carried, or refuses the offer, and the
// initiator refuses an answer that selects a value it never offered.
// PREVENTS: the silent mis-key. espProposalMatches read Transform Type 5 into a case that
// did nothing, and espProposalToWire answered value 0 whatever arrived. A peer offering
// only Extended Sequence Numbers was told its proposal was accepted and handed a
// transform it never proposed. The tunnel would establish, ze would count 32 bits of
// sequence number where the peer counted 64, and the peer's anti-replay window would drop
// every packet ze sent.
package engine

import (
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// esnExtended is Transform Type 5 value 1, "Extended Sequence Numbers" (RFC 7296
// Section 3.3.2). It is the value ze cannot key, and every case below is built around
// whether the peer leaves ze an alternative to it.
const esnExtended uint16 = 1

// esnSAPayload builds an ESP SA payload offering the suite testESPGroup configures, with
// the given Transform Type 5 values appended. The ESN list is the axis these tests vary,
// so everything else about the proposal is held equal.
//
// The same shape serves as an OFFER read by the responder and as an ANSWER read by the
// initiator: RFC 7296 Section 3.3 gives both directions one encoding.
func esnSAPayload(t *testing.T, number uint8, esnIDs ...uint16) *wire.PayloadSA {
	t.Helper()
	encID, keyLen, integID := espOurs(t)
	transforms := []wire.Transform{
		espEncTransform(encID, keyLen),
		{Type: wire.TransformTypeINTG, ID: integID},
	}
	for _, id := range esnIDs {
		transforms = append(transforms, wire.Transform{Type: wire.TransformTypeESN, ID: id})
	}
	return &wire.PayloadSA{Proposals: []wire.Proposal{{
		Number:     number,
		ProtocolID: wire.ProtocolESP,
		SPISize:    4,
		SPI:        []byte{0x11, 0x22, 0x33, 0x44},
		Transforms: transforms,
	}}}
}

// esnIDsOf lists the Transform Type 5 values one wire proposal carries, in order.
func esnIDsOf(p wire.Proposal) []uint16 {
	var ids []uint16
	for _, tr := range p.Transforms {
		if tr.Type == wire.TransformTypeESN {
			ids = append(ids, tr.ID)
		}
	}
	return ids
}

// esnResponderSA returns a responder SA with the test ESP group and a traffic-selector
// policy, narrowed so buildChildSAResponsePayloads can build the real SAr2. It mirrors
// the order buildAuthResponse (responder.go) runs in: narrow first, then select.
func esnResponderSA(t *testing.T) *SA {
	t.Helper()
	sa := testSA()
	sa.IsInitiator = false
	sa.ESPGroup = testESPGroup()
	sa.PeerCfg.TrafficSelectors = []ipsec.TrafficSelectorPolicy{{
		Number:       "1",
		LocalPrefix:  mustNet(t, "10.0.0.0/8"),
		LocalPort:    ipsec.AnyPort(),
		RemotePrefix: mustNet(t, "10.0.0.0/8"),
		RemotePort:   ipsec.AnyPort(),
	}}
	if err := narrowChildSelectors(sa,
		tsPayload(t, wire.PayloadTypeTSi, "10.1.0.0/16"),
		tsPayload(t, wire.PayloadTypeTSr, "10.2.0.0/16"),
		nil); err != nil {
		t.Fatalf("narrowChildSelectors: %v", err)
	}
	return sa
}

// RFC requirement: RFC7296-2.7-1 negative -- "The accepted cryptographic suite MUST contain
// exactly one transform of each type included in the proposal" (RFC 7296 Section 2.7,
// rfc/full/rfc7296.txt:1976-1980), and "The responder MUST accept a single proposal or
// reject them all and return an error. The error is given in a notification of type
// NO_PROPOSAL_CHOSEN". A proposal carrying Extended Sequence Numbers alone leaves ze no
// transform of that type it can select, so ze rejects it. RFC 7296 Section 3.3.2 states
// what such a proposal means: "A proposal containing a single ESN transform with value
// '1' means that using normal (non-extended) sequence numbers is not acceptable".
// RFC requirement: RFC7296-2.7-1 positive -- an offer that includes value 0, alone or beside
// value 1, is accepted, and the SAr2 answer carries exactly one transform of the type
// whose value the offer carried.
func TestEsnResponderAnswersOnlyAValueTheOfferCarried(t *testing.T) {
	cases := []struct {
		name     string
		offered  []uint16
		accepted bool
	}{
		{name: "no-extended-only", offered: []uint16{espESNNotExtended}, accepted: true},
		{name: "both-values", offered: []uint16{espESNNotExtended, esnExtended}, accepted: true},
		{name: "extended-only", offered: []uint16{esnExtended}, accepted: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sa := esnResponderSA(t)
			err := selectResponderESP(sa, esnSAPayload(t, 3, c.offered...))

			if !c.accepted {
				if !errors.Is(err, crypto.ErrNoProposalChosen) {
					t.Fatalf("an offer of ESN %v was answered with %v, want NO_PROPOSAL_CHOSEN", c.offered, err)
				}
				if got := notifyForRefusal(err); got != wire.NotifyNoProposalChosen {
					t.Errorf("the refusal maps to notify %d, want NO_PROPOSAL_CHOSEN (%d)", got, wire.NotifyNoProposalChosen)
				}
				return
			}

			if err != nil {
				t.Fatalf("an offer of ESN %v was refused: %v", c.offered, err)
			}
			_, saPayload, _, _, err := buildChildSAResponsePayloads(sa)
			if err != nil {
				t.Fatalf("buildChildSAResponsePayloads: %v", err)
			}
			if len(saPayload.Proposals) != 1 {
				t.Fatalf("SAr2 carries %d proposals, want the single accepted one", len(saPayload.Proposals))
			}
			answered := esnIDsOf(saPayload.Proposals[0])
			if len(answered) != 1 {
				t.Fatalf("SAr2 carries %d ESN transforms (%v), want exactly one", len(answered), answered)
			}
			if !slices.Contains(c.offered, answered[0]) {
				t.Errorf("SAr2 answers ESN %d, which the offer %v never carried", answered[0], c.offered)
			}
		})
	}
}

// RFC requirement: RFC7296-3.3.6-3 negative -- "The initiator of an exchange MUST check that the
// accepted offer is consistent with one of its proposals, and if not MUST terminate the
// exchange" (RFC 7296 Section 3.3.6, rfc/full/rfc7296.txt:4906-4909). Ze offers one value
// for Transform Type 5, so a response selecting Extended Sequence Numbers, or selecting
// two values where RFC 7296 Section 2.7 allows one, is not consistent with anything ze
// sent.
// RFC requirement: RFC7296-3.3.6-3 positive -- the response that selects the value ze did offer is
// accepted, so the refusal names the ESN selection rather than the answer as such.
// RFC requirement: RFC4303-2.2.1-2 positive -- RFC 4303 Section 2.2.1: "Use of an ESN MUST be
// negotiated by an SA management protocol." Ze's ESP offer carries a Transform Type 5 value,
// so the sequence number width of every Child SA it keys is a negotiated selection, and the
// answer that selects the offered value is accepted.
// RFC requirement: RFC4303-2.2.1-2 negative -- an answer that selects Extended Sequence Numbers
// ze never offered is refused with errAcceptedOfferESN, so no SA is keyed with an ESN that was
// not negotiated.
func TestEsnInitiatorRefusesAnESNValueItNeverOffered(t *testing.T) {
	espGroup := testESPGroup()

	// The offer states what this side can key, and the checks below are read against it.
	offered := esnIDsOf(buildWireESPProposals(espGroup, 0x0a0b0c0d, dhGroupNone)[0])
	if !slices.Equal(offered, []uint16{espESNNotExtended}) {
		t.Fatalf("the ESP offer carries ESN %v, want the single value %d ze can key",
			offered, espESNNotExtended)
	}

	cases := []struct {
		name     string
		answered []uint16
		accepted bool
	}{
		{name: "the-value-we-offered", answered: []uint16{espESNNotExtended}, accepted: true},
		{name: "extended", answered: []uint16{esnExtended}, accepted: false},
		{name: "two-values-is-no-selection", answered: []uint16{espESNNotExtended, esnExtended}, accepted: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := verifyAcceptedOffer(esnSAPayload(t, 1, c.answered...), ipsec.IKEGroup{}, espGroup)
			if c.accepted {
				if err != nil {
					t.Fatalf("an answer of ESN %v was refused: %v", c.answered, err)
				}
				return
			}
			if !errors.Is(err, errAcceptedOfferESN) {
				t.Fatalf("an answer of ESN %v returned %v, want the exchange to end", c.answered, err)
			}
		})
	}
}

// esnUnassigned is a Transform Type 5 value RFC 7296 Section 3.3.2 does not define (it
// defines 0 and 1). A peer that answers it has answered nothing ze can key.
const esnUnassigned uint16 = 7

// esnSetIDs replaces the Transform Type 5 values of every proposal in an SA payload with
// the given list, in order. An empty list removes the type altogether.
func esnSetIDs(sa *wire.PayloadSA, esnIDs []uint16) {
	for i := range sa.Proposals {
		kept := sa.Proposals[i].Transforms[:0]
		for _, tr := range sa.Proposals[i].Transforms {
			if tr.Type != wire.TransformTypeESN {
				kept = append(kept, tr)
			}
		}
		for _, id := range esnIDs {
			kept = append(kept, wire.Transform{Type: wire.TransformTypeESN, ID: id})
		}
		sa.Proposals[i].Transforms = kept
	}
}

// VALIDATES: the initiator accepts an SAr2 only when it selects exactly the one
// Transform Type 5 value ze offered, and refuses a missing, duplicated, unknown or
// unoffered selection before any Child SA is keyed.
// PREVENTS: reading an answer with no ESN transform as the No-ESN default.
// acceptedESPESNConsistent answered esns <= 1, so a responder that dropped the type was
// accepted and the Child SA was installed from a suite that was never fully selected.
//
// Method: the unit half calls verifyAcceptedOffer with each answer. The boundary half
// runs a real handshake, rewrites the SAr2 the responder built, seals it again, and hands
// it to handleAuthResponse: a refused answer leaves the SA dead with no outbound Child SA
// SPI adopted, and the offered value leaves it established with one.
//
// RFC requirement: RFC7296-3.3.6-3 negative -- RFC 7296 Section 3.3.6: "The initiator of
// an exchange MUST check that the accepted offer is consistent with one of its proposals,
// and if not MUST terminate the exchange." RFC 7296 Section 2.7: "The accepted
// cryptographic suite MUST contain exactly one transform of each type included in the
// proposal." Ze's ESP offer includes Transform Type 5, so an SAr2 carrying no ESN
// transform, two of them, an unassigned value or Extended Sequence Numbers is refused
// with errAcceptedOfferESN, and handleAuthResponse ends the exchange before the Child SA
// SPI is adopted.
// RFC requirement: RFC7296-3.3.6-3 positive -- the SAr2 that selects the single value ze
// offered is accepted, and handleAuthResponse establishes the SA and adopts the Child SA
// SPI, so the refusals name the ESN selection rather than the answer as such.
func TestInitiatorParseAgreedESN(t *testing.T) {
	log := slogutil.DiscardLogger()
	espGroup := testESPGroup()

	// The offer states what this side can key, and every case is read against it.
	offered := esnIDsOf(buildWireESPProposals(espGroup, 0x0a0b0c0d, dhGroupNone)[0])
	if !slices.Equal(offered, []uint16{espESNNotExtended}) {
		t.Fatalf("the ESP offer carries ESN %v, want the single value %d ze can key",
			offered, espESNNotExtended)
	}

	cases := []struct {
		name     string
		answered []uint16
		accepted bool
	}{
		{name: "the-value-we-offered", answered: []uint16{espESNNotExtended}, accepted: true},
		{name: "missing", answered: nil, accepted: false},
		{name: "duplicated", answered: []uint16{espESNNotExtended, espESNNotExtended}, accepted: false},
		{name: "unassigned", answered: []uint16{esnUnassigned}, accepted: false},
		{name: "unoffered-extended", answered: []uint16{esnExtended}, accepted: false},
	}
	for _, c := range cases {
		t.Run(c.name+"/verify", func(t *testing.T) {
			_, err := verifyAcceptedOffer(esnSAPayload(t, 1, c.answered...), ipsec.IKEGroup{}, espGroup)
			if c.accepted {
				if err != nil {
					t.Fatalf("an answer of ESN %v was refused: %v", c.answered, err)
				}
				return
			}
			if !errors.Is(err, errAcceptedOfferESN) {
				t.Fatalf("an answer of ESN %v returned %v, want errAcceptedOfferESN", c.answered, err)
			}
		})
		t.Run(c.name+"/ike-auth-response", func(t *testing.T) {
			ini, answer := negAuthResponseRewritten(t, func(p *wire.PayloadSA) { esnSetIDs(p, c.answered) })
			if ini.ChildOutboundSPI != 0 {
				t.Fatalf("the initiator holds Child SA SPI %#x before the response, so its absence "+
					"below proves nothing", ini.ChildOutboundSPI)
			}
			handleAuthResponse(ini, parseMsg(t, answer), answer, nil, nil, log)
			if c.accepted {
				if ini.State != StateEstablished {
					t.Fatalf("state = %v for an SAr2 selecting ESN %v, want established", ini.State, c.answered)
				}
				if ini.ChildOutboundSPI == 0 {
					t.Fatalf("an SAr2 selecting ESN %v was accepted but no Child SA SPI was adopted", c.answered)
				}
				return
			}
			if ini.State != StateDead {
				t.Fatalf("state = %v for an SAr2 selecting ESN %v, want dead", ini.State, c.answered)
			}
			if ini.ChildOutboundSPI != 0 {
				t.Fatalf("an SAr2 selecting ESN %v was refused after Child SA SPI %#x was adopted",
					c.answered, ini.ChildOutboundSPI)
			}
		})
	}
}
