// Design: docs/architecture/ike/ipsec-14-responder.md -- the IKE_AUTH response
// Related: rfc7296_cp_test.go -- the sweep over every CP payload Ze builds
// VALIDATES: Ze, which supports no Configuration payload, answers a peer's request for its
// APPLICATION_VERSION with no CP payload (RFC 7296 Section 2.20).
// PREVENTS: a responder that answers the request with a version string, or that fails the
// exchange because a request carried a CP payload it does not support.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC7296ResponderAnswersAVersionRequestWithNoCP drives Ze's responder with an
// IKE_AUTH request that asks for the application version.
//
// Goal: the sweep in rfc7296_cp_test.go proves no builder emits APPLICATION_VERSION, but
// no builder is ever asked for it. Method: the initiator's real IKE_AUTH request is
// decrypted, a CP(CFG_REQUEST) payload carrying an empty APPLICATION_VERSION attribute is
// appended to it, and it is encrypted again with the initiator's keys. handleAuthRequest
// answers it. The response MUST carry no CP payload, and the IKE SA MUST be established,
// so the answer is the normal IKE_AUTH response and not an error that happens to carry
// no CP payload.
//
// RFC 7296 Section 2.20: "An IKE implementation MAY decline to give out version
// information prior to authentication or even after authentication in case some
// implementation is known to have some security weakness. In that case, it MUST either
// return an empty string or no CP payload if CP is not supported."
//
// RFC requirement: RFC7296-2.20-1 positive -- an IKE_AUTH request carrying CP(CFG_REQUEST) with an empty APPLICATION_VERSION draws an IKE_AUTH response that carries no CP payload, and the responder's IKE SA is established.
func TestRFC7296ResponderAnswersAVersionRequestWithNoCP(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, resp, ps := peerSPIPreAuth(t)

	hdr := parseMsg(t, ini.LastSentMsg).Header
	inner := mbDecrypt(t, resp, ini.LastSentMsg)
	inner = append(inner, wire.PayloadEntry{Payload: &wire.PayloadCP{
		CFGType: wire.CFGTypeRequest,
		Attrs:   []wire.ConfigAttr{{Type: wire.CPAttrApplicationVersion}},
	}})
	request, err := buildEncryptedMessageEx(ini, inner, hdr.MessageID, hdr.ExchangeType, hdr.Flags)
	if err != nil {
		t.Fatalf("buildEncryptedMessageEx: %v", err)
	}

	ps.handleAuthRequest(resp, parseMsg(t, request), request, nil, nil, log)
	if resp.State != StateEstablished {
		t.Fatalf("responder state = %v, want established: a CP request is not a reason to fail IKE_AUTH", resp.State)
	}
	answer := mbDecrypt(t, ini, resp.LastSentMsg)
	for _, entry := range answer {
		if cp, ok := entry.Payload.(*wire.PayloadCP); ok {
			t.Fatalf("the IKE_AUTH response carries a CP payload (%+v), want none: Ze supports no CP", cp)
		}
	}
	if len(answer) == 0 {
		t.Fatal("the IKE_AUTH response carries no payload at all")
	}
}
