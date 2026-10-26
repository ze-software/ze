// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- a proposal whose families never meet is refused

package engine

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestNarrowRefusesAMixedFamilyProposal drives narrowChildSelectors with a peer that
// configures no traffic selector, so its policy allows every proposal, and a proposal
// whose only combination pairs an IPv4 TSi with an IPv6 TSr.
//
// VALIDATES: the responder answers errTSUnacceptable, which notifyForRefusal maps to
// TS_UNACCEPTABLE (RFC 7296 Section 2.9), and records no selector on the SA.
// PREVENTS: the accepted mixed pair leaving closedSubset an empty answer that
// narrowChildSelectors then indexed, which panicked the daemon on one crafted request.
func TestNarrowRefusesAMixedFamilyProposal(t *testing.T) {
	sa := &SA{PeerCfg: ipsec.SiteToSitePeer{Name: "mixed-family"}}
	tsi := tsPayload(t, wire.PayloadTypeTSi, "10.0.0.0/8")
	tsr := tsPayload(t, wire.PayloadTypeTSr, "2001:db8::/32")

	err := narrowChildSelectors(sa, tsi, tsr, nil)
	if !errors.Is(err, errTSUnacceptable) {
		t.Fatalf("narrowChildSelectors = %v, want errTSUnacceptable", err)
	}
	if got := notifyForRefusal(err); got != wire.NotifyTSUnacceptable {
		t.Errorf("the refusal maps to notify %d (%s), want TS_UNACCEPTABLE", got, wire.NotifyTypeName(got))
	}
	if sa.NegotiatedPairs != nil {
		t.Errorf("a refused proposal recorded selectors %v", sa.NegotiatedPairs)
	}
}

// TestNarrowSkipsAMixedFirstChoice proposes TSi {10.0.0.0/8, 2001:db8:1::/48} and TSr
// {2001:db8::/32} to a peer with no configured traffic selector. The first choice pairs
// IPv4 with IPv6; the second combination is IPv6 on both halves.
//
// VALIDATES: the answer is the one same-family pair, and nothing of the mixed first
// choice reaches the recorded selectors.
// PREVENTS: a refusal of the whole proposal because its first choice matches no packet,
// when a later combination does.
func TestNarrowSkipsAMixedFirstChoice(t *testing.T) {
	sa := &SA{PeerCfg: ipsec.SiteToSitePeer{Name: "mixed-first"}}
	tsi := tsPayload(t, wire.PayloadTypeTSi, "10.0.0.0/8", "2001:db8:1::/48")
	tsr := tsPayload(t, wire.PayloadTypeTSr, "2001:db8::/32")

	if err := narrowChildSelectors(sa, tsi, tsr, nil); err != nil {
		t.Fatalf("narrowChildSelectors refused a proposal holding one same-family pair: %v", err)
	}
	if len(sa.NegotiatedPairs) != 1 {
		t.Fatalf("recorded %d pairs %v, want the one IPv6 pair", len(sa.NegotiatedPairs), sa.NegotiatedPairs)
	}
	if got := sa.NegotiatedTSi.String(); got != "2001:db8:1::/48" {
		t.Errorf("TSi = %s, want 2001:db8:1::/48", got)
	}
	if got := sa.NegotiatedTSr.String(); got != "2001:db8::/32" {
		t.Errorf("TSr = %s, want 2001:db8::/32", got)
	}
}

// TestResponderAnswersTSUnacceptableToAMixedFamilyChild sends a new Child SA request
// through handleCreateChildSAOwned, the responder entry for CREATE_CHILD_SA, from a peer
// that configures no traffic selector. The request proposes TSi {10.0.0.0/8} and TSr
// {2001:db8::/32}, a combination no packet can match.
//
// VALIDATES: the answer carries exactly TS_UNACCEPTABLE, no Child SA is installed, and the
// IKE SA stays established.
// PREVENTS: the crafted request crashing the responder with an index out of range.
func TestResponderAnswersTSUnacceptableToAMixedFamilyChild(t *testing.T) {
	log := slogutil.DiscardLogger()
	link := errLink(t)
	ps := link.ps
	ps.peerName = "create-child-mixed-family"
	ps.espGroup = testESPGroup()
	ps.setChildSA(nil)

	inner := peerNewChildRequest(t, ps.espGroup, 0x0c0ffee2, testNonce(63), nil, false, "10.0.0.0/8", "2001:db8::/32")
	msg := &wire.Message{Header: wire.Header{MessageID: link.resp.ExpectedMsgID}}
	out := ps.handleCreateChildSAOwned(link.resp, msg, inner, false, link.myTr, nil, log)

	raw := rtxRecv(t, link.peerTr)
	if raw == nil {
		t.Fatal("the refused request drew no answer")
	}
	types := errNotifyIn(t, link.ini, raw)
	if len(types) != 1 || types[0] != wire.NotifyTSUnacceptable {
		t.Errorf("the answer carries notifies %v, want exactly TS_UNACCEPTABLE", types)
	}
	if ps.getChildSA() != nil || out.createdChild != nil {
		t.Error("a refused request installed a Child SA")
	}
	if link.resp.State != StateEstablished {
		t.Errorf("the IKE SA is %v after a refused Child SA, want established", link.resp.State)
	}
}
