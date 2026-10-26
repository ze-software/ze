// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- IKEv2 transform type registry
// Related: dataplane/rfc8221_esp_install_linux_test.go -- what the MUST algorithms install
//
// VALIDATES: the RFC 8221 MUST NOT grades for ESP. No ESP configuration name resolves to
// a forbidden encryption or integrity Transform ID, and the Child SA negotiation
// (NegotiateESP) refuses an offer carrying one against every ESP cipher this build can
// be configured with, while the implemented ciphers are negotiated.
// PREVENTS: a Child SA keyed with DES, Blowfish, an UNSPECIFIED cipher, MD5 or DES-MAC.
package crypto

import (
	"testing"
)

// rfc8221ImplementedESPCiphers is the set of ESP encryption Transform IDs this build can
// configure: ENCR_3DES (SHOULD NOT), ENCR_NULL, ENCR_AES_CBC and ENCR_AES_GCM_16 (MUST),
// and ENCR_CHACHA20_POLY1305 (SHOULD), per RFC 8221 Section 5. AES CCM is in the IKE
// registry and refused for ESP at config parse.
var rfc8221ImplementedESPCiphers = map[EncryptionID]bool{
	ENCR_3DES:              true,
	ENCR_NULL:              true,
	ENCR_AES_CBC:           true,
	ENCR_AES_CCM_8:         true,
	ENCR_AES_CCM_12:        true,
	ENCR_AES_CCM_16:        true,
	ENCR_AES_GCM_16:        true,
	ENCR_CHACHA20_POLY1305: true,
}

// rfc8221ESPPolicies answers one local ESP proposal for every ESP encryption name this
// build can configure, each with HMAC-SHA2-256-128 beside a non-AEAD cipher and integrity
// NONE beside an AEAD one.
func rfc8221ESPPolicies(t *testing.T) []ESPProposal {
	t.Helper()
	integ, err := LookupESPIntegrity("sha256")
	if err != nil {
		t.Fatalf("LookupESPIntegrity(sha256): %v", err)
	}
	var policies []ESPProposal
	for _, name := range SupportedESPEncryptionNames() {
		enc, err := LookupESPEncryption(name)
		if err != nil {
			t.Fatalf("LookupESPEncryption(%s), a supported name: %v", name, err)
		}
		p := ESPProposal{Number: 1, Encryption: enc, Integrity: integ}
		if enc.ID.IsAEAD() {
			p.Integrity = IntegrityTransform{ID: AUTH_NONE}
		}
		policies = append(policies, p)
	}
	return policies
}

// rfc8221RefuseESPCipher fails the test when an ESP configuration name resolves to id, or
// when NegotiateESP accepts an offer carrying it against any configurable ESP policy.
func rfc8221RefuseESPCipher(t *testing.T, id EncryptionID, policies []ESPProposal) {
	t.Helper()
	for _, name := range SupportedESPEncryptionNames() {
		enc, err := LookupESPEncryption(name)
		if err != nil {
			t.Fatalf("LookupESPEncryption(%s), a supported name: %v", name, err)
		}
		if enc.ID == id {
			t.Errorf("the ESP configuration name %q resolves to the forbidden Transform ID %d", name, id)
		}
	}
	for _, local := range policies {
		offer := local
		offer.Encryption = EncryptionTransform{ID: id, KeyLength: local.Encryption.KeyLength}
		if got, err := NegotiateESP([]ESPProposal{offer}, []ESPProposal{local}); err == nil {
			t.Errorf("NegotiateESP accepted ENCR %d against the %s policy: %+v", id, local.Encryption.ID, got)
		}
	}
}

// rfc8221NegotiatesEveryPolicy fails the test unless each configurable policy, offered
// back unchanged, is negotiated.
func rfc8221NegotiatesEveryPolicy(t *testing.T, policies []ESPProposal) {
	t.Helper()
	for _, local := range policies {
		if _, err := NegotiateESP([]ESPProposal{local}, []ESPProposal{local}); err != nil {
			t.Errorf("NegotiateESP refused the configurable %s policy offered unchanged: %v", local.Encryption.ID, err)
		}
	}
}

