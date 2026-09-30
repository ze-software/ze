// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- IKE_SA_INIT and IKE_AUTH
// Related: responder.go -- handleSAInitRequest, handleAuthRequest
// Related: fsm.go -- handleSAInitResponse, handleAuthResponse
//
// VALIDATES: RFC 7296 Section 1.2 over Ze's own four messages, both roles built and read
// by production code: IKE_SA_INIT negotiates the algorithms, exchanges nonces and runs
// the Diffie-Hellman exchange; IKE_AUTH authenticates the previous messages, exchanges
// identities and certificates, and establishes the first Child SA. The encryption
// boundary of the same paragraph is TestInitialExchangeEncryptionBoundary.
package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// iexFirst returns the first payload of type T in a chain whose generic type matches
// payloadType, and whether one was there.
func iexFirst[T wire.Payload](entries []wire.PayloadEntry, payloadType uint8) (T, bool) {
	for _, e := range entries {
		if e.Payload.Type() != payloadType {
			continue
		}
		if p, ok := e.Payload.(T); ok {
			return p, true
		}
	}
	var none T
	return none, false
}

// iexX509 is the certificate configuration autLoadPKI installs.
func iexX509() ipsec.AuthConfig {
	return ipsec.AuthConfig{Mode: ipsec.AuthX509, Certificate: autCertName, CACertificate: autCAName}
}

// iexSAInit checks one IKE_SA_INIT message: SA, KE and Nonce are all there, and none is
// encrypted. It returns the three.
func iexSAInit(t *testing.T, raw []byte, who string) (*wire.PayloadSA, *wire.PayloadKE, *wire.PayloadNonce) {
	t.Helper()
	msg := parseMsg(t, raw)
	if msg.Header.ExchangeType != wire.ExchangeIKESAInit {
		t.Fatalf("%s: exchange type %d, want IKE_SA_INIT", who, msg.Header.ExchangeType)
	}
	sa, okSA := iexFirst[*wire.PayloadSA](msg.Payloads, wire.PayloadTypeSA)
	ke, okKE := iexFirst[*wire.PayloadKE](msg.Payloads, wire.PayloadTypeKE)
	nonce, okNonce := iexFirst[*wire.PayloadNonce](msg.Payloads, wire.PayloadTypeNonce)
	if !okSA || !okKE || !okNonce {
		t.Fatalf("%s: SA=%v KE=%v Nonce=%v, want all three", who, okSA, okKE, okNonce)
	}
	return sa, ke, nonce
}

// iexAuthChain checks one decrypted IKE_AUTH chain: the identity of the sender's role,
// a CERT, an AUTH, an SA whose proposals are all ESP, TSi and TSr.
// It returns the identity.
func iexAuthChain(t *testing.T, inner []wire.PayloadEntry, idType uint8, who string) *wire.PayloadID {
	t.Helper()
	id, okID := iexFirst[*wire.PayloadID](inner, idType)
	_, okCert := iexFirst[*wire.PayloadCERT](inner, wire.PayloadTypeCERT)
	_, okAuth := iexFirst[*wire.PayloadAUTH](inner, wire.PayloadTypeAUTH)
	sa, okSA := iexFirst[*wire.PayloadSA](inner, wire.PayloadTypeSA)
	_, okTSi := iexFirst[*wire.PayloadTS](inner, wire.PayloadTypeTSi)
	_, okTSr := iexFirst[*wire.PayloadTS](inner, wire.PayloadTypeTSr)
	if !okID || !okCert || !okAuth || !okSA || !okTSi || !okTSr {
		t.Fatalf("%s: ID=%v CERT=%v AUTH=%v SA=%v TSi=%v TSr=%v, want all six",
			who, okID, okCert, okAuth, okSA, okTSi, okTSr)
	}
	for _, p := range sa.Proposals {
		if p.ProtocolID != wire.ProtocolESP {
			t.Fatalf("%s: SA proposal for protocol %d, want ESP (the first Child SA)", who, p.ProtocolID)
		}
	}
	return id
}

