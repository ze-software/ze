// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- MOBIKE negotiation in IKE_AUTH
// Related: mobike_test.go -- the single-IKE_AUTH negotiation and the offer's empty data
// Related: rfc7427_auth_test.go -- testSAWithGCMKeys and the EAP-mode IKE_AUTH request
//
// VALIDATES: RFC 4555 Section 3.2, the offer sits in the IKE_AUTH message carrying the
// SA payload when EAP spreads IKE_AUTH over several exchanges.
// PREVENTS: MOBIKE in use on an IKE SA whose own IKE_AUTH carried no MOBIKE_SUPPORTED.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// rfc4555OfferSA returns an initiator SA in EAP mode whose MOBIKE capability is live:
// a NAT-T socket is bound and the dataplane can migrate. A MOBIKE_SUPPORTED that is
// missing from a message is then a placement decision, never a missing capability.
func rfc4555OfferSA(t *testing.T) *SA {
	t.Helper()
	sa := testSAWithGCMKeys(t)
	sa.ESPGroup = testESPGroup()
	sa.PeerCfg.Auth.Mode = ipsec.AuthEAPMSCHAPv2
	sa.PeerCfg.Auth.PSK = eapwPassword
	_, myTr := rtxPeerLink(t, sa)
	sa.bindSockets(nil, myTr)
	sa.mobike.canMigrate = true
	if !mobikeAvailable(sa) {
		t.Fatal("fixture: MOBIKE is not available, so no placement can be observed")
	}
	return sa
}

// rfc4555Inner decrypts a message the SA built, the way its peer reads it.
func rfc4555Inner(t *testing.T, sa *SA, raw []byte) []wire.PayloadEntry {
	t.Helper()
	inner, err := decryptAndParse(eapwFarEnd(sa), parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("the message does not authenticate under the IKE SA: %v", err)
	}
	return inner
}

func rfc4555HasSA(inner []wire.PayloadEntry) bool {
	for i := range inner {
		if _, ok := inner[i].Payload.(*wire.PayloadSA); ok {
			return true
		}
	}
	return false
}

func rfc4555HasAuth(inner []wire.PayloadEntry) bool {
	for i := range inner {
		if _, ok := inner[i].Payload.(*wire.PayloadAUTH); ok {
			return true
		}
	}
	return false
}

// TestRFC4555EAPOfferRidesTheIKEAuthMessageCarryingSA proves where Ze puts its offer
// when IKE_AUTH takes several exchanges.
//
// RFC 4555 Section 3.2: "Implementations that wish to use MOBIKE for a particular
// IKE_SA MUST include a MOBIKE_SUPPORTED notification in the IKE_AUTH exchange (in case
// of multiple IKE_AUTH exchanges, in the message containing the SA payload)."
//
// Method: an EAP initiator runs several IKE_AUTH exchanges (RFC 7296 Section 2.16). Its
// first IKE_AUTH request (buildAuthRequest) carries the SA payload and no AUTH; each
// later request carrying an EAP response (buildEAPResponse) carries no SA payload.
// Both are decrypted off the wire under the far end's view of the IKE SA. The offer is
// in the SA-carrying message and in no other, and MOBIKE is then enabled by the
// peer's answer.
func TestRFC4555EAPOfferRidesTheIKEAuthMessageCarryingSA(t *testing.T) {
	sa := rfc4555OfferSA(t)

	first, err := buildAuthRequest(sa)
	if err != nil {
		t.Fatalf("first IKE_AUTH request: %v", err)
	}
	inner := rfc4555Inner(t, sa, first)
	if !rfc4555HasSA(inner) {
		t.Fatal("the first EAP-mode IKE_AUTH request carries no SA payload")
	}
	if rfc4555HasAuth(inner) {
		t.Fatal("the first EAP-mode IKE_AUTH request carries AUTH, so this is not the multi-exchange form")
	}
	// RFC requirement: RFC4555-x-1 positive -- with several IKE_AUTH exchanges (EAP), Ze's
	// MOBIKE_SUPPORTED is in the IKE_AUTH request carrying the SA payload and not in the
	// later EAP-response request, and the peer's answer then puts MOBIKE in use.
	if notifyOf(inner, wire.NotifyMobikeSupported) == nil {
		t.Fatal("the IKE_AUTH message carrying the SA payload has no MOBIKE_SUPPORTED")
	}

	eapResponse := eapwRequest(3, 26, []byte{0x02})
	eapResponse.Code = 2
	later, err := buildEAPResponse(sa, eapResponse.Encode())
	if err != nil {
		t.Fatalf("later IKE_AUTH request carrying an EAP response: %v", err)
	}
	laterInner := rfc4555Inner(t, sa, later)
	if rfc4555HasSA(laterInner) {
		t.Fatal("fixture: the EAP-response IKE_AUTH request carries an SA payload")
	}
	if notifyOf(laterInner, wire.NotifyMobikeSupported) != nil {
		t.Fatal("MOBIKE_SUPPORTED rides an IKE_AUTH message that carries no SA payload")
	}

	peerOffer := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyMobikeSupported}}}
	sa.acceptMobikeOffer(peerOffer)
	if !sa.mobike.enabled {
		t.Fatal("the offer was sent and the peer answered with its own, yet MOBIKE is not in use")
	}
}

// TestRFC4555NoOwnOfferNoMOBIKE proves the other side of the same sentence: an IKE SA
// whose IKE_AUTH did not include Ze's MOBIKE_SUPPORTED never uses MOBIKE, even when the
// peer offers it.
//
// Method: two initiator SAs receive the peer's MOBIKE_SUPPORTED in the IKE_AUTH
// response. The first has a live MOBIKE capability but never built its offer; the
// second has no migrating dataplane, so its IKE_AUTH request carries no offer at all
// (checked off the wire). Neither enables MOBIKE, so no later exchange can migrate the
// SA. The positive test above is the control: the same answer after Ze's own offer
// enables it.
func TestRFC4555NoOwnOfferNoMOBIKE(t *testing.T) {
	peerOffer := []wire.PayloadEntry{{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyMobikeSupported}}}

	// RFC requirement: RFC4555-x-1 negative -- an initiator IKE SA whose own IKE_AUTH carried
	// no MOBIKE_SUPPORTED (offer never built, or no migrating dataplane) does not put MOBIKE
	// in use when the peer's IKE_AUTH response offers it.
	unsent := rfc4555OfferSA(t)
	unsent.acceptMobikeOffer(peerOffer)
	if unsent.mobike.enabled {
		t.Fatal("MOBIKE enabled on an IKE SA whose own IKE_AUTH never carried MOBIKE_SUPPORTED")
	}

	incapable := rfc4555OfferSA(t)
	incapable.mobike.canMigrate = false
	request, err := buildAuthRequest(incapable)
	if err != nil {
		t.Fatalf("IKE_AUTH request without a migrating dataplane: %v", err)
	}
	if notifyOf(rfc4555Inner(t, incapable, request), wire.NotifyMobikeSupported) != nil {
		t.Fatal("fixture: an SA that cannot migrate still offered MOBIKE")
	}
	incapable.acceptMobikeOffer(peerOffer)
	if incapable.mobike.enabled {
		t.Fatal("MOBIKE enabled on an IKE SA that did not offer it")
	}
}