// TestRFC8221ESPRefusesForbiddenCiphers checks the encryption algorithms RFC 8221 Section 5
// grades MUST NOT, and the UNSPECIFIED and non-IPsec code points it forbids.
//
// RFC 8221 Section 5: "These algorithms were not mentioned in [RFC7321], and this document
// clarifies that such algorithms MUST NOT be implemented for IPsec communications." "As a
// result, these algorithms MUST NOT be implemented." "Various ciphers that are older, not
// well tested, and never widely implemented have been changed to MUST NOT." "ENCR_BLOWFISH
// has been downgraded to MUST NOT as it has been deprecated for years by TWOFISH, which is
// not standardized for ESP and therefore not listed in this document."
func TestRFC8221ESPRefusesForbiddenCiphers(t *testing.T) {
	policies := rfc8221ESPPolicies(t)
	// RFC requirement: RFC8221-5-2 negative -- the UNSPECIFIED ciphers ENCR_DES_IV64 (1),
	// ENCR_3IDEA (8) and ENCR_DES_IV32 (9) are held by no ESP configuration name, and
	// NegotiateESP refuses an offer of each against every configurable ESP policy.
	for _, id := range []EncryptionID{1, 8, 9} {
		rfc8221RefuseESPCipher(t, id, policies)
	}
	// RFC requirement: RFC8221-5-3 negative -- every ENCR Transform ID from 1 to 1023
	// outside ENCR_3DES, ENCR_NULL, ENCR_AES_CBC, the three AES CCM IDs, ENCR_AES_GCM_16 and
	// ENCR_CHACHA20_POLY1305 is held by no ESP configuration name, and NegotiateESP refuses
	// an offer of it against every configurable ESP policy.
	for id := EncryptionID(1); id < 1024; id++ {
		if rfc8221ImplementedESPCiphers[id] {
			continue
		}
		rfc8221RefuseESPCipher(t, id, policies)
	}
	// RFC requirement: RFC8221-5-4 negative -- the MUST NOT ciphers of the Section 5 table,
	// ENCR_DES_IV64 (1), ENCR_DES (2), ENCR_BLOWFISH (7), ENCR_3IDEA (8) and ENCR_DES_IV32
	// (9), are held by no ESP configuration name, and NegotiateESP refuses an offer of each
	// against every configurable ESP policy.
	for _, id := range []EncryptionID{1, 2, 7, 8, 9} {
		rfc8221RefuseESPCipher(t, id, policies)
	}
	// RFC requirement: RFC8221-5-5 negative -- ENCR_BLOWFISH (7) is held by no ESP
	// configuration name, and NegotiateESP refuses an offer of it against every
	// configurable ESP policy.
	rfc8221RefuseESPCipher(t, 7, policies)

	// RFC requirement: RFC8221-5-2 positive -- every configurable ESP policy offered back
	// unchanged is negotiated, so the refusals above belong to the cipher alone.
	// RFC requirement: RFC8221-5-3 positive -- every configurable ESP policy offered back
	// unchanged is negotiated, so the refusals above belong to the cipher alone.
	// RFC requirement: RFC8221-5-4 positive -- every configurable ESP policy offered back
	// unchanged is negotiated, so the refusals above belong to the cipher alone.
	// RFC requirement: RFC8221-5-5 positive -- every configurable ESP policy offered back
	// unchanged is negotiated, so the refusals above belong to the cipher alone.
	rfc8221NegotiatesEveryPolicy(t, policies)
}

// TestRFC8221ESPRefusesMD5AndDESMACIntegrity checks the integrity algorithms RFC 8221
// Section 6 grades MUST NOT.
//
// RFC 8221 Section 6, for AUTH_HMAC_MD5_96 and AUTH_KPDK_MD5: "As MD5 is known to be
// vulnerable to collisions, these algorithms MUST NOT be used."
// RFC 8221 Section 6, for AUTH_DES_MAC: "As DES is known to be vulnerable, it MUST NOT
// be used."
func TestRFC8221ESPRefusesMD5AndDESMACIntegrity(t *testing.T) {
	policies := rfc8221ESPPolicies(t)
	refuse := func(id IntegrityID) {
		t.Helper()
		for _, name := range SupportedESPIntegrityNames() {
			integ, err := LookupESPIntegrity(name)
			if err != nil {
				t.Fatalf("LookupESPIntegrity(%s), a supported name: %v", name, err)
			}
			if integ.ID == id {
				t.Errorf("the ESP integrity name %q resolves to the forbidden Transform ID %d", name, id)
			}
		}
		for _, local := range policies {
			if local.Encryption.ID.IsAEAD() {
				continue
			}
			offer := local
			offer.Integrity = IntegrityTransform{ID: id}
			if got, err := NegotiateESP([]ESPProposal{offer}, []ESPProposal{local}); err == nil {
				t.Errorf("NegotiateESP accepted INTEG %d beside %s: %+v", id, local.Encryption.ID, got)
			}
		}
	}
	// RFC requirement: RFC8221-6-3 negative -- AUTH_HMAC_MD5_96 (1) and AUTH_KPDK_MD5 (4)
	// are held by no ESP integrity name, and NegotiateESP refuses an offer of each beside
	// every configurable non-AEAD ESP cipher.
	refuse(rfc8247AuthHMACMD596)
	refuse(rfc8247AuthKPDKMD5)
	// RFC requirement: RFC8221-6-5 negative -- AUTH_DES_MAC (3) is held by no ESP integrity
	// name, and NegotiateESP refuses an offer of it beside every configurable non-AEAD ESP
	// cipher.
	refuse(rfc8247AuthDESMAC)
	// RFC requirement: RFC8221-6-3 positive -- every configurable ESP policy, each non-AEAD
	// one with HMAC-SHA2-256-128, offered back unchanged is negotiated, so the refusals
	// above belong to the integrity algorithm alone.
	// RFC requirement: RFC8221-6-5 positive -- every configurable ESP policy, each non-AEAD
	// one with HMAC-SHA2-256-128, offered back unchanged is negotiated, so the refusals
	// above belong to the integrity algorithm alone.
	rfc8221NegotiatesEveryPolicy(t, policies)
}
