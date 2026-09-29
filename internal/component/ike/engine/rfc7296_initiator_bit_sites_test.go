// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the IKE header flags
// Related: rfc7296_initiator_bit_test.go -- the IKE_SA_INIT, IKE_AUTH, DPD and INFORMATIONAL sites
// VALIDATES: the I (Initiator) flag of the messages delete.go, rekey.go, notify_error.go
// and mobike.go build follows the ORIGINAL role of the sender, for both roles, in requests
// and in responses (RFC 7296 Section 3.1).
// PREVENTS: one call site that sets the I flag from the exchange direction, or a constant,
// while the others stay right.

package engine

import (
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// initiatorBitRoleFixture returns an established pair's SA for the given original role as
// the sending side of a MOBIKE fixture, so a message it sends arrives on f.peerTr.
func initiatorBitRoleFixture(t *testing.T, originalInitiator bool) *mbFixture {
	t.Helper()
	ini, resp, _ := establishPSK(t)
	if originalInitiator {
		return mbOwner(t, ini, resp, false)
	}
	return mbOwner(t, resp, ini, false)
}

// initiatorBitChildSA returns a keyed SA of the given original role with one installed
// Child SA, the setup TestRespondChildRekey uses.
func initiatorBitChildSA(t *testing.T, originalInitiator bool) (*SA, *ChildSA, *mockDP) {
	t.Helper()
	sa := testSAWithGCMKeys(t)
	sa.IsInitiator = originalInitiator
	sa.ESPGroup = testESPGroup()
	dp := &mockDP{}
	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	return sa, old, dp
}

// initiatorBitChildRekeyRequest is the peer's CREATE_CHILD_SA rekey request body, as
// TestRespondChildRekey builds it, with the extra transforms the caller names.
func initiatorBitChildRekeyRequest(t *testing.T, extra ...wire.Transform) []wire.PayloadEntry {
	t.Helper()
	offer := espSAPayload(0x01020304)
	offer.Proposals[0].Transforms = append(offer.Proposals[0].Transforms, extra...)
	return []wire.PayloadEntry{
		{Payload: offer},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(3)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, "10.0.0.2/32")},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, "10.0.0.1/32")},
	}
}

