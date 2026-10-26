// Design: docs/architecture/ike/ipsec-6-ikev2-crypto.md -- IKEv2 transform type registry
// Related: rfc7296_null_negotiation_test.go -- ikePolicy and the RFC 7296 Section 5 refusal
//
// VALIDATES: the RFC 8247 Section 2 grades for the IKE SA transforms. Each MUST algorithm
// resolves from its configuration name, is computed as specified and is negotiated by the
// responder (NegotiateIKE). Each MUST NOT algorithm is held by no configuration name, is
// refused by the negotiation with local policy naming the same value, and is refused by
// the primitive that would compute it.
// PREVENTS: an IKE SA keyed with DES, NULL, MD5, DES-MAC or a 768-bit or group 22 MODP
// group, and a mandatory algorithm silently dropped from the registries.
package crypto

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"testing"
)

// Transform IDs RFC 8247 grades that this implementation names no constant for, because
// it implements none of them (IANA IKEv2 Transform Type 1, 2, 3 and 4 registries).
//
// They are untyped on purpose. A typed constant of EncryptionID, PRFID, IntegrityID or
// DHGroupID declared here would join that enum for the exhaustive linter, which would then
// demand a case for it in every switch of the shipping code.
const (
	rfc8247EncrDES        = 2
	rfc8247Encr3DES       = 3
	rfc8247PRFHMACMD5     = 1
	rfc8247PRFAES128XCBC  = 4
	rfc8247AuthHMACMD596  = 1
	rfc8247AuthDESMAC     = 3
	rfc8247AuthKPDKMD5    = 4
	rfc8247AuthAESXCBC96  = 5
	rfc8247DHGroupMODP768 = 1
	rfc8247DHGroupMODP1K  = 2
	rfc8247DHGroup22      = 22
)

// rfc8247Proposal answers ikePolicy's AES-CBC, HMAC-SHA2-256, group 14 proposal with one
// change applied. The same value serves as the peer's offer and as local policy, so a
// refusal can only come from the changed transform.
func rfc8247Proposal(change func(*IKEProposal)) []IKEProposal {
	p := ikePolicy()[0]
	p.Number = 1
	change(&p)
	return []IKEProposal{p}
}

// rfc8247Negotiate runs the responder's selection with the proposal as both the offer and
// local policy.
func rfc8247Negotiate(change func(*IKEProposal)) (IKEProposal, error) {
	return NegotiateIKE(rfc8247Proposal(change), rfc8247Proposal(change))
}

func rfc8247Unchanged(*IKEProposal) {}

// TestRFC8247NonAEADCipherNeedsIntegrity checks the pairing rule of RFC 8247 Section 2.1.
//
// RFC 8247 Section 2.1: "Algorithms that are not AEAD MUST be used in conjunction with
// one of the integrity algorithms in Section 2.3."
func TestRFC8247NonAEADCipherNeedsIntegrity(t *testing.T) {
	// RFC requirement: RFC8247-2.1-1 negative -- NegotiateIKE refuses an IKE proposal pairing
	// AES-CBC with integrity NONE with ErrProposalIncomplete, with local policy holding the
	// same proposal.
	got, err := rfc8247Negotiate(func(p *IKEProposal) { p.Integrity = IntegrityTransform{ID: AUTH_NONE} })
	if !errors.Is(err, ErrProposalIncomplete) {
		t.Errorf("AES-CBC with integrity NONE: NegotiateIKE = %+v, %v; want ErrProposalIncomplete", got, err)
	}
	// RFC requirement: RFC8247-2.1-1 positive -- the same proposal with HMAC-SHA2-256-128 is
	// negotiated, and an AES-GCM proposal with integrity NONE is negotiated, so the refusal
	// belongs to a non-AEAD cipher without integrity.
	if _, err := rfc8247Negotiate(rfc8247Unchanged); err != nil {
		t.Errorf("AES-CBC with HMAC-SHA2-256-128: NegotiateIKE = %v, want it negotiated", err)
	}
	gcm, err := LookupEncryption("aes128gcm")
	if err != nil {
		t.Fatalf("LookupEncryption(aes128gcm): %v", err)
	}
	if _, err := rfc8247Negotiate(func(p *IKEProposal) {
		p.Encryption = gcm
		p.Integrity = IntegrityTransform{ID: AUTH_NONE}
	}); err != nil {
		t.Errorf("AES-GCM with integrity NONE: NegotiateIKE = %v, want it negotiated", err)
	}
}

