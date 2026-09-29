// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- Child SA rekey
// Related: rekey_test.go -- TestRekeyWithoutTrafficSelectorsIsRefused
// Related: child_rekey_initiator_answer_test.go -- TestChildRekeyAnswerWithoutTrafficSelectorsIsRefused
// VALIDATES: the payloads of a CREATE_CHILD_SA exchange that rekeys a Child SA
// (RFC 7296 Section 1.3.3): what ze sends in the request and in the response, and what
// ze refuses when the peer leaves a payload out.
// PREVENTS: a rekey request or response that drops the SA offer, the nonce or a Traffic
// Selector payload, and a receiver that answers or installs from such a message.

package engine

import (
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// rekeyPayloadCounts counts the payload kinds of a decrypted CREATE_CHILD_SA body.
type rekeyPayloadCounts struct {
	sa, nonce, ke, tsi, tsr int
	selectors               int
}

func countRekeyPayloads(inner []wire.PayloadEntry) rekeyPayloadCounts {
	var c rekeyPayloadCounts
	for i := range inner {
		switch p := inner[i].Payload.(type) {
		case *wire.PayloadSA:
			c.sa++
		case *wire.PayloadNonce:
			c.nonce++
		case *wire.PayloadKE:
			c.ke++
		case *wire.PayloadTS:
			c.selectors += len(p.TrafficSelectors)
			switch p.TSPayloadType {
			case wire.PayloadTypeTSi:
				c.tsi++
			case wire.PayloadTypeTSr:
				c.tsr++
			}
		}
	}
	return c
}

// TestRFC7296ChildRekeyRequestCarriesTheOfferNonceAndSelectors reads the rekey request ze
// builds, as the peer decrypts it, with PFS enabled and with PFS disabled.
//
// Goal: every payload the RFC 7296 Section 1.3.3 request carries is present once, and the
// optional Diffie-Hellman value is present exactly when the esp-group asks for PFS.
// Method: initiateChildRekey builds the request; the peer's IKE SA decrypts it.
//
// RFC requirement: RFC7296-1.3.3-3 positive -- ze's Child SA rekey request carries one SA payload, one Ni payload, one TSi and one TSr payload each holding at least one selector, and a KEi payload with PFS enabled and none with PFS disabled.
func TestRFC7296ChildRekeyRequestCarriesTheOfferNonceAndSelectors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mode   ipsec.PFSMode
		wantKE int
	}{
		{"pfs enable", ipsec.PFSEnable, 1},
		{"pfs disable", ipsec.PFSDisable, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, peer, _, _, raw := pfsRekeyRequest(t, tc.mode)
			got := countRekeyPayloads(lcyDecrypt(t, peer, raw))
			if got.sa != 1 {
				t.Errorf("the rekey request carries %d SA payloads, want 1", got.sa)
			}
			if got.nonce != 1 {
				t.Errorf("the rekey request carries %d Ni payloads, want 1", got.nonce)
			}
			if got.tsi != 1 || got.tsr != 1 {
				t.Errorf("the rekey request carries %d TSi and %d TSr payloads, want 1 of each", got.tsi, got.tsr)
			}
			if got.selectors < 2 {
				t.Errorf("the rekey request proposes %d traffic selectors, want at least one in TSi and one in TSr", got.selectors)
			}
			if got.ke != tc.wantKE {
				t.Errorf("the rekey request carries %d KEi payloads, want %d", got.ke, tc.wantKE)
			}
		})
	}
}