// RFC requirement: RFC7296-1.2-1 positive -- one X.509 handshake between two Ze SAs, all
// four messages captured. IKE_SA_INIT: request and response each carry SA, KE and Nonce;
// the response's single proposal is one the request offered and both SAs hold it; each
// side holds the other's nonce and the two nonces differ; both KE payloads name the
// negotiated group, each side holds the other's public value, and both derive the same
// SK_d. IKE_AUTH: the request carries IDi, CERT, AUTH, an ESP SA, TSi and TSr, the
// response IDr, CERT, AUTH, one ESP SA proposal, TSi and TSr; each side records the
// other's identity, both reach Established, and the initiator holds the negotiated
// Child SA selectors.
func TestRFC7296InitialExchangesCarryWhatSection12Names(t *testing.T) {
	autLoadPKI(t)
	log := slogutil.DiscardLogger()
	ini, resp, ps := autSAInitPair(t, iexX509())

	// IKE_SA_INIT: negotiate cryptographic algorithms.
	reqSA, reqKE, reqNonce := iexSAInit(t, ini.InitiatorSAInitMsg, "IKE_SA_INIT request")
	respSA, respKE, respNonce := iexSAInit(t, ini.ResponderSAInitMsg, "IKE_SA_INIT response")
	if len(respSA.Proposals) != 1 {
		t.Fatalf("IKE_SA_INIT response carries %d proposals, want 1", len(respSA.Proposals))
	}
	chosen := respSA.Proposals[0]
	offered := false
	for _, p := range reqSA.Proposals {
		if p.Number != chosen.Number || len(p.Transforms) < len(chosen.Transforms) {
			continue
		}
		matched := 0
		for _, ct := range chosen.Transforms {
			for _, pt := range p.Transforms {
				if pt.Type == ct.Type && pt.ID == ct.ID {
					matched++
					break
				}
			}
		}
		offered = offered || matched == len(chosen.Transforms)
	}
	if !offered {
		t.Fatalf("the accepted proposal %+v is not one the request offered", chosen)
	}
	if ini.Proposal != resp.Proposal {
		t.Fatalf("the two SAs negotiated different proposals: initiator %+v, responder %+v", ini.Proposal, resp.Proposal)
	}

	// IKE_SA_INIT: exchange nonces.
	if !bytes.Equal(ini.LocalNonce, reqNonce.NonceData) || !bytes.Equal(resp.RemoteNonce, reqNonce.NonceData) {
		t.Fatal("the initiator's nonce is not the one both sides hold as Ni")
	}
	if !bytes.Equal(resp.LocalNonce, respNonce.NonceData) || !bytes.Equal(ini.RemoteNonce, respNonce.NonceData) {
		t.Fatal("the responder's nonce is not the one both sides hold as Nr")
	}
	if bytes.Equal(reqNonce.NonceData, respNonce.NonceData) {
		t.Fatal("Ni equals Nr")
	}

	// IKE_SA_INIT: do a Diffie-Hellman exchange.
	group := uint16(ini.Proposal.DHGroup.ID)
	if reqKE.DHGroup != group || respKE.DHGroup != group {
		t.Fatalf("KE groups %d / %d, want the negotiated group %d", reqKE.DHGroup, respKE.DHGroup, group)
	}
	if !bytes.Equal(resp.RemoteDHPub, reqKE.KeyExchangeData) || !bytes.Equal(ini.RemoteDHPub, respKE.KeyExchangeData) {
		t.Fatal("a side does not hold the other's KE public value")
	}
	if ini.SKKeys == nil || resp.SKKeys == nil || len(ini.SKKeys.SK_d) == 0 {
		t.Fatal("IKE_SA_INIT derived no SK_d")
	}
	if !bytes.Equal(ini.SKKeys.SK_d, resp.SKKeys.SK_d) {
		t.Fatal("the two sides derived different SK_d from the Diffie-Hellman exchange")
	}

	// IKE_AUTH request: identities, certificates, AUTH, the first Child SA's SA and TS.
	authReq := ini.LastSentMsg
	reqInner, err := decryptAndParse(resp, parseMsg(t, authReq), authReq)
	if err != nil {
		t.Fatalf("decrypt the IKE_AUTH request: %v", err)
	}
	idi := iexAuthChain(t, reqInner, wire.PayloadTypeIDi, "IKE_AUTH request")

	ps.handleAuthRequest(resp, parseMsg(t, authReq), authReq, nil, nil, log)
	if resp.State != StateEstablished {
		t.Fatalf("responder state %v after IKE_AUTH, want Established", resp.State)
	}
	if resp.RemoteIDPayload == nil || !bytes.Equal(resp.RemoteIDPayload.IDData, idi.IDData) {
		t.Fatal("the responder did not record the initiator's identity")
	}

	// IKE_AUTH response.
	authResp := resp.LastSentMsg
	respInner, err := decryptAndParse(ini, parseMsg(t, authResp), authResp)
	if err != nil {
		t.Fatalf("decrypt the IKE_AUTH response: %v", err)
	}
	idr := iexAuthChain(t, respInner, wire.PayloadTypeIDr, "IKE_AUTH response")
	if sa, _ := iexFirst[*wire.PayloadSA](respInner, wire.PayloadTypeSA); len(sa.Proposals) != 1 {
		t.Fatalf("IKE_AUTH response SA carries %d proposals, want the one accepted", len(sa.Proposals))
	}

	handleAuthResponse(ini, parseMsg(t, authResp), authResp, nil, nil, log)
	if ini.State != StateEstablished {
		t.Fatalf("initiator state %v after IKE_AUTH, want Established", ini.State)
	}
	if ini.RemoteIDPayload == nil || !bytes.Equal(ini.RemoteIDPayload.IDData, idr.IDData) {
		t.Fatal("the initiator did not record the responder's identity")
	}
	if ini.NegotiatedTSi == nil || ini.NegotiatedTSr == nil {
		t.Fatal("the initiator holds no negotiated Child SA selectors")
	}
}

