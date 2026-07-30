// VALIDATES: the RFC 7296 MUST-level obligations the IKEv2 crypto layer discharges.
// The list covers PRF key sizing (§1.7, §2.13) and the transform registry's fixed and
// preferred key lengths (§2.13). It also covers Child SA KEYMAT ordering and split
// (§2.17). Last, it covers the refusal of a null cipher or a null integrity algorithm
// for an IKE SA (§5). Every test carries an `RFC requirement:` tag binding it to its
// checklist id.
// PREVENTS: a key-derivation or registry change that silently reorders KEYMAT, resizes a
// key, or admits an algorithm the RFC forbids for the IKE SA.
package crypto

import (
	"bytes"
	"testing"
)

// RFC requirement: RFC7296-1.7-2 positive -- every PRF takes a variable-sized key. PRF builds an
// HMAC over the caller's key with no length constraint (prf.go:27-35). The same PRF ID
// accepts a 1-octet key, a 32-octet key and a 200-octet key, and produces its full output
// for each.
// RFC requirement: RFC7296-1.7-2 negative -- variable-sized does not mean the key is ignored. Two
// different keys of the same length produce different outputs. The key material is really
// consumed rather than discarded to make any length "work".
func TestPRFTakesVariableSizedKeys(t *testing.T) {
	for _, id := range []PRFID{PRF_HMAC_SHA2_256, PRF_HMAC_SHA2_384, PRF_HMAC_SHA2_512} {
		want := prfHashFunc(id)().Size()
		var prev []byte
		for _, keyLen := range []int{1, 16, 32, 64, 200} {
			key := bytes.Repeat([]byte{0xab}, keyLen)
			out, err := PRF(id, key, []byte("seed"))
			if err != nil {
				t.Fatalf("PRF(id=%d, %d-octet key) = %v, want a result", id, keyLen, err)
			}
			if len(out) != want {
				t.Errorf("PRF(id=%d, %d-octet key) output = %d octets, want %d",
					id, keyLen, len(out), want)
			}
			if prev != nil && bytes.Equal(prev, out) {
				t.Errorf("PRF(id=%d) produced the same output for a %d-octet key as for the "+
					"previous length; the key must be consumed", id, keyLen)
			}
			prev = out
		}
	}

	// Negative: the key is consumed, not ignored.
	a, err := PRF(PRF_HMAC_SHA2_256, bytes.Repeat([]byte{1}, 32), []byte("seed"))
	if err != nil {
		t.Fatalf("PRF: %v", err)
	}
	b, err := PRF(PRF_HMAC_SHA2_256, bytes.Repeat([]byte{2}, 32), []byte("seed"))
	if err != nil {
		t.Fatalf("PRF: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Error("two different 32-octet keys produced the same PRF output; the key is not " +
			"being consumed")
	}
}

// RFC requirement: RFC7296-2.13-1 positive -- a variable-length-key algorithm carries its fixed key
// size in the negotiated transform. Every entry of encryptionRegistry states a KeyLength in
// bits (transform.go:119-124), so aes128 and aes256 are separate transforms rather than one
// entry sized later.
// RFC requirement: RFC7296-2.13-1 negative -- the size belongs to the transform, not to a default,
// so an unknown algorithm name returns ErrUnsupportedAlgorithm and a zero transform.
//
// RFC requirement: RFC7296-2.13-4 positive -- each PRF states its own preferred key size. prfRegistry
// gives sha256/sha384/sha512 KeyLength 32/48/64 octets (transform.go:126-130). Each size
// equals that PRF's output length.
// RFC requirement: RFC7296-2.13-4 negative -- an unregistered PRF name yields ErrUnsupportedAlgorithm
// and a zero PRFTransform, so a PRF with no stated preferred key size is never negotiated.
func TestTransformRegistryStatesKeySizes(t *testing.T) {
	wantEnc := map[string]uint16{"aes128": 128, "aes256": 256, "aes128gcm": 128, "aes256gcm": 256}
	for name, bits := range wantEnc {
		got, err := LookupEncryption(name)
		if err != nil {
			t.Fatalf("LookupEncryption(%q) = %v", name, err)
		}
		if got.KeyLength != bits {
			t.Errorf("LookupEncryption(%q).KeyLength = %d bits, want %d", name, got.KeyLength, bits)
		}
	}
	if _, err := LookupEncryption("aes192"); err == nil {
		t.Error("LookupEncryption(\"aes192\") succeeded; an algorithm with no registered key " +
			"size must not be negotiable")
	}
	zero, err := LookupEncryption("aes192")
	if err != nil && zero.KeyLength != 0 {
		t.Errorf("a failed encryption lookup returned KeyLength %d; it must not guess a size",
			zero.KeyLength)
	}

	wantPRF := map[string]uint16{"sha256": 32, "sha384": 48, "sha512": 64}
	for name, octets := range wantPRF {
		got, err := LookupPRF(name)
		if err != nil {
			t.Fatalf("LookupPRF(%q) = %v", name, err)
		}
		if got.KeyLength != octets {
			t.Errorf("LookupPRF(%q).KeyLength = %d octets, want %d", name, got.KeyLength, octets)
		}
		if got.OutputLength != octets {
			t.Errorf("LookupPRF(%q).OutputLength = %d octets, want %d (the preferred key size "+
				"equals the PRF output for an HMAC PRF)", name, got.OutputLength, octets)
		}
	}
	if p, err := LookupPRF("sha224"); err == nil || p.KeyLength != 0 {
		t.Errorf("LookupPRF(\"sha224\") = (%+v, %v); an unregistered PRF must not yield a "+
			"preferred key size", p, err)
	}
}