// TestRFC7296InitiatorBitAtEveryRemainingCallSite drives each product function that builds
// a delete, CREATE_CHILD_SA, error-notify or MOBIKE message, once as the original
// initiator and once as the original responder, and reads the I flag of what it built or
// sent.
//
// Goal: initiatorFlag is passed per call site, so one site proves nothing about another.
// Method: every message is produced by the product function itself, never assembled by
// the test. Messages that go to a socket are read from the peer's socket. A site that
// set the flag from the exchange direction goes red on its response or on the original
// responder's request, and a site that hard-coded either value goes red for one role.
//
// RFC 7296 Section 3.1: "I (Initiator) - This bit MUST be set in messages sent by the
// original initiator of the IKE SA and MUST be cleared in messages sent by the original
// responder."
//
// RFC requirement: RFC7296-3.1-13 positive -- for the original initiator and the original responder, the I flag is set exactly when the sender is the original initiator on: the Delete request (writeDelete), the teardown Delete request (sendIKESATeardown), the MOBIKE request (startMobikeRequest) and MOBIKE error response (respondMobikeError), the error-notify response (buildErrorNotifyResponse), the Child SA rekey request (initiateChildRekey) and its response and INVALID_KE_PAYLOAD response (respondChildRekey), and the IKE SA rekey response and INVALID_KE_PAYLOAD response (respondIKERekey).
func TestRFC7296InitiatorBitAtEveryRemainingCallSite(t *testing.T) {
	log := slogutil.DiscardLogger()
	for _, originalInitiator := range []bool{true, false} {
		t.Run("original-initiator="+strconv.FormatBool(originalInitiator), func(t *testing.T) {
			check := func(site string, raw []byte, response bool) {
				t.Helper()
				hdr := parseMsg(t, raw).Header
				if got := hdr.Flags&wire.FlagResponse != 0; got != response {
					t.Fatalf("%s: R flag = %v, want %v", site, got, response)
				}
				if got := hdr.Flags&wire.FlagInitiator != 0; got != originalInitiator {
					t.Errorf("%s: I flag = %v, want %v (sender is the original initiator: %v)",
						site, got, originalInitiator, originalInitiator)
				}
			}

			f := initiatorBitRoleFixture(t, originalInitiator)
			f.ps.writeDelete(f.local, f.myTr, &wire.PayloadDelete{ProtocolID: wire.ProtocolIKE}, log)
			check("writeDelete", mbReceive(t, f.peerTr).Data, false)

			f = initiatorBitRoleFixture(t, originalInitiator)
			sendIKESATeardown(f.local, f.myTr, wire.NotifyAuthenticationFailed, log)
			check("sendIKESATeardown", mbReceive(t, f.peerTr).Data, false)

			f = initiatorBitRoleFixture(t, originalInitiator)
			if err := f.ps.startMobikeRequest(f.local, f.myTr, originalInitiator, log); err != nil {
				t.Fatalf("startMobikeRequest: %v", err)
			}
			check("startMobikeRequest", mbReceive(t, f.peerTr).Data, false)

			f = initiatorBitRoleFixture(t, originalInitiator)
			respondMobikeError(f.local, nil, 3, wire.ExchangeInformational, wire.NotifyUnexpectedNATDetected, f.myTr, log)
			check("respondMobikeError", mbReceive(t, f.peerTr).Data, true)

			f = initiatorBitRoleFixture(t, originalInitiator)
			raw, err := buildErrorNotifyResponse(f.local, 4, wire.ExchangeCreateChildSA, wire.NotifyInvalidSyntax, nil)
			if err != nil {
				t.Fatalf("buildErrorNotifyResponse: %v", err)
			}
			check("buildErrorNotifyResponse", raw, true)

			sa, old, _ := initiatorBitChildSA(t, originalInitiator)
			raw, _, err = initiateChildRekey(sa, old)
			if err != nil {
				t.Fatalf("initiateChildRekey: %v", err)
			}
			check("initiateChildRekey", raw, false)

			sa, old, dp := initiatorBitChildSA(t, originalInitiator)
			raw, _, err = respondChildRekey(sa, initiatorBitChildRekeyRequest(t), old, 5, dp, log)
			if err != nil {
				t.Fatalf("respondChildRekey: %v", err)
			}
			check("respondChildRekey (accepted)", raw, true)

			// With pfs enabled the request offers the IKE SA's group but carries no KEi,
			// so the answer is the INVALID_KE_PAYLOAD response.
			sa, old, dp = initiatorBitChildSA(t, originalInitiator)
			old.ESPGroup.PFS = ipsec.PFSEnable
			group := wire.Transform{Type: wire.TransformTypeDH, ID: uint16(sa.Proposal.DHGroup.ID)}
			raw, child, err := respondChildRekey(sa, initiatorBitChildRekeyRequest(t, group), old, 5, dp, log)
			if err != nil {
				t.Fatalf("respondChildRekey (no KEi under pfs): %v", err)
			}
			if child != nil {
				t.Fatal("respondChildRekey without KEi under pfs keyed a Child SA, want the INVALID_KE_PAYLOAD answer")
			}
			check("respondChildRekey (INVALID_KE_PAYLOAD)", raw, true)

			ini, resp, _ := establishPSK(t)
			local, peer := ini, resp
			if !originalInitiator {
				local, peer = resp, ini
			}
			req, pending, err := initiateIKERekey(peer, testIKEGroup())
			if err != nil {
				t.Fatalf("initiateIKERekey: %v", err)
			}
			inner, err := decryptAndParse(local, parseMsg(t, req), req)
			if err != nil {
				t.Fatalf("decrypt the IKE rekey request: %v", err)
			}
			raw, _, err = respondIKERekey(local, inner, pending.messageID, log)
			if err != nil {
				t.Fatalf("respondIKERekey: %v", err)
			}
			check("respondIKERekey (accepted)", raw, true)

			ini, resp, _ = establishPSK(t)
			local, peer = ini, resp
			if !originalInitiator {
				local, peer = resp, ini
			}
			req, pending, err = initiateIKERekey(peer, testIKEGroup())
			if err != nil {
				t.Fatalf("initiateIKERekey: %v", err)
			}
			inner, err = decryptAndParse(local, parseMsg(t, req), req)
			if err != nil {
				t.Fatalf("decrypt the IKE rekey request: %v", err)
			}
			for _, entry := range inner {
				if ke, ok := entry.Payload.(*wire.PayloadKE); ok {
					ke.DHGroup = 19
				}
			}
			raw, newSA, err := respondIKERekey(local, inner, pending.messageID, log)
			if err != nil {
				t.Fatalf("respondIKERekey (KEi of another group): %v", err)
			}
			if newSA != nil {
				t.Fatal("respondIKERekey over a KEi of another group built an IKE SA, want the INVALID_KE_PAYLOAD answer")
			}
			check("respondIKERekey (INVALID_KE_PAYLOAD)", raw, true)
		})
	}
}
