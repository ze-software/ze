// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the INFORMATIONAL exchange
// Related: rfc7296_cp_version_test.go -- the same request inside IKE_AUTH
// VALIDATES: Ze answers a CP(CFG_REQUEST) for APPLICATION_VERSION sent in an INFORMATIONAL
// exchange after the IKE SA exists, the method RFC 7296 Section 2.20 describes, with a
// response that carries no CP payload (RFC 7296 Section 2.20).
// PREVENTS: an INFORMATIONAL responder that answers with a version string, or that
// reflects the version string the requester put in its own request.

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// informationalVersionAnswer sends Ze's responder an INFORMATIONAL request carrying
// CP(CFG_REQUEST) with one APPLICATION_VERSION attribute of the given value, and returns
// the decrypted inner payloads of the response it built.
func informationalVersionAnswer(t *testing.T, version []byte) []wire.PayloadEntry {
	t.Helper()
	log := slogutil.DiscardLogger()
	ini, resp, ps := establishPSK(t)
	resp.mobike.enabled = true

	request := &wire.Message{Header: wire.Header{
		InitiatorSPI: resp.InitiatorSPI, ResponderSPI: resp.ResponderSPI,
		MajorVersion: 2, ExchangeType: wire.ExchangeInformational,
		Flags: wire.FlagInitiator, MessageID: resp.ExpectedMsgID,
	}}
	inner := []wire.PayloadEntry{{Payload: &wire.PayloadCP{
		CFGType: wire.CFGTypeRequest,
		Attrs:   []wire.ConfigAttr{{Type: wire.CPAttrApplicationVersion, Value: version}},
	}}}
	resp.lastResponse = nil
	resp.lastResponseSet = false
	ps.handleInformationalOwned(resp, request, inner, false, nil, nil, log)
	if resp.lastResponse == nil {
		t.Fatal("the responder built no INFORMATIONAL response to the version request")
	}
	hdr := parseMsg(t, resp.lastResponse).Header
	if hdr.ExchangeType != wire.ExchangeInformational || hdr.Flags&wire.FlagResponse == 0 {
		t.Fatalf("the answer is exchange %d flags %#x, want an INFORMATIONAL response",
			hdr.ExchangeType, hdr.Flags)
	}
	return mbDecrypt(t, ini, resp.lastResponse)
}

// TestRFC7296InformationalVersionRequestDrawsNoCP asks Ze for its application version in
// an INFORMATIONAL exchange after the IKE SA is established.
//
// Goal: Section 2.20 names the INFORMATIONAL exchange as where a peer asks for the
// version, and Ze, which supports no CP, MUST answer with no CP payload. Method: the
// product INFORMATIONAL handler answers a CP(CFG_REQUEST) with an empty
// APPLICATION_VERSION, and the response the peer decrypts carries no CP payload.
//
// RFC 7296 Section 2.20: "An IKE implementation MAY decline to give out version
// information prior to authentication or even after authentication in case some
// implementation is known to have some security weakness. In that case, it MUST either
// return an empty string or no CP payload if CP is not supported."
//
// RFC requirement: RFC7296-2.20-1 positive -- after the IKE SA is established, an INFORMATIONAL request carrying CP(CFG_REQUEST) with an empty APPLICATION_VERSION draws an INFORMATIONAL response that carries no CP payload.
func TestRFC7296InformationalVersionRequestDrawsNoCP(t *testing.T) {
	for _, entry := range informationalVersionAnswer(t, nil) {
		if cp, ok := entry.Payload.(*wire.PayloadCP); ok {
			t.Fatalf("the INFORMATIONAL answer carries CP type %d with %d attributes, want no CP payload",
				cp.CFGType, len(cp.Attrs))
		}
	}
}

// TestRFC7296InformationalVersionRequestReflectsNoVersionString pushes the INFORMATIONAL
// responder toward the non-compliant answer: the request carries a non-empty version
// string, so a responder that copied the requested attributes into its answer would return
// a non-empty version string.
//
// RFC requirement: RFC7296-2.20-1 negative -- an INFORMATIONAL request whose APPLICATION_VERSION attribute carries a non-empty version string draws a response with no CP payload and no APPLICATION_VERSION attribute, so Ze returns no non-empty version string.
func TestRFC7296InformationalVersionRequestReflectsNoVersionString(t *testing.T) {
	for _, entry := range informationalVersionAnswer(t, []byte("peer-ike 6.0.1")) {
		cp, ok := entry.Payload.(*wire.PayloadCP)
		if !ok {
			continue
		}
		for _, attr := range cp.Attrs {
			if attr.Type == wire.CPAttrApplicationVersion && len(attr.Value) > 0 {
				t.Fatalf("the INFORMATIONAL answer returns the version string %q", attr.Value)
			}
		}
		t.Fatalf("the INFORMATIONAL answer carries CP type %d, want no CP payload", cp.CFGType)
	}
}
