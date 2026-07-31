// VALIDATES: the responder holds the SPI Size rule that belongs to an initial IKE SA
// negotiation. An IKE_SA_INIT request whose proposals carry an SPI is refused, and the
// same request with the size the section requires is answered.
// PREVENTS: a receiver that reads the parse-layer set alone. That set accepts 0 or 8 for
// IKE, because a later negotiation carries 8. Only the exchange says which of the two is
// legal, and the parse layer never sees the exchange.
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// spzSetSPISize rewrites every proposal of an IKE_SA_INIT request to carry the given SPI
// Size, and returns the datagram again. A size of 8 is what a rekey negotiation carries.
func spzSetSPISize(t *testing.T, req []byte, size uint8) []byte {
	t.Helper()
	msg := parseMsg(t, req)
	found := false
	for _, pe := range msg.Payloads {
		sa, ok := pe.Payload.(*wire.PayloadSA)
		if !ok {
			continue
		}
		for i := range sa.Proposals {
			sa.Proposals[i].SPISize = size
			sa.Proposals[i].SPI = make([]byte, size)
		}
		found = true
	}
	if !found {
		t.Fatal("the IKE_SA_INIT request carries no SA payload")
	}
	buf := make([]byte, 4096)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		t.Fatalf("rebuild the IKE_SA_INIT request: %v", err)
	}
	return buf[:n]
}

// RFC requirement: RFC7296-3.3.1-2 negative -- RFC 7296 Section 3.3.1: "For an initial IKE SA
// negotiation, this field MUST be zero". The SPI comes from the outer header.
// An IKE_SA_INIT request whose proposals carry an 8-octet SPI is an initial negotiation.
// It breaks the rule. The responder refuses it and answers nothing.
// RFC requirement: RFC7296-3.3.1-2 positive -- the same request with SPI Size zero is answered, so
// the refusal names the SPI rather than the exchange. Ze builds its own IKE_SA_INIT
// proposals with SPI Size zero, which the accepted request shows.
func TestSpzInitialIKESANegotiationNeedsZeroSPISize(t *testing.T) {
	log := slogutil.DiscardLogger()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "spisize-psk")
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()

	build := func(t *testing.T) ([]byte, *SA) {
		t.Helper()
		ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
		if err != nil {
			t.Fatalf("newInitiatorSA: %v", err)
		}
		req := buildSAInitRequest(ini, ikeGroup)
		resp, err := newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
		if err != nil {
			t.Fatalf("newResponderSA: %v", err)
		}
		return req, resp
	}

	req, resp := build(t)
	handleSAInitRequest(resp, parseMsg(t, req), req, nil, nil, log)
	if resp.State != StateSAInitReceived {
		t.Fatalf("the request ze builds left the responder at %v, want sa-init-responded", resp.State)
	}
	sa := pnmSAPayload(t, parseMsg(t, req).Payloads)
	for i := range sa.Proposals {
		if sa.Proposals[i].SPISize != 0 {
			t.Errorf("proposal %d of our own IKE_SA_INIT carries SPI Size %d, want 0",
				i, sa.Proposals[i].SPISize)
		}
	}

	rekeySized, resp := build(t)
	rekeySized = spzSetSPISize(t, rekeySized, 8)
	handleSAInitRequest(resp, parseMsg(t, rekeySized), rekeySized, nil, nil, log)
	if resp.State != StateDead {
		t.Errorf("an IKE_SA_INIT carrying an 8-octet SPI left the responder at %v, want dead", resp.State)
	}
	if len(resp.LastSentMsg) != 0 {
		t.Error("the responder answered a request whose SPI Size the section forbids")
	}
}