// TestRFC8247AESCBCKeysTheIKESA checks that ENCR_AES_CBC is implemented for the IKE SA with
// 128-bit and 256-bit keys.
//
// RFC 8247 Section 2.1: "ENCR_AES_CBC is raised from SHOULD+ for 128-bit keys and MAY for
// 256-bit keys in [RFC4307] to MUST."
func TestRFC8247AESCBCKeysTheIKESA(t *testing.T) {
	// RFC requirement: RFC8247-2.1-2 positive -- the configuration names aes128 and aes256
	// resolve to ENCR_AES_CBC with a 128-bit and a 256-bit key, NegotiateIKE selects each,
	// and the AES-CBC cipher round-trips a block-aligned plaintext under each key size.
	for name, bits := range map[string]uint16{"aes128": 128, "aes256": 256} {
		enc, err := LookupEncryption(name)
		if err != nil {
			t.Fatalf("LookupEncryption(%s): %v", name, err)
		}
		if enc.ID != ENCR_AES_CBC || enc.KeyLength != bits {
			t.Fatalf("LookupEncryption(%s) = %+v, want ENCR_AES_CBC with a %d-bit key", name, enc, bits)
		}
		chosen, err := rfc8247Negotiate(func(p *IKEProposal) { p.Encryption = enc })
		if err != nil {
			t.Fatalf("NegotiateIKE over %s: %v", name, err)
		}
		if chosen.Encryption.ID != ENCR_AES_CBC || chosen.Encryption.KeyLength != bits {
			t.Errorf("NegotiateIKE over %s chose %+v", name, chosen.Encryption)
		}
		key := bytes.Repeat([]byte{0x2b}, int(bits)/8)
		plaintext := bytes.Repeat([]byte{0x61}, 32)
		sealed, err := encryptAESCBC(key, plaintext)
		if err != nil {
			t.Fatalf("encryptAESCBC with a %d-bit key: %v", bits, err)
		}
		opened, err := decryptAESCBC(key, sealed)
		if err != nil {
			t.Fatalf("decryptAESCBC with a %d-bit key: %v", bits, err)
		}
		if !bytes.Equal(opened, plaintext) {
			t.Errorf("AES-CBC with a %d-bit key round-tripped %x, want %x", bits, opened, plaintext)
		}
	}
	// RFC requirement: RFC8247-2.1-2 negative -- the same negotiation refuses ENCR_3DES, a
	// cipher the IKE registry does not hold, with ErrTransformUnspecified, so the
	// acceptance above is not a blanket accept.
	_, err := rfc8247Negotiate(func(p *IKEProposal) {
		p.Encryption = EncryptionTransform{ID: rfc8247Encr3DES}
	})
	if !errors.Is(err, ErrTransformUnspecified) {
		t.Errorf("ENCR_3DES for the IKE SA: NegotiateIKE = %v, want ErrTransformUnspecified", err)
	}
}

// rfc8247NoEncryptionName fails the test when an IKE configuration name resolves to id.
func rfc8247NoEncryptionName(t *testing.T, id EncryptionID) {
	t.Helper()
	for _, name := range SupportedEncryptionNames() {
		enc, err := LookupEncryption(name)
		if err != nil {
			t.Fatalf("LookupEncryption(%s), a supported name: %v", name, err)
		}
		if enc.ID == id {
			t.Errorf("the IKE configuration name %q resolves to Transform ID %d", name, id)
		}
	}
}