// RFC requirement: RFC7296-2.13-2 positive -- the transform fixes how a key is derived from arbitrary
// values, and the caller does not. DeriveSKKeys sizes every SK_* slice from the negotiated
// transforms' own KeyLength fields (keys.go:45-90). A change of the integrity transform from
// sha256 to sha512 therefore resizes SK_ai and SK_ar alone.
// RFC requirement: RFC7296-2.13-2 negative -- the derivation is not a free-form truncation of one
// stream. The same seed with the same transforms reproduces the keys byte for byte.
//
// RFC requirement: RFC7296-2.13-3 positive -- the preferred key size gives the length of SK_d, SK_pi
// and SK_pr. DeriveSKKeys takes skDLen from the PRF's OutputLength (keys.go:51-57) and
// prfKeyLen from its KeyLength, so all three hold that PRF's own size.
// RFC requirement: RFC7296-2.13-3 negative -- the PRF's size governs only those three keys. SK_ai and
// SK_ar take the integrity transform's length, and SK_ei and SK_er take the encryption
// transform's length. A change of PRF resizes neither pair.
func TestSKKeyLengthsComeFromTransforms(t *testing.T) {
	ni := bytes.Repeat([]byte{1}, 32)
	nr := bytes.Repeat([]byte{2}, 32)
	spiI := bytes.Repeat([]byte{3}, 8)
	spiR := bytes.Repeat([]byte{4}, 8)
	seed := bytes.Repeat([]byte{5}, 32)

	enc, err := LookupEncryption("aes256")
	if err != nil {
		t.Fatalf("LookupEncryption: %v", err)
	}
	integ256, err := LookupIntegrity("sha256")
	if err != nil {
		t.Fatalf("LookupIntegrity: %v", err)
	}
	integ512, err := LookupIntegrity("sha512")
	if err != nil {
		t.Fatalf("LookupIntegrity: %v", err)
	}
	prf256, err := LookupPRF("sha256")
	if err != nil {
		t.Fatalf("LookupPRF: %v", err)
	}
	prf512, err := LookupPRF("sha512")
	if err != nil {
		t.Fatalf("LookupPRF: %v", err)
	}

	base, err := DeriveSKKeys(PRF_HMAC_SHA2_256, seed, ni, nr, spiI, spiR, enc, integ256)
	if err != nil {
		t.Fatalf("DeriveSKKeys: %v", err)
	}
	// SK_d, SK_pi and SK_pr take the PRF's own sizes.
	if len(base.SK_d) != int(prf256.OutputLength) {
		t.Errorf("SK_d = %d octets, want the PRF output length %d", len(base.SK_d), prf256.OutputLength)
	}
	if len(base.SK_pi) != int(prf256.KeyLength) || len(base.SK_pr) != int(prf256.KeyLength) {
		t.Errorf("SK_pi/SK_pr = %d/%d octets, want the PRF preferred key size %d",
			len(base.SK_pi), len(base.SK_pr), prf256.KeyLength)
	}
	// SK_ai/SK_ar take the integrity transform's stated length.
	if len(base.SK_ai) != int(integ256.KeyLength) || len(base.SK_ar) != int(integ256.KeyLength) {
		t.Errorf("SK_ai/SK_ar = %d/%d octets, want the integrity key length %d",
			len(base.SK_ai), len(base.SK_ar), integ256.KeyLength)
	}
	// SK_ei/SK_er take the encryption transform's stated length in octets.
	wantEnc := int(enc.KeyLength) / 8
	if len(base.SK_ei) != wantEnc || len(base.SK_er) != wantEnc {
		t.Errorf("SK_ei/SK_er = %d/%d octets, want the encryption key length %d",
			len(base.SK_ei), len(base.SK_er), wantEnc)
	}

	// Changing ONLY the integrity transform resizes only SK_ai/SK_ar.
	swapInteg, err := DeriveSKKeys(PRF_HMAC_SHA2_256, seed, ni, nr, spiI, spiR, enc, integ512)
	if err != nil {
		t.Fatalf("DeriveSKKeys: %v", err)
	}
	if len(swapInteg.SK_ai) != int(integ512.KeyLength) {
		t.Errorf("SK_ai after the integrity swap = %d octets, want %d",
			len(swapInteg.SK_ai), integ512.KeyLength)
	}
	if len(swapInteg.SK_ei) != wantEnc {
		t.Errorf("SK_ei changed with the integrity transform: %d octets, want %d",
			len(swapInteg.SK_ei), wantEnc)
	}
	if len(swapInteg.SK_d) != int(prf256.OutputLength) {
		t.Errorf("SK_d changed with the integrity transform: %d octets, want %d",
			len(swapInteg.SK_d), prf256.OutputLength)
	}

	// Changing ONLY the PRF resizes SK_d/SK_pi/SK_pr and nothing else.
	swapPRF, err := DeriveSKKeys(PRF_HMAC_SHA2_512, seed, ni, nr, spiI, spiR, enc, integ256)
	if err != nil {
		t.Fatalf("DeriveSKKeys: %v", err)
	}
	if len(swapPRF.SK_d) != int(prf512.OutputLength) {
		t.Errorf("SK_d after the PRF swap = %d octets, want %d", len(swapPRF.SK_d), prf512.OutputLength)
	}
	if len(swapPRF.SK_pi) != int(prf512.KeyLength) {
		t.Errorf("SK_pi after the PRF swap = %d octets, want %d", len(swapPRF.SK_pi), prf512.KeyLength)
	}
	if len(swapPRF.SK_ai) != int(integ256.KeyLength) {
		t.Errorf("SK_ai changed with the PRF: %d octets, want %d",
			len(swapPRF.SK_ai), integ256.KeyLength)
	}
	if len(swapPRF.SK_ei) != wantEnc {
		t.Errorf("SK_ei changed with the PRF: %d octets, want %d", len(swapPRF.SK_ei), wantEnc)
	}

	// The derivation is a rule, not an arbitrary truncation: it reproduces exactly.
	again, err := DeriveSKKeys(PRF_HMAC_SHA2_256, seed, ni, nr, spiI, spiR, enc, integ256)
	if err != nil {
		t.Fatalf("DeriveSKKeys: %v", err)
	}
	if !bytes.Equal(base.SK_d, again.SK_d) || !bytes.Equal(base.SK_ei, again.SK_ei) {
		t.Error("the same seed and transforms produced different keys; the derivation rule " +
			"must be fixed by the transform")
	}
}

