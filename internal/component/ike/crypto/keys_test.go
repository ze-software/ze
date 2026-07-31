package crypto

import (
	"crypto/hmac"
	"testing"
)

func TestSKEYSEEDDerivation(t *testing.T) {
	ni := []byte("initiator-nonce-16b!")
	nr := []byte("responder-nonce-16b!")
	sharedSecret := make([]byte, 32)
	for i := range sharedSecret {
		sharedSecret[i] = byte(i)
	}

	skeyseed, err := DeriveSKEYSEED(PRF_HMAC_SHA2_256, ni, nr, sharedSecret)
	if err != nil {
		t.Fatalf("DeriveSKEYSEED: %v", err)
	}
	if len(skeyseed) != 32 {
		t.Errorf("SKEYSEED length = %d, want 32", len(skeyseed))
	}

	nonceKey := append(append([]byte(nil), ni...), nr...)
	expected, err := PRF(PRF_HMAC_SHA2_256, nonceKey, sharedSecret)
	if err != nil {
		t.Fatal(err)
	}
	if !hmac.Equal(skeyseed, expected) {
		t.Error("SKEYSEED does not match prf(Ni|Nr, g^ir)")
	}
}

func TestSKEYSEEDDerivationSHA384(t *testing.T) {
	ni := []byte("nonce-i-for-384-test")
	nr := []byte("nonce-r-for-384-test")
	sharedSecret := make([]byte, 48)

	skeyseed, err := DeriveSKEYSEED(PRF_HMAC_SHA2_384, ni, nr, sharedSecret)
	if err != nil {
		t.Fatalf("DeriveSKEYSEED(SHA384): %v", err)
	}
	if len(skeyseed) != 48 {
		t.Errorf("SKEYSEED length = %d, want 48", len(skeyseed))
	}
}

func TestSKHierarchy(t *testing.T) {
	skeyseed := make([]byte, 32)
	for i := range skeyseed {
		skeyseed[i] = byte(i + 1)
	}
	ni := []byte("nonce-initiator!")
	nr := []byte("nonce-responder!")
	spiI := []byte{0, 0, 0, 0, 0, 0, 0, 1}
	spiR := []byte{0, 0, 0, 0, 0, 0, 0, 2}

	enc := EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128, IsAEAD: false}
	integ := IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16}

	keys, err := DeriveSKKeys(PRF_HMAC_SHA2_256, skeyseed, ni, nr, spiI, spiR, enc, integ)
	if err != nil {
		t.Fatalf("DeriveSKKeys: %v", err)
	}
	defer keys.Clear()

	if len(keys.SK_d) != 32 {
		t.Errorf("SK_d length = %d, want 32 (PRF output length)", len(keys.SK_d))
	}
	if len(keys.SK_ai) != 32 {
		t.Errorf("SK_ai length = %d, want 32 (integrity key length)", len(keys.SK_ai))
	}
	if len(keys.SK_ar) != 32 {
		t.Errorf("SK_ar length = %d, want 32", len(keys.SK_ar))
	}
	if len(keys.SK_ei) != 16 {
		t.Errorf("SK_ei length = %d, want 16 (128-bit AES key)", len(keys.SK_ei))
	}
	if len(keys.SK_er) != 16 {
		t.Errorf("SK_er length = %d, want 16", len(keys.SK_er))
	}
	if len(keys.SK_pi) != 32 {
		t.Errorf("SK_pi length = %d, want 32 (PRF key length)", len(keys.SK_pi))
	}
	if len(keys.SK_pr) != 32 {
		t.Errorf("SK_pr length = %d, want 32", len(keys.SK_pr))
	}

	if hmac.Equal(keys.SK_ei, keys.SK_er) {
		t.Error("SK_ei and SK_er should differ")
	}
	if hmac.Equal(keys.SK_ai, keys.SK_ar) {
		t.Error("SK_ai and SK_ar should differ")
	}
}