// TestRFC8247IKERefusesENCRDESAndENCRNULL checks the two MUST NOT ciphers of RFC 8247
// Section 2.1.
//
// RFC 8247 Section 2.1: "ENCR_DES can be brute-forced using off-the-shelf hardware. It
// provides no meaningful security whatsoever and, therefore, MUST NOT be implemented."
// and "This was fixed and this document now lists ENCR_NULL as MUST NOT."
func TestRFC8247IKERefusesENCRDESAndENCRNULL(t *testing.T) {
	// RFC requirement: RFC8247-2.1-3 negative -- no IKE configuration name resolves to
	// ENCR_DES (2), and NegotiateIKE refuses an offer of it with ErrTransformUnspecified,
	// with local policy naming the same cipher.
	rfc8247NoEncryptionName(t, rfc8247EncrDES)
	_, err := rfc8247Negotiate(func(p *IKEProposal) { p.Encryption = EncryptionTransform{ID: rfc8247EncrDES} })
	if !errors.Is(err, ErrTransformUnspecified) {
		t.Errorf("ENCR_DES: NegotiateIKE = %v, want ErrTransformUnspecified", err)
	}
	// RFC requirement: RFC8247-2.1-4 negative -- no IKE configuration name resolves to
	// ENCR_NULL (11), and NegotiateIKE refuses an offer of it with ErrTransformUnspecified,
	// with local policy naming the same cipher.
	rfc8247NoEncryptionName(t, ENCR_NULL)
	_, err = rfc8247Negotiate(func(p *IKEProposal) { p.Encryption = EncryptionTransform{ID: ENCR_NULL} })
	if !errors.Is(err, ErrTransformUnspecified) {
		t.Errorf("ENCR_NULL: NegotiateIKE = %v, want ErrTransformUnspecified", err)
	}
	// RFC requirement: RFC8247-2.1-3 positive -- the same proposal carrying AES-CBC is
	// negotiated, so the refusal belongs to the cipher alone.
	// RFC requirement: RFC8247-2.1-4 positive -- the same proposal carrying AES-CBC is
	// negotiated, so the refusal belongs to the cipher alone.
	if _, err := rfc8247Negotiate(rfc8247Unchanged); err != nil {
		t.Errorf("AES-CBC: NegotiateIKE = %v, want it negotiated", err)
	}
}

