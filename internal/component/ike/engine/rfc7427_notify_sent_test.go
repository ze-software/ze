// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the IKE_SA_INIT exchange
// Related: rfc7427_sighash_test.go -- the received half of the same condition
// VALIDATES: Ze sends the SIGNATURE_HASH_ALGORITHMS notify in its IKE_SA_INIT request and
// in its IKE_SA_INIT response (RFC 7427 Section 3), which is the "sent" half of the
// condition under which AUTH method 14 is allowed.
// PREVENTS: an IKE_SA_INIT that drops the notify while Ze still signs with method 14.

package engine

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// signatureHashAlgos answers the hash algorithm identifiers of every
// SIGNATURE_HASH_ALGORITHMS notify in one message, and how many such notifies it holds.
func signatureHashAlgos(t *testing.T, raw []byte) (algos []uint16, notifies int) {
	t.Helper()
	msg := parseMsg(t, raw)
	for _, entry := range msg.Payloads {
		notify, ok := entry.Payload.(*wire.PayloadNotify)
		if !ok || notify.NotifyMsgType != wire.NotifySignatureHashAlgorithms {
			continue
		}
		notifies++
		data := notify.NotificationData
		if len(data)%2 != 0 {
			t.Fatalf("the notify data is %d octets, want a whole number of 2-octet identifiers", len(data))
		}
		for i := 0; i < len(data); i += 2 {
			algos = append(algos, binary.BigEndian.Uint16(data[i:i+2]))
		}
	}
	return algos, notifies
}

// TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify proves each end of an
// IKE_SA_INIT exchange Ze takes part in sends the notify.
//
// Goal: computeX509Auth emits method 14 once the peer's list has arrived, and it does not
// check that Ze sent its own list, because Ze always does. This test is what makes that
// premise a checked fact. Method: Ze's real IKE_SA_INIT request, and the real response Ze's
// responder answers it with, are each parsed. Each MUST carry exactly one
// SIGNATURE_HASH_ALGORITHMS notify listing SHA2-256, SHA2-384 and SHA2-512.
//
// RFC 7427 Section 3: "the peer is only allowed to use this authentication method if the
// Notify payload of type SIGNATURE_HASH_ALGORITHMS has been sent and received by each
// peer."
//
// RFC requirement: RFC7427-3-4 positive -- the sent half: Ze's IKE_SA_INIT request and its IKE_SA_INIT response each carry exactly one SIGNATURE_HASH_ALGORITHMS notify, listing SHA2-256, SHA2-384 and SHA2-512.
func TestRFC7427IKESAInitSendsTheSignatureHashAlgorithmsNotify(t *testing.T) {
	log := slogutil.DiscardLogger()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "rfc7427-sent")
	ini, err := newInitiatorSA("ze", iniPeer, testIKEGroup(), testESPGroup())
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	request := buildSAInitRequest(ini, testIKEGroup())

	resp, err := newResponderSA("ze", respPeer, testIKEGroup(), testESPGroup(), ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	handleSAInitRequest(resp, parseMsg(t, request), request, nil, nil, log)
	if len(resp.LastSentMsg) == 0 {
		t.Fatal("the responder sent no IKE_SA_INIT response")
	}

	want := []uint16{hashAlgoSHA2256, hashAlgoSHA2384, hashAlgoSHA2512}
	for who, raw := range map[string][]byte{"request": request, "response": resp.LastSentMsg} {
		algos, notifies := signatureHashAlgos(t, raw)
		if notifies != 1 {
			t.Fatalf("the IKE_SA_INIT %s carries %d SIGNATURE_HASH_ALGORITHMS notifies, want 1", who, notifies)
		}
		if len(algos) != len(want) {
			t.Fatalf("the IKE_SA_INIT %s lists %v, want %v", who, algos, want)
		}
		for i := range want {
			if algos[i] != want[i] {
				t.Fatalf("the IKE_SA_INIT %s lists %v, want %v", who, algos, want)
			}
		}
	}
}