// RFC requirement: RFC7296-2.17-1 positive -- Child SA keying material comes from prf+ in the order
// RFC 7296 fixes. DeriveChildSAKeys reads EncryptKeyI and IntegKeyI, the two
// initiator-to-responder keys, before EncryptKeyR and IntegKeyR (keys.go:109-134). Every
// initiator-to-responder key therefore comes before every responder-to-initiator key.
// RFC requirement: RFC7296-2.17-2 positive -- for ESP the encryption key comes from the first bits
// and the integrity key from the remaining bits. The two directions sit in the KEYMAT stream
// as encrypt|integ, then encrypt|integ.
//
// RFC requirement: RFC7296-2.17-1 negative -- the order is a real split of one stream, not four
// independent draws. The test recomputes prf+ over the same SK_d and nonces, then slices it
// in that order, and gets the four keys exactly. No direction can swap without a change to
// the bytes it gets.
// RFC requirement: RFC7296-2.17-2 negative -- the split is positional. The initiator's integrity key
// is NOT the initiator's encryption key. The responder's encryption key follows the
// initiator's integrity key, and the stream does not restart.
func TestChildSAKeymatOrder(t *testing.T) {
	skD := bytes.Repeat([]byte{9}, 32)
	ni := bytes.Repeat([]byte{1}, 32)
	nr := bytes.Repeat([]byte{2}, 32)

	enc, err := LookupEncryption("aes256")
	if err != nil {
		t.Fatalf("LookupEncryption: %v", err)
	}
	integ, err := LookupIntegrity("sha256")
	if err != nil {
		t.Fatalf("LookupIntegrity: %v", err)
	}

	keys, err := DeriveChildSAKeys(PRF_HMAC_SHA2_256, skD, ni, nr, enc, integ)
	if err != nil {
		t.Fatalf("DeriveChildSAKeys: %v", err)
	}

	encLen := int(enc.KeyLength) / 8
	integLen := int(integ.KeyLength)
	total := 2*encLen + 2*integLen

	// The KEYMAT stream the RFC defines: prf+(SK_d, Ni | Nr).
	seed := append(append([]byte(nil), ni...), nr...)
	keymat, err := PRFPlus(PRF_HMAC_SHA2_256, skD, seed, total)
	if err != nil {
		t.Fatalf("PRFPlus: %v", err)
	}

	off := 0
	wantEncI := keymat[off : off+encLen]
	off += encLen
	wantIntegI := keymat[off : off+integLen]
	off += integLen
	wantEncR := keymat[off : off+encLen]
	off += encLen
	wantIntegR := keymat[off : off+integLen]

	if !bytes.Equal(keys.EncryptKeyI, wantEncI) {
		t.Error("the initiator encryption key is not the FIRST slice of KEYMAT")
	}
	if !bytes.Equal(keys.IntegKeyI, wantIntegI) {
		t.Error("the initiator integrity key does not follow the initiator encryption key")
	}
	if !bytes.Equal(keys.EncryptKeyR, wantEncR) {
		t.Error("the responder encryption key is not taken after both initiator keys; all " +
			"initiator-to-responder keys must be taken first")
	}
	if !bytes.Equal(keys.IntegKeyR, wantIntegR) {
		t.Error("the responder integrity key is not the LAST slice of KEYMAT")
	}

	// Negative: the four keys are distinct positional slices, not repeats.
	if bytes.Equal(keys.EncryptKeyI, keys.IntegKeyI[:min(encLen, integLen)]) {
		t.Error("the initiator encryption and integrity keys share a prefix; the split must " +
			"be positional")
	}
	if bytes.Equal(keys.EncryptKeyI, keys.EncryptKeyR) {
		t.Error("both directions got the same encryption key; the stream must not restart")
	}
	if bytes.Equal(keys.IntegKeyI, keys.IntegKeyR) {
		t.Error("both directions got the same integrity key; the stream must not restart")
	}
}