// TestRFC8247PRFHMACSHA2256 checks that PRF_HMAC_SHA2_256 is implemented.
//
// RFC 8247 Section 2.2: "PRF_HMAC_SHA2_256 MUST be implemented in order to replace SHA1
// and PRF_HMAC_SHA1."
func TestRFC8247PRFHMACSHA2256(t *testing.T) {
	// RFC requirement: RFC8247-2.2-1 positive -- the configuration name sha256 resolves to
	// PRF_HMAC_SHA2_256 (5) with a 32-octet key and output, PRF computes HMAC-SHA-256 of
	// the data under the key, and NegotiateIKE selects the PRF.
	prf, err := LookupPRF("sha256")
	if err != nil {
		t.Fatalf("LookupPRF(sha256): %v", err)
	}
	if prf.ID != PRF_HMAC_SHA2_256 || prf.KeyLength != 32 || prf.OutputLength != 32 {
		t.Fatalf("LookupPRF(sha256) = %+v, want PRF_HMAC_SHA2_256 with 32-octet key and output", prf)
	}
	key := bytes.Repeat([]byte{0x0b}, 32)
	data := []byte("Hi There")
	got, err := PRF(PRF_HMAC_SHA2_256, key, data)
	if err != nil {
		t.Fatalf("PRF(PRF_HMAC_SHA2_256): %v", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	if want := mac.Sum(nil); !bytes.Equal(got, want) {
		t.Errorf("PRF(PRF_HMAC_SHA2_256) = %x, want HMAC-SHA-256 %x", got, want)
	}
	chosen, err := rfc8247Negotiate(func(p *IKEProposal) { p.PRF = prf })
	if err != nil {
		t.Fatalf("NegotiateIKE over PRF_HMAC_SHA2_256: %v", err)
	}
	if chosen.PRF.ID != PRF_HMAC_SHA2_256 {
		t.Errorf("NegotiateIKE chose PRF %d, want PRF_HMAC_SHA2_256", chosen.PRF.ID)
	}
	// RFC requirement: RFC8247-2.2-1 negative -- PRF_AES128_XCBC (4), which this build does
	// not implement, is refused by PRF and by NegotiateIKE with ErrTransformUnspecified, so
	// the acceptance above is not a blanket accept.
	if out, err := PRF(rfc8247PRFAES128XCBC, key, data); err == nil {
		t.Errorf("PRF(PRF_AES128_XCBC) = %x, want an error", out)
	}
	_, err = rfc8247Negotiate(func(p *IKEProposal) { p.PRF = PRFTransform{ID: rfc8247PRFAES128XCBC} })
	if !errors.Is(err, ErrTransformUnspecified) {
		t.Errorf("PRF_AES128_XCBC: NegotiateIKE = %v, want ErrTransformUnspecified", err)
	}
}

// TestRFC8247IKERefusesPRFHMACMD5 checks the MUST NOT PRF of RFC 8247 Section 2.2.
//
// RFC 8247 Section 2.2: "PRF_HMAC_MD5 has been downgraded from MAY in RFC 4307 to MUST
// NOT."
func TestRFC8247IKERefusesPRFHMACMD5(t *testing.T) {
	// RFC requirement: RFC8247-2.2-3 negative -- no configuration name resolves to
	// PRF_HMAC_MD5 (1), PRF refuses to compute it, and NegotiateIKE refuses an offer of it
	// with ErrTransformUnspecified, with local policy naming the same PRF.
	for _, name := range SupportedPRFNames() {
		prf, err := LookupPRF(name)
		if err != nil {
			t.Fatalf("LookupPRF(%s), a supported name: %v", name, err)
		}
		if prf.ID == rfc8247PRFHMACMD5 {
			t.Errorf("the PRF name %q resolves to PRF_HMAC_MD5", name)
		}
	}
	if out, err := PRF(rfc8247PRFHMACMD5, []byte("key"), []byte("data")); err == nil {
		t.Errorf("PRF(PRF_HMAC_MD5) = %x, want an error", out)
	}
	_, err := rfc8247Negotiate(func(p *IKEProposal) { p.PRF = PRFTransform{ID: rfc8247PRFHMACMD5} })
	if !errors.Is(err, ErrTransformUnspecified) {
		t.Errorf("PRF_HMAC_MD5: NegotiateIKE = %v, want ErrTransformUnspecified", err)
	}
	// RFC requirement: RFC8247-2.2-3 positive -- the same proposal carrying
	// PRF_HMAC_SHA2_256 is negotiated, so the refusal belongs to the PRF alone.
	if _, err := rfc8247Negotiate(rfc8247Unchanged); err != nil {
		t.Errorf("PRF_HMAC_SHA2_256: NegotiateIKE = %v, want it negotiated", err)
	}
}

// TestRFC8247IntegrityHMACSHA2256128 checks that AUTH_HMAC_SHA2_256_128 is implemented
// for the IKE SA.
//
// RFC 8247 Section 2.3: "AUTH_HMAC_SHA2_256_128 MUST be implemented in order to replace
// AUTH_HMAC_SHA1_96."
func TestRFC8247IntegrityHMACSHA2256128(t *testing.T) {
	// RFC requirement: RFC8247-2.3-1 positive -- the configuration name sha256 resolves to
	// AUTH_HMAC_SHA2_256_128 (12) with a 32-octet key and a 16-octet ICV, ComputeIntegrity
	// answers the first 16 octets of HMAC-SHA-256, and NegotiateIKE selects it.
	integ, err := LookupIntegrity("sha256")
	if err != nil {
		t.Fatalf("LookupIntegrity(sha256): %v", err)
	}
	if integ.ID != AUTH_HMAC_SHA2_256_128 || integ.KeyLength != 32 || integ.TruncatedLength != 16 {
		t.Fatalf("LookupIntegrity(sha256) = %+v, want AUTH_HMAC_SHA2_256_128, 32-octet key, 16-octet ICV", integ)
	}
	key := bytes.Repeat([]byte{0x0c}, 32)
	data := []byte("the IKE message the checksum covers")
	got, err := ComputeIntegrity(AUTH_HMAC_SHA2_256_128, key, data)
	if err != nil {
		t.Fatalf("ComputeIntegrity(AUTH_HMAC_SHA2_256_128): %v", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	if want := mac.Sum(nil)[:16]; !bytes.Equal(got, want) {
		t.Errorf("ComputeIntegrity(AUTH_HMAC_SHA2_256_128) = %x, want %x", got, want)
	}
	chosen, err := rfc8247Negotiate(func(p *IKEProposal) { p.Integrity = integ })
	if err != nil {
		t.Fatalf("NegotiateIKE over AUTH_HMAC_SHA2_256_128: %v", err)
	}
	if chosen.Integrity.ID != AUTH_HMAC_SHA2_256_128 {
		t.Errorf("NegotiateIKE chose integrity %d, want AUTH_HMAC_SHA2_256_128", chosen.Integrity.ID)
	}
	// RFC requirement: RFC8247-2.3-1 negative -- AUTH_AES_XCBC_96 (5), which this build
	// does not implement, is refused by ComputeIntegrity and by NegotiateIKE with
	// ErrTransformUnspecified, so the acceptance above is not a blanket accept.
	if out, err := ComputeIntegrity(rfc8247AuthAESXCBC96, key, data); err == nil {
		t.Errorf("ComputeIntegrity(AUTH_AES_XCBC_96) = %x, want an error", out)
	}
	_, err = rfc8247Negotiate(func(p *IKEProposal) { p.Integrity = IntegrityTransform{ID: rfc8247AuthAESXCBC96} })
	if !errors.Is(err, ErrTransformUnspecified) {
		t.Errorf("AUTH_AES_XCBC_96: NegotiateIKE = %v, want ErrTransformUnspecified", err)
	}
}

// rfc8247RefuseIntegrity fails the test unless id is held by no IKE integrity name, is
// refused by ComputeIntegrity, and is refused by NegotiateIKE.
func rfc8247RefuseIntegrity(t *testing.T, id IntegrityID) {
	t.Helper()
	for _, name := range SupportedIntegrityNames() {
		integ, err := LookupIntegrity(name)
		if err != nil {
			t.Fatalf("LookupIntegrity(%s), a supported name: %v", name, err)
		}
		if integ.ID == id {
			t.Errorf("the IKE integrity name %q resolves to Transform ID %d", name, id)
		}
	}
	if out, err := ComputeIntegrity(id, []byte("key"), []byte("data")); err == nil {
		t.Errorf("ComputeIntegrity(%d) = %x, want an error", id, out)
	}
	_, err := rfc8247Negotiate(func(p *IKEProposal) { p.Integrity = IntegrityTransform{ID: id} })
	if !errors.Is(err, ErrTransformUnspecified) {
		t.Errorf("integrity %d: NegotiateIKE = %v, want ErrTransformUnspecified", id, err)
	}
}

// TestRFC8247IKERefusesDESMACAndMD5Integrity checks the MUST NOT integrity algorithms of
// RFC 8247 Section 2.3.
//
// RFC 8247 Section 2.3: "AUTH_DES_MAC and AUTH_KPDK_MD5 were not mentioned in RFC 4307, so
// their default statuses were MAY. These have been downgraded to MUST NOT.
// AUTH_HMAC_MD5_96 is also demoted to MUST NOT."
func TestRFC8247IKERefusesDESMACAndMD5Integrity(t *testing.T) {
	// RFC requirement: RFC8247-2.3-3 negative -- AUTH_DES_MAC (3) and AUTH_KPDK_MD5 (4) are
	// held by no IKE integrity name, are refused by ComputeIntegrity, and NegotiateIKE
	// refuses an offer of each with ErrTransformUnspecified, with local policy naming it.
	rfc8247RefuseIntegrity(t, rfc8247AuthDESMAC)
	rfc8247RefuseIntegrity(t, rfc8247AuthKPDKMD5)
	// RFC requirement: RFC8247-2.3-4 negative -- AUTH_HMAC_MD5_96 (1) is held by no IKE
	// integrity name, is refused by ComputeIntegrity, and NegotiateIKE refuses an offer of
	// it with ErrTransformUnspecified, with local policy naming it.
	rfc8247RefuseIntegrity(t, rfc8247AuthHMACMD596)
	// RFC requirement: RFC8247-2.3-3 positive -- the same proposal carrying
	// AUTH_HMAC_SHA2_256_128 is negotiated, so the refusal belongs to the algorithm alone.
	// RFC requirement: RFC8247-2.3-4 positive -- the same proposal carrying
	// AUTH_HMAC_SHA2_256_128 is negotiated, so the refusal belongs to the algorithm alone.
	if _, err := rfc8247Negotiate(rfc8247Unchanged); err != nil {
		t.Errorf("AUTH_HMAC_SHA2_256_128: NegotiateIKE = %v, want it negotiated", err)
	}
}

// TestRFC8247DHGroup14 checks that the 2048-bit MODP group is implemented.
//
// RFC 8247 Section 2.4: "Group 14 or the 2048-bit MODP Group is raised from SHOULD+ in RFC
// 4307 to MUST as a replacement for the 1024-bit MODP Group."
func TestRFC8247DHGroup14(t *testing.T) {
	// RFC requirement: RFC8247-2.4-1 positive -- group 14 resolves to DH_MODP_2048, two
	// exchanges on it publish 256-octet public values and compute the same shared secret,
	// and NegotiateIKE selects the group.
	group, err := LookupDHGroup(14)
	if err != nil {
		t.Fatalf("LookupDHGroup(14): %v", err)
	}
	if group.ID != DH_MODP_2048 {
		t.Fatalf("LookupDHGroup(14) = %+v, want DH_MODP_2048", group)
	}
	initiator, err := NewDHExchange(DH_MODP_2048)
	if err != nil {
		t.Fatalf("NewDHExchange(14): %v", err)
	}
	defer initiator.Clear()
	responder, err := NewDHExchange(DH_MODP_2048)
	if err != nil {
		t.Fatalf("NewDHExchange(14): %v", err)
	}
	defer responder.Clear()
	if len(initiator.PublicKey) != 256 || len(responder.PublicKey) != 256 {
		t.Fatalf("group 14 public values are %d and %d octets, want 256",
			len(initiator.PublicKey), len(responder.PublicKey))
	}
	secretI, err := initiator.SharedSecret(responder.PublicKey)
	if err != nil {
		t.Fatalf("initiator SharedSecret: %v", err)
	}
	secretR, err := responder.SharedSecret(initiator.PublicKey)
	if err != nil {
		t.Fatalf("responder SharedSecret: %v", err)
	}
	if !bytes.Equal(secretI, secretR) {
		t.Error("the two ends of a group 14 exchange computed different shared secrets")
	}
	chosen, err := rfc8247Negotiate(func(p *IKEProposal) { p.DHGroup = group })
	if err != nil {
		t.Fatalf("NegotiateIKE over group 14: %v", err)
	}
	if chosen.DHGroup.ID != DH_MODP_2048 {
		t.Errorf("NegotiateIKE chose group %d, want 14", chosen.DHGroup.ID)
	}
	// RFC requirement: RFC8247-2.4-1 negative -- group 2, which this build does not
	// implement, is refused by LookupDHGroup, by NewDHExchange and by NegotiateIKE with
	// ErrTransformUnspecified, so the acceptance above is not a blanket accept.
	rfc8247RefuseDHGroup(t, rfc8247DHGroupMODP1K)
}

// rfc8247RefuseDHGroup fails the test unless the group is refused by LookupDHGroup, by
// NewDHExchange and by NegotiateIKE.
func rfc8247RefuseDHGroup(t *testing.T, id DHGroupID) {
	t.Helper()
	if g, err := LookupDHGroup(uint8(id)); err == nil {
		t.Errorf("LookupDHGroup(%d) = %+v, want an error", id, g)
	}
	if ex, err := NewDHExchange(id); err == nil {
		ex.Clear()
		t.Errorf("NewDHExchange(%d) built an exchange, want an error", id)
	}
	_, err := rfc8247Negotiate(func(p *IKEProposal) { p.DHGroup = DHGroupTransform{ID: id} })
	if !errors.Is(err, ErrTransformUnspecified) {
		t.Errorf("group %d: NegotiateIKE = %v, want ErrTransformUnspecified", id, err)
	}
}

// TestRFC8247IKERefusesDHGroups1And22 checks the two MUST NOT groups of RFC 8247 Section
// 2.4.
//
// RFC 8247 Section 2.4: "It provides no security whatsoever. It has, therefore, been
// downgraded to MUST NOT." (group 1) and "This has resulted in group 22 to be demoted to
// MUST NOT."
func TestRFC8247IKERefusesDHGroups1And22(t *testing.T) {
	// RFC requirement: RFC8247-2.4-2 negative -- group 1, the 768-bit MODP group, is
	// refused by LookupDHGroup, by NewDHExchange and by NegotiateIKE with
	// ErrTransformUnspecified, with local policy naming the same group.
	rfc8247RefuseDHGroup(t, rfc8247DHGroupMODP768)
	// RFC requirement: RFC8247-2.4-3 negative -- group 22 is refused by LookupDHGroup, by
	// NewDHExchange and by NegotiateIKE with ErrTransformUnspecified, with local policy
	// naming the same group.
	rfc8247RefuseDHGroup(t, rfc8247DHGroup22)
	// RFC requirement: RFC8247-2.4-2 positive -- the same proposal carrying group 14 is
	// negotiated, so the refusal belongs to the group alone.
	// RFC requirement: RFC8247-2.4-3 positive -- the same proposal carrying group 14 is
	// negotiated, so the refusal belongs to the group alone.
	if _, err := rfc8247Negotiate(rfc8247Unchanged); err != nil {
		t.Errorf("group 14: NegotiateIKE = %v, want it negotiated", err)
	}
}