func TestSKHierarchyAESGCM256(t *testing.T) {
	skeyseed := make([]byte, 32)
	ni := []byte("ni-gcm-test-data")
	nr := []byte("nr-gcm-test-data")
	spiI := make([]byte, 8)
	spiR := make([]byte, 8)

	enc := EncryptionTransform{ID: ENCR_AES_GCM_16, KeyLength: 256, IsAEAD: true}
	integ := IntegrityTransform{ID: AUTH_NONE, KeyLength: 0, TruncatedLength: 0}

	keys, err := DeriveSKKeys(PRF_HMAC_SHA2_256, skeyseed, ni, nr, spiI, spiR, enc, integ)
	if err != nil {
		t.Fatalf("DeriveSKKeys(AES-GCM-256): %v", err)
	}
	defer keys.Clear()

	if len(keys.SK_ei) != 36 {
		t.Errorf("SK_ei length = %d, want 36 (256-bit key + 4-byte salt)", len(keys.SK_ei))
	}
	if len(keys.SK_ai) != 0 {
		t.Errorf("SK_ai length = %d, want 0 (AEAD, no integrity key)", len(keys.SK_ai))
	}
}

func TestSKHierarchyDeterministic(t *testing.T) {
	skeyseed := make([]byte, 32)
	ni := []byte("ni-det")
	nr := []byte("nr-det")
	spiI := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	spiR := []byte{8, 7, 6, 5, 4, 3, 2, 1}

	enc := EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128}
	integ := IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16}

	a, err := DeriveSKKeys(PRF_HMAC_SHA2_256, skeyseed, ni, nr, spiI, spiR, enc, integ)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Clear()

	b, err := DeriveSKKeys(PRF_HMAC_SHA2_256, skeyseed, ni, nr, spiI, spiR, enc, integ)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Clear()

	if !hmac.Equal(a.SK_d, b.SK_d) || !hmac.Equal(a.SK_ei, b.SK_ei) || !hmac.Equal(a.SK_pi, b.SK_pi) {
		t.Error("DeriveSKKeys is not deterministic")
	}
}

func TestChildSAKeymat(t *testing.T) {
	skD := make([]byte, 32)
	for i := range skD {
		skD[i] = byte(i + 10)
	}
	ni := []byte("child-nonce-init")
	nr := []byte("child-nonce-resp")

	enc := EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128}
	integ := IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16}

	keys, err := DeriveChildSAKeys(PRF_HMAC_SHA2_256, skD, ni, nr, enc, integ)
	if err != nil {
		t.Fatalf("DeriveChildSAKeys: %v", err)
	}
	defer keys.Clear()

	if len(keys.EncryptKeyI) != 16 {
		t.Errorf("EncryptKeyI length = %d, want 16", len(keys.EncryptKeyI))
	}
	if len(keys.IntegKeyI) != 32 {
		t.Errorf("IntegKeyI length = %d, want 32", len(keys.IntegKeyI))
	}
	if len(keys.EncryptKeyR) != 16 {
		t.Errorf("EncryptKeyR length = %d, want 16", len(keys.EncryptKeyR))
	}
	if len(keys.IntegKeyR) != 32 {
		t.Errorf("IntegKeyR length = %d, want 32", len(keys.IntegKeyR))
	}

	if hmac.Equal(keys.EncryptKeyI, keys.EncryptKeyR) {
		t.Error("EncryptKeyI and EncryptKeyR should differ")
	}
}