// RFC requirement: RFC7296-5-2 positive -- an IKE SA can negotiate neither NONE integrity nor a null
// cipher. integrityRegistry registers no name for AUTH_NONE (transform.go:132-136), and
// encryptionRegistry registers no null cipher (transform.go:119-124). LookupIntegrity and
// LookupEncryption refuse both, so no IKEProposal can carry them.
// RFC requirement: RFC7296-5-2 negative -- the refusal is specific. The real integrity and
// encryption algorithms this implementation supports do resolve, so the registries are not
// empty.
func TestIKENeverNegotiatesNullAlgorithms(t *testing.T) {
	for _, name := range []string{"none", "null"} {
		if got, err := LookupIntegrity(name); err == nil {
			t.Errorf("LookupIntegrity(%q) = %+v, want ErrUnsupportedAlgorithm; NONE must not "+
				"be negotiable as the IKE integrity algorithm", name, got)
		}
	}
	for _, name := range []string{"null", "encr_null", "none"} {
		if got, err := LookupEncryption(name); err == nil {
			t.Errorf("LookupEncryption(%q) = %+v, want ErrUnsupportedAlgorithm; ENCR_NULL must "+
				"not be negotiable as the IKE encryption algorithm", name, got)
		}
	}
	// AUTH_NONE exists as a wire constant (it is legal for an AEAD ESP proposal) but has no
	// registry name, so it can never be looked up into an IKE proposal's Integrity field.
	for name, tr := range integrityRegistry {
		if tr.ID == AUTH_NONE {
			t.Errorf("integrityRegistry maps %q to AUTH_NONE; that makes NONE negotiable for "+
				"the IKE SA", name)
		}
	}

	// Negative: the algorithms this implementation does support resolve.
	if _, err := LookupIntegrity("sha256"); err != nil {
		t.Errorf("LookupIntegrity(\"sha256\") = %v, want a transform", err)
	}
	if _, err := LookupEncryption("aes256"); err != nil {
		t.Errorf("LookupEncryption(\"aes256\") = %v, want a transform", err)
	}
}
