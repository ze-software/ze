package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: AC-1. skSend*/skRecv* select the SK_* key by sa.IsInitiator: the IKE
// SA initiator sends with SK_ei/SK_ai and receives with SK_er/SK_ar; the responder
// is the exact mirror. This is the single generalization that lets one SK crypto
// path serve both roles (spec-ipsec-14 Phase 1).
// PREVENTS: a responder encrypting with the initiator's keys (charon would fail to
// decrypt), and silently regressing the initiator's key selection.
func TestSKKeyDirectionHelpers(t *testing.T) {
	sa := testSAWithKeys(t)
	k := sa.SKKeys

	// Initiator role: send=i, recv=r.
	sa.IsInitiator = true
	if !bytes.Equal(skSendEncKey(sa), k.SK_ei) || !bytes.Equal(skSendIntegKey(sa), k.SK_ai) {
		t.Error("initiator must SEND with SK_ei/SK_ai")
	}
	if !bytes.Equal(skRecvEncKey(sa), k.SK_er) || !bytes.Equal(skRecvIntegKey(sa), k.SK_ar) {
		t.Error("initiator must RECEIVE with SK_er/SK_ar")
	}

	// Responder role: send=r, recv=i (mirror).
	sa.IsInitiator = false
	if !bytes.Equal(skSendEncKey(sa), k.SK_er) || !bytes.Equal(skSendIntegKey(sa), k.SK_ar) {
		t.Error("responder must SEND with SK_er/SK_ar")
	}
	if !bytes.Equal(skRecvEncKey(sa), k.SK_ei) || !bytes.Equal(skRecvIntegKey(sa), k.SK_ai) {
		t.Error("responder must RECEIVE with SK_ei/SK_ai")
	}
}

// decryptOneNonce parses a freshly built SK message and returns the single Nonce
// payload's data after decrypting with the given SA's receive keys.
func decryptOneNonce(t *testing.T, sa *SA, raw []byte) []byte {
	t.Helper()
	var msg wire.Message
	if err := msg.ReadFrom(raw); err != nil {
		t.Fatalf("parse SK message: %v", err)
	}
	var sk *wire.PayloadSK
	for _, pe := range msg.Payloads {
		if s, ok := pe.Payload.(*wire.PayloadSK); ok {
			sk = s
		}
	}
	if sk == nil {
		t.Fatal("no SK payload in message")
	}
	plain, err := decryptSKPayload(sa, raw, sk)
	if err != nil {
		t.Fatalf("decryptSKPayload: %v", err)
	}
	inner, err := wire.ParsePayloadChain(plain, sk.InnerNextPayload)
	if err != nil {
		t.Fatalf("parse inner chain: %v", err)
	}
	for _, pe := range inner {
		if n, ok := pe.Payload.(*wire.PayloadNonce); ok {
			return n.NonceData
		}
	}
	t.Fatal("no Nonce payload after decrypt")
	return nil
}

// skRoundTrip seals a known nonce as `sender` and decrypts as `receiver`, asserting
// the plaintext survives. sender and receiver share one SKKeys but hold opposite
// roles, simulating the two ends of a real IKE SA.
func skRoundTrip(t *testing.T, sender, receiver *SA, senderFlags uint8) {
	t.Helper()
	marker := []byte("ROLE-DIRECTION-ROUNDTRIP-NONCE!!")
	inner := []wire.PayloadEntry{{Payload: &wire.PayloadNonce{NonceData: marker}}}
	raw, err := buildEncryptedMessageEx(sender, inner, 1, wire.ExchangeInformational, senderFlags)
	if err != nil {
		t.Fatalf("build SK message: %v", err)
	}
	got := decryptOneNonce(t, receiver, raw)
	if !bytes.Equal(got, marker) {
		t.Fatalf("round-trip nonce mismatch: got %q want %q", got, marker)
	}
}

// VALIDATES: AC-1 (round-trip). A message the initiator SEALS decrypts under the
// responder's RECEIVE keys and vice versa, for both AES-CBC+HMAC and AES-GCM.
// Sharing one SKKeys with opposite roles is exactly the two-endpoint case: it fails
// if either direction picks the wrong SK_* half.
// PREVENTS: the responder being undecryptable by a real peer (the core reason the
// responder was unimplemented).
func TestSKDirectionInitiatorResponderRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		mk   func(t *testing.T) *SA
	}{
		{"cbc", testSAWithKeys},
		{"gcm", testSAWithGCMKeys},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ini := tc.mk(t)
			ini.IsInitiator = true

			resp := *ini // shares SKKeys/Proposal, opposite role
			resp.IsInitiator = false

			// Initiator sends -> responder receives.
			skRoundTrip(t, ini, &resp, wire.FlagInitiator)
			// Responder sends -> initiator receives.
			skRoundTrip(t, &resp, ini, wire.FlagResponse)
		})
	}
}