func TestChildSAKeymatPFS(t *testing.T) {
	skD := make([]byte, 32)
	for i := range skD {
		skD[i] = byte(i + 10)
	}
	dhSecret := make([]byte, 32)
	for i := range dhSecret {
		dhSecret[i] = byte(i + 50)
	}
	ni := []byte("child-nonce-init")
	nr := []byte("child-nonce-resp")

	enc := EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128}
	integ := IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16}

	keysPFS, err := DeriveChildSAKeysPFS(PRF_HMAC_SHA2_256, skD, dhSecret, ni, nr, enc, integ)
	if err != nil {
		t.Fatalf("DeriveChildSAKeysPFS: %v", err)
	}
	defer keysPFS.Clear()

	keysNoPFS, err := DeriveChildSAKeys(PRF_HMAC_SHA2_256, skD, ni, nr, enc, integ)
	if err != nil {
		t.Fatalf("DeriveChildSAKeys: %v", err)
	}
	defer keysNoPFS.Clear()

	if hmac.Equal(keysPFS.EncryptKeyI, keysNoPFS.EncryptKeyI) {
		t.Error("PFS keys should differ from non-PFS keys")
	}
	if len(keysPFS.EncryptKeyI) != 16 {
		t.Errorf("PFS EncryptKeyI length = %d, want 16", len(keysPFS.EncryptKeyI))
	}
}

// VALIDATES: DeriveChildSAKeys and DeriveChildSAKeysPFS request four octets more
// KEYMAT for each AES-GCM key than the AES key alone needs. RFC 4106 Section 8.1:
// "The size of the KEYMAT for the AES-GCM-ESP MUST be four octets longer than is
// needed for the associated AES key." A 128 bit key takes 20 octets. A 256 bit key
// takes 36 octets. The last four octets are the salt value in the nonce.
// PREVENTS: an ESP Child SA keyed with 32 octets for AES-GCM-256. The Linux kernel
// refuses that key with EPROTONOSUPPORT. A peer such as strongSwan reads the
// responder key at offset 36, so both ends key differently even where a kernel
// accepts the short key.
func TestChildSAKeymatAEADCarriesSalt(t *testing.T) {
	skD := make([]byte, 32)
	for i := range skD {
		skD[i] = byte(i + 10)
	}
	dhSecret := make([]byte, 32)
	ni := []byte("child-nonce-init")
	nr := []byte("child-nonce-resp")
	integ := IntegrityTransform{ID: AUTH_NONE}

	tests := []struct {
		name    string
		keyBits uint16
		wantLen int
	}{
		{"aes-gcm-128", 128, 20},
		{"aes-gcm-256", 256, 36},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := EncryptionTransform{ID: ENCR_AES_GCM_16, KeyLength: tt.keyBits, IsAEAD: true}

			keys, err := DeriveChildSAKeys(PRF_HMAC_SHA2_256, skD, ni, nr, enc, integ)
			if err != nil {
				t.Fatalf("DeriveChildSAKeys: %v", err)
			}
			defer keys.Clear()

			if len(keys.EncryptKeyI) != tt.wantLen {
				t.Errorf("EncryptKeyI length = %d, want %d (AES key plus 4-byte salt)",
					len(keys.EncryptKeyI), tt.wantLen)
			}
			if len(keys.EncryptKeyR) != tt.wantLen {
				t.Errorf("EncryptKeyR length = %d, want %d (AES key plus 4-byte salt)",
					len(keys.EncryptKeyR), tt.wantLen)
			}

			pfs, err := DeriveChildSAKeysPFS(PRF_HMAC_SHA2_256, skD, dhSecret, ni, nr, enc, integ)
			if err != nil {
				t.Fatalf("DeriveChildSAKeysPFS: %v", err)
			}
			defer pfs.Clear()

			if len(pfs.EncryptKeyI) != tt.wantLen {
				t.Errorf("PFS EncryptKeyI length = %d, want %d", len(pfs.EncryptKeyI), tt.wantLen)
			}
			if len(pfs.EncryptKeyR) != tt.wantLen {
				t.Errorf("PFS EncryptKeyR length = %d, want %d", len(pfs.EncryptKeyR), tt.wantLen)
			}
		})
	}
}

