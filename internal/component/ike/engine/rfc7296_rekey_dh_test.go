// VALIDATES: RFC 7296 Section 2.18 on the IKE SA rekey path itself, both roles: the
// CREATE_CHILD_SA request initiateIKERekey builds proposes a real Diffie-Hellman group and
// carries its KE payload, and respondIKERekey refuses a rekey request whose only proposal
// offers the D-H value NONE.
// PREVENTS: an IKE SA rekey that reuses the old keys' strength without a fresh
// Diffie-Hellman exchange, from either side.
package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rekeyDHOffer answers the D-H transform IDs of every proposal in the SA payload of a
// decrypted CREATE_CHILD_SA request, and the group of its KE payload (zero with no KE).
func rekeyDHOffer(inner []wire.PayloadEntry) (dhIDs []uint16, keGroup uint16) {
	for _, pe := range inner {
		switch p := pe.Payload.(type) {
		case *wire.PayloadSA:
			for _, prop := range p.Proposals {
				for _, tr := range prop.Transforms {
					if tr.Type == wire.TransformTypeDH {
						dhIDs = append(dhIDs, tr.ID)
					}
				}
			}
		case *wire.PayloadKE:
			keGroup = p.DHGroup
		}
	}
	return dhIDs, keGroup
}

// RFC requirement: RFC7296-2.18-3 positive -- the initiator of an IKE SA rekey proposes
// a real Diffie-Hellman group: every proposal in the request's SA payload carries a D-H
// transform whose value is the configured group 14 and never NONE (0), and the request
// carries the KE payload of that group, so a new exchange is performed.
// RFC requirement: RFC7296-2.18-3 negative -- forced toward the violation, the initiator
// still does not propose NONE: with the configured group set to NONE, initiateIKERekey
// builds no request at all and consumes no message ID.
//
// RFC 7296 Section 2.18: "implementations MUST perform a new Diffie-Hellman exchange when
// rekeying the IKE SA. In other words, an initiator MUST NOT propose the value "NONE" for
// the Diffie-Hellman transform, and a responder MUST NOT accept such a proposal." The
// request under test is the rekey, never the initial exchange.
func TestRFC7296IKERekeyInitiatorNeverProposesDHNone(t *testing.T) {
	ini, resp, _ := establishPSK(t)

	reqBytes, pending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()
	req := parseMsg(t, reqBytes)
	inner, err := decryptAndParse(resp, req, reqBytes)
	if err != nil {
		t.Fatalf("the peer could not decrypt the ike rekey request: %v", err)
	}
	dhIDs, keGroup := rekeyDHOffer(inner)
	if len(dhIDs) == 0 {
		t.Fatal("the ike rekey request proposes no Diffie-Hellman transform")
	}
	for _, id := range dhIDs {
		if id != 14 {
			t.Errorf("the ike rekey request proposes D-H %d, want the configured group 14 "+
				"and never NONE (0)", id)
		}
	}
	if keGroup != 14 {
		t.Errorf("the ike rekey request's KE payload is for group %d, want 14", keGroup)
	}

	group := testIKEGroup()
	group.Proposals[0].DHGroup = 0
	nextMsgID := ini.NextMsgID
	msg, none, err := initiateIKERekey(ini, group)
	if err == nil {
		none.clear()
		t.Fatal("initiateIKERekey built an IKE rekey request with the D-H group NONE")
	}
	if msg != nil {
		t.Errorf("initiateIKERekey refused but still answered %d octets", len(msg))
	}
	if ini.NextMsgID != nextMsgID {
		t.Errorf("the refused rekey consumed message ID %d", nextMsgID)
	}
}

// RFC requirement: RFC7296-2.18-3 negative -- the responder of an IKE SA rekey does not
// accept a proposal of NONE: a CREATE_CHILD_SA request whose proposal's D-H transform is
// rewritten to NONE is refused by respondIKERekey with ErrDHGroupNone and yields no new
// IKE SA, even with local policy also set to NONE so that nothing but the NONE value can
// cause the refusal.
// RFC requirement: RFC7296-2.18-3 positive -- the same request unmodified, proposing group
// 14 with its KE payload, is accepted and yields the new IKE SA.
func TestRFC7296IKERekeyResponderRefusesDHNone(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)

	reqBytes, pending, err := initiateIKERekey(ini, testIKEGroup())
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()
	req := parseMsg(t, reqBytes)
	inner, err := decryptAndParse(resp, req, reqBytes)
	if err != nil {
		t.Fatalf("the peer could not decrypt the ike rekey request: %v", err)
	}

	noneInner := make([]wire.PayloadEntry, len(inner))
	copy(noneInner, inner)
	for i, pe := range noneInner {
		sa, ok := pe.Payload.(*wire.PayloadSA)
		if !ok {
			continue
		}
		props := make([]wire.Proposal, len(sa.Proposals))
		for j, prop := range sa.Proposals {
			props[j] = prop
			props[j].Transforms = make([]wire.Transform, len(prop.Transforms))
			copy(props[j].Transforms, prop.Transforms)
			for k := range props[j].Transforms {
				if props[j].Transforms[k].Type == wire.TransformTypeDH {
					props[j].Transforms[k].ID = 0
				}
			}
		}
		rewritten := *sa
		rewritten.Proposals = props
		noneInner[i].Payload = &rewritten
	}
	if dhIDs, _ := rekeyDHOffer(noneInner); len(dhIDs) == 0 || dhIDs[0] != 0 {
		t.Fatalf("the rewritten request proposes D-H %v, want NONE (0)", dhIDs)
	}

	policy := resp.IKEGroup
	nonePolicy := testIKEGroup()
	nonePolicy.Proposals[0].DHGroup = 0
	resp.IKEGroup = nonePolicy
	msg, newSA, err := respondIKERekey(resp, noneInner, pending.messageID, log)
	if !errors.Is(err, crypto.ErrDHGroupNone) {
		t.Errorf("respondIKERekey(proposal of D-H NONE) = %v, want ErrDHGroupNone", err)
	}
	if newSA != nil || msg != nil {
		t.Error("respondIKERekey accepted a proposal of D-H NONE and built a new IKE SA")
	}
	resp.IKEGroup = policy

	msg, newSA, err = respondIKERekey(resp, inner, pending.messageID, log)
	if err != nil {
		t.Fatalf("respondIKERekey(proposal of group 14) = %v, want acceptance", err)
	}
	if newSA == nil || len(msg) == 0 {
		t.Fatal("respondIKERekey accepted group 14 but built no new IKE SA or response")
	}
}