// VALIDATES: AC-1 / A-5 fix. installChildSA keys the SEND (outbound) SA with the
// KEYMAT half matching this side's role: the exchange initiator's outbound uses
// EncryptKeyI, the responder's outbound uses EncryptKeyR, with inbound the mirror.
// PREVENTS: a responder installing ESP keys backwards (the latent respondChildRekey
// swap) so decrypted ESP traffic would fail integrity/decrypt on a real dataplane.
func TestChildInstallKeyDirectionByRole(t *testing.T) {
	log := slogutil.DiscardLogger()

	for _, tc := range []struct {
		name             string
		localIsInitiator bool
	}{
		{"initiator-role", true},
		{"responder-role", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sa := testSA()
			sa.IsInitiator = tc.localIsInitiator
			// Distinct nonces so EncryptKeyI != EncryptKeyR in the derived KEYMAT.
			for i := range sa.LocalNonce {
				sa.LocalNonce[i] = byte(i)
			}
			for i := range sa.RemoteNonce {
				sa.RemoteNonce[i] = byte(255 - i)
			}
			dp := &mockDP{}
			child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
			if err != nil {
				t.Fatalf("createFirstChildSA: %v", err)
			}
			if child.LocalIsInitiator != tc.localIsInitiator {
				t.Fatalf("child.LocalIsInitiator = %v, want %v", child.LocalIsInitiator, tc.localIsInitiator)
			}
			if len(dp.sas) != 2 {
				t.Fatalf("installed SAs = %d, want 2", len(dp.sas))
			}
			inbound, outbound := dp.sas[0], dp.sas[1]

			wantOutEnc := child.Keys.EncryptKeyR
			wantInEnc := child.Keys.EncryptKeyI
			if tc.localIsInitiator {
				wantOutEnc = child.Keys.EncryptKeyI
				wantInEnc = child.Keys.EncryptKeyR
			}
			if !bytes.Equal(outbound.EncKey, wantOutEnc) {
				t.Error("outbound SA keyed with the wrong KEYMAT half for this role")
			}
			if !bytes.Equal(inbound.EncKey, wantInEnc) {
				t.Error("inbound SA keyed with the wrong KEYMAT half for this role")
			}
			child.Clear()
		})
	}
}

// VALIDATES: AC-1. A responder-role first Child SA derives KEYMAT from the absolute
// (Ni | Nr) nonce pair, not this side's (Local | Remote). For a responder Local=Nr
// and Remote=Ni, so feeding Local/Remote directly would compute prf+(SK_d, Nr|Ni)
// and mismatch the peer. Two SAs with mirrored Local/Remote nonces (the two ends)
// must derive the SAME KEYMAT.
// PREVENTS: silent ESP key mismatch on the responder (packets drop, no error).
func TestChildKeymatAbsoluteNonceOrder(t *testing.T) {
	log := slogutil.DiscardLogger()

	ni := make([]byte, 32)
	nr := make([]byte, 32)
	for i := range ni {
		ni[i] = byte(i + 1)
		nr[i] = byte(100 + i)
	}
	skD := make([]byte, 32)
	for i := range skD {
		skD[i] = byte(7 * i)
	}

	mk := func(isInitiator bool) *SA {
		sa := testSA()
		sa.IsInitiator = isInitiator
		sa.SKKeys.SK_d = skD
		if isInitiator { // Local=Ni, Remote=Nr
			copy(sa.LocalNonce, ni)
			copy(sa.RemoteNonce, nr)
		} else { // Local=Nr, Remote=Ni
			copy(sa.LocalNonce, nr)
			copy(sa.RemoteNonce, ni)
		}
		return sa
	}

	iniChild, err := createFirstChildSA(mk(true), testESPGroup(), "10.0.0.1", "10.0.0.2", 1, &mockDP{}, log)
	if err != nil {
		t.Fatalf("initiator createFirstChildSA: %v", err)
	}
	respChild, err := createFirstChildSA(mk(false), testESPGroup(), "10.0.0.2", "10.0.0.1", 1, &mockDP{}, log)
	if err != nil {
		t.Fatalf("responder createFirstChildSA: %v", err)
	}
	// Both ends derive the identical KEYMAT halves from the absolute nonce order.
	if !bytes.Equal(iniChild.Keys.EncryptKeyI, respChild.Keys.EncryptKeyI) ||
		!bytes.Equal(iniChild.Keys.EncryptKeyR, respChild.Keys.EncryptKeyR) {
		t.Error("initiator and responder derived different Child KEYMAT: nonce order not absolute")
	}
	iniChild.Clear()
	respChild.Clear()
}