// VALIDATES: the KEYMAT length of an AEAD Child SA follows the transform ID, not the
// IsAEAD field. A construction site that fills the ID and leaves the field at its zero
// value still gets the salt.
// PREVENTS: the zero-value trap of ai/rules/fail-closed-guards.md. wireProposalsToESP
// filled the ID alone, and a false IsAEAD reads as a valid answer that shortens every
// AES-GCM key by four octets.
func TestChildSAKeymatAEADIgnoresUnsetFlag(t *testing.T) {
	skD := make([]byte, 32)
	ni := []byte("child-nonce-init")
	nr := []byte("child-nonce-resp")

	// IsAEAD is deliberately absent. The ID alone names an AEAD cipher.
	enc := EncryptionTransform{ID: ENCR_AES_GCM_16, KeyLength: 256}
	integ := IntegrityTransform{ID: AUTH_NONE}

	keys, err := DeriveChildSAKeys(PRF_HMAC_SHA2_256, skD, ni, nr, enc, integ)
	if err != nil {
		t.Fatalf("DeriveChildSAKeys: %v", err)
	}
	defer keys.Clear()

	if len(keys.EncryptKeyI) != 36 {
		t.Errorf("EncryptKeyI length = %d, want 36 even with IsAEAD unset", len(keys.EncryptKeyI))
	}
}

// VALIDATES: a non-AEAD cipher takes no salt, so the ESP key is the AES key alone.
// PREVENTS: a blanket four-octet increase that would over-key AES-CBC and break every
// CBC Child SA.
func TestChildSAKeymatCBCTakesNoSalt(t *testing.T) {
	skD := make([]byte, 32)
	enc := EncryptionTransform{ID: ENCR_AES_CBC, KeyLength: 128}
	integ := IntegrityTransform{ID: AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16}

	keys, err := DeriveChildSAKeys(PRF_HMAC_SHA2_256, skD, []byte("ni"), []byte("nr"), enc, integ)
	if err != nil {
		t.Fatalf("DeriveChildSAKeys: %v", err)
	}
	defer keys.Clear()

	if len(keys.EncryptKeyI) != 16 {
		t.Errorf("EncryptKeyI length = %d, want 16 (128-bit AES key, no salt)", len(keys.EncryptKeyI))
	}
}

// VALIDATES: every encryption transform in the registry carries an IsAEAD field that
// agrees with the verdict its ID gives.
// PREVENTS: a registry entry whose cached flag drifts from the single AEAD predicate.
func TestEncryptionRegistryAEADAgreesWithID(t *testing.T) {
	for name, transform := range encryptionRegistry {
		if transform.IsAEAD != transform.ID.IsAEAD() {
			t.Errorf("%s: IsAEAD field = %v, ID verdict = %v",
				name, transform.IsAEAD, transform.ID.IsAEAD())
		}
	}
}

func TestRekeyedSKEYSEED(t *testing.T) {
	skDOld := make([]byte, 32)
	for i := range skDOld {
		skDOld[i] = byte(i)
	}
	newSecret := make([]byte, 32)
	for i := range newSecret {
		newSecret[i] = byte(i + 100)
	}
	ni := []byte("rekey-nonce-init")
	nr := []byte("rekey-nonce-resp")

	skeyseed, err := DeriveRekeyedSKEYSEED(PRF_HMAC_SHA2_256, skDOld, newSecret, ni, nr)
	if err != nil {
		t.Fatalf("DeriveRekeyedSKEYSEED: %v", err)
	}
	if len(skeyseed) != 32 {
		t.Errorf("rekeyed SKEYSEED length = %d, want 32", len(skeyseed))
	}

	data := append(append(append([]byte(nil), newSecret...), ni...), nr...)
	expected, err := PRF(PRF_HMAC_SHA2_256, skDOld, data)
	if err != nil {
		t.Fatal(err)
	}
	if !hmac.Equal(skeyseed, expected) {
		t.Error("rekeyed SKEYSEED does not match prf(SK_d_old, g^ir|Ni|Nr)")
	}
}