// TestRFC7296ChildRekeyRequestMissingAPayloadIsRefused hands the responder a peer's rekey
// request with one payload removed at a time.
//
// Goal: a request that lacks the SA offer, the nonce, TSi or TSr is refused as malformed
// and installs nothing, while the complete request is answered.
// Method: respondChildRekey over peerRekeyRequest, minus one payload per case.
//
// RFC requirement: RFC7296-1.3.3-3 negative -- a peer's Child SA rekey request without its SA, Ni, TSi or TSr payload is refused with errMalformedRequest (INVALID_SYNTAX) and installs no Child SA, while the complete request is answered.
func TestRFC7296ChildRekeyRequestMissingAPayloadIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		drop func(wire.PayloadEntry) bool
	}{
		{"no SA payload", func(pe wire.PayloadEntry) bool { _, ok := pe.Payload.(*wire.PayloadSA); return ok }},
		{"no Ni payload", func(pe wire.PayloadEntry) bool { _, ok := pe.Payload.(*wire.PayloadNonce); return ok }},
		{"no TSi payload", func(pe wire.PayloadEntry) bool {
			ts, ok := pe.Payload.(*wire.PayloadTS)
			return ok && ts.TSPayloadType == wire.PayloadTypeTSi
		}},
		{"no TSr payload", func(pe wire.PayloadEntry) bool {
			ts, ok := pe.Payload.(*wire.PayloadTS)
			return ok && ts.TSPayloadType == wire.PayloadTypeTSr
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa, old, dp, log := peerRekeyFixture(t)
			installedBefore := len(dp.sas)
			inner := slices.DeleteFunc(peerRekeyRequest(t, "10.2.0.0/24", "10.1.0.0/24"), tc.drop)

			_, child, err := respondChildRekey(sa, inner, old, 5, dp, log)
			if err == nil {
				t.Fatal("respondChildRekey answered a rekey request with a payload missing")
			}
			if !errors.Is(err, errMalformedRequest) {
				t.Errorf("refusal = %v, want errMalformedRequest", err)
			}
			if got := notifyForRefusal(err); got != wire.NotifyInvalidSyntax {
				t.Errorf("notify = %d, want INVALID_SYNTAX (%d)", got, wire.NotifyInvalidSyntax)
			}
			if child != nil {
				t.Error("a refused rekey installed a replacement Child SA")
			}
			if len(dp.sas) != installedBefore {
				t.Errorf("a refused rekey installed %d dataplane SAs", len(dp.sas)-installedBefore)
			}
		})
	}

	t.Run("the complete request is answered", func(t *testing.T) {
		sa, old, dp, log := peerRekeyFixture(t)
		_, child, err := respondChildRekey(sa, peerRekeyRequest(t, "10.2.0.0/24", "10.1.0.0/24"), old, 5, dp, log)
		if err != nil {
			t.Fatalf("respondChildRekey refused a complete rekey request: %v", err)
		}
		if child == nil {
			t.Fatal("no replacement Child SA was installed")
		}
	})
}

// TestRFC7296ChildRekeyResponseCarriesTheSelectorsAndMayNarrow reads the rekey response ze
// builds as responder, and gives ze as initiator a response narrowed below its proposal.
//
// Goal: ze's response names the traffic of the new SA in TSi and TSr, and ze accepts a
// response whose selectors are a subset of what it proposed and installs that subset.
// Method: respondChildRekey's response decrypted through answeredScope; then
// applyChildRekeyResponse over zeRekeyFixture, which proposed 10.1.0.0/24 <-> 10.2.0.0/24.
//
// RFC requirement: RFC7296-1.3.3-4 positive -- ze's rekey response carries TSi and TSr naming the installed scope, and ze as rekey initiator installs a response narrowed to 10.1.0.0/25 <-> 10.2.0.0/24 from a 10.1.0.0/24 <-> 10.2.0.0/24 proposal.
func TestRFC7296ChildRekeyResponseCarriesTheSelectorsAndMayNarrow(t *testing.T) {
	t.Run("ze's response carries the selectors", func(t *testing.T) {
		sa, old, dp, log := peerRekeyFixture(t)
		resp, child, err := respondChildRekey(sa, peerRekeyRequest(t, "10.2.0.0/24", "10.1.0.0/24"), old, 5, dp, log)
		if err != nil {
			t.Fatalf("respondChildRekey: %v", err)
		}
		announced := scopeText(answeredScope(t, sa, resp))
		if want := []string{"10.2.0.0/24 <-> 10.1.0.0/24"}; !slices.Equal(announced, want) {
			t.Errorf("the response announces %v, want %v", announced, want)
		}
		if installed := scopeText(installedScope(child)); !slices.Equal(announced, installed) {
			t.Errorf("the response announces %v and the replacement was installed with %v", announced, installed)
		}
	})

	t.Run("a response narrowed to a subset of the proposal is installed", func(t *testing.T) {
		sa, _, pending, dp, log := zeRekeyFixture(t, "10.1.0.0/25", "10.2.0.0/24")
		child, err := applyChildRekeyResponse(sa, pending, peerRekeyAnswer(t, "10.1.0.0/25", "10.2.0.0/24"), dp, log)
		if err != nil {
			t.Fatalf("applyChildRekeyResponse refused a response narrowed inside the proposal: %v", err)
		}
		want := []string{"10.1.0.0/25 <-> 10.2.0.0/24"}
		if got := scopeText(installedInitiatorScope(child)); !slices.Equal(got, want) {
			t.Errorf("the replacement was installed with %v, want the answered subset %v", got, want)
		}
	})
}