// RFC requirement: RFC7296-1.2-1 negative -- "authenticate the previous messages": in
// PSK and X.509 mode, a single altered octet in the IKE_SA_INIT message a side stored
// (the responder's copy of the request, or the initiator's copy of the response) makes
// that side's IKE_AUTH verification fail and the SA does not reach Established; the
// same pair untouched does.
func TestRFC7296TamperedIKESAInitFailsIKEAuth(t *testing.T) {
	autLoadPKI(t)
	log := slogutil.DiscardLogger()
	modes := map[string]ipsec.AuthConfig{
		"psk":  {Mode: ipsec.AuthPreSharedSecret, PSK: "iex-secret"},
		"x509": iexX509(),
	}
	for name, auth := range modes {
		// Control: untouched, both sides establish.
		ini, resp, ps := autSAInitPair(t, auth)
		ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
		handleAuthResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, nil, nil, log)
		if resp.State != StateEstablished || ini.State != StateEstablished {
			t.Fatalf("%s control: responder %v initiator %v, want both Established", name, resp.State, ini.State)
		}

		// The responder's stored copy of the IKE_SA_INIT request.
		ini, resp, ps = autSAInitPair(t, auth)
		resp.InitiatorSAInitMsg[len(resp.InitiatorSAInitMsg)-1] ^= 0x01
		ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
		if resp.State == StateEstablished {
			t.Errorf("%s: the responder authenticated an IKE_AUTH against an altered IKE_SA_INIT request", name)
		}

		// The initiator's stored copy of the IKE_SA_INIT response.
		ini, resp, ps = autSAInitPair(t, auth)
		ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
		if resp.State != StateEstablished {
			t.Fatalf("%s: responder %v, want Established before the initiator's check", name, resp.State)
		}
		ini.ResponderSAInitMsg[len(ini.ResponderSAInitMsg)-1] ^= 0x01
		handleAuthResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, nil, nil, log)
		if ini.State == StateEstablished {
			t.Errorf("%s: the initiator authenticated an IKE_AUTH against an altered IKE_SA_INIT response", name)
		}
	}
}
