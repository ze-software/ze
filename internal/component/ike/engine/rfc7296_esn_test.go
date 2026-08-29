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
func TestEsnInitiatorRefusesAnESNValueItNeverOffered(t *testing.T) {
	espGroup := testESPGroup()

	// The offer states what this side can key, and the checks below are read against it.
	offered := esnIDsOf(buildWireESPProposals(espGroup, 0x0a0b0c0d)[0])
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
